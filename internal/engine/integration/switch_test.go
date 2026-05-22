package integration

import (
	"context"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/service"
	slurmPkg "github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

func TestSlurmToOpenStackFullFlow(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	runner := engine.NewRunner(s)
	slurmClient := NewFakeSlurmClient()
	slurmClient.NextJobID = "job-100"
	slurmClient.Nodes["gpu-01"] = &slurmPkg.NodeState{
		NodeName: "gpu-01",
		State:    "idle",
		GRES:     []string{"gpu:a100:8"},
	}

	osClient := NewFakeOpenStackClient()

	svc := service.NewSwitchService(s)
	allocHandler := slurmPkg.NewAllocationHandler(s, runner, slurmClient)

	execID, err := svc.RequestSwitch(ctx, service.SwitchRequest{
		Direction:       domain.DirectionSlurmToOpenStack,
		RequestedBy:     "test-operator",
		SlurmConstraint: "gpu-a100",
	})
	if err != nil {
		t.Fatalf("request switch: %v", err)
	}

	if err := allocHandler.SubmitPlaceholder(ctx, execID); err != nil {
		t.Fatalf("submit placeholder: %v", err)
	}

	exec, _ := s.GetExecution(ctx, execID)
	if exec.CurrentState != domain.StateAwaitingSourceAllocation {
		t.Fatalf("expected awaiting_source_allocation, got %s", exec.CurrentState)
	}

	err = allocHandler.HandleAllocationEvent(ctx, slurmPkg.AllocationEvent{
		ExecutionID:  execID,
		JobID:        "job-100",
		NodeName:     "gpu-01",
		StateVersion: exec.StateVersion,
	})
	if err != nil {
		t.Fatalf("allocation event: %v", err)
	}

	exec, _ = s.GetExecution(ctx, execID)
	if exec.CurrentState != domain.StateNodeIdentified {
		t.Fatalf("expected node_identified, got %s", exec.CurrentState)
	}
	if exec.NodeName != "gpu-01" {
		t.Fatalf("expected node gpu-01, got %s", exec.NodeName)
	}

	if err := runner.Transition(ctx, execID, domain.StateLocked); err != nil {
		t.Fatalf("transition to locked: %v", err)
	}
	if err := s.AcquireLease(ctx, &domain.NodeLease{
		NodeName:    "gpu-01",
		ExecutionID: execID,
		Holder:      "daemon",
		ExpiresAt:   time.Now().Add(30 * time.Minute),
	}); err != nil {
		t.Fatalf("acquire lease: %v", err)
	}

	if err := runner.Transition(ctx, execID, domain.StatePrecheckPassed); err != nil {
		t.Fatalf("transition to precheck_passed: %v", err)
	}

	quiesce := slurmPkg.NewQuiesceHandler(slurmClient)
	if err := runner.RunStep(ctx, execID, quiesce); err != nil {
		t.Fatalf("quiesce step: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateSourceQuiescing); err != nil {
		t.Fatalf("transition to source_quiescing: %v", err)
	}

	detach := slurmPkg.NewDetachHandler(slurmClient)
	if err := runner.RunStep(ctx, execID, detach); err != nil {
		t.Fatalf("detach step: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateSourceDetached); err != nil {
		t.Fatalf("transition to source_detached: %v", err)
	}

	if err := runner.Transition(ctx, execID, domain.StateHostReconfiguring); err != nil {
		t.Fatalf("transition to host_reconfiguring: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateTargetAttaching); err != nil {
		t.Fatalf("transition to target_attaching: %v", err)
	}

	osClient.Services["gpu-01"] = &openstack.ComputeServiceStatus{
		Host: "gpu-01", Enabled: true, State: "up",
	}
	osVerify := openstack.NewVerifyHandler(osClient)
	if err := runner.RunStep(ctx, execID, osVerify); err != nil {
		t.Fatalf("verify step: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateVerifying); err != nil {
		t.Fatalf("transition to verifying: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateCompleted); err != nil {
		t.Fatalf("transition to completed: %v", err)
	}

	exec, _ = s.GetExecution(ctx, execID)
	if exec.OverallStatus != domain.OverallStatusSucceeded {
		t.Fatalf("expected succeeded, got %s", exec.OverallStatus)
	}
}

func TestOpenStackToSlurmFullFlow(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	runner := engine.NewRunner(s)
	slurmClient := NewFakeSlurmClient()
	slurmClient.Nodes["gpu-02"] = &slurmPkg.NodeState{
		NodeName: "gpu-02",
		State:    "drained",
		GRES:     []string{"gpu:a100:8"},
	}

	osClient := NewFakeOpenStackClient()
	osClient.Services["gpu-02"] = &openstack.ComputeServiceStatus{
		Host: "gpu-02", Enabled: false, State: "down",
	}

	svc := service.NewSwitchService(s)

	execID, err := svc.RequestSwitch(ctx, service.SwitchRequest{
		NodeName:    "gpu-02",
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "test-operator",
	})
	if err != nil {
		t.Fatalf("request switch: %v", err)
	}

	if err := runner.Transition(ctx, execID, domain.StateLocked); err != nil {
		t.Fatalf("transition to locked: %v", err)
	}
	if err := s.AcquireLease(ctx, &domain.NodeLease{
		NodeName:    "gpu-02",
		ExecutionID: execID,
		Holder:      "daemon",
		ExpiresAt:   time.Now().Add(30 * time.Minute),
	}); err != nil {
		t.Fatalf("acquire lease: %v", err)
	}

	precheck := openstack.NewPrecheckHandler(osClient)
	if err := runner.RunStep(ctx, execID, precheck); err != nil {
		t.Fatalf("precheck step: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StatePrecheckPassed); err != nil {
		t.Fatalf("transition to precheck_passed: %v", err)
	}

	osQuiesce := openstack.NewQuiesceHandler(osClient)
	if err := runner.RunStep(ctx, execID, osQuiesce); err != nil {
		t.Fatalf("quiesce step: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateSourceQuiescing); err != nil {
		t.Fatalf("transition to source_quiescing: %v", err)
	}

	osDetach := openstack.NewDetachHandler(osClient)
	if err := runner.RunStep(ctx, execID, osDetach); err != nil {
		t.Fatalf("detach step: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateSourceDetached); err != nil {
		t.Fatalf("transition to source_detached: %v", err)
	}

	if err := runner.Transition(ctx, execID, domain.StateHostReconfiguring); err != nil {
		t.Fatalf("transition to host_reconfiguring: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateTargetAttaching); err != nil {
		t.Fatalf("transition to target_attaching: %v", err)
	}

	slurmClient.Nodes["gpu-02"].State = "idle"
	slurmAttach := slurmPkg.NewAttachHandler(slurmClient)
	if err := runner.RunStep(ctx, execID, slurmAttach); err != nil {
		t.Fatalf("attach step: %v", err)
	}

	slurmVerify := slurmPkg.NewVerifyHandler(slurmClient)
	if err := runner.RunStep(ctx, execID, slurmVerify); err != nil {
		t.Fatalf("verify step: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateVerifying); err != nil {
		t.Fatalf("transition to verifying: %v", err)
	}
	if err := runner.Transition(ctx, execID, domain.StateCompleted); err != nil {
		t.Fatalf("transition to completed: %v", err)
	}

	exec, _ := s.GetExecution(ctx, execID)
	if exec.OverallStatus != domain.OverallStatusSucceeded {
		t.Fatalf("expected succeeded, got %s", exec.OverallStatus)
	}
}

func TestOpenStackToSlurmPrecheckBlocksWithInstances(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	runner := engine.NewRunner(s)

	osClient := NewFakeOpenStackClient()
	osClient.Instances["gpu-03"] = []openstack.Instance{{ID: "vm-1", Name: "test-vm", Status: "ACTIVE"}}
	osClient.Services["gpu-03"] = &openstack.ComputeServiceStatus{
		Host: "gpu-03", Enabled: false, State: "down",
	}

	svc := service.NewSwitchService(s)
	execID, _ := svc.RequestSwitch(ctx, service.SwitchRequest{
		NodeName:    "gpu-03",
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "test-operator",
	})

	runner.Transition(ctx, execID, domain.StateLocked)

	precheck := openstack.NewPrecheckHandler(osClient)
	err := runner.RunStep(ctx, execID, precheck)
	if err == nil {
		t.Fatal("precheck should fail when instances exist")
	}

	steps, _ := s.ListSteps(ctx, execID)
	if len(steps) == 0 {
		t.Fatal("step record should be created")
	}
	if steps[0].Status != domain.StepStatusFailed {
		t.Fatalf("step should be failed, got %s", steps[0].Status)
	}
}
