/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package e2e

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
	"github.com/NVIDIA/holodeck/cmd/cli/common"
	"github.com/NVIDIA/holodeck/internal/logger"
	"github.com/NVIDIA/holodeck/pkg/jyaml"
	"github.com/NVIDIA/holodeck/pkg/provisioner"
)

const (
	// sshImage is linuxserver/openssh-server pinned by manifest-list digest
	// (covers linux/amd64 for CI and linux/arm64 for local Apple Silicon). It
	// is Alpine-based and ships only busybox ash, so the harness installs bash
	// (route A) before provisioning — holodeck's provision Shebang is bash.
	sshImage = "linuxserver/openssh-server@sha256:deec9e402eb9a656de1749c3ea93e99478a565cf187ecfb051f8446d93b12483"
	// sshUser is created inside the container via USER_NAME and granted
	// passwordless sudo via SUDO_ACCESS — the provision script's CommonFunctions
	// preamble runs `sudo mkdir` unconditionally, so sudo must work.
	sshUser = "holo"
	// markerContent is what the fixture's post-install custom template writes to
	// /tmp/holodeck-marker on the container; the read-back asserts it verbatim.
	markerContent = "holodeck-was-here\n"
)

// requireDocker hard-fails (never Skip) when docker is unavailable and the
// real-ssh label is selected. A silently-skipping credential-free tier is worse
// than none: it would report green while proving nothing.
func requireDocker() {
	GinkgoHelper()
	if _, err := exec.LookPath("docker"); err != nil {
		Fail("real-ssh tier requires docker, but 'docker' is not on PATH")
	}
	if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
		Fail(fmt.Sprintf("real-ssh tier requires a running docker daemon: %v\n%s", err, out))
	}
}

// isolateHostKeyCache redirects the TOFU known_hosts file (os.UserCacheDir ->
// $CACHE/holodeck/known_hosts) into a per-spec temp dir so repeated local runs
// that reuse an ephemeral host port can never collide with a stale host-key
// entry from a prior container (which would fail accept-new as a MITM mismatch).
// os.UserCacheDir honors XDG_CACHE_HOME on Linux and $HOME/Library/Caches on
// macOS, so both are pointed at the temp dir. Setenv restores automatically.
func isolateHostKeyCache() {
	GinkgoHelper()
	dir := GinkgoT().TempDir()
	GinkgoT().Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	GinkgoT().Setenv("HOME", dir)
}

// generateSSHKey creates an ed25519 keypair, writes the private key as an
// OpenSSH PEM under a pattern-safe temp dir (ValidateTemplateInputs restricts
// PrivateKey to [a-zA-Z0-9/._-~]), and returns the private-key path plus the
// single-line authorized_keys entry for the container's PUBLIC_KEY env.
func generateSSHKey() (keyPath, authorizedKey string) {
	GinkgoHelper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).NotTo(HaveOccurred(), "generate ed25519 key")
	block, err := ssh.MarshalPrivateKey(priv, "")
	Expect(err).NotTo(HaveOccurred(), "marshal private key")

	keyDir, err := os.MkdirTemp("/tmp", "holodeck-ssh-e2e-")
	Expect(err).NotTo(HaveOccurred(), "create key dir")
	DeferCleanup(func() { _ = os.RemoveAll(keyDir) })

	keyPath = filepath.Join(keyDir, "id_ed25519")
	Expect(os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600)).To(Succeed(), "write private key")

	sshPub, err := ssh.NewPublicKey(pub)
	Expect(err).NotTo(HaveOccurred(), "build ssh public key")
	authorizedKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
	return keyPath, authorizedKey
}

// startSSHContainer boots a detached openssh-server container publishing its
// internal 2222 on a random 127.0.0.1 port, installs bash (route A), and
// returns the container ID and the "127.0.0.1:<port>" address. The container is
// killed via DeferCleanup.
func startSSHContainer(authorizedKey string) (containerID, hostURL string) {
	GinkgoHelper()
	//nolint:gosec // G204: controlled test args; image is digest-pinned, key is test-generated.
	out, err := exec.Command("docker", "run", "-d", "--rm",
		"-p", "127.0.0.1:0:2222",
		"-e", "PUID=1000",
		"-e", "PGID=1000",
		"-e", "USER_NAME="+sshUser,
		"-e", "SUDO_ACCESS=true",
		"-e", "PUBLIC_KEY="+authorizedKey,
		sshImage,
	).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "docker run failed: %s", out)
	containerID = strings.TrimSpace(string(out))
	Expect(containerID).NotTo(BeEmpty(), "docker run returned no container id")
	DeferCleanup(func() {
		//nolint:gosec // G204: containerID is the id docker just returned.
		_ = exec.Command("docker", "kill", containerID).Run()
	})

	installBash(containerID)
	return containerID, dockerHostPort(containerID)
}

// installBash runs `apk add bash` in the container, retrying until the image's
// package DB is ready. holo's login shell is /bin/bash, so bash must exist
// before sshd execs the (bash-only) provision script.
func installBash(containerID string) {
	GinkgoHelper()
	var lastErr error
	for i := 0; i < 30; i++ {
		//nolint:gosec // G204: containerID is docker-provided; args are constant.
		out, err := exec.Command("docker", "exec", containerID, "apk", "add", "--no-cache", "bash").CombinedOutput()
		if err == nil {
			return
		}
		lastErr = fmt.Errorf("%v: %s", err, out)
		time.Sleep(time.Second)
	}
	Fail(fmt.Sprintf("apk add bash never succeeded in container %s: %v", containerID, lastErr))
}

// dockerHostPort resolves the host-side address mapped to the container's 2222.
func dockerHostPort(containerID string) string {
	GinkgoHelper()
	//nolint:gosec // G204: containerID is docker-provided; args are constant.
	out, err := exec.Command("docker", "port", containerID, "2222").CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "docker port failed: %s", out)
	line := strings.TrimSpace(string(out))
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	Expect(line).NotTo(BeEmpty(), "docker port returned no mapping")
	_, port, err := net.SplitHostPort(line)
	Expect(err).NotTo(HaveOccurred(), "unexpected docker port output %q", line)
	return net.JoinHostPort("127.0.0.1", port)
}

// waitForSSH blocks until the container accepts a TCP connection and completes a
// real SSH handshake through the production ConnectSSH path (also seeding the
// isolated TOFU known_hosts), so provisioning starts against a ready daemon.
func waitForSSH(hostURL, keyPath string, log *logger.FunLogger) {
	GinkgoHelper()
	Eventually(func() error {
		conn, err := net.DialTimeout("tcp", hostURL, 2*time.Second)
		if err != nil {
			return err
		}
		_ = conn.Close()
		client, err := common.ConnectSSH(log, keyPath, sshUser, hostURL)
		if err != nil {
			return err
		}
		_ = client.Close()
		return nil
	}, 60*time.Second, 2*time.Second).Should(Succeed(), "sshd never became ready")
}

var _ = Describe("Real SSH ProviderSSH E2E", Label("real-ssh"), func() {
	It("provisions over SSH and reads back the on-container marker", func() {
		requireDocker()

		log := logger.NewLogger()
		log.Out = GinkgoWriter

		isolateHostKeyCache()
		keyPath, authorizedKey := generateSSHKey()
		_, hostURL := startSSHContainer(authorizedKey)
		waitForSSH(hostURL, keyPath, log)

		// Load the ssh-provider fixture and point it at the live container.
		cfgPath := filepath.Join(packagePath, "data", "test_ssh.yaml")
		cfg, err := jyaml.UnmarshalFromFile[v1alpha1.Environment](cfgPath)
		Expect(err).NotTo(HaveOccurred(), "failed to load %s", cfgPath)
		Expect(cfg.Spec.Provider).To(Equal(v1alpha1.ProviderSSH), "fixture must be a provider: ssh env")
		cfg.Spec.PrivateKey = keyPath
		cfg.Spec.Username = sshUser
		cfg.Spec.HostUrl = hostURL

		// Production single-node SSH path (mirrors cmd/cli/create
		// runSingleNodeProvision): connect, then run the dependency graph, whose
		// only node here is the post-install marker template. Run executes it on
		// the container, writing /tmp/holodeck-marker.
		p, err := provisioner.New(log, cfg.Spec.PrivateKey, cfg.Spec.Username, cfg.Spec.HostUrl)
		Expect(err).NotTo(HaveOccurred(), "provisioner.New failed to connect")
		DeferCleanup(func() { _ = p.Close() })

		_, err = p.Run(cfg)
		Expect(err).NotTo(HaveOccurred(), "provisioning run failed")

		// Read the marker back through the holodeck ssh exec path.
		client, err := common.ConnectSSH(log, keyPath, sshUser, hostURL)
		Expect(err).NotTo(HaveOccurred(), "ConnectSSH read-back failed")
		DeferCleanup(func() { _ = client.Close() })

		session, err := client.NewSession()
		Expect(err).NotTo(HaveOccurred(), "open read-back session")

		out, err := session.Output("cat /tmp/holodeck-marker")
		Expect(err).NotTo(HaveOccurred(), "reading marker failed")
		Expect(string(out)).To(Equal(markerContent),
			"provision script did not write the expected marker")
	})
})
