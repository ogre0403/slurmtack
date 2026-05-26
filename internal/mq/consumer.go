package mq

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type Consumer struct {
	conn   *Connection
	store  store.Store
	logger *slog.Logger
}

func NewConsumer(conn *Connection, s store.Store, logger *slog.Logger) *Consumer {
	return &Consumer{conn: conn, store: s, logger: trace.OrDefault(logger)}
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		err := c.consume(ctx)
		if err == nil || ctx.Err() != nil {
			return ctx.Err()
		}
		c.logger.Warn("mq.reconnecting", "error", err.Error())
		if err := c.conn.Reconnect(ctx); err != nil {
			return err
		}
	}
}

func (c *Consumer) consume(ctx context.Context) error {
	ch := c.conn.Channel()
	if ch == nil {
		return errors.New("no channel available")
	}

	allocMsgs, err := ch.Consume(AllocationQueueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	drainedMsgs, err := ch.Consume(DrainedQueueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	closeCh := c.conn.NotifyClose()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-closeCh:
			if err != nil {
				return err
			}
			return errors.New("connection closed")
		case msg, ok := <-allocMsgs:
			if !ok {
				return errors.New("allocation channel closed")
			}
			c.handleAllocation(ctx, msg)
		case msg, ok := <-drainedMsgs:
			if !ok {
				return errors.New("drained channel closed")
			}
			c.handleDrained(ctx, msg)
		}
	}
}

func (c *Consumer) handleAllocation(ctx context.Context, msg amqp.Delivery) {
	var evt AllocationEvent
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		c.logger.Warn("mq.malformed_message", "queue", AllocationQueueName, "error", err.Error())
		msg.Ack(false)
		return
	}

	if evt.ExecutionID == "" || evt.JobID == "" || evt.NodeName == "" {
		c.logger.Warn("mq.missing_fields", "queue", AllocationQueueName)
		msg.Ack(false)
		return
	}

	execLog := c.logger.With("execution_id", evt.ExecutionID, "job_id", evt.JobID, "node_name", evt.NodeName)
	execLog.Info(trace.EventWaitProgress, "component", "mq", "event_type", "allocation")

	exec, err := c.store.GetExecution(ctx, evt.ExecutionID)
	if errors.Is(err, store.ErrNotFound) {
		execLog.Warn("mq.unknown_execution")
		msg.Ack(false)
		return
	}
	if err != nil {
		execLog.Warn("mq.store_error", "error", err.Error())
		msg.Nack(false, true)
		return
	}

	if exec.CurrentState != domain.StateAwaitingSourceAllocation {
		execLog.Warn("mq.unexpected_state", "current_state", string(exec.CurrentState))
		msg.Ack(false)
		return
	}

	exec.NodeName = evt.NodeName
	exec.PlaceholderJobID = evt.JobID
	now := time.Now()
	exec.AllocationEventAt = &now
	if err := c.store.UpdateExecution(ctx, exec); err != nil {
		execLog.Warn("mq.update_failed", "error", err.Error())
		msg.Nack(false, true)
		return
	}

	err = c.store.AdvanceState(ctx, evt.ExecutionID, exec.StateVersion, domain.StateNodeIdentified)
	if errors.Is(err, store.ErrVersionConflict) {
		execLog.Info("mq.version_conflict")
		msg.Ack(false)
		return
	}
	if err != nil {
		execLog.Warn("mq.advance_failed", "error", err.Error())
		msg.Nack(false, true)
		return
	}

	execLog.Info(trace.EventWaitSatisfied, "component", "mq", "event_type", "allocation", "new_state", string(domain.StateNodeIdentified))
	msg.Ack(false)
}

func (c *Consumer) handleDrained(ctx context.Context, msg amqp.Delivery) {
	var evt NodeDrainedEvent
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		c.logger.Warn("mq.malformed_message", "queue", DrainedQueueName, "error", err.Error())
		msg.Ack(false)
		return
	}

	if evt.ExecutionID == "" || evt.NodeName == "" {
		c.logger.Warn("mq.missing_fields", "queue", DrainedQueueName)
		msg.Ack(false)
		return
	}

	execLog := c.logger.With("execution_id", evt.ExecutionID, "node_name", evt.NodeName)
	execLog.Info(trace.EventWaitProgress, "component", "mq", "event_type", "drained")

	exec, err := c.store.GetExecution(ctx, evt.ExecutionID)
	if errors.Is(err, store.ErrNotFound) {
		execLog.Warn("mq.unknown_execution")
		msg.Ack(false)
		return
	}
	if err != nil {
		execLog.Warn("mq.store_error", "error", err.Error())
		msg.Nack(false, true)
		return
	}

	if exec.CurrentState != domain.StateSourceQuiescing {
		execLog.Warn("mq.unexpected_state", "current_state", string(exec.CurrentState))
		msg.Ack(false)
		return
	}

	err = c.store.AdvanceState(ctx, evt.ExecutionID, exec.StateVersion, domain.StateSourceDetached)
	if errors.Is(err, store.ErrVersionConflict) {
		execLog.Info("mq.version_conflict")
		msg.Ack(false)
		return
	}
	if err != nil {
		execLog.Warn("mq.advance_failed", "error", err.Error())
		msg.Nack(false, true)
		return
	}

	execLog.Info(trace.EventWaitSatisfied, "component", "mq", "event_type", "drained", "new_state", string(domain.StateSourceDetached))
	msg.Ack(false)
}
