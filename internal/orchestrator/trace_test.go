package orchestrator

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

// capturedRecord holds the merged attrs from the handler WithAttrs chain plus
// record-level attrs so tests can assert on all fields unconditionally.
type capturedRecord struct {
	Message string
	Level   slog.Level
	Attrs   map[string]string
}

type captureStore struct {
	mu      sync.Mutex
	records []*capturedRecord
}

func (s *captureStore) findAll(msg string) []*capturedRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*capturedRecord
	for _, r := range s.records {
		if r.Message == msg {
			out = append(out, r)
		}
	}
	return out
}

func (s *captureStore) find(msg string) *capturedRecord {
	all := s.findAll(msg)
	if len(all) == 0 {
		return nil
	}
	return all[0]
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
	rec := &capturedRecord{
		Message: r.Message,
		Level:   r.Level,
		Attrs:   make(map[string]string),
	}
	for _, a := range h.attrs {
		rec.Attrs[a.Key] = a.Value.String()
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.String()
		return true
	})
	h.shared.mu.Lock()
	h.shared.records = append(h.shared.records, rec)
	h.shared.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &captureHandler{shared: h.shared, attrs: newAttrs}
}

func (h *captureHandler) WithGroup(name string) slog.Handler { return h }

// TestOrchestratorEmitsActionSelected verifies that action.selected is logged
// when the orchestrator picks up an actionable execution.
func TestOrchestratorEmitsActionSelected(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	// O2S execution in precheck_passed: orchestrator will select actionQuiesce,
	// which will fail because no openstack client is provided.
	exec := &domain.Execution{
		ID:            "trace-orch-1",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StatePrecheckPassed,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		StateVersion:  3,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create: %v", err)
	}

	logger, cs := newCaptureLogger()
	runner := engine.NewRunner(s, logger)
	orch := New(s, runner, nil, nil, nil, Config{
		TickInterval:    50 * time.Millisecond,
		SSHPollInterval: 1 * time.Second,
		SSHPollTimeout:  5 * time.Second,
	}, logger)

	tickCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	orch.Run(tickCtx)

	// action.selected must be emitted with the correct fields
	sel := cs.find(trace.EventActionSelected)
	if sel == nil {
		t.Fatal("expected action.selected log, got none")
	}
	if sel.Attrs["execution_id"] != "trace-orch-1" {
		t.Errorf("action.selected: execution_id = %q, want %q", sel.Attrs["execution_id"], "trace-orch-1")
	}
	if sel.Attrs["action"] != "quiesce" {
		t.Errorf("action.selected: action = %q, want %q", sel.Attrs["action"], "quiesce")
	}

	// action.failed must follow because no openstack client is configured
	failed := cs.find(trace.EventActionFailed)
	if failed == nil {
		t.Fatal("expected action.failed log, got none")
	}
	if failed.Attrs["action"] != "quiesce" {
		t.Errorf("action.failed: action = %q, want %q", failed.Attrs["action"], "quiesce")
	}

	// execution.failed must be emitted by the runner after FailExecution
	execFailed := cs.find(trace.EventExecutionFailed)
	if execFailed == nil {
		t.Fatal("expected execution.failed log, got none")
	}
	if execFailed.Attrs["execution_id"] != "trace-orch-1" {
		t.Errorf("execution.failed: execution_id = %q, want %q", execFailed.Attrs["execution_id"], "trace-orch-1")
	}
}

// TestOrchestratorEmitsExecutionCompleted verifies execution.completed is
// logged when the orchestrator drives an execution all the way to StateCompleted.
func TestOrchestratorEmitsExecutionCompleted(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	// Execution in verifying: orchestrator will select actionComplete, which
	// transitions to StateCompleted directly (no external deps needed).
	exec := &domain.Execution{
		ID:            "trace-orch-2",
		NodeName:      "gpu-node-02",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateVerifying,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		StateVersion:  8,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create: %v", err)
	}

	logger, cs := newCaptureLogger()
	runner := engine.NewRunner(s, logger)
	orch := New(s, runner, nil, nil, nil, Config{
		TickInterval:    50 * time.Millisecond,
		SSHPollInterval: 1 * time.Second,
		SSHPollTimeout:  5 * time.Second,
	}, logger)

	tickCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	orch.Run(tickCtx)

	sel := cs.find(trace.EventActionSelected)
	if sel == nil {
		t.Fatal("expected action.selected log, got none")
	}
	if sel.Attrs["action"] != "complete" {
		t.Errorf("action.selected: action = %q, want %q", sel.Attrs["action"], "complete")
	}

	completed := cs.find(trace.EventExecutionCompleted)
	if completed == nil {
		t.Fatal("expected execution.completed log, got none")
	}
	if completed.Attrs["execution_id"] != "trace-orch-2" {
		t.Errorf("execution.completed: execution_id = %q, want %q", completed.Attrs["execution_id"], "trace-orch-2")
	}

	// State should be completed in the store
	updated, _ := s.GetExecution(ctx, "trace-orch-2")
	if updated.CurrentState != domain.StateCompleted {
		t.Errorf("expected state %s, got %s", domain.StateCompleted, updated.CurrentState)
	}
}
