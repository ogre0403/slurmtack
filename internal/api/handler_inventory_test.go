package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type fakeSlurmInventoryClient struct {
	partitions []slurm.Partition
	nodes      map[string]*slurm.NodeState
}

func (f *fakeSlurmInventoryClient) ListPartitions(_ context.Context) ([]slurm.Partition, error) {
	return f.partitions, nil
}

func (f *fakeSlurmInventoryClient) GetNodeState(_ context.Context, name string) (*slurm.NodeState, error) {
	if s, ok := f.nodes[name]; ok {
		return s, nil
	}
	return nil, nil
}

func (f *fakeSlurmInventoryClient) GetNodeStateWithIdentity(_ context.Context, name string, _ slurm.WorkloadIdentity) (*slurm.NodeState, error) {
	return f.GetNodeState(context.Background(), name)
}

func (f *fakeSlurmInventoryClient) SubmitPlaceholderJob(_ context.Context, _ slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	return nil, nil
}
func (f *fakeSlurmInventoryClient) DrainNode(_ context.Context, _, _ string) error              { return nil }
func (f *fakeSlurmInventoryClient) ResumeNode(_ context.Context, _ string) error               { return nil }
func (f *fakeSlurmInventoryClient) CancelJob(_ context.Context, _ string) error                { return nil }
func (f *fakeSlurmInventoryClient) CancelJobWithIdentity(_ context.Context, _ string, _ slurm.WorkloadIdentity) error { return nil }
func (f *fakeSlurmInventoryClient) VerifyToken(_ context.Context, _, _ string) error             { return nil }

type fakeOSInventoryClient struct {
	services   map[string]*openstack.ComputeServiceStatus
	instances  map[string][]openstack.Instance
	migrations map[string][]string
}

func (f *fakeOSInventoryClient) GetComputeService(_ context.Context, host string) (*openstack.ComputeServiceStatus, error) {
	if s, ok := f.services[host]; ok {
		return s, nil
	}
	return nil, nil
}

func (f *fakeOSInventoryClient) ListInstances(_ context.Context, host string) ([]openstack.Instance, error) {
	return f.instances[host], nil
}

func (f *fakeOSInventoryClient) ListActiveMigrations(_ context.Context, host string) ([]string, error) {
	return f.migrations[host], nil
}

func (f *fakeOSInventoryClient) DisableComputeService(_ context.Context, _, _ string) error {
	return nil
}
func (f *fakeOSInventoryClient) EnableComputeService(_ context.Context, _ string) error {
	return nil
}

func setupInventoryTest(t *testing.T, sc *fakeSlurmInventoryClient, oc *fakeOSInventoryClient) (*gin.Engine, *store.SQLiteStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sqlStore, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { sqlStore.Close() })

	handler := NewInventoryHandler(sc, oc, sqlStore)
	engine := gin.New()
	engine.GET("/v1/dashboard/inventory", handler.Get)
	return engine, sqlStore
}

func TestInventory_BasicPartitionsAndNodes(t *testing.T) {
	sc := &fakeSlurmInventoryClient{
		partitions: []slurm.Partition{
			{Name: "gpu-maint", Nodes: []string{"gpu-01", "gpu-02"}},
			{Name: "gpu-prod", Nodes: []string{"gpu-02", "gpu-03"}},
		},
		nodes: map[string]*slurm.NodeState{
			"gpu-01": {NodeName: "gpu-01", State: "idle", GRES: []string{"gpu:a100:8"}},
			"gpu-02": {NodeName: "gpu-02", State: "drained", GRES: []string{"gpu:a100:8"}},
			"gpu-03": {NodeName: "gpu-03", State: "idle"},
		},
	}
	oc := &fakeOSInventoryClient{
		services: map[string]*openstack.ComputeServiceStatus{
			"gpu-01": {Host: "gpu-01", Enabled: false, Status: "disabled", State: "down"},
			"gpu-02": {Host: "gpu-02", Enabled: true, Status: "enabled", State: "up"},
			"gpu-03": {Host: "gpu-03", Enabled: false, Status: "disabled", State: "down"},
		},
		instances:  map[string][]openstack.Instance{},
		migrations: map[string][]string{},
	}

	engine, _ := setupInventoryTest(t, sc, oc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/dashboard/inventory", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp InventoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.Partitions) != 2 {
		t.Fatalf("expected 2 partitions, got %d", len(resp.Partitions))
	}
	if len(resp.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(resp.Nodes))
	}

	nodeMap := make(map[string]InventoryNode)
	for _, n := range resp.Nodes {
		nodeMap[n.NodeName] = n
	}

	gpu01 := nodeMap["gpu-01"]
	if gpu01.Owner != "slurm" {
		t.Errorf("gpu-01 owner: want slurm, got %s", gpu01.Owner)
	}
	if gpu01.AvailableDirection != "slurm_to_openstack" {
		t.Errorf("gpu-01 available_direction: want slurm_to_openstack, got %s", gpu01.AvailableDirection)
	}

	gpu02 := nodeMap["gpu-02"]
	if gpu02.Owner != "openstack" {
		t.Errorf("gpu-02 owner: want openstack, got %s", gpu02.Owner)
	}

	gpu03 := nodeMap["gpu-03"]
	if gpu03.Owner != "slurm" {
		t.Errorf("gpu-03 owner: want slurm, got %s", gpu03.Owner)
	}
}

func TestInventory_PartitionFilter(t *testing.T) {
	sc := &fakeSlurmInventoryClient{
		partitions: []slurm.Partition{
			{Name: "gpu-maint", Nodes: []string{"gpu-01"}},
			{Name: "gpu-prod", Nodes: []string{"gpu-02"}},
		},
		nodes: map[string]*slurm.NodeState{
			"gpu-01": {NodeName: "gpu-01", State: "idle"},
		},
	}
	oc := &fakeOSInventoryClient{
		services:   map[string]*openstack.ComputeServiceStatus{},
		instances:  map[string][]openstack.Instance{},
		migrations: map[string][]string{},
	}

	engine, _ := setupInventoryTest(t, sc, oc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/dashboard/inventory?partition=gpu-maint", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp InventoryResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Partitions) != 1 || resp.Partitions[0].Name != "gpu-maint" {
		t.Errorf("expected 1 partition gpu-maint, got %+v", resp.Partitions)
	}
	if len(resp.Nodes) != 1 || resp.Nodes[0].NodeName != "gpu-01" {
		t.Errorf("expected 1 node gpu-01, got %+v", resp.Nodes)
	}
}

func TestInventory_ActiveExecution(t *testing.T) {
	sc := &fakeSlurmInventoryClient{
		partitions: []slurm.Partition{{Name: "p1", Nodes: []string{"gpu-01"}}},
		nodes:      map[string]*slurm.NodeState{"gpu-01": {NodeName: "gpu-01", State: "drained"}},
	}
	oc := &fakeOSInventoryClient{
		services:   map[string]*openstack.ComputeServiceStatus{"gpu-01": {Host: "gpu-01", Enabled: true}},
		instances:  map[string][]openstack.Instance{},
		migrations: map[string][]string{},
	}

	engine, sqlStore := setupInventoryTest(t, sc, oc)

	exec := &domain.Execution{
		ID: "exec-active-1", NodeName: "gpu-01",
		Direction: domain.DirectionOpenStackToSlurm, OverallStatus: domain.OverallStatusActive,
		CurrentState: domain.StateLocked, RequestedAt: time.Now(), RequestedBy: "test",
		DesiredOwner: domain.OwnerSlurm, PreviousOwner: domain.OwnerOpenStack,
	}
	sqlStore.CreateExecution(context.Background(), exec)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/dashboard/inventory", nil)
	engine.ServeHTTP(w, req)

	var resp InventoryResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(resp.Nodes))
	}
	node := resp.Nodes[0]
	if node.Owner != "switching" {
		t.Errorf("expected owner=switching, got %s", node.Owner)
	}
	if node.Switch == nil || node.Switch.ActiveExecutionID != "exec-active-1" {
		t.Errorf("expected switch info, got %+v", node.Switch)
	}
}
