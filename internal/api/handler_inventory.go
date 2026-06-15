package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type InventoryHandler struct {
	slurmClient slurm.Client
	osClient    openstack.Client
	store       store.Store
}

type inventoryEnrichment struct {
	slurmState *slurm.NodeState
	osService  *openstack.ComputeServiceStatus
	instances  int
	migrations int
}

func NewInventoryHandler(sc slurm.Client, oc openstack.Client, s store.Store) *InventoryHandler {
	return &InventoryHandler{slurmClient: sc, osClient: oc, store: s}
}

// Get returns the current node inventory.
// @Summary     Get node inventory
// @Description Returns the current inventory of Slurm partitions and nodes, enriched with OpenStack compute service state and active switch execution status. Only available when both Slurm and OpenStack clients are configured.
// @Tags        dashboard
// @Produce     json
// @Security    BearerAuth
// @Param       partition query    string false "Filter results to a single Slurm partition by name"
// @Success     200 {object} InventoryResponse
// @Failure     500 {object} ErrorResponse
// @Router      /v1/dashboard/inventory [get]
func (h *InventoryHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	partitionFilter := c.Query("partition")

	partitions, err := h.slurmClient.ListPartitions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list partitions: " + err.Error()})
		return
	}

	if partitionFilter != "" {
		var filtered []slurm.Partition
		for _, p := range partitions {
			if p.Name == partitionFilter {
				filtered = append(filtered, p)
				break
			}
		}
		partitions = filtered
	}

	nodeSet := make(map[string][]string)
	for _, p := range partitions {
		for _, n := range p.Nodes {
			nodeSet[n] = append(nodeSet[n], p.Name)
		}
	}

	activeExecs, err := h.store.ListActiveExecutions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list active executions: " + err.Error()})
		return
	}
	activeByNode := make(map[string]*domain.Execution)
	for _, e := range activeExecs {
		if e.NodeName != "" {
			activeByNode[e.NodeName] = e
		}
	}

	enrichments := make(map[string]*inventoryEnrichment, len(nodeSet))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for nodeName := range nodeSet {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			e := &inventoryEnrichment{}

			slurmState, _ := h.slurmClient.GetNodeState(ctx, name)
			e.slurmState = slurmState

			svc, err1 := h.osClient.GetComputeService(ctx, name)
			if err1 != nil {
				slog.Error("GetComputeService error", "node", name, "error", err1)
			}
			e.osService = svc

			instances, err2 := h.osClient.ListInstances(ctx, name)
			if err2 != nil {
				slog.Error("ListInstances error", "node", name, "error", err2)
			}
			e.instances = len(instances)

			migrations, err3 := h.osClient.ListActiveMigrations(ctx, name)
			if err3 != nil {
				slog.Error("ListActiveMigrations error", "node", name, "error", err3)
			}
			e.migrations = len(migrations)

			mu.Lock()
			enrichments[name] = e
			mu.Unlock()
		}(nodeName)
	}
	wg.Wait()

	allExecs, err := h.store.ListExecutions(ctx, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to list executions: " + err.Error()})
		return
	}
	lastExecByNode := make(map[string]*domain.Execution)
	for _, e := range allExecs {
		if e.NodeName == "" || e.OverallStatus == domain.OverallStatusActive {
			continue
		}
		if existing, ok := lastExecByNode[e.NodeName]; !ok || e.RequestedAt.After(existing.RequestedAt) {
			lastExecByNode[e.NodeName] = e
		}
	}

	now := time.Now().UTC()
	resp := InventoryResponse{
		GeneratedAt: now,
		Partitions:  make([]InventoryPartition, 0, len(partitions)),
		Nodes:       make([]InventoryNode, 0, len(nodeSet)),
	}

	for _, p := range partitions {
		resp.Partitions = append(resp.Partitions, InventoryPartition{
			Name:  p.Name,
			Nodes: p.Nodes,
		})
	}

	for nodeName, partitionNames := range nodeSet {
		enrichment := enrichments[nodeName]
		node := buildInventoryNode(ctx, nodeName, partitionNames, enrichment, activeByNode[nodeName], lastExecByNode[nodeName])
		resp.Nodes = append(resp.Nodes, node)
	}

	c.JSON(http.StatusOK, resp)
}

func buildInventoryNode(_ context.Context, nodeName string, partitions []string, enrichment *inventoryEnrichment, activeExec *domain.Execution, lastExec *domain.Execution) InventoryNode {
	node := InventoryNode{
		NodeName:   nodeName,
		Partitions: partitions,
	}

	if enrichment != nil && enrichment.slurmState != nil {
		node.Slurm = &InventoryNodeSlurm{
			State:       enrichment.slurmState.State,
			GRES:        enrichment.slurmState.GRES,
			RunningJobs: enrichment.slurmState.RunningJob,
		}
	}

	if enrichment != nil && enrichment.osService != nil {
		node.OpenStack = &InventoryNodeOpenStack{
			ComputeService: InventoryComputeService{
				Enabled: enrichment.osService.Enabled,
				Status:  enrichment.osService.Status,
				State:   enrichment.osService.State,
			},
			InstanceCount:        enrichment.instances,
			ActiveMigrationCount: enrichment.migrations,
		}
	}

	if activeExec != nil {
		node.Switch = &InventoryNodeSwitch{
			ActiveExecutionID: activeExec.ID,
			ActiveState:       string(activeExec.CurrentState),
		}
	}

	if lastExec != nil {
		node.LastExecution = &InventoryLastExecution{
			ID:            lastExec.ID,
			Direction:     string(lastExec.Direction),
			OverallStatus: string(lastExec.OverallStatus),
			RequestedAt:   lastExec.RequestedAt,
		}
	}

	node.Owner = classifyOwner(node.Slurm, node.OpenStack, activeExec)
	node.OwnerSource = "derived"
	if activeExec != nil {
		node.AvailableDirection = ""
	} else if node.Owner == "slurm" {
		node.AvailableDirection = "slurm_to_openstack"
	} else if node.Owner == "openstack" {
		node.AvailableDirection = "openstack_to_slurm"
	}

	return node
}

func classifyOwner(slurmInfo *InventoryNodeSlurm, osInfo *InventoryNodeOpenStack, activeExec *domain.Execution) string {
	if activeExec != nil {
		return "switching"
	}

	slurmAttached := false
	if slurmInfo != nil {
		disp := slurm.ClassifyAttachState(slurmInfo.State)
		slurmAttached = disp == slurm.AttachStateReady
	}

	osVisible := false
	if osInfo != nil {
		osVisible = osInfo.ComputeService.Enabled
	}

	if slurmAttached && !osVisible {
		return "slurm"
	}
	if osVisible && !slurmAttached {
		return "openstack"
	}
	if slurmAttached && osVisible {
		return "conflict"
	}
	return "unknown"
}
