package domain

import (
	"context"
	"strings"
)

// opsRequestPolicy blocks inside executing: a task labelled gate:pre_deploy waits for its own Approval.
type opsRequestPolicy struct {
	approvals ApprovalLookup
}

func (opsRequestPolicy) PlanPreconditions(_ context.Context, _ Request, p PlanProposal) error {
	return CheckRunbook(p)
}

// PreExecutionGate reads the label at dispatch time because UpdateTask can rewrite it.
func (p opsRequestPolicy) PreExecutionGate(ctx context.Context, req Request, task TaskRef) (*GateRequirement, error) {
	if !task.HasLabel(PolicyLabelGatePreDeploy) {
		return nil, nil
	}
	if p.approvals != nil {
		a, err := p.approvals.LatestApproval(ctx, req.ID, SubjectPreDeploy, task.ID)
		if err != nil {
			return nil, err
		}
		if a != nil && a.Status == ApprovalStatusApproved {
			return nil, nil
		}
	}
	return &GateRequirement{SubjectType: SubjectPreDeploy, SubjectID: task.ID, Stage: "task"}, nil
}

func (opsRequestPolicy) CompletionChecks(_ Request, checks []RequestCheck) ([]CheckVerdict, error) {
	return []CheckVerdict{humanConfirmedVerdict(checks, CheckOpsResult, "kết quả vận hành (ops_result)")}, nil
}

func (opsRequestPolicy) OnCompleted(Request) ([]FollowUp, error) { return nil, nil }

// FailureHint names the rollback tasks that run after the failed step, so the person who gets the Request back
// can decide on the rollback; Orca never rolls back by itself.
func (opsRequestPolicy) FailureHint(_ Request, failed TaskRef, all []TaskRef) string {
	var names []string
	for _, t := range all {
		if t.HasLabel(PolicyLabelRollback) && t.ID != failed.ID && dependsOnTransitively(all, t.ID, failed.ID) {
			names = append(names, t.Title)
		}
	}
	if len(names) == 0 {
		for _, t := range all {
			if t.HasLabel(PolicyLabelRollback) {
				names = append(names, t.Title)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "Task rollback liên quan: " + strings.Join(names, ", ")
}

func dependsOnTransitively(all []TaskRef, id, target string) bool {
	byID := make(map[string]TaskRef, len(all))
	for _, t := range all {
		byID[t.ID] = t
	}
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(cur string) bool {
		if seen[cur] {
			return false
		}
		seen[cur] = true
		for _, d := range byID[cur].DependsOn {
			if d == target || walk(d) {
				return true
			}
		}
		return false
	}
	return walk(id)
}
