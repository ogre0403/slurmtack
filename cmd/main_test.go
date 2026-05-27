package main

import (
	"reflect"
	"testing"

	"github.com/slurmtack/slurmtack/internal/config"
	"github.com/slurmtack/slurmtack/internal/remote"
)

func TestBuildSSHRunnerDisabledWithoutSSHConfig(t *testing.T) {
	if got := buildSSHRunner(&config.Config{}); got != nil {
		t.Fatalf("buildSSHRunner() = %#v, want nil", got)
	}
}

func TestBuildSSHExecutorConfig(t *testing.T) {
	cfg := &config.Config{
		SSHUser:           "slurm",
		SSHPort:           "2222",
		SSHOptions:        "StrictHostKeyChecking=accept-new ConnectTimeout=5",
		SSHPrivateKeyPath: "/run/secrets/node-key",
	}

	got := buildSSHExecutorConfig(cfg)
	want := remote.SSHExecutorConfig{
		User:         "slurm",
		Port:         "2222",
		Options:      []string{"StrictHostKeyChecking=accept-new", "ConnectTimeout=5"},
		IdentityFile: "/run/secrets/node-key",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildSSHExecutorConfig() = %#v, want %#v", got, want)
	}
}

func TestBuildSSHRunnerEnabledWithSSHConfig(t *testing.T) {
	cfg := &config.Config{SSHPrivateKeyPath: "/run/secrets/node-key"}

	if got := buildSSHRunner(cfg); got == nil {
		t.Fatal("buildSSHRunner() = nil, want configured runner")
	}
}
