package main

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/config"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/store"
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

type stubMQConnection struct {
	connectCalls   int
	reconnectCalls int
	closeCalls     int
}

func (c *stubMQConnection) Connect(context.Context) error {
	c.connectCalls++
	return nil
}

func (c *stubMQConnection) Reconnect(context.Context) error {
	c.reconnectCalls++
	return nil
}

func (c *stubMQConnection) Close() error {
	c.closeCalls++
	return nil
}

type blockingConsumer struct {
	started chan struct{}
	once    sync.Once
}

func (c *blockingConsumer) Run(ctx context.Context) error {
	c.once.Do(func() { close(c.started) })
	<-ctx.Done()
	return ctx.Err()
}

func TestBuildSSHRunnerDisabledWithoutSSHConfig(t *testing.T) {
	if got := buildSSHRunner(&config.Config{}, slog.Default()); got != nil {
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

	if got := buildSSHRunner(cfg, slog.Default()); got == nil {
		t.Fatal("buildSSHRunner() = nil, want configured runner")
	}
}

func TestStartMQRetriesUntilConsumerRuns(t *testing.T) {
	t.Parallel()

	logger, logs := newCaptureLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &stubMQConnection{}
	consumer := &blockingConsumer{started: make(chan struct{})}
	declareCalls := 0
	consumerCreations := 0
	var wg sync.WaitGroup

	gotConn := startMQ(ctx, &wg, "amqp://broker", store.NewMemoryStore(), logger, mqStartupDeps{
		newConnection: func(string, *slog.Logger) mqConnection {
			return conn
		},
		declareTopology: func(mqConnection) error {
			declareCalls++
			if declareCalls < 3 {
				return errors.New("broker not ready")
			}
			return nil
		},
		newConsumer: func(mqConnection, store.Store, *slog.Logger) mqConsumer {
			consumerCreations++
			return consumer
		},
	})
	if gotConn != conn {
		t.Fatalf("startMQ() connection = %#v, want stub connection", gotConn)
	}

	select {
	case <-consumer.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mq consumer to start")
	}

	cancel()
	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mq supervisor to stop")
	}

	if conn.connectCalls != 1 {
		t.Fatalf("connect calls = %d, want 1", conn.connectCalls)
	}
	if conn.reconnectCalls != 2 {
		t.Fatalf("reconnect calls = %d, want 2", conn.reconnectCalls)
	}
	if declareCalls != 3 {
		t.Fatalf("declare calls = %d, want 3", declareCalls)
	}
	if consumerCreations != 1 {
		t.Fatalf("consumer creations = %d, want 1", consumerCreations)
	}
	if rec := logs.find("mq.activated_after_retry"); rec == nil {
		t.Fatal("expected mq.activated_after_retry log")
	} else if rec.Attrs["attempts"] != "2" {
		t.Fatalf("mq.activated_after_retry attempts = %q, want %q", rec.Attrs["attempts"], "2")
	}
	if logs.find("mq.consumer.started") == nil {
		t.Fatal("expected mq.consumer.started log")
	}
}

func TestStartMQDisabledWithoutAMQPURL(t *testing.T) {
	t.Parallel()

	logger, logs := newCaptureLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	newConnectionCalls := 0
	var wg sync.WaitGroup

	gotConn := startMQ(ctx, &wg, "", store.NewMemoryStore(), logger, mqStartupDeps{
		newConnection: func(string, *slog.Logger) mqConnection {
			newConnectionCalls++
			return &stubMQConnection{}
		},
	})
	if gotConn != nil {
		t.Fatalf("startMQ() connection = %#v, want nil", gotConn)
	}
	if newConnectionCalls != 0 {
		t.Fatalf("new connection calls = %d, want 0", newConnectionCalls)
	}
	if rec := logs.find("mq.disabled"); rec == nil {
		t.Fatal("expected mq.disabled log")
	}
}
