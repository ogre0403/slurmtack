package mq

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestRetryWithBackoffEventuallySucceeds(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var waits []time.Duration
	attempts := 0

	gotAttempts, err := retryWithBackoff(context.Background(), logger, func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}, "mq.connect_attempt_failed", func() error {
		attempts++
		if attempts < 3 {
			return errors.New("broker not ready")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retryWithBackoff() error = %v, want nil", err)
	}
	if gotAttempts != 2 {
		t.Fatalf("retryWithBackoff() attempts = %d, want 2", gotAttempts)
	}
	if len(waits) != 2 {
		t.Fatalf("wait calls = %d, want 2", len(waits))
	}
	if waits[0] != time.Second {
		t.Fatalf("first backoff = %v, want %v", waits[0], time.Second)
	}
	if waits[1] != 2*time.Second {
		t.Fatalf("second backoff = %v, want %v", waits[1], 2*time.Second)
	}
}

func TestRetryWithBackoffStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	waitCalls := 0

	gotAttempts, err := retryWithBackoff(ctx, logger, func(ctx context.Context, delay time.Duration) error {
		waitCalls++
		cancel()
		return ctx.Err()
	}, "mq.connect_attempt_failed", func() error {
		return errors.New("broker not ready")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("retryWithBackoff() error = %v, want context canceled", err)
	}
	if gotAttempts != 1 {
		t.Fatalf("retryWithBackoff() attempts = %d, want 1", gotAttempts)
	}
	if waitCalls != 1 {
		t.Fatalf("wait calls = %d, want 1", waitCalls)
	}
}
