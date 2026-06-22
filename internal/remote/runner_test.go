package remote

import (
	"context"
	"testing"
	"time"
)

type capturingSSHExecutor struct {
	req        CommandRequest
	stdout     string
	stderr     string
	exitCode   int
	err        error
	called     bool
	copyCalled bool
	copyErr    error
	stageReq   StageRequest
}

func (e *capturingSSHExecutor) Run(_ context.Context, req CommandRequest) (stdout, stderr string, exitCode int, err error) {
	e.called = true
	e.req = req
	return e.stdout, e.stderr, e.exitCode, e.err
}

func (e *capturingSSHExecutor) Copy(_ context.Context, req StageRequest) error {
	e.copyCalled = true
	e.stageReq = req
	return e.copyErr
}

func TestSSHRunnerExecutePreservesCommandPayload(t *testing.T) {
	executor := &capturingSSHExecutor{
		stdout:   "{\"status\":\"ok\"}",
		exitCode: 0,
	}
	runner := NewSSHRunner(executor)

	result, err := runner.Execute(context.Background(), CommandRequest{
		Host:        "gpu-01",
		Command:     "hostname",
		Args:        []string{"--fqdn"},
		ExecutionID: "exec-1",
		StepName:    "ssh_probe",
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !executor.called {
		t.Fatal("expected executor to be called")
	}
	if executor.req.Command != "hostname" {
		t.Fatalf("executor req.Command = %q, want %q", executor.req.Command, "hostname")
	}
	if len(executor.req.Args) != 1 || executor.req.Args[0] != "--fqdn" {
		t.Fatalf("executor req.Args = %#v, want %#v", executor.req.Args, []string{"--fqdn"})
	}
	if executor.req.ExecutionID != "exec-1" {
		t.Fatalf("executor req.ExecutionID = %q, want %q", executor.req.ExecutionID, "exec-1")
	}
	if executor.req.StepName != "ssh_probe" {
		t.Fatalf("executor req.StepName = %q, want %q", executor.req.StepName, "ssh_probe")
	}
	if result.ExitCode != 0 {
		t.Fatalf("result.ExitCode = %d, want 0", result.ExitCode)
	}
	if string(result.Data) != "{\"status\":\"ok\"}" {
		t.Fatalf("result.Data = %s, want %s", string(result.Data), "{\"status\":\"ok\"}")
	}
}
