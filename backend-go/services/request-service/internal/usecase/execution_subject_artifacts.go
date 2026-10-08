package usecase

import (
	"context"
	"sort"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PhaseArtifacts backs the "phase" approval subject with task-service. Describe runs inside the approval
// transaction (the Request row is locked) and makes one task-service call; it has no write side effect.
type PhaseArtifacts struct {
	Tasks TaskClient
}

var _ SubjectArtifacts = (*PhaseArtifacts)(nil)

// Describe digests the Phase's children and the depends_on edges among them, so a Phase edited after the
// approval opened no longer matches what the approver saw.
func (a *PhaseArtifacts) Describe(ctx context.Context, req domain.Request, _ domain.SubjectType, hintedID string) (string, string, error) {
	if hintedID == "" {
		return "", "", domain.ErrPhaseNotInPlan("")
	}
	sub, err := a.Tasks.GetSubtree(ctx, hintedID)
	if err != nil {
		return "", "", domain.ErrExecutionTaskServiceUnavailable(err)
	}
	root, ok := findTask(sub.Tasks, hintedID)
	if !ok || root.Type != domain.TaskTypePhase || root.RequestID != req.ID {
		return "", "", domain.ErrPhaseNotInPlan(hintedID)
	}
	return hintedID, domain.SubjectDigest(domain.SubjectPhase, subtreeDigestParts(sub, hintedID)...), nil
}

func (a *PhaseArtifacts) Decided(context.Context, domain.Approval, bool) error  { return nil }
func (a *PhaseArtifacts) Closed(context.Context, domain.Approval, string) error { return nil }

// PreDeployArtifacts backs "pre_deploy": the single fix task of a hotfix, the Plan of a security request, or
// one irreversible task of an ops_request.
type PreDeployArtifacts struct {
	Tasks TaskClient
}

var _ SubjectArtifacts = (*PreDeployArtifacts)(nil)

func (a *PreDeployArtifacts) Describe(ctx context.Context, req domain.Request, _ domain.SubjectType, hintedID string) (string, string, error) {
	tasks, err := a.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if err != nil {
		return "", "", domain.ErrExecutionTaskServiceUnavailable(err)
	}
	tree := domain.BuildExecutionTree(tasks)
	switch req.Type {
	case domain.RequestTypeHotfix:
		return a.describeHotfix(req, tree, hintedID)
	case domain.RequestTypeSecurity:
		return a.describePlan(ctx, req, tree, hintedID)
	case domain.RequestTypeOpsRequest:
		return a.describeOpsStep(req, tree, hintedID)
	}
	return "", "", domain.ErrApprovalSubjectTypeNotAllowed
}

func (a *PreDeployArtifacts) describeHotfix(req domain.Request, tree domain.ExecutionTree, hintedID string) (string, string, error) {
	var fix *domain.TaskView
	for i := range tree.Leaves {
		l := tree.Leaves[i]
		if l.ParentID != "" || l.Status == domain.TaskStatusCancelled || (hintedID != "" && l.ID != hintedID) {
			continue
		}
		if fix != nil {
			return "", "", domain.ErrHotfixPlanShape("a hotfix has more than one fix task")
		}
		fix = &l
	}
	if fix == nil {
		return "", "", domain.ErrPhaseNotInPlan(hintedID)
	}
	return fix.ID, taskDigest(*fix), nil
}

func (a *PreDeployArtifacts) describePlan(ctx context.Context, req domain.Request, tree domain.ExecutionTree, hintedID string) (string, string, error) {
	if tree.Plan == nil || tree.Plan.Status == domain.TaskStatusCancelled || (hintedID != "" && tree.Plan.ID != hintedID) {
		return "", "", domain.ErrPhaseNotInPlan(hintedID)
	}
	sub, err := a.Tasks.GetSubtree(ctx, tree.Plan.ID)
	if err != nil {
		return "", "", domain.ErrExecutionTaskServiceUnavailable(err)
	}
	return tree.Plan.ID, domain.SubjectDigest(domain.SubjectPreDeploy, subtreeDigestParts(sub, tree.Plan.ID)...), nil
}

func (a *PreDeployArtifacts) describeOpsStep(req domain.Request, tree domain.ExecutionTree, hintedID string) (string, string, error) {
	if hintedID == "" {
		return "", "", domain.ErrPreDeployRequired("")
	}
	for _, l := range tree.Leaves {
		if l.ID == hintedID && l.Status != domain.TaskStatusCancelled && l.Ref().HasLabel(domain.PolicyLabelGatePreDeploy) {
			return l.ID, taskDigest(l), nil
		}
	}
	return "", "", domain.ErrPreDeployRequired(hintedID)
}

func (a *PreDeployArtifacts) Decided(context.Context, domain.Approval, bool) error  { return nil }
func (a *PreDeployArtifacts) Closed(context.Context, domain.Approval, string) error { return nil }

func findTask(tasks []domain.TaskView, id string) (domain.TaskView, bool) {
	for _, t := range tasks {
		if t.ID == id {
			return t, true
		}
	}
	return domain.TaskView{}, false
}

func taskDigest(t domain.TaskView) string {
	labels := append([]string(nil), t.Labels...)
	sort.Strings(labels)
	return domain.SubjectDigest(domain.SubjectPreDeploy, t.ID, t.Title, strings.Join(labels, ","))
}

// subtreeDigestParts is canonical: the root's descendants by id with title and labels, then the edges between them.
func subtreeDigestParts(sub SubtreeView, rootID string) []string {
	var tasks []domain.TaskView
	in := map[string]bool{}
	for _, t := range sub.Tasks {
		if t.ID == rootID {
			continue
		}
		tasks = append(tasks, t)
		in[t.ID] = true
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	var parts []string
	for _, t := range tasks {
		labels := append([]string(nil), t.Labels...)
		sort.Strings(labels)
		parts = append(parts, "t:"+t.ID+"|"+t.Title+"|"+strings.Join(labels, ","))
	}
	var edges []string
	for _, e := range sub.DependsOn {
		if in[e.From] && in[e.To] {
			edges = append(edges, "e:"+e.From+">"+e.To)
		}
	}
	sort.Strings(edges)
	return append(parts, edges...)
}
