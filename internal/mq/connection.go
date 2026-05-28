package mq

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Connection struct {
	url    string
	conn   *amqp.Connection
	ch     *amqp.Channel
	logger *slog.Logger
	mu     sync.Mutex
}

func NewConnection(url string, logger *slog.Logger) *Connection {
	if logger == nil {
		logger = slog.Default()
	}
	return &Connection{url: url, logger: logger}
}

func (c *Connection) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	attempts, err := retryWithBackoff(ctx, c.logger, waitWithContext, "mq.connect_attempt_failed", c.connectLocked)
	if err != nil {
		return err
	}
	if attempts > 0 {
		c.logger.Info("mq.connected_after_retry", "attempts", attempts)
	}
	return err
}

func (c *Connection) connectLocked() error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	c.conn = conn
	c.ch = ch
	return nil
}

func (c *Connection) Channel() *amqp.Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ch
}

func (c *Connection) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closeLocked()

	attempts, err := retryWithBackoff(ctx, c.logger, waitWithContext, "mq.reconnect_attempt_failed", c.connectLocked)
	if err != nil {
		return err
	}
	if attempts > 0 {
		c.logger.Info("mq.reconnected", "attempts", attempts)
	}
	return nil
}

func retryWithBackoff(ctx context.Context, logger *slog.Logger, wait func(context.Context, time.Duration) error, failureEvent string, op func() error) (int, error) {
	var attempt int
	for {
		select {
		case <-ctx.Done():
			return attempt, ctx.Err()
		default:
		}

		if err := op(); err != nil {
			attempt++
			backoff := retryBackoff(attempt)
			logger.Warn(failureEvent, "attempt", attempt, "error", err, "retry_in", backoff)
			if err := wait(ctx, backoff); err != nil {
				return attempt, err
			}
			continue
		}

		return attempt, nil
	}
}

func retryBackoff(attempt int) time.Duration {
	return time.Duration(math.Min(float64(time.Second)*math.Pow(2, float64(attempt-1)), float64(30*time.Second)))
}

func waitWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeLocked()
}

func (c *Connection) closeLocked() error {
	if c.ch != nil {
		c.ch.Close()
		c.ch = nil
	}
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *Connection) NotifyClose() chan *amqp.Error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		ch := make(chan *amqp.Error, 1)
		close(ch)
		return ch
	}
	return c.conn.NotifyClose(make(chan *amqp.Error, 1))
}

func (c *Connection) Publish(ctx context.Context, exchange, routingKey string, publishing amqp.Publishing) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return errors.New("no connection available")
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return ch.PublishWithContext(ctx, exchange, routingKey, false, false, publishing)
}
