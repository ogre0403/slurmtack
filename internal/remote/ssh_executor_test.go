package remote

import (
	"reflect"
	"testing"
)

func TestShellQuoteArgs(t *testing.T) {
	got := shellQuoteArgs([]string{"--execution-id", "abc 123", "has'quote"})
	want := "'--execution-id' 'abc 123' 'has'\"'\"'quote'"
	if got != want {
		t.Fatalf("shellQuoteArgs() = %q, want %q", got, want)
	}
}

func TestExecSSHExecutorBuildSSHArgs(t *testing.T) {
	executor := NewExecSSHExecutor(SSHExecutorConfig{
		User:         "slurm",
		Port:         "2222",
		Options:      []string{"StrictHostKeyChecking=accept-new", "ConnectTimeout=5"},
		IdentityFile: "/run/secrets/node-key",
	})

	got := executor.buildSSHArgs("gpu-01", "hostname", nil)
	want := []string{
		"-o", "BatchMode=yes",
		"-p", "2222",
		"-i", "/run/secrets/node-key",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=5",
		"slurm@gpu-01",
		"hostname",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSSHArgs() = %#v, want %#v", got, want)
	}
}
