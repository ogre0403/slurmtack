package mq

import (
	"context"
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
	return c.connectLocked(ctx)
}

func (c *Connection) connectLocked(ctx context.Context) error {
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

	var attempt int
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.connectLocked(ctx); err != nil {
			attempt++
			backoff := time.Duration(math.Min(float64(time.Second)*math.Pow(2, float64(attempt)), float64(30*time.Second)))
			c.logger.Warn("mq.reconnect_attempt_failed", "attempt", attempt, "error", err, "retry_in", backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			continue
		}
		c.logger.Info("mq.reconnected", "attempts", attempt)
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
