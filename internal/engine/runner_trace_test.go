package engine

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

// capturedRecord holds the merged attrs from the handler WithAttrs chain plus
// the record-level attrs, so tests can assert on all fields regardless of
// whether they were attached via With() or passed directly to the log call.
type capturedRecord struct {
	Message string
	Level   slog.Level
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

// fakeStepHandler is a minimal StepHandler for trace tests.
type fakeStepHandler struct {
	name string
	err  error
}

func (f *fakeStepHandler) Name() string                                         { return f.name }
func (f *fakeStepHandler) Execute(_ context.Context, _ *domain.Execution) error { return f.err }

func TestRunnerTransitionLogsSuccess(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()
	exec := &domain.Execution{
		ID:            "trace-exec-1",
		NodeName:      "gpu-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		CurrentState:  domain.StateRequested,
		StateVersion:  0,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	logger, cs := newCaptureLogger()
	r := NewRunner(s, logger)

	if err := r.Transition(ctx, "trace-exec-1", domain.StateLocked); err != nil {
		t.Fatalf("Transition: %v", err)
	}

	req := cs.find(trace.EventTransitionRequested)
	if req == nil {
		t.Fatal("expected transition.requested log, got none")
	}
	if req.Attrs["execution_id"] != "trace-exec-1" {
		t.Errorf("transition.requested: execution_id = %q, want %q", req.Attrs["execution_id"], "trace-exec-1")
	}
	if req.Attrs["from_state"] != string(domain.StateRequested) {
		t.Errorf("transition.requested: from_state = %q, want %q", req.Attrs["from_state"], domain.StateRequested)
	}
	if req.Attrs["to_state"] != string(domain.StateLocked) {
		t.Errorf("transition.requested: to_state = %q, want %q", req.Attrs["to_state"], domain.StateLocked)
	}

	succ := cs.find(trace.EventTransitionSucceeded)
	if succ == nil {
		t.Fatal("expected transition.succeeded log, got none")
	}
	if succ.Attrs["to_state"] != string(domain.StateLocked) {
		t.Errorf("transition.succeeded: to_state = %q, want %q", succ.Attrs["to_state"], domain.StateLocked)
	}
}

func TestRunnerTransitionLogsInvalidTransition(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()
	exec := &domain.Execution{
		ID:            "trace-exec-2",
		CurrentState:  domain.StateRequested,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	logger, cs := newCaptureLogger()
	r := NewRunner(s, logger)

	err := r.Transition(ctx, "trace-exec-2", domain.StateCompleted) // invalid jump
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}

	rec := cs.find(trace.EventTransitionFailed)
	if rec == nil {
		t.Fatal("expected transition.failed log, got none")
	}
	if rec.Attrs["reason"] == "" {
		t.Error("transition.failed: expected non-empty reason attr")
	}
}

func TestRunnerFailExecutionLogs(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()
	exec := &domain.Execution{
		ID:            "trace-exec-3",
		NodeName:      "gpu-02",
		Direction:     domain.DirectionSlurmToOpenStack,
		CurrentState:  domain.StatePrecheckPassed,
		StateVersion:  2,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	logger, cs := newCaptureLogger()
	r := NewRunner(s, logger)

	if err := r.FailExecution(ctx, "trace-exec-3", domain.FailurePrecheckBlocked, "precheck_failed", "node not reachable"); err != nil {
		t.Fatalf("FailExecution: %v", err)
	}

	rec := cs.find(trace.EventExecutionFailed)
	if rec == nil {
		t.Fatal("expected execution.failed log, got none")
	}
	if rec.Attrs["execution_id"] != "trace-exec-3" {
		t.Errorf("execution.failed: execution_id = %q, want %q", rec.Attrs["execution_id"], "trace-exec-3")
	}
	if rec.Attrs["error_code"] != "precheck_failed" {
		t.Errorf("execution.failed: error_code = %q, want %q", rec.Attrs["error_code"], "precheck_failed")
	}
	if rec.Attrs["failure_class"] != string(domain.FailurePrecheckBlocked) {
		t.Errorf("execution.failed: failure_class = %q, want %q", rec.Attrs["failure_class"], domain.FailurePrecheckBlocked)
	}
}

func TestRunnerFailExecutionFromHostReachable(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()
	exec := &domain.Execution{
		ID:            "trace-exec-attach",
		NodeName:      "gpu-05",
		Direction:     domain.DirectionOpenStackToSlurm,
		CurrentState:  domain.StateHostReachable,
		StateVersion:  7,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s, nil)

	if err := r.FailExecution(ctx, "trace-exec-attach", domain.FailureMutationPartial, "step_error", "attach failed"); err != nil {
		t.Fatalf("FailExecution: %v", err)
	}

	updated, err := s.GetExecution(ctx, "trace-exec-attach")
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if updated.CurrentState != domain.StateFailedNeedsRollback {
		t.Fatalf("CurrentState = %s, want %s", updated.CurrentState, domain.StateFailedNeedsRollback)
	}
	if !updated.CurrentState.IsTerminal() {
		t.Fatalf("state %s should be terminal", updated.CurrentState)
	}
	if updated.FinalErrorSummary != "attach failed" {
		t.Fatalf("FinalErrorSummary = %q, want %q", updated.FinalErrorSummary, "attach failed")
	}
}

func TestRunnerRunStepLogsSucceeded(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()
	exec := &domain.Execution{
		ID:            "trace-exec-4",
		NodeName:      "gpu-03",
		Direction:     domain.DirectionOpenStackToSlurm,
		CurrentState:  domain.StateLocked,
		StateVersion:  1,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	logger, cs := newCaptureLogger()
	r := NewRunner(s, logger)

	if err := r.RunStep(ctx, "trace-exec-4", &fakeStepHandler{name: "my-step"}); err != nil {
		t.Fatalf("RunStep: %v", err)
	}

	started := cs.find(trace.EventStepStarted)
	if started == nil {
		t.Fatal("expected step.started log, got none")
	}
	if started.Attrs["step_name"] != "my-step" {
		t.Errorf("step.started: step_name = %q, want %q", started.Attrs["step_name"], "my-step")
	}

	succeeded := cs.find(trace.EventStepSucceeded)
	if succeeded == nil {
		t.Fatal("expected step.succeeded log, got none")
	}
}

func TestRunnerRunStepLogsFailed(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()
	exec := &domain.Execution{
		ID:            "trace-exec-5",
		NodeName:      "gpu-04",
		Direction:     domain.DirectionOpenStackToSlurm,
		CurrentState:  domain.StateLocked,
		StateVersion:  1,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	logger, cs := newCaptureLogger()
	r := NewRunner(s, logger)

	stepErr := errors.New("step exploded")
	err := r.RunStep(ctx, "trace-exec-5", &fakeStepHandler{name: "failing-step", err: stepErr})
	if err == nil {
		t.Fatal("expected RunStep to return error")
	}

	rec := cs.find(trace.EventStepFailed)
	if rec == nil {
		t.Fatal("expected step.failed log, got none")
	}
	if rec.Attrs["step_name"] != "failing-step" {
		t.Errorf("step.failed: step_name = %q, want %q", rec.Attrs["step_name"], "failing-step")
	}
	if rec.Attrs["error"] != stepErr.Error() {
		t.Errorf("step.failed: error = %q, want %q", rec.Attrs["error"], stepErr.Error())
	}
}
