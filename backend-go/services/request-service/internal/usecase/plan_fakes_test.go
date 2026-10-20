package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// fakePlanWriter stands in for task-service: one Plan per Request, so a second call reports AlreadyExists like the real RPC.
type fakePlanWriter struct {
	mu    sync.Mutex
	calls []PlanTreeInput
	trees map[string]PlanTreeResult
	err   error
	// errFn, when set, decides per call whether task-service refuses.
	errFn func(PlanTreeInput) error
}

func newFakePlanWriter() *fakePlanWriter { return &fakePlanWriter{trees: map[string]PlanTreeResult{}} }

func (f *fakePlanWriter) CreatePlanTree(_ context.Context, in PlanTreeInput) (PlanTreeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	if f.err != nil {
		return PlanTreeResult{}, f.err
	}
	if f.errFn != nil {
		if err := f.errFn(in); err != nil {
			return PlanTreeResult{}, err
		}
	}
	if t, ok := f.trees[in.Request.ID]; ok && in.SupersedesPlanID == "" {
		t.AlreadyExists = true
		return t, nil
	}
	t := PlanTreeResult{PlanTaskID: uuid.NewString()}
	for range in.Proposal.Phases {
		t.PhaseTaskIDs = append(t.PhaseTaskIDs, uuid.NewString())
	}
	n := len(in.Proposal.Tasks)
	for _, ph := range in.Proposal.Phases {
		n += len(ph.Tasks)
	}
	for i := 0; i < n; i++ {
		t.TaskIDs = append(t.TaskIDs, uuid.NewString())
	}
	f.trees[in.Request.ID] = t
	return t, nil
}

func (f *fakePlanWriter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// planApprovals records OpenApproval calls and can be made to fail.
type planApprovals struct {
	w      *solWorld
	opened []OpenApprovalInput
	err    error
}

func (a *planApprovals) Execute(ctx context.Context, in OpenApprovalInput) (*domain.Approval, error) {
	if a.err != nil {
		return nil, a.err
	}
	req, err := a.w.Get(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if !domain.ApprovalAllowedInStatus(in.SubjectType, req.Status) {
		return nil, domain.ErrApprovalStageMismatch
	}
	a.opened = append(a.opened, in)
	return &domain.Approval{ID: uuid.NewString(), RequestID: in.RequestID, SubjectType: in.SubjectType, SubjectID: in.SubjectID}, nil
}

type fakePolicies struct{ reg *domain.PolicyRegistry }

func (f fakePolicies) PolicyFor(t domain.RequestType) domain.TypePolicy { return f.reg.PolicyFor(t) }

// planJSON builds a model reply holding a valid Plan of the given shape.
func planJSON(phased bool, labels ...string) string {
	task := map[string]any{"title": "Implement cache", "description": "done when tests pass", "task_type": "task", "labels": labels}
	doc := map[string]any{"title": "Plan", "summary": "s"}
	if phased {
		doc["phases"] = []any{map[string]any{"title": "Phase 1", "tasks": []any{task}}}
	} else {
		doc["tasks"] = []any{task}
	}
	b, _ := json.Marshal(doc)
	return "Here is the plan:\n```json\n" + string(b) + "\n```"
}

func (w *solWorld) seedApprovedSolution(requestID string, kind domain.SolutionKind) domain.Solution {
	chosen := 0
	s := domain.Solution{
		ID: uuid.NewString(), TenantID: "t1", RequestID: requestID, Kind: kind, Status: domain.SolutionStatusApproved,
		OptionsJSON:  []byte(`{"schema_version":1,"options":[{"id":"opt-1","title":"Use a cache","summary":"Cache pages","approach":"Add an LRU"}]}`),
		ChosenOption: &chosen, Version: 2,
	}
	w.solutions[s.ID] = s
	return s
}

type planKit struct {
	w        *solWorld
	spawner  *capturingSpawner
	generate *GeneratePlan
	writer   *fakePlanWriter
	opener   *planApprovals
	commit   *CommitPlan
	// proposal is set by the openspec worker tests only.
	proposal *proposalFixtures
}

func newPlanKit(conns AnalysisConnectionResolver) *planKit {
	w := newSolWorld()
	k := &planKit{w: w, spawner: &capturingSpawner{}, writer: newFakePlanWriter(), opener: &planApprovals{w: w}}
	if conns == nil {
		conns = fakeConns{}
	}
	k.generate = NewGeneratePlan(GeneratePlanDeps{
		Requests: w, Solutions: fakeSolutions{w}, Runs: fakeRuns{w}, Conns: conns, Auth: NewPlanActorAuthorizer(allowAll{}, nil, false), Approvals: fakeCanceller{w},
		Transition: w.transitioner(), Tx: w, Spawner: k.spawner, LeaseOwner: testOwner,
	})
	k.commit = NewCommitPlan(CommitPlanDeps{
		Requests: w, Auth: NewPlanActorAuthorizer(allowAll{}, nil, false), Writer: k.writer, Policies: fakePolicies{domain.NewPolicyRegistry(domain.PolicyDeps{})},
		Approvals: k.opener, Transition: w.transitioner(), Tx: w, Outbox: w, Limits: domain.DefaultPlanLimits,
	})
	return k
}

func (k *planKit) worker(completer ProjectAICompleter) *RunPlanGeneration {
	return NewRunPlanGeneration(RunPlanGenerationDeps{
		Requests: k.w, Solutions: fakeSolutions{k.w}, Completer: completer, Policies: fakePolicies{domain.NewPolicyRegistry(domain.PolicyDeps{})},
		Runs: fakeRuns{k.w}, Tx: k.w, Limits: domain.DefaultPlanLimits, Owner: testOwner,
	})
}

func (k *planKit) startedRun(reqID string) domain.AnalysisRun {
	res, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: reqID})
	if err != nil {
		panic(fmt.Sprintf("generate: %v", err))
	}
	return k.w.runs[res.RunID]
}
