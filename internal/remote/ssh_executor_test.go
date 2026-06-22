package remote

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/trace"
)

type capturedRecord struct {
	Message string
	Attrs   map[string]string
}

type captureStore struct {
	mu      sync.Mutex
	records []*capturedRecord
}

func (s *captureStore) find(msg string) *capturedRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.records {
		if r.Message == msg {
			return r
		}
	}
	return nil
}

type captureHandler struct {
	shared *captureStore
	attrs  []slog.Attr
}

func newCaptureLogger() (*slog.Logger, *captureStore) {
	cs := &captureStore{}
	return slog.New(&captureHandler{shared: cs}), cs
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := &capturedRecord{Message: r.Message, Attrs: make(map[string]string)}
	for _, attr := range h.attrs {
		rec.Attrs[attr.Key] = attr.Value.String()
	}
	r.Attrs(func(attr slog.Attr) bool {
		rec.Attrs[attr.Key] = attr.Value.String()
		return true
	})
	h.shared.mu.Lock()
	h.shared.records = append(h.shared.records, rec)
	h.shared.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	copy(merged[len(h.attrs):], attrs)
	return &captureHandler{shared: h.shared, attrs: merged}
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func TestShellQuoteArgs(t *testing.T) {
	got := shellQuoteArgs([]string{"--execution-id", "abc 123", "has'quote"})
	want := "'--execution-id' 'abc 123' 'has'\"'\"'quote'"
	if got != want {
		t.Fatalf("shellQuoteArgs() = %q, want %q", got, want)
	}
}

func TestExecSSHExecutorRenderInvocation(t *testing.T) {
	executor := NewExecSSHExecutor(SSHExecutorConfig{
		User:         "slurm",
		Port:         "2222",
		Options:      []string{"StrictHostKeyChecking=accept-new", "ConnectTimeout=5"},
		IdentityFile: "/run/secrets/node-key",
	}, nil)

	invocation := executor.renderInvocation(CommandRequest{
		Host:    "gpu-01",
		Command: "hostname",
		Args:    []string{"--fqdn"},
	})
	want := []string{
		"-o", "BatchMode=yes",
		"-p", "2222",
		"-i", "/run/secrets/node-key",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=5",
		"slurm@gpu-01",
		"hostname '--fqdn'",
	}

	if invocation.target != "slurm@gpu-01" {
		t.Fatalf("renderInvocation().target = %q, want %q", invocation.target, "slurm@gpu-01")
	}
	if invocation.remoteCommand != "hostname '--fqdn'" {
		t.Fatalf("renderInvocation().remoteCommand = %q, want %q", invocation.remoteCommand, "hostname '--fqdn'")
	}
	if !reflect.DeepEqual(invocation.args, want) {
		t.Fatalf("renderInvocation().args = %#v, want %#v", invocation.args, want)
	}
}

func TestExecSSHExecutorRenderCopyInvocation(t *testing.T) {
	executor := NewExecSSHExecutor(SSHExecutorConfig{
		User:         "slurm",
		Port:         "2222",
		Options:      []string{"StrictHostKeyChecking=accept-new", "ConnectTimeout=5"},
		IdentityFile: "/run/secrets/node-key",
	}, nil)

	invocation := executor.renderCopyInvocation(StageRequest{
		Host:       "gpu-01",
		LocalPath:  "/local/scripts/reconfigure.sh",
		RemotePath: "/tmp/exec-1/reconfigure.sh",
	})
	// scp reuses the same transport config as ssh but takes the port via -P.
	want := []string{
		"-o", "BatchMode=yes",
		"-P", "2222",
		"-i", "/run/secrets/node-key",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=5",
		"/local/scripts/reconfigure.sh",
		"slurm@gpu-01:/tmp/exec-1/reconfigure.sh",
	}

	if invocation.target != "slurm@gpu-01" {
		t.Fatalf("renderCopyInvocation().target = %q, want %q", invocation.target, "slurm@gpu-01")
	}
	if invocation.remoteSpec != "slurm@gpu-01:/tmp/exec-1/reconfigure.sh" {
		t.Fatalf("renderCopyInvocation().remoteSpec = %q, want %q", invocation.remoteSpec, "slurm@gpu-01:/tmp/exec-1/reconfigure.sh")
	}
	if !reflect.DeepEqual(invocation.args, want) {
		t.Fatalf("renderCopyInvocation().args = %#v, want %#v", invocation.args, want)
	}
}

func TestExecSSHExecutorCopyLogsAndRendersScp(t *testing.T) {
	binDir := t.TempDir()
	argsFile := filepath.Join(binDir, "scp.args")
	scpPath := filepath.Join(binDir, "scp")
	scpScript := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SCP_ARGS_FILE\"\n"
	if err := os.WriteFile(scpPath, []byte(scpScript), 0o755); err != nil {
		t.Fatalf("write fake scp: %v", err)
	}
	localFile := filepath.Join(binDir, "reconfigure.sh")
	if err := os.WriteFile(localFile, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write local file: %v", err)
	}

	t.Setenv("SCP_ARGS_FILE", argsFile)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	logger, captured := newCaptureLogger()
	executor := NewExecSSHExecutor(SSHExecutorConfig{
		User:         "slurm",
		Port:         "2222",
		Options:      []string{"StrictHostKeyChecking=accept-new"},
		IdentityFile: "/run/secrets/node-key",
	}, logger)

	err := executor.Copy(context.Background(), StageRequest{
		Host:        "gpu-01",
		LocalPath:   localFile,
		RemotePath:  "/tmp/exec-1/reconfigure.sh",
		ExecutionID: "exec-1",
		StepName:    "stage_reconfigure",
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read captured scp args: %v", err)
	}
	gotArgs := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	wantArgs := []string{
		"-o", "BatchMode=yes",
		"-P", "2222",
		"-i", "/run/secrets/node-key",
		"-o", "StrictHostKeyChecking=accept-new",
		localFile,
		"slurm@gpu-01:/tmp/exec-1/reconfigure.sh",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("scp argv = %#v, want %#v", gotArgs, wantArgs)
	}

	dispatch := captured.find(trace.EventSCPDispatch)
	if dispatch == nil {
		t.Fatal("expected scp.dispatch log")
	}
	if dispatch.Attrs["target"] != "slurm@gpu-01" {
		t.Fatalf("scp.dispatch target = %q, want %q", dispatch.Attrs["target"], "slurm@gpu-01")
	}
	if dispatch.Attrs["remote_path"] != "/tmp/exec-1/reconfigure.sh" {
		t.Fatalf("scp.dispatch remote_path = %q, want %q", dispatch.Attrs["remote_path"], "/tmp/exec-1/reconfigure.sh")
	}
	if dispatch.Attrs["execution_id"] != "exec-1" {
		t.Fatalf("scp.dispatch execution_id = %q, want %q", dispatch.Attrs["execution_id"], "exec-1")
	}
	if dispatch.Attrs["step_name"] != "stage_reconfigure" {
		t.Fatalf("scp.dispatch step_name = %q, want %q", dispatch.Attrs["step_name"], "stage_reconfigure")
	}
}

func TestExecSSHExecutorCopyPropagatesFailure(t *testing.T) {
	binDir := t.TempDir()
	scpPath := filepath.Join(binDir, "scp")
	// Fake scp that fails with a diagnostic on stderr.
	scpScript := "#!/bin/sh\necho 'scp: connection refused' >&2\nexit 1\n"
	if err := os.WriteFile(scpPath, []byte(scpScript), 0o755); err != nil {
		t.Fatalf("write fake scp: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	executor := NewExecSSHExecutor(SSHExecutorConfig{User: "slurm"}, nil)
	err := executor.Copy(context.Background(), StageRequest{
		Host:       "gpu-01",
		LocalPath:  "/local/reconfigure.sh",
		RemotePath: "/tmp/reconfigure.sh",
		Timeout:    5 * time.Second,
	})
	if err == nil {
		t.Fatal("Copy() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "scp command failed") {
		t.Fatalf("Copy() error = %q, want it to mention scp command failed", err.Error())
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("Copy() error = %q, want it to include scp stderr", err.Error())
	}
}

func TestSSHRunnerStagePropagatesError(t *testing.T) {
	executor := &capturingSSHExecutor{copyErr: errors.New("scp boom")}
	runner := NewSSHRunner(executor)

	err := runner.Stage(context.Background(), StageRequest{
		Host:       "gpu-01",
		LocalPath:  "/local/reconfigure.sh",
		RemotePath: "/tmp/reconfigure.sh",
	})
	if err == nil {
		t.Fatal("Stage() error = nil, want failure")
	}
	if !executor.copyCalled {
		t.Fatal("expected executor.Copy to be called")
	}
	if executor.stageReq.RemotePath != "/tmp/reconfigure.sh" {
		t.Fatalf("executor stageReq.RemotePath = %q, want %q", executor.stageReq.RemotePath, "/tmp/reconfigure.sh")
	}
	if !strings.Contains(err.Error(), "gpu-01") {
		t.Fatalf("Stage() error = %q, want it to mention host", err.Error())
	}
}

func TestExecSSHExecutorRunLogsRenderedCommand(t *testing.T) {
	binDir := t.TempDir()
	argsFile := filepath.Join(binDir, "ssh.args")
	sshPath := filepath.Join(binDir, "ssh")
	sshScript := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SSH_ARGS_FILE\"\nprintf '{\"ok\":true}'\n"
	if err := os.WriteFile(sshPath, []byte(sshScript), 0o755); err != nil {
		t.Fatalf("write fake ssh: %v", err)
	}

	t.Setenv("SSH_ARGS_FILE", argsFile)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	logger, captured := newCaptureLogger()
	executor := NewExecSSHExecutor(SSHExecutorConfig{
		User:         "slurm",
		Port:         "2222",
		Options:      []string{"StrictHostKeyChecking=accept-new", "ConnectTimeout=5"},
		IdentityFile: "/run/secrets/node-key",
	}, logger)

	stdout, stderr, exitCode, err := executor.Run(context.Background(), CommandRequest{
		Host:        "gpu-01",
		Command:     "hostname",
		ExecutionID: "exec-1",
		StepName:    "ssh_probe",
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Run() exitCode = %d, want 0", exitCode)
	}
	if stderr != "" {
		t.Fatalf("Run() stderr = %q, want empty", stderr)
	}
	if stdout != "{\"ok\":true}" {
		t.Fatalf("Run() stdout = %q, want %q", stdout, "{\"ok\":true}")
	}

	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read captured ssh args: %v", err)
	}
	gotArgs := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	wantArgs := []string{
		"-o", "BatchMode=yes",
		"-p", "2222",
		"-i", "/run/secrets/node-key",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=5",
		"slurm@gpu-01",
		"hostname",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("ssh argv = %#v, want %#v", gotArgs, wantArgs)
	}

	dispatch := captured.find(trace.EventSSHDispatch)
	if dispatch == nil {
		t.Fatal("expected ssh.dispatch log")
	}
	if dispatch.Attrs["target"] != "slurm@gpu-01" {
		t.Fatalf("ssh.dispatch target = %q, want %q", dispatch.Attrs["target"], "slurm@gpu-01")
	}
	if dispatch.Attrs["remote_command"] != "hostname" {
		t.Fatalf("ssh.dispatch remote_command = %q, want %q", dispatch.Attrs["remote_command"], "hostname")
	}
	if dispatch.Attrs["execution_id"] != "exec-1" {
		t.Fatalf("ssh.dispatch execution_id = %q, want %q", dispatch.Attrs["execution_id"], "exec-1")
	}
	if dispatch.Attrs["step_name"] != "ssh_probe" {
		t.Fatalf("ssh.dispatch step_name = %q, want %q", dispatch.Attrs["step_name"], "ssh_probe")
	}
	if _, ok := dispatch.Attrs["identity_file"]; ok {
		t.Fatal("ssh.dispatch should not include identity_file")
	}
	if _, ok := dispatch.Attrs["ssh_options"]; ok {
		t.Fatal("ssh.dispatch should not include ssh_options")
	}
}
