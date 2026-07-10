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

// Package sshtest provides an in-process SSH server for exercising the sshutil
// Dialer against a real handshake, publickey auth, exec channel, keepalive
// accounting, and direct-tcpip forwarding (so it can stand in as a bastion).
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"
)

// Server is a running in-process SSH server. Construct via NewServer.
type Server struct {
	ln         net.Listener
	execOutput string
	forwarding bool
	keepalives atomic.Int32
	forwards   atomic.Int32
}

// Option configures a Server.
type Option func(*Server)

// WithExecOutput sets the stdout the server returns for any exec request.
func WithExecOutput(s string) Option { return func(srv *Server) { srv.execOutput = s } }

// WithForwarding enables direct-tcpip channel forwarding (bastion behavior).
func WithForwarding() Option { return func(srv *Server) { srv.forwarding = true } }

// GenerateKey creates an ed25519 private key, writes it as an OpenSSH PEM file
// under t.TempDir(), and returns the path plus the matching public key.
func GenerateKey(t testing.TB) (keyPath string, pub ssh.PublicKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath = filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return keyPath, signer.PublicKey()
}

func hostSigner(t testing.TB) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("host key: %v", err)
	}
	s, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}
	return s
}

// NewServer starts a server on 127.0.0.1:0 accepting publickey auth for
// clientPub. It is torn down via t.Cleanup.
func NewServer(t testing.TB, clientPub ssh.PublicKey, opts ...Option) *Server {
	t.Helper()
	srv := &Server{}
	for _, o := range opts {
		o(srv)
	}
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(clientPub.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, errors.New("unauthorized key")
		},
	}
	cfg.AddHostKey(hostSigner(t))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv.ln = ln
	go srv.serve(cfg)
	t.Cleanup(func() { _ = ln.Close() })
	return srv
}

// Addr returns the host:port the server is listening on.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Keepalives returns the count of keepalive@holodeck global requests received.
func (s *Server) Keepalives() int { return int(s.keepalives.Load()) }

// Forwards returns the count of direct-tcpip channels opened (bastion hops).
func (s *Server) Forwards() int { return int(s.forwards.Load()) }

func (s *Server) serve(cfg *ssh.ServerConfig) {
	for {
		nConn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(nConn, cfg)
	}
}

func (s *Server) handleConn(nConn net.Conn, cfg *ssh.ServerConfig) {
	sc, chans, reqs, err := ssh.NewServerConn(nConn, cfg)
	if err != nil {
		_ = nConn.Close()
		return
	}
	go s.handleGlobalRequests(reqs)
	for newCh := range chans {
		switch newCh.ChannelType() {
		case "session":
			ch, chReqs, err := newCh.Accept()
			if err != nil {
				continue
			}
			go s.handleSession(ch, chReqs)
		case "direct-tcpip":
			if s.forwarding {
				go s.handleForward(newCh)
			} else {
				_ = newCh.Reject(ssh.Prohibited, "forwarding disabled")
			}
		default:
			_ = newCh.Reject(ssh.UnknownChannelType, "unsupported channel")
		}
	}
	_ = sc.Close()
}

func (s *Server) handleGlobalRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		if req.Type == "keepalive@holodeck" {
			s.keepalives.Add(1)
		}
		if req.WantReply {
			_ = req.Reply(false, nil)
		}
	}
}

func (s *Server) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	for req := range reqs {
		switch req.Type {
		case "exec", "shell":
			_, _ = io.WriteString(ch, s.execOutput)
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
			_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{0}))
			_ = ch.Close()
			return
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

func (s *Server) handleForward(newCh ssh.NewChannel) {
	var p struct {
		DestHost   string
		DestPort   uint32
		OriginHost string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(newCh.ExtraData(), &p); err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, "bad direct-tcpip payload")
		return
	}
	dest := net.JoinHostPort(p.DestHost, fmt.Sprintf("%d", p.DestPort))
	upstream, err := net.Dial("tcp", dest)
	if err != nil {
		_ = newCh.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	s.forwards.Add(1)
	ch, reqs, err := newCh.Accept()
	if err != nil {
		_ = upstream.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	go func() { _, _ = io.Copy(ch, upstream); _ = ch.Close() }()
	go func() { _, _ = io.Copy(upstream, ch); _ = upstream.Close() }()
}
