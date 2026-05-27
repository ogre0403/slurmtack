package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	APIToken   string
	ListenAddr string
	DBPath     string

	SlurmAPIURL         string
	SlurmJWTToken       string
	SlurmAPIUser        string
	SlurmAdminUser      string
	SlurmAdminJWTToken  string
	OSAuthURL           string
	OSProjectName       string
	OSUsername          string
	OSPassword          string
	OSUserDomainName    string
	OSProjectDomainName string
	AMQPURL             string
	SSHPollInterval     time.Duration
	SSHPollTimeout      time.Duration
	PlaceholderSIFPath  string
	SSHUser             string
	SSHPort             string
	SSHOptions          string
	SSHPrivateKeyPath   string
}

func Load() (*Config, error) {
	cfg := &Config{
		APIToken:            os.Getenv("API_TOKEN"),
		ListenAddr:          os.Getenv("LISTEN_ADDR"),
		DBPath:              os.Getenv("DB_PATH"),
		SlurmAPIURL:         os.Getenv("SLURM_API_URL"),
		SlurmJWTToken:       os.Getenv("SLURM_JWT_TOKEN"),
		SlurmAPIUser:        os.Getenv("SLURM_API_USER"),
		SlurmAdminUser:      os.Getenv("SLURM_ADMIN_USER"),
		SlurmAdminJWTToken:  os.Getenv("SLURM_ADMIN_JWT_TOKEN"),
		OSAuthURL:           os.Getenv("OS_AUTH_URL"),
		OSProjectName:       os.Getenv("OS_PROJECT_NAME"),
		OSUsername:          os.Getenv("OS_USERNAME"),
		OSPassword:          os.Getenv("OS_PASSWORD"),
		OSUserDomainName:    os.Getenv("OS_USER_DOMAIN_NAME"),
		OSProjectDomainName: os.Getenv("OS_PROJECT_DOMAIN_NAME"),
		AMQPURL:             os.Getenv("AMQP_URL"),
		PlaceholderSIFPath:  os.Getenv("PLACEHOLDER_SIF_PATH"),
		SSHUser:             os.Getenv("SSH_USER"),
		SSHPort:             os.Getenv("SSH_PORT"),
		SSHOptions:          os.Getenv("SSH_OPTIONS"),
		SSHPrivateKeyPath:   os.Getenv("SSH_PRIVATE_KEY_PATH"),
	}

	cfg.SSHPollInterval = parseDuration(os.Getenv("SSH_POLL_INTERVAL"), 10*time.Second)
	cfg.SSHPollTimeout = parseDuration(os.Getenv("SSH_POLL_TIMEOUT"), 10*time.Minute)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "slurmtack.db"
	}
	if cfg.SlurmAPIURL != "" && cfg.SlurmJWTToken == "" {
		return nil, fmt.Errorf("SLURM_JWT_TOKEN is required when SLURM_API_URL is set")
	}
	if cfg.SlurmAPIUser == "" {
		cfg.SlurmAPIUser = "cloud-user"
	}
	if cfg.SlurmAdminUser == "" {
		cfg.SlurmAdminUser = cfg.SlurmAPIUser
	}
	if cfg.SlurmAdminJWTToken == "" {
		cfg.SlurmAdminJWTToken = cfg.SlurmJWTToken
	}

	if cfg.OSUserDomainName == "" {
		cfg.OSUserDomainName = "Default"
	}
	if cfg.OSProjectDomainName == "" {
		cfg.OSProjectDomainName = "Default"
	}

	return cfg, nil
}

func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}

func (c *Config) validate() error {
	if c.APIToken == "" {
		return fmt.Errorf("required environment variable API_TOKEN is not set")
	}
	if c.SSHRunnerEnabled() {
		if c.SSHPrivateKeyPath == "" {
			return fmt.Errorf("SSH_PRIVATE_KEY_PATH is required when SSH runner configuration is enabled")
		}
		file, err := os.Open(c.SSHPrivateKeyPath)
		if err != nil {
			return fmt.Errorf("SSH_PRIVATE_KEY_PATH must point to a readable file: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("SSH_PRIVATE_KEY_PATH must point to a readable file: %w", err)
		}
	}
	return nil
}

func (c *Config) SSHRunnerEnabled() bool {
	return c.SSHUser != "" || c.SSHPort != "" || c.SSHOptions != "" || c.SSHPrivateKeyPath != ""
}
