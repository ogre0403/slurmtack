package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWithSSHRunnerConfig(t *testing.T) {
	t.Setenv("API_TOKEN", "token")
	t.Setenv("SSH_USER", "slurm")
	t.Setenv("SSH_PORT", "2222")
	t.Setenv("SSH_OPTIONS", "StrictHostKeyChecking=accept-new")

	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	t.Setenv("SSH_PRIVATE_KEY_PATH", keyPath)
	if err := os.WriteFile(keyPath, []byte("test-key"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SSHPrivateKeyPath != keyPath {
		t.Fatalf("SSHPrivateKeyPath = %q, want %q", cfg.SSHPrivateKeyPath, keyPath)
	}
	if !cfg.SSHRunnerEnabled() {
		t.Fatal("expected SSH runner configuration to be enabled")
	}
}

func TestLoadRejectsSSHRunnerConfigWithoutPrivateKey(t *testing.T) {
	t.Setenv("API_TOKEN", "token")
	t.Setenv("SSH_USER", "slurm")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing key path error")
	}
	if !strings.Contains(err.Error(), "SSH_PRIVATE_KEY_PATH is required") {
		t.Fatalf("Load() error = %q, want SSH_PRIVATE_KEY_PATH requirement", err)
	}
}

func TestLoadRejectsUnreadableSSHPrivateKeyPath(t *testing.T) {
	t.Setenv("API_TOKEN", "token")
	t.Setenv("SSH_PRIVATE_KEY_PATH", filepath.Join(t.TempDir(), "missing-key"))

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want unreadable key path error")
	}
	if !strings.Contains(err.Error(), "SSH_PRIVATE_KEY_PATH must point to a readable file") {
		t.Fatalf("Load() error = %q, want readable file error", err)
	}
}
