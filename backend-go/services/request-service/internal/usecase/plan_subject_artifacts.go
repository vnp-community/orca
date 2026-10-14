package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanArtifacts backs the "plan" and "task_list" approvals. The approver is shown the Plan task and everything
// under it, so Describe digests that subtree (titles, labels and depends_on edges): a Plan edited after the
// approval opened no longer matches what the approver saw and the decision is refused.
type PlanArtifacts struct {
	Tasks TaskClient
	// Changes, when set, adds the authored OpenSpec change to what the approver is shown: a change edited or regenerated
	// after the approval opened no longer matches, exactly like an edited task.
	Changes OpenSpecChangeRepository
}

var _ SubjectArtifacts = (*PlanArtifacts)(nil)

func (a *PlanArtifacts) Describe(ctx context.Context, req domain.Request, st domain.SubjectType, hintedID string) (string, string, error) {
	planID := req.PlanTaskID
	if planID == "" {
		return "", "", domain.ErrPlanNotCommitted(req.ID)
	}
	if hintedID != "" && hintedID != planID {
		return "", "", domain.ErrPhaseNotInPlan(hintedID)
	}
	sub, err := a.Tasks.GetSubtree(ctx, planID)
	if err != nil {
		return "", "", domain.ErrExecutionTaskServiceUnavailable(err)
	}
	root, ok := findTask(sub.Tasks, planID)
	if !ok || root.Type != domain.TaskTypePlan || root.RequestID != req.ID || root.Status == domain.TaskStatusCancelled {
		return "", "", domain.ErrPhaseNotInPlan(planID)
	}
	parts := subtreeDigestParts(sub, planID)
	if a.Changes != nil {
		change, found, err := a.Changes.GetByRequest(ctx, req.ID)
		if err != nil {
			return "", "", fmt.Errorf("read the openspec change for the plan digest: %w", err)
		}
		if found && change.Status == domain.ChangeStatusReady {
			parts = append(parts, "b:"+change.TasksMDDigest+"|"+change.BundleDigest())
		}
	}
	return planID, domain.SubjectDigest(st, parts...), nil
}

func (a *PlanArtifacts) Decided(context.Context, domain.Approval, bool) error  { return nil }
func (a *PlanArtifacts) Closed(context.Context, domain.Approval, string) error { return nil }
