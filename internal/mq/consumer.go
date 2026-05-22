package mq

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

type Consumer struct {
	conn  *Connection
	store store.Store
}

func NewConsumer(conn *Connection, s store.Store) *Consumer {
	return &Consumer{conn: conn, store: s}
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		err := c.consume(ctx)
		if err == nil || ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("mq: consumer error: %v, reconnecting...", err)
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
		log.Printf("mq: malformed allocation message: %v", err)
		msg.Ack(false)
		return
	}

	if evt.ExecutionID == "" || evt.JobID == "" || evt.NodeName == "" {
		log.Printf("mq: allocation message missing required fields")
		msg.Ack(false)
		return
	}

	exec, err := c.store.GetExecution(ctx, evt.ExecutionID)
	if errors.Is(err, store.ErrNotFound) {
		log.Printf("mq: unknown execution %s, discarding", evt.ExecutionID)
		msg.Ack(false)
		return
	}
	if err != nil {
		log.Printf("mq: store error for %s: %v, nacking", evt.ExecutionID, err)
		msg.Nack(false, true)
		return
	}

	if exec.CurrentState != domain.StateAwaitingSourceAllocation {
		log.Printf("mq: execution %s not in awaiting_source_allocation (is %s), discarding", evt.ExecutionID, exec.CurrentState)
		msg.Ack(false)
		return
	}

	exec.NodeName = evt.NodeName
	exec.PlaceholderJobID = evt.JobID
	now := time.Now()
	exec.AllocationEventAt = &now
	if err := c.store.UpdateExecution(ctx, exec); err != nil {
		log.Printf("mq: failed to bind node for %s: %v, nacking", evt.ExecutionID, err)
		msg.Nack(false, true)
		return
	}

	err = c.store.AdvanceState(ctx, evt.ExecutionID, exec.StateVersion, domain.StateNodeIdentified)
	if errors.Is(err, store.ErrVersionConflict) {
		log.Printf("mq: version conflict advancing %s, acking (already handled)", evt.ExecutionID)
		msg.Ack(false)
		return
	}
	if err != nil {
		log.Printf("mq: failed to advance %s: %v, nacking", evt.ExecutionID, err)
		msg.Nack(false, true)
		return
	}

	msg.Ack(false)
}

func (c *Consumer) handleDrained(ctx context.Context, msg amqp.Delivery) {
	var evt NodeDrainedEvent
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		log.Printf("mq: malformed drained message: %v", err)
		msg.Ack(false)
		return
	}

	if evt.ExecutionID == "" || evt.NodeName == "" {
		log.Printf("mq: drained message missing required fields")
		msg.Ack(false)
		return
	}

	exec, err := c.store.GetExecution(ctx, evt.ExecutionID)
	if errors.Is(err, store.ErrNotFound) {
		log.Printf("mq: unknown execution %s, discarding", evt.ExecutionID)
		msg.Ack(false)
		return
	}
	if err != nil {
		log.Printf("mq: store error for %s: %v, nacking", evt.ExecutionID, err)
		msg.Nack(false, true)
		return
	}

	if exec.CurrentState != domain.StateSourceQuiescing {
		log.Printf("mq: execution %s not in source_quiescing (is %s), discarding", evt.ExecutionID, exec.CurrentState)
		msg.Ack(false)
		return
	}

	err = c.store.AdvanceState(ctx, evt.ExecutionID, exec.StateVersion, domain.StateSourceDetached)
	if errors.Is(err, store.ErrVersionConflict) {
		log.Printf("mq: version conflict advancing %s, acking (already handled)", evt.ExecutionID)
		msg.Ack(false)
		return
	}
	if err != nil {
		log.Printf("mq: failed to advance %s: %v, nacking", evt.ExecutionID, err)
		msg.Nack(false, true)
		return
	}

	msg.Ack(false)
}
