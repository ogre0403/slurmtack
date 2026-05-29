package mq

import (
	"context"
	"encoding/json"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type Publisher struct {
	conn   *Connection
	logger *slog.Logger
}

func NewPublisher(conn *Connection, logger *slog.Logger) *Publisher {
	return &Publisher{conn: conn, logger: trace.OrDefault(logger)}
}

func (p *Publisher) PublishRequested(ctx context.Context, executionID string, direction domain.SwitchDirection) error {
	return p.publish(ctx, RequestedRoutingKey, RequestedEvent{
		ExecutionID: executionID,
		Direction:   direction,
	})
}

func (p *Publisher) PublishNodeSelected(ctx context.Context, executionID, nodeName string) error {
	return p.publish(ctx, NodeSelectedRoutingKey, NodeSelectedEvent{
		ExecutionID: executionID,
		NodeName:    nodeName,
	})
}

func (p *Publisher) publish(ctx context.Context, routingKey string, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return p.conn.Publish(ctx, ExchangeName, routingKey, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}
