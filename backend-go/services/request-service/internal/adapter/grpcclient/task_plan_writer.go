package grpcclient

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// taskPlanTimeout covers the one task-service transaction that writes up to a hundred tasks and their edges.
const taskPlanTimeout = 60 * time.Second

// maxAIContextRunes keeps the context handed to each task short: it is prepended to the agent's prompt.
const maxAIContextRunes = 2000

var _ usecase.TaskPlanWriter = (*TaskClient)(nil)

// CreatePlanTree writes the Plan through task-service's single-transaction RPC. A repeated call for the same
// Request returns the existing Plan with AlreadyExists set, which is what makes CommitPlan safe to retry.
func (c *TaskClient) CreatePlanTree(ctx context.Context, in usecase.PlanTreeInput) (usecase.PlanTreeResult, error) {
	ctx, cancel, err := callContext(ctx, true, taskPlanTimeout)
	if err != nil {
		return usecase.PlanTreeResult{}, err
	}
	defer cancel()
	resp, err := c.client.CreatePlanTree(ctx, planTreeRequest(in))
	if err != nil {
		return usecase.PlanTreeResult{}, planTreeError(err)
	}
	out := usecase.PlanTreeResult{PlanTaskID: resp.GetPlan().GetId(), AlreadyExists: resp.GetAlreadyExists()}
	for _, p := range resp.GetPhases() {
		out.PhaseTaskIDs = append(out.PhaseTaskIDs, p.GetId())
	}
	for _, t := range resp.GetTasks() {
		out.TaskIDs = append(out.TaskIDs, t.GetId())
	}
	if out.PlanTaskID == "" {
		return usecase.PlanTreeResult{}, fmt.Errorf("grpcclient: create plan tree: task-service returned no Plan")
	}
	return out, nil
}

func planTreeRequest(in usecase.PlanTreeInput) *taskv1.CreatePlanTreeRequest {
	req := &taskv1.CreatePlanTreeRequest{
		RequestId: in.Request.ID, ProjectId: in.Request.ProjectID, Title: in.Proposal.Title, Description: in.Proposal.Summary,
		AiPlanJson: in.AIPlanJSON, CreatorId: in.CreatorID, SupersedesPlanId: in.SupersedesPlanID,
	}
	aiContext := planTaskContext(in)
	for _, ph := range in.Proposal.Phases {
		p := &taskv1.PlanTreePhase{Title: ph.Title, Description: ph.Description}
		for _, i := range ph.DependsOnPhaseIndices {
			p.DependsOnPhaseIndices = append(p.DependsOnPhaseIndices, int32(i))
		}
		for _, t := range ph.Tasks {
			p.Tasks = append(p.Tasks, planTreeTask(t, aiContext))
		}
		req.Phases = append(req.Phases, p)
	}
	for _, t := range in.Proposal.Tasks {
		req.Tasks = append(req.Tasks, planTreeTask(t, aiContext))
	}
	return req
}

func planTreeTask(t domain.TaskProposal, aiContext string) *taskv1.PlanTreeTask {
	out := &taskv1.PlanTreeTask{
		Title: t.Title, Description: t.Description, TaskType: t.TaskType, PromptTemplate: t.PromptTemplate, AiContext: aiContext,
		Labels: domain.EffectiveLabels(t),
	}
	if t.EstimatedHours > 0 {
		out.EstimatedHours = wrapperspb.Double(t.EstimatedHours)
	}
	for _, i := range t.DependsOnIndices {
		out.DependsOnIndices = append(out.DependsOnIndices, int32(i))
	}
	return out
}

// planTaskContext is what every task of the Plan tells the agent about its origin: which Request, and the Plan's own summary.
func planTaskContext(in usecase.PlanTreeInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "REQ-%d: %s", in.Request.Number, in.Request.Title)
	if s := strings.TrimSpace(in.Proposal.Summary); s != "" {
		b.WriteString("\n" + s)
	}
	out := b.String()
	if utf8.RuneCountInString(out) > maxAIContextRunes {
		out = string([]rune(out)[:maxAIContextRunes])
	}
	return out
}

// planTreeError keeps task-service's refusal reasons that a person can act on, and reports an outage as one.
func planTreeError(err error) error {
	switch code := taskErrorCode(err); {
	case code == "TASK_PLAN_HAS_RUNNING_TASKS":
		return domain.ErrPlanHasRunningTasks()
	case code == "TASK_PLAN_SUPERSEDE_MISMATCH":
		return domain.ErrPlanSupersedeMismatch()
	case strings.HasPrefix(code, "TASK_PLAN_TREE_") && status.Code(err) == codes.InvalidArgument,
		strings.HasPrefix(code, "TASK_DEPENDENCY"), strings.HasPrefix(code, "TASK_HIERARCHY"):
		return domain.ErrPlanInvalidField(status.Convert(err).Message())
	}
	if c := status.Code(err); c == codes.Unavailable || c == codes.DeadlineExceeded {
		return domain.ErrPlanTaskServiceUnavailable(err)
	}
	return fmt.Errorf("grpcclient: create plan tree: %w", err)
}

// CreatePlanTree on the placeholder client reports the missing address like every other call.
func (UnavailableTaskClient) CreatePlanTree(context.Context, usecase.PlanTreeInput) (usecase.PlanTreeResult, error) {
	return usecase.PlanTreeResult{}, domain.ErrPlanTaskServiceUnavailable(errNoTaskService)
}

var _ usecase.TaskPlanWriter = UnavailableTaskClient{}

var _ usecase.FixTaskWriter = (*TaskClient)(nil)

// CreateFixTask creates the one parentless task of a hotfix. Idempotency lives in the use case (it looks the task up
// first), because task-service's plain CreateTask would happily create a second one.
func (c *TaskClient) CreateFixTask(ctx context.Context, in usecase.PlanTreeInput) (string, error) {
	if len(in.Proposal.Tasks) != 1 || len(in.Proposal.Phases) != 0 {
		return "", domain.ErrHotfixPlanShape("a hotfix creates exactly one task")
	}
	ctx, cancel, err := callContext(ctx, true, taskReadTimeout)
	if err != nil {
		return "", err
	}
	defer cancel()
	t := in.Proposal.Tasks[0]
	req := &taskv1.CreateTaskRequest{
		Title: t.Title, ProjectId: in.Request.ProjectID, Description: t.Description, TaskType: t.TaskType, PromptTemplate: t.PromptTemplate,
		AiContext: planTaskContext(in), CreatorId: in.CreatorID, RequestId: in.Request.ID, Labels: domain.EffectiveLabels(t),
	}
	if t.EstimatedHours > 0 {
		req.EstimatedHours = wrapperspb.Double(t.EstimatedHours)
	}
	resp, err := c.client.CreateTask(ctx, req)
	if err != nil {
		if c := status.Code(err); c == codes.Unavailable || c == codes.DeadlineExceeded {
			return "", domain.ErrHotfixFixTaskUnavailable(err)
		}
		return "", fmt.Errorf("grpcclient: create hotfix fix task: %w", err)
	}
	if resp.GetTask().GetId() == "" {
		return "", fmt.Errorf("grpcclient: create hotfix fix task: task-service returned no task")
	}
	return resp.GetTask().GetId(), nil
}

func (UnavailableTaskClient) CreateFixTask(context.Context, usecase.PlanTreeInput) (string, error) {
	return "", domain.ErrHotfixFixTaskUnavailable(errNoTaskService)
}

var _ usecase.FixTaskWriter = UnavailableTaskClient{}
