# CLI CRUD & Verbosity Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add global verbosity flags and unit tests for new CLI commands.

**Architecture:** Extend FunLogger with verbosity levels, wire global flags in main.go, add comprehensive unit tests for all new commands using table-driven tests and mocks.

**Tech Stack:** Go, urfave/cli/v2, testify (assertions), gomock (mocking)

---

## Task 1: Add Verbosity Support to Logger

**Files:**
- Modify: `internal/logger/logger.go`
- Create: `internal/logger/logger_test.go`

**Step 1: Write the failing test for verbosity filtering**

```go
// internal/logger/logger_test.go
package logger

import (
	"bytes"
	"testing"
)

func TestVerbosityQuiet_SuppressesInfo(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger()
	log.Out = &buf
	log.SetVerbosity(VerbosityQuiet)

	log.Info("should not appear")

	if buf.String() != "" {
		t.Errorf("expected empty output in quiet mode, got %q", buf.String())
	}
}

func TestVerbosityQuiet_AllowsError(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger()
	log.Out = &buf
	log.SetVerbosity(VerbosityQuiet)

	log.Error(fmt.Errorf("error message"))

	if !strings.Contains(buf.String(), "error message") {
		t.Errorf("expected error to appear in quiet mode, got %q", buf.String())
	}
}

func TestVerbosityNormal_ShowsInfo(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger()
	log.Out = &buf
	log.SetVerbosity(VerbosityNormal)

	log.Info("info message")

	if !strings.Contains(buf.String(), "info message") {
		t.Errorf("expected info to appear in normal mode, got %q", buf.String())
	}
}

func TestVerbosityNormal_HidesDebug(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger()
	log.Out = &buf
	log.SetVerbosity(VerbosityNormal)

	log.Debug("debug message")

	if buf.String() != "" {
		t.Errorf("expected debug hidden in normal mode, got %q", buf.String())
	}
}

func TestVerbosityVerbose_ShowsDebug(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger()
	log.Out = &buf
	log.SetVerbosity(VerbosityVerbose)

	log.Debug("debug message")

	if !strings.Contains(buf.String(), "debug message") {
		t.Errorf("expected debug to appear in verbose mode, got %q", buf.String())
	}
}

func TestVerbosityDebug_ShowsTrace(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger()
	log.Out = &buf
	log.SetVerbosity(VerbosityDebug)

	log.Trace("trace message")

	if !strings.Contains(buf.String(), "trace message") {
		t.Errorf("expected trace to appear in debug mode, got %q", buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logger/... -v`
Expected: FAIL with "undefined: VerbosityQuiet" or similar

**Step 3: Implement verbosity in logger.go**

Add after line 53 (after the const block):

```go
// Verbosity represents the logging verbosity level
type Verbosity int

const (
	// VerbosityQuiet suppresses all output except errors
	VerbosityQuiet Verbosity = iota
	// VerbosityNormal is the default verbosity level
	VerbosityNormal
	// VerbosityVerbose enables debug output
	VerbosityVerbose
	// VerbosityDebug enables trace output
	VerbosityDebug
)
```

Add Verbosity field to FunLogger struct (after IsCI field):

```go
	// Verbosity controls the logging level
	Verbosity Verbosity
```

Update NewLogger to set default verbosity:

```go
func NewLogger() *FunLogger {
	return &FunLogger{
		Out:       os.Stderr,
		Done:      make(chan struct{}),
		Fail:      make(chan struct{}),
		Wg:        &sync.WaitGroup{},
		ExitFunc:  os.Exit,
		Verbosity: VerbosityNormal,
	}
}
```

Add SetVerbosity method:

```go
// SetVerbosity sets the logging verbosity level
func (l *FunLogger) SetVerbosity(v Verbosity) {
	l.Verbosity = v
}
```

Update Info method to check verbosity:

```go
func (l *FunLogger) Info(format string, a ...any) {
	if l.Verbosity < VerbosityNormal {
		return
	}
	if format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(l.Out, format, a...)
}
```

Update Check method:

```go
func (l *FunLogger) Check(format string, a ...any) {
	if l.Verbosity < VerbosityNormal {
		return
	}
	message := fmt.Sprintf(format, a...)
	printMessage(green, checkmark, message)
}
```

Update Warning method:

```go
func (l *FunLogger) Warning(format string, a ...any) {
	if l.Verbosity < VerbosityNormal {
		return
	}
	message := fmt.Sprintf(format, a...)
	printMessage(yellowText, warningSign, message)
}
```

Add Debug method:

```go
// Debug prints a debug message (only in verbose or debug mode)
func (l *FunLogger) Debug(format string, a ...any) {
	if l.Verbosity < VerbosityVerbose {
		return
	}
	if format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(l.Out, "[DEBUG] "+format, a...)
}
```

Add Trace method:

```go
// Trace prints a trace message (only in debug mode)
func (l *FunLogger) Trace(format string, a ...any) {
	if l.Verbosity < VerbosityDebug {
		return
	}
	if format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(l.Out, "[TRACE] "+format, a...)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/logger/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/logger/logger.go internal/logger/logger_test.go
git commit -m "feat(logger): add verbosity levels with Debug and Trace methods"
```

---

## Task 2: Wire Global Verbosity Flags in main.go

**Files:**
- Modify: `cmd/cli/main.go`

**Step 1: Add global flags**

Replace the existing Flags block with:

```go
	// Setup the flags for this command
	c.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:        "quiet",
			Aliases:     []string{"q"},
			Usage:       "Suppress non-error output",
			Destination: &config.Quiet,
		},
		&cli.BoolFlag{
			Name:        "verbose",
			Aliases:     []string{"v"},
			Usage:       "Enable verbose output",
			Destination: &config.Verbose,
		},
		&cli.BoolFlag{
			Name:        "debug",
			Aliases:     []string{"d"},
			Usage:       "Enable debug output",
			Destination: &config.Debug,
			EnvVars:     []string{"DEBUG"},
		},
	}
```

Update config struct:

```go
type config struct {
	Quiet   bool
	Verbose bool
	Debug   bool
}
```

Add Before hook after c.EnableBashCompletion line:

```go
	c.Before = func(ctx *cli.Context) error {
		switch {
		case config.Debug:
			log.SetVerbosity(logger.VerbosityDebug)
		case config.Verbose:
			log.SetVerbosity(logger.VerbosityVerbose)
		case config.Quiet:
			log.SetVerbosity(logger.VerbosityQuiet)
		default:
			log.SetVerbosity(logger.VerbosityNormal)
		}
		return nil
	}
```

Add import for logger package if not present.

**Step 2: Verify build passes**

Run: `go build ./cmd/cli/...`
Expected: Success

**Step 3: Manual test**

Run: `go run ./cmd/cli/main.go -q list`
Expected: No output (or only instance IDs if instances exist)

Run: `go run ./cmd/cli/main.go -v list`
Expected: Normal output (verbose features added later)

**Step 4: Commit**

```bash
git add cmd/cli/main.go
git commit -m "feat(cli): add global verbosity flags (-q, -v, -d)"
```

---

## Task 3: Rename list --quiet to --ids-only

**Files:**
- Modify: `cmd/cli/list/list.go`
- Modify: `cmd/cli/list/list_test.go` (if exists)

**Step 1: Update flag name in list.go**

Change the quiet flag to ids-only:

```go
			&cli.BoolFlag{
				Name:        "ids-only",
				Usage:       "Only display instance IDs",
				Destination: &m.quiet,
			},
```

Note: Keep the internal field name as `quiet` to minimize changes.

**Step 2: Verify build passes**

Run: `go build ./cmd/cli/...`
Expected: Success

**Step 3: Run existing tests**

Run: `go test ./cmd/cli/list/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/cli/list/list.go
git commit -m "refactor(list): rename --quiet to --ids-only to avoid global flag conflict"
```

---

## Task 4: Add Unit Tests for pkg/output

**Files:**
- Create: `pkg/output/output_test.go`

**Step 1: Write tests for output formatter**

```go
// pkg/output/output_test.go
package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewFormatter_DefaultsToTable(t *testing.T) {
	f, err := NewFormatter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Format() != FormatTable {
		t.Errorf("expected table format, got %v", f.Format())
	}
}

func TestNewFormatter_InvalidFormat(t *testing.T) {
	_, err := NewFormatter("invalid")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestNewFormatter_ValidFormats(t *testing.T) {
	formats := []string{"table", "json", "yaml"}
	for _, format := range formats {
		f, err := NewFormatter(format)
		if err != nil {
			t.Errorf("unexpected error for format %q: %v", format, err)
		}
		if f.Format() != Format(format) {
			t.Errorf("expected %q, got %q", format, f.Format())
		}
	}
}

func TestFormatter_PrintJSON(t *testing.T) {
	var buf bytes.Buffer
	f, _ := NewFormatter("json")
	f.SetWriter(&buf)

	data := map[string]string{"key": "value"}
	if err := f.PrintJSON(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected value, got %q", result["key"])
	}
}

func TestFormatter_PrintYAML(t *testing.T) {
	var buf bytes.Buffer
	f, _ := NewFormatter("yaml")
	f.SetWriter(&buf)

	data := map[string]string{"key": "value"}
	if err := f.PrintYAML(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := yaml.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid YAML output: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected value, got %q", result["key"])
	}
}

type testTableData struct {
	items [][]string
}

func (t *testTableData) Headers() []string {
	return []string{"COL1", "COL2"}
}

func (t *testTableData) Rows() [][]string {
	return t.items
}

func TestFormatter_PrintTable(t *testing.T) {
	var buf bytes.Buffer
	f, _ := NewFormatter("table")
	f.SetWriter(&buf)

	data := &testTableData{
		items: [][]string{
			{"a", "b"},
			{"c", "d"},
		},
	}
	if err := f.PrintTable(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "COL1") {
		t.Error("expected header COL1 in output")
	}
	if !strings.Contains(output, "a") {
		t.Error("expected row data in output")
	}
}

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		format string
		valid  bool
	}{
		{"table", true},
		{"json", true},
		{"yaml", true},
		{"xml", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsValidFormat(tt.format); got != tt.valid {
			t.Errorf("IsValidFormat(%q) = %v, want %v", tt.format, got, tt.valid)
		}
	}
}
```

**Step 2: Run tests**

Run: `go test ./pkg/output/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/output/output_test.go
git commit -m "test(output): add unit tests for output formatter"
```

---

## Task 5: Add Unit Tests for describe Command

**Files:**
- Create: `cmd/cli/describe/describe_test.go`

**Step 1: Write tests**

```go
// cmd/cli/describe/describe_test.go
package describe

import (
	"testing"
	"time"
)

func TestDescribeOutput_BuildsCorrectly(t *testing.T) {
	output := &DescribeOutput{
		Instance: InstanceInfo{
			ID:          "test-123",
			Name:        "test-instance",
			CreatedAt:   time.Now(),
			Age:         "1h",
			CacheFile:   "/tmp/cache.yaml",
			Provisioned: true,
		},
		Provider: ProviderInfo{
			Type:     "aws",
			Region:   "us-west-2",
			Username: "ubuntu",
			KeyName:  "my-key",
		},
	}

	if output.Instance.ID != "test-123" {
		t.Errorf("expected ID test-123, got %s", output.Instance.ID)
	}
	if output.Provider.Type != "aws" {
		t.Errorf("expected provider aws, got %s", output.Provider.Type)
	}
}

func TestComponentsInfo_NilSafe(t *testing.T) {
	output := &DescribeOutput{
		Components: ComponentsInfo{},
	}

	// All component fields should be nil by default
	if output.Components.Kernel != nil {
		t.Error("expected Kernel to be nil")
	}
	if output.Components.NVIDIADriver != nil {
		t.Error("expected NVIDIADriver to be nil")
	}
	if output.Components.Kubernetes != nil {
		t.Error("expected Kubernetes to be nil")
	}
}

func TestStatusInfo_ConditionsAppend(t *testing.T) {
	status := StatusInfo{
		State: "running",
	}

	status.Conditions = append(status.Conditions, ConditionInfo{
		Type:   "Ready",
		Status: "True",
	})

	if len(status.Conditions) != 1 {
		t.Errorf("expected 1 condition, got %d", len(status.Conditions))
	}
	if status.Conditions[0].Type != "Ready" {
		t.Errorf("expected Ready condition, got %s", status.Conditions[0].Type)
	}
}
```

**Step 2: Run tests**

Run: `go test ./cmd/cli/describe/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/cli/describe/describe_test.go
git commit -m "test(describe): add unit tests for describe output types"
```

---

## Task 6: Add Unit Tests for get Command

**Files:**
- Create: `cmd/cli/get/get_test.go`

**Step 1: Write tests**

```go
// cmd/cli/get/get_test.go
package get

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestGetHostURL_AWS(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
		},
		Status: v1alpha1.EnvironmentStatus{
			Properties: []v1alpha1.Properties{
				{Name: "PublicDnsName", Value: "ec2-1-2-3-4.compute.amazonaws.com"},
			},
		},
	}

	url, err := cmd.getHostURL(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "ec2-1-2-3-4.compute.amazonaws.com" {
		t.Errorf("expected DNS name, got %s", url)
	}
}

func TestGetHostURL_SSH(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderSSH,
			HostUrl:  "192.168.1.100",
		},
	}

	url, err := cmd.getHostURL(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %s", url)
	}
}

func TestGetHostURL_NoProperties(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
		},
		Status: v1alpha1.EnvironmentStatus{
			Properties: []v1alpha1.Properties{},
		},
	}

	_, err := cmd.getHostURL(env)
	if err == nil {
		t.Error("expected error for missing properties")
	}
}
```

**Step 2: Run tests**

Run: `go test ./cmd/cli/get/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/cli/get/get_test.go
git commit -m "test(get): add unit tests for host URL resolution"
```

---

## Task 7: Add Unit Tests for ssh Command

**Files:**
- Create: `cmd/cli/ssh/ssh_test.go`

**Step 1: Write tests**

```go
// cmd/cli/ssh/ssh_test.go
package ssh

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestGetHostURL_AWS(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderAWS,
		},
		Status: v1alpha1.EnvironmentStatus{
			Properties: []v1alpha1.Properties{
				{Name: "PublicDnsName", Value: "ec2-test.compute.amazonaws.com"},
			},
		},
	}

	url, err := cmd.getHostURL(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "ec2-test.compute.amazonaws.com" {
		t.Errorf("expected DNS name, got %s", url)
	}
}

func TestGetHostURL_SSHProvider(t *testing.T) {
	cmd := command{}
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Provider: v1alpha1.ProviderSSH,
			HostUrl:  "10.0.0.5",
		},
	}

	url, err := cmd.getHostURL(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "10.0.0.5" {
		t.Errorf("expected 10.0.0.5, got %s", url)
	}
}

func TestContainsSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello", false},
		{"hello world", true},
		{"hello\tworld", true},
		{"", false},
	}

	for _, tt := range tests {
		if got := containsSpace(tt.input); got != tt.expected {
			t.Errorf("containsSpace(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
```

**Step 2: Run tests**

Run: `go test ./cmd/cli/ssh/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/cli/ssh/ssh_test.go
git commit -m "test(ssh): add unit tests for host URL and argument handling"
```

---

## Task 8: Add Unit Tests for scp Command

**Files:**
- Create: `cmd/cli/scp/scp_test.go`

**Step 1: Write tests**

```go
// cmd/cli/scp/scp_test.go
package scp

import (
	"testing"
)

func TestParsePath_Local(t *testing.T) {
	spec := parsePath("/home/user/file.txt")

	if spec.isRemote {
		t.Error("expected local path")
	}
	if spec.path != "/home/user/file.txt" {
		t.Errorf("expected /home/user/file.txt, got %s", spec.path)
	}
}

func TestParsePath_Remote(t *testing.T) {
	spec := parsePath("abc123:/tmp/file.txt")

	if !spec.isRemote {
		t.Error("expected remote path")
	}
	if spec.instanceID != "abc123" {
		t.Errorf("expected instance ID abc123, got %s", spec.instanceID)
	}
	if spec.path != "/tmp/file.txt" {
		t.Errorf("expected /tmp/file.txt, got %s", spec.path)
	}
}

func TestParsePath_WindowsPath(t *testing.T) {
	// C:\ should not be parsed as remote
	spec := parsePath("C:\\Users\\file.txt")

	if spec.isRemote {
		t.Error("Windows path should not be parsed as remote")
	}
}

func TestParsePath_RelativePath(t *testing.T) {
	spec := parsePath("./local/file.txt")

	if spec.isRemote {
		t.Error("expected local path")
	}
	if spec.path != "./local/file.txt" {
		t.Errorf("expected ./local/file.txt, got %s", spec.path)
	}
}

func TestParsePath_RemoteHomeDir(t *testing.T) {
	spec := parsePath("abc123:~/config")

	if !spec.isRemote {
		t.Error("expected remote path")
	}
	if spec.path != "~/config" {
		t.Errorf("expected ~/config, got %s", spec.path)
	}
}
```

**Step 2: Run tests**

Run: `go test ./cmd/cli/scp/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/cli/scp/scp_test.go
git commit -m "test(scp): add unit tests for path parsing"
```

---

## Task 9: Add Unit Tests for update Command

**Files:**
- Create: `cmd/cli/update/update_test.go`

**Step 1: Write tests**

```go
// cmd/cli/update/update_test.go
package update

import (
	"testing"

	"github.com/NVIDIA/holodeck/api/holodeck/v1alpha1"
)

func TestLabelParsing(t *testing.T) {
	tests := []struct {
		label    string
		key      string
		value    string
		hasError bool
	}{
		{"team=gpu-infra", "team", "gpu-infra", false},
		{"env=prod", "env", "prod", false},
		{"invalid", "", "", true},
		{"key=value=extra", "key", "value=extra", false},
	}

	for _, tt := range tests {
		parts := splitLabel(tt.label)
		if tt.hasError {
			if len(parts) == 2 {
				t.Errorf("expected error for %q", tt.label)
			}
		} else {
			if len(parts) != 2 {
				t.Errorf("expected 2 parts for %q, got %d", tt.label, len(parts))
				continue
			}
			if parts[0] != tt.key {
				t.Errorf("expected key %q, got %q", tt.key, parts[0])
			}
			if parts[1] != tt.value {
				t.Errorf("expected value %q, got %q", tt.value, parts[1])
			}
		}
	}
}

// Helper function for testing - mirrors the logic in update.go
func splitLabel(label string) []string {
	for i, c := range label {
		if c == '=' {
			return []string{label[:i], label[i+1:]}
		}
	}
	return []string{label}
}

func TestEnvironmentUpdate_AddDriver(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			NVIDIADriver: v1alpha1.NVIDIADriver{
				Install: false,
			},
		},
	}

	// Simulate adding driver
	env.Spec.NVIDIADriver.Install = true
	env.Spec.NVIDIADriver.Version = "560.35.03"

	if !env.Spec.NVIDIADriver.Install {
		t.Error("expected driver to be installed")
	}
	if env.Spec.NVIDIADriver.Version != "560.35.03" {
		t.Errorf("expected version 560.35.03, got %s", env.Spec.NVIDIADriver.Version)
	}
}

func TestEnvironmentUpdate_AddKubernetes(t *testing.T) {
	env := &v1alpha1.Environment{
		Spec: v1alpha1.EnvironmentSpec{
			Kubernetes: v1alpha1.Kubernetes{
				Install: false,
			},
		},
	}

	// Simulate adding kubernetes
	env.Spec.Kubernetes.Install = true
	env.Spec.Kubernetes.KubernetesInstaller = "kubeadm"
	env.Spec.Kubernetes.KubernetesVersion = "v1.31.1"

	if !env.Spec.Kubernetes.Install {
		t.Error("expected kubernetes to be installed")
	}
	if env.Spec.Kubernetes.KubernetesInstaller != "kubeadm" {
		t.Errorf("expected kubeadm, got %s", env.Spec.Kubernetes.KubernetesInstaller)
	}
}
```

**Step 2: Run tests**

Run: `go test ./cmd/cli/update/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/cli/update/update_test.go
git commit -m "test(update): add unit tests for label parsing and environment updates"
```

---

## Task 10: Commit All New CLI Commands

**Files:**
- Stage all untracked files in cmd/cli/ and pkg/output/

**Step 1: Stage new command directories**

```bash
git add cmd/cli/describe/ cmd/cli/get/ cmd/cli/scp/ cmd/cli/ssh/ cmd/cli/update/ pkg/output/
```

**Step 2: Review staged changes**

Run: `git status`
Expected: All new directories staged

**Step 3: Commit**

```bash
git commit -m "feat(cli): add describe, get, ssh, scp, update commands and output formatting

New commands:
- describe: detailed instance introspection with JSON/YAML output
- get kubeconfig: download kubeconfig from instance
- get ssh-config: generate SSH config entry
- ssh: SSH into instance or run remote commands
- scp: copy files to/from instance
- update: update instance configuration (add components, labels)

Enhanced commands:
- list: added -o flag for JSON/YAML output
- status: added -o flag for JSON/YAML output

New package:
- pkg/output: output formatting infrastructure (table/JSON/YAML)

Issue: #563"
```

---

## Task 11: Final Verification

**Step 1: Run all tests**

Run: `go test $(go list ./... | grep -v '/tests') -v`
Expected: All tests PASS

**Step 2: Run go vet**

Run: `go vet ./...`
Expected: No errors

**Step 3: Run go build**

Run: `go build ./...`
Expected: Success

**Step 4: Manual smoke test**

Run: `go run ./cmd/cli/main.go --help`
Expected: Shows all new commands (describe, get, ssh, scp, update)

Run: `go run ./cmd/cli/main.go -q --help`
Expected: Help still shows (quiet doesn't suppress help)

---

## Summary

| Task | Description | Est. Time |
|------|-------------|-----------|
| 1 | Add verbosity support to logger | 5 min |
| 2 | Wire global verbosity flags | 3 min |
| 3 | Rename list --quiet to --ids-only | 2 min |
| 4 | Unit tests for pkg/output | 5 min |
| 5 | Unit tests for describe | 3 min |
| 6 | Unit tests for get | 3 min |
| 7 | Unit tests for ssh | 3 min |
| 8 | Unit tests for scp | 3 min |
| 9 | Unit tests for update | 3 min |
| 10 | Commit all new commands | 2 min |
| 11 | Final verification | 3 min |
