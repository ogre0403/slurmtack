package store

import (
	"context"
	"sync"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
)

type MemoryStore struct {
	mu            sync.Mutex
	executions    map[string]*domain.Execution
	leases        map[string]*domain.NodeLease
	steps         map[string][]*domain.StepRecord
	tokenRenewals []*domain.AdminTokenRenewal
	nextRenewalID int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		executions: make(map[string]*domain.Execution),
		leases:     make(map[string]*domain.NodeLease),
		steps:      make(map[string][]*domain.StepRecord),
	}
}

func (m *MemoryStore) CreateExecution(_ context.Context, exec *domain.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *exec
	m.executions[exec.ID] = &cp
	return nil
}

func (m *MemoryStore) GetExecution(_ context.Context, id string) (*domain.Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	exec, ok := m.executions[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *exec
	return &cp, nil
}

func (m *MemoryStore) ListExecutions(_ context.Context, nodeName string) ([]*domain.Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Execution
	for _, exec := range m.executions {
		if nodeName == "" || exec.NodeName == nodeName {
			cp := *exec
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *MemoryStore) ListActiveExecutions(_ context.Context) ([]*domain.Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Execution
	for _, exec := range m.executions {
		if exec.OverallStatus == domain.OverallStatusActive {
			cp := *exec
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *MemoryStore) AdvanceState(_ context.Context, executionID string, expectedVersion int64, newState domain.SwitchState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	exec, ok := m.executions[executionID]
	if !ok {
		return ErrNotFound
	}
	if exec.StateVersion != expectedVersion {
		return ErrVersionConflict
	}
	if !domain.IsValidTransition(exec.CurrentState, newState) {
		return ErrVersionConflict
	}
	exec.CurrentState = newState
	exec.StateVersion++
	if newState.IsTerminal() {
		if newState == domain.StateCompleted {
			exec.OverallStatus = domain.OverallStatusSucceeded
		} else {
			exec.OverallStatus = domain.OverallStatusFailed
		}
	}
	return nil
}

func (m *MemoryStore) UpdateExecution(_ context.Context, exec *domain.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.executions[exec.ID]; !ok {
		return ErrNotFound
	}
	cp := *exec
	m.executions[exec.ID] = &cp
	return nil
}

func (m *MemoryStore) AcquireLease(_ context.Context, lease *domain.NodeLease) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.leases[lease.NodeName]
	if ok && existing.ExecutionID != lease.ExecutionID && time.Now().Before(existing.ExpiresAt) {
		return ErrLeaseHeld
	}
	cp := *lease
	m.leases[lease.NodeName] = &cp
	return nil
}

func (m *MemoryStore) ReleaseLease(_ context.Context, nodeName string, executionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.leases[nodeName]
	if !ok {
		return ErrLeaseNotHeld
	}
	if existing.ExecutionID != executionID {
		return ErrLeaseNotHeld
	}
	delete(m.leases, nodeName)
	return nil
}

func (m *MemoryStore) GetLease(_ context.Context, nodeName string) (*domain.NodeLease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease, ok := m.leases[nodeName]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *lease
	return &cp, nil
}

func (m *MemoryStore) CreateStep(_ context.Context, step *domain.StepRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *step
	m.steps[step.ExecutionID] = append(m.steps[step.ExecutionID], &cp)
	return nil
}

func (m *MemoryStore) UpdateStep(_ context.Context, step *domain.StepRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	steps, ok := m.steps[step.ExecutionID]
	if !ok {
		return ErrNotFound
	}
	for i, s := range steps {
		if s.StepName == step.StepName && s.Sequence == step.Sequence {
			cp := *step
			steps[i] = &cp
			return nil
		}
	}
	return ErrNotFound
}

func (m *MemoryStore) ListSteps(_ context.Context, executionID string) ([]*domain.StepRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	steps := m.steps[executionID]
	result := make([]*domain.StepRecord, len(steps))
	for i, s := range steps {
		cp := *s
		result[i] = &cp
	}
	return result, nil
}

func (m *MemoryStore) RecordAdminTokenRenewal(_ context.Context, renewal *domain.AdminTokenRenewal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextRenewalID++
	cp := *renewal
	cp.ID = m.nextRenewalID
	m.tokenRenewals = append(m.tokenRenewals, &cp)
	return nil
}

func (m *MemoryStore) ListAdminTokenRenewals(_ context.Context) ([]*domain.AdminTokenRenewal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*domain.AdminTokenRenewal, len(m.tokenRenewals))
	for i, r := range m.tokenRenewals {
		cp := *r
		result[i] = &cp
	}
	return result, nil
}
