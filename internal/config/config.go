package config

import (
	"fmt"
	"os"
)

type Config struct {
	APIToken   string
	ListenAddr string
	DBPath     string

	SlurmAPIURL   string
	SlurmJWTToken string
	OSAuthURL     string
	OSProjectName string
	OSUsername    string
	OSPassword    string
	AMQPURL       string
}

func Load() (*Config, error) {
	cfg := &Config{
		APIToken:      os.Getenv("API_TOKEN"),
		ListenAddr:    os.Getenv("LISTEN_ADDR"),
		DBPath:        os.Getenv("DB_PATH"),
		SlurmAPIURL:   os.Getenv("SLURM_API_URL"),
		SlurmJWTToken: os.Getenv("SLURM_JWT_TOKEN"),
		OSAuthURL:     os.Getenv("OS_AUTH_URL"),
		OSProjectName: os.Getenv("OS_PROJECT_NAME"),
		OSUsername:    os.Getenv("OS_USERNAME"),
		OSPassword:    os.Getenv("OS_PASSWORD"),
		AMQPURL:       os.Getenv("AMQP_URL"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "slurmtack.db"
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.APIToken == "" {
		return fmt.Errorf("required environment variable API_TOKEN is not set")
	}
	return nil
}
