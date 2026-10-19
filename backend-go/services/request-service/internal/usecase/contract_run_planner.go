package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// KnowledgeHubProbe says whether the project's dev server can give an agent the knowledge graph.
type KnowledgeHubProbe interface {
	Supports(ctx context.Context, projectID string) bool
}

// OpenSpecContractRuns puts the tasks of an OpenSpec plan on the contract path: the agent reports in a block only it can
// write (the nonce), may read the project's knowledge graph, and a task with a Verify step is believed done only with a
// passing check. Tasks of any other Request keep the plain run.
type OpenSpecContractRuns struct {
	Knowledge KnowledgeHubProbe
	NewNonce  func() string
}

var _ ContractRunPlanner = OpenSpecContractRuns{}

func (p OpenSpecContractRuns) Plan(ctx context.Context, req domain.Request, task domain.TaskView) (ContractRun, bool) {
	if req.SolutionEngine == nil || *req.SolutionEngine != domain.EngineOpenSpec {
		return ContractRun{}, false
	}
	nonce := randomNonce
	if p.NewNonce != nil {
		nonce = p.NewNonce
	}
	run := ContractRun{ResultNonce: nonce(), RequirePassingCheck: domain.HasVerifyStep(task.Description)}
	run.UseKnowledgeHub = p.Knowledge != nil && p.Knowledge.Supports(ctx, req.ProjectID)
	return run, true
}

// CapabilityKnowledgeProbe reads the agent's own report: an agent that does not advertise the knowledge hub gets none.
type CapabilityKnowledgeProbe struct {
	Conns AnalysisConnectionResolver
	Caps  DevServerCapabilityReader
}

func (p CapabilityKnowledgeProbe) Supports(ctx context.Context, projectID string) bool {
	if p.Conns == nil || p.Caps == nil {
		return false
	}
	conn, err := p.Conns.ResolveForProject(ctx, projectID)
	if err != nil || conn.RepoPath == "" {
		return false
	}
	profile, err := p.Caps.Get(ctx, DevServerRef{ConnectionID: conn.ConnectionID, DevServerID: conn.DevServerID}, false)
	return err == nil && domain.SupportsKnowledgeHub(profile)
}
