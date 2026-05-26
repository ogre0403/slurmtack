package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	exitSuccess        = 0
	exitStartupFailure = 1
	exitPollTimeout    = 2
	exitMQFailure      = 3
)

type agentConfig struct {
	ExecutionID  string
	AMQPURL      string
	SlurmAPIURL  string
	SlurmJWT     string
	SlurmAPIUser string
	SlurmJobID   string
	PollInterval time.Duration
	PollTimeout  time.Duration
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(exitStartupFailure)
	}

	logger := newLogger(cfg.ExecutionID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		logger.Info("received signal, shutting down")
		cancel()
	}()

	hostname := discoverHostname()
	logger.Info(fmt.Sprintf("discovered hostname: %s", hostname))

	conn, ch, err := connectMQ(cfg.AMQPURL)
	if err != nil {
		logger.Error(fmt.Sprintf("MQ connection failed: %v", err))
		os.Exit(exitStartupFailure)
	}
	defer conn.Close()
	defer ch.Close()

	if err := publishAllocationEvent(ctx, ch, cfg.ExecutionID, cfg.SlurmJobID, hostname); err != nil {
		logger.Error(fmt.Sprintf("failed to publish allocation event: %v", err))
		// Retry once
		conn.Close()
		ch.Close()
		conn, ch, err = connectMQ(cfg.AMQPURL)
		if err != nil {
			logger.Error(fmt.Sprintf("MQ reconnect failed: %v", err))
			os.Exit(exitMQFailure)
		}
		defer conn.Close()
		defer ch.Close()
		if err := publishAllocationEvent(ctx, ch, cfg.ExecutionID, cfg.SlurmJobID, hostname); err != nil {
			logger.Error(fmt.Sprintf("retry publish allocation event failed: %v", err))
			os.Exit(exitMQFailure)
		}
	}
	logger.Info("allocation event published")

	client := &http.Client{Timeout: 10 * time.Second}
	if err := pollDrainLoop(ctx, client, cfg, hostname, logger); err != nil {
		if err == errPollTimeout {
			logger.Error("poll timeout: node did not reach drained state")
			os.Exit(exitPollTimeout)
		}
		logger.Error(fmt.Sprintf("poll loop error: %v", err))
		os.Exit(exitStartupFailure)
	}

	if err := publishNodeDrainedEvent(ctx, ch, cfg.ExecutionID, hostname); err != nil {
		logger.Error(fmt.Sprintf("failed to publish drained event: %v", err))
		// Retry once
		conn.Close()
		ch.Close()
		conn, ch, err = connectMQ(cfg.AMQPURL)
		if err != nil {
			logger.Error(fmt.Sprintf("MQ reconnect failed: %v", err))
			os.Exit(exitMQFailure)
		}
		defer conn.Close()
		defer ch.Close()
		if err := publishNodeDrainedEvent(ctx, ch, cfg.ExecutionID, hostname); err != nil {
			logger.Error(fmt.Sprintf("retry publish drained event failed: %v", err))
			os.Exit(exitMQFailure)
		}
	}
	logger.Info("node_drained event published")

	logger.Info("placeholder agent completed successfully")
	os.Exit(exitSuccess)
}

func loadConfig() (*agentConfig, error) {
	executionID := os.Getenv("EXECUTION_ID")
	if executionID == "" {
		return nil, fmt.Errorf("EXECUTION_ID is required")
	}
	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		return nil, fmt.Errorf("AMQP_URL is required")
	}
	slurmAPIURL := os.Getenv("SLURM_API_URL")
	if slurmAPIURL == "" {
		return nil, fmt.Errorf("SLURM_API_URL is required")
	}
	slurmJWT := os.Getenv("SLURM_JWT_TOKEN")
	if slurmJWT == "" {
		return nil, fmt.Errorf("SLURM_JWT_TOKEN is required")
	}

	pollInterval := parseDuration(os.Getenv("POLL_INTERVAL"), 5*time.Second)
	pollTimeout := parseDuration(os.Getenv("POLL_TIMEOUT"), 30*time.Minute)

	slurmAPIUser := os.Getenv("SLURM_API_USER")
	if slurmAPIUser == "" {
		slurmAPIUser = "cloud-user"
	}

	return &agentConfig{
		ExecutionID:  executionID,
		AMQPURL:      amqpURL,
		SlurmAPIURL:  strings.TrimRight(slurmAPIURL, "/"),
		SlurmJWT:     slurmJWT,
		SlurmAPIUser: slurmAPIUser,
		SlurmJobID:   os.Getenv("SLURM_JOB_ID"),
		PollInterval: pollInterval,
		PollTimeout:  pollTimeout,
	}, nil
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

func discoverHostname() string {
	hostname, _ := os.Hostname()
	if idx := strings.Index(hostname, "."); idx != -1 {
		hostname = hostname[:idx]
	}
	return hostname
}

// MQ functions

func connectMQ(url string) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("channel: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		conn.Close()
		return nil, nil, fmt.Errorf("confirm mode: %w", err)
	}
	return conn, ch, nil
}

func publishAllocationEvent(ctx context.Context, ch *amqp.Channel, executionID, jobID, nodeName string) error {
	body, _ := json.Marshal(map[string]string{
		"execution_id": executionID,
		"job_id":       jobID,
		"node_name":    nodeName,
	})
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx,
		"gpu-switch.events",
		"execution.allocation",
		false, false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		return err
	}
	if !confirm.Wait() {
		return fmt.Errorf("publish not confirmed by broker")
	}
	return nil
}

func publishNodeDrainedEvent(ctx context.Context, ch *amqp.Channel, executionID, nodeName string) error {
	body, _ := json.Marshal(map[string]string{
		"execution_id": executionID,
		"node_name":    nodeName,
	})
	confirm, err := ch.PublishWithDeferredConfirmWithContext(ctx,
		"gpu-switch.events",
		"execution.drained",
		false, false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		return err
	}
	if !confirm.Wait() {
		return fmt.Errorf("publish not confirmed by broker")
	}
	return nil
}

// Drain poll functions

var errPollTimeout = fmt.Errorf("poll timeout")

var drainedStates = map[string]bool{
	"drained":  true,
	"drained*": true,
	"down":     true,
	"down*":    true,
}

func pollDrainLoop(ctx context.Context, client *http.Client, cfg *agentConfig, hostname string, logger *logger) error {
	deadline := time.After(cfg.PollTimeout)
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return errPollTimeout
		case <-ticker.C:
			state, err := getNodeState(ctx, client, cfg, hostname)
			if err != nil {
				logger.Warn(fmt.Sprintf("slurmrestd poll error: %v", err))
				continue
			}
			logger.Info(fmt.Sprintf("node state: %s", state))
			if drainedStates[state] {
				return nil
			}
		}
	}
}

type nodeResponse struct {
	Nodes []struct {
		State []string `json:"state"`
	} `json:"nodes"`
	Errors []struct {
		Error string `json:"error"`
	} `json:"errors"`
}

func getNodeState(ctx context.Context, client *http.Client, cfg *agentConfig, hostname string) (string, error) {
	url := fmt.Sprintf("%s/slurm/v0.0.40/node/%s", cfg.SlurmAPIURL, hostname)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-SLURM-USER-NAME", cfg.SlurmAPIUser)
	req.Header.Set("X-SLURM-USER-TOKEN", cfg.SlurmJWT)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("slurmrestd returned %d", resp.StatusCode)
	}

	var result nodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(result.Errors) > 0 {
		return "", fmt.Errorf("slurmrestd error: %s", result.Errors[0].Error)
	}

	if len(result.Nodes) == 0 || len(result.Nodes[0].State) == 0 {
		return "", fmt.Errorf("no node data returned")
	}

	return strings.Join(result.Nodes[0].State, "+"), nil
}

// Logger

type logger struct {
	executionID string
}

func newLogger(executionID string) *logger {
	return &logger{executionID: executionID}
}

func (l *logger) log(level, msg string) {
	entry, _ := json.Marshal(map[string]string{
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"level":        level,
		"execution_id": l.executionID,
		"message":      msg,
	})
	fmt.Println(string(entry))
}

func (l *logger) Info(msg string)  { l.log("info", msg) }
func (l *logger) Warn(msg string)  { l.log("warn", msg) }
func (l *logger) Error(msg string) { l.log("error", msg) }
