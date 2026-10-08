package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ErrActivePlanExists is returned by a repository when inserting a Plan would
// violate the "one active Plan per Request" unique index.
var ErrActivePlanExists = errors.New("an active plan already exists for this request")

// ErrTxConflict marks a transaction aborted by a deadlock or serialization failure; the whole tx is safe to retry.
var ErrTxConflict = errors.New("transaction conflict, retry")

// Bounds keep the single transaction short (hundreds of statements at most).
const (
	maxPlanTreeTasks    = 100
	maxPlanTreePhases   = 50
	maxPlanTreeAttempts = 5
)

type PlanTreeTask struct {
	Title, Description, Type, PromptTemplate, AIContext string
	EstimatedHours                                      *float64
	Labels                                              []string
	// DependsOnIndices index sibling tasks (same phase, or Plan.Tasks).
	DependsOnIndices []int
}

type PlanTreePhase struct {
	Title, Description string
	Tasks              []PlanTreeTask
	// DependsOnPhaseIndices index CreatePlanTreeInput.Phases.
	DependsOnPhaseIndices []int
}

type CreatePlanTreeInput struct {
	RequestID, ProjectID, Title, Description, AIPlanJSON, CreatorID, SupersedesPlanID string
	Phases                                                                            []PlanTreePhase
	Tasks                                                                             []PlanTreeTask
}

type CreatePlanTreeResult struct {
	Plan          domain.Task
	Phases, Tasks []domain.Task
	AlreadyExists bool
}

// CreatePlanTree writes a whole Plan (phases, tasks, parent_child and depends_on
// edges) in one transaction, so a failure never leaves a partial tree. It is
// idempotent per request_id: a second call returns the existing active tree.
type CreatePlanTree struct {
	tx     TxRunner
	tasks  TaskRepository // pool-scoped: reads outside the transaction
	grants GrantRepository
}

func NewCreatePlanTree(tx TxRunner, tasks TaskRepository, grants GrantRepository) *CreatePlanTree {
	return &CreatePlanTree{tx: tx, tasks: tasks, grants: grants}
}

func (uc *CreatePlanTree) Execute(ctx context.Context, in CreatePlanTreeInput) (CreatePlanTreeResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return CreatePlanTreeResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if err := validatePlanTreeInput(in); err != nil {
		return CreatePlanTreeResult{}, err
	}

	var result CreatePlanTreeResult
	// Concurrent creates for one request can deadlock on the unique index (MySQL); the retry then sees the winner.
	for attempt := 1; ; attempt++ {
		result = CreatePlanTreeResult{}
		err = uc.runOnce(ctx, tenantID, in, &result)
		if !errors.Is(err, ErrTxConflict) || attempt >= maxPlanTreeAttempts {
			break
		}
		time.Sleep(time.Duration(attempt) * 25 * time.Millisecond)
	}
	if err != nil {
		if errors.Is(err, ErrActivePlanExists) {
			// Lost the race against a concurrent create: the winner's tree is the answer.
			return uc.reread(ctx, tenantID, in.RequestID)
		}
		return CreatePlanTreeResult{}, err
	}
	if result.AlreadyExists {
		return uc.reread(ctx, tenantID, in.RequestID)
	}

	uc.refresh(ctx, tenantID, &result)
	if in.CreatorID != "" && uc.grants != nil {
		if _, err := uc.grants.Grant(ctx, tenantID, domain.Grant{
			TaskID: result.Plan.ID, SubjectID: in.CreatorID, Level: domain.GrantLevelOwner, ApplyTree: true,
		}); err != nil {
			// Owner grants cannot join the tx (no GrantRepository in it); the creator stays owner via Task.OwnerID.
			slog.ErrorContext(ctx, "create_plan_tree: owner grant failed", slog.String("plan_id", result.Plan.ID), slog.Any("error", err))
		}
	}
	return result, nil
}

func (uc *CreatePlanTree) runOnce(ctx context.Context, tenantID string, in CreatePlanTreeInput, result *CreatePlanTreeResult) error {
	return uc.tx.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error {
		existing, found, err := findActivePlan(ctx, tasks, tenantID, in.RequestID)
		if err != nil {
			return err
		}
		if found {
			if in.SupersedesPlanID == "" {
				result.AlreadyExists = true
				return nil
			}
			if existing.ID != in.SupersedesPlanID {
				// A retry after a successful replan: the named plan is already cancelled and the new one is active.
				old, gerr := tasks.Get(ctx, tenantID, in.SupersedesPlanID)
				if gerr == nil && old.Status == domain.StatusCancelled && old.RequestID == in.RequestID {
					result.AlreadyExists = true
					return nil
				}
				return apperrors.New(apperrors.KindFailedPrecondition, "TASK_PLAN_SUPERSEDE_MISMATCH", "supersedes_plan_id is not the request's active plan", nil)
			}
			if err := cancelPlanTree(ctx, tasks, tenantID, existing); err != nil {
				return err
			}
		}
		return buildPlanTree(ctx, tasks, edges, tenantID, in, result)
	})
}

func planTreeInvalid(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "TASK_PLAN_TREE_INVALID", msg, nil)
}

func validatePlanTreeInput(in CreatePlanTreeInput) error {
	if in.RequestID == "" || in.ProjectID == "" || in.Title == "" {
		return planTreeInvalid("request_id, project_id and title are required")
	}
	if len(in.Phases) > 0 && len(in.Tasks) > 0 {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_PLAN_TREE_MIXED_CHILDREN", "a plan has phases or direct tasks, not both", nil)
	}
	if len(in.Phases) > maxPlanTreePhases {
		return planTreeInvalid("too many phases")
	}
	total := len(in.Tasks)
	if err := validateSiblingTasks(in.Tasks); err != nil {
		return err
	}
	for i, ph := range in.Phases {
		if ph.Title == "" {
			return planTreeInvalid("phase title is required")
		}
		total += len(ph.Tasks)
		if err := validateSiblingTasks(ph.Tasks); err != nil {
			return err
		}
		if err := validateIndices(ph.DependsOnPhaseIndices, i, len(in.Phases)); err != nil {
			return err
		}
	}
	if total > maxPlanTreeTasks {
		return planTreeInvalid("too many tasks")
	}
	return nil
}

func validateSiblingTasks(tasks []PlanTreeTask) error {
	for i, t := range tasks {
		if t.Title == "" {
			return planTreeInvalid("task title is required")
		}
		if err := validateIndices(t.DependsOnIndices, i, len(tasks)); err != nil {
			return err
		}
	}
	return nil
}

func validateIndices(idx []int, self, n int) error {
	for _, d := range idx {
		if d < 0 || d >= n || d == self {
			return planTreeInvalid("depends_on index out of range or self-referencing")
		}
	}
	return nil
}

// findActivePlan reads the request's non-cancelled Plan (at most one, by unique index).
func findActivePlan(ctx context.Context, tasks TaskRepository, tenantID, requestID string) (domain.Task, bool, error) {
	plans, _, err := tasks.List(ctx, tenantID, ListFilter{TaskTypes: []string{domain.TypePlan}, RequestIDs: []string{requestID}, PageSize: 50})
	if err != nil {
		return domain.Task{}, false, apperrors.New(apperrors.KindInternal, "TASK_PLAN_TREE_FAILED", "failed to look up the active plan", err)
	}
	for _, p := range plans {
		if p.Status != domain.StatusCancelled {
			return p, true, nil
		}
	}
	return domain.Task{}, false, nil
}

// cancelPlanTree cancels the old Plan and every unfinished descendant; a running task blocks the replan.
func cancelPlanTree(ctx context.Context, tasks TaskRepository, tenantID string, plan domain.Task) error {
	nodes, _, err := tasks.GetSubtree(ctx, tenantID, plan.ID, domain.DefaultMaxAncestorDepth)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_PLAN_TREE_FAILED", "failed to read the plan being superseded", err)
	}
	for _, n := range nodes {
		if !domain.IsContainerType(n.Type) && n.Status == domain.StatusInProgress {
			return apperrors.New(apperrors.KindFailedPrecondition, "TASK_PLAN_HAS_RUNNING_TASKS", "the plan has running tasks and cannot be superseded", nil)
		}
	}
	for _, n := range nodes {
		if n.Status == domain.StatusDone || n.Status == domain.StatusCancelled {
			continue
		}
		if domain.IsContainerType(n.Type) {
			continue // containers last, once their children are settled
		}
		if err := tasks.UpdateStatus(ctx, tenantID, n.ID, domain.StatusCancelled); err != nil {
			return apperrors.New(apperrors.KindInternal, "TASK_PLAN_TREE_FAILED", "failed to cancel a superseded task", err)
		}
	}
	for _, n := range nodes {
		if !domain.IsContainerType(n.Type) || n.Status == domain.StatusCancelled {
			continue
		}
		// A done phase stays done, but the plan itself must leave the active-plan unique index.
		if n.Status == domain.StatusDone && n.ID != plan.ID {
			continue
		}
		changed, err := tasks.UpdateContainerStatus(ctx, tenantID, n.ID, n.Status, domain.StatusCancelled, nil)
		if err != nil || !changed {
			return apperrors.New(apperrors.KindInternal, "TASK_PLAN_TREE_FAILED", "failed to cancel a superseded container", err)
		}
	}
	return nil
}

func buildPlanTree(ctx context.Context, tasks TaskRepository, edges EdgeRepository, tenantID string, in CreatePlanTreeInput, out *CreatePlanTreeResult) error {
	createTask := NewCreateTask(tasks, nil) // no owner grant inside the tx; granted after commit
	fail := func(msg string, err error) error {
		var ae *apperrors.AppError
		if errors.As(err, &ae) && ae.Kind != apperrors.KindInternal {
			return err
		}
		return apperrors.New(apperrors.KindInternal, "TASK_PLAN_TREE_FAILED", msg, err)
	}
	link := func(parentID, childID string) error {
		edge, err := domain.NewTaskEdge(parentID, childID, domain.EdgeKindParentChild)
		if err != nil {
			return fail("invalid parent edge", err)
		}
		return addEdgeWithinTx(ctx, tenantID, tasks, edges, edge)
	}
	newTask := func(parentID string, t PlanTreeTask, typ string) (domain.Task, error) {
		created, err := createTask.Execute(ctx, CreateTaskInput{
			Title: t.Title, Description: t.Description, ParentID: parentID, ProjectID: in.ProjectID,
			Type: typ, EstimatedHours: t.EstimatedHours,
			PromptTemplate: t.PromptTemplate, AIContext: t.AIContext, Labels: t.Labels, RequestID: in.RequestID,
		})
		if err != nil {
			return domain.Task{}, fail("failed to create a task", err)
		}
		if err := link(parentID, created.ID); err != nil {
			return domain.Task{}, err
		}
		return created, nil
	}
	// createSiblings creates one sibling group, then its depends_on edges (second pass, ids needed).
	createSiblings := func(parentID string, group []PlanTreeTask) error {
		ids := make([]string, len(group))
		for i, t := range group {
			created, err := newTask(parentID, t, normalizeProposalType(t.Type))
			if err != nil {
				return err
			}
			ids[i] = created.ID
			out.Tasks = append(out.Tasks, created)
		}
		for i, t := range group {
			for _, d := range t.DependsOnIndices {
				if _, err := NewAddEdge(tasks, edges).Execute(ctx, AddEdgeInput{FromTaskID: ids[i], ToTaskID: ids[d], Kind: domain.EdgeKindDependsOn}); err != nil {
					return err
				}
			}
		}
		return nil
	}

	plan, err := createTask.Execute(ctx, CreateTaskInput{
		Title: in.Title, Description: in.Description, ProjectID: in.ProjectID, Type: domain.TypePlan, RequestID: in.RequestID,
	})
	if err != nil {
		return fail("failed to create the plan", err)
	}
	out.Plan = plan

	phaseIDs := make([]string, len(in.Phases))
	for i, ph := range in.Phases {
		phase, err := newTask(plan.ID, PlanTreeTask{Title: ph.Title, Description: ph.Description}, domain.TypePhase)
		if err != nil {
			return err
		}
		phaseIDs[i] = phase.ID
		out.Phases = append(out.Phases, phase)
		if err := createSiblings(phase.ID, ph.Tasks); err != nil {
			return err
		}
	}
	for i, ph := range in.Phases {
		for _, d := range ph.DependsOnPhaseIndices {
			if _, err := NewAddEdge(tasks, edges).Execute(ctx, AddEdgeInput{FromTaskID: phaseIDs[i], ToTaskID: phaseIDs[d], Kind: domain.EdgeKindDependsOn}); err != nil {
				return err
			}
		}
	}
	if err := createSiblings(plan.ID, in.Tasks); err != nil {
		return err
	}
	if in.AIPlanJSON != "" {
		if err := tasks.UpdateAIPlanJSON(ctx, tenantID, plan.ID, in.AIPlanJSON); err != nil {
			return fail("failed to persist ai_plan_json", err)
		}
	}
	return nil
}

// refresh swaps in the committed rows: AddEdge may have blocked tasks after they were created.
func (uc *CreatePlanTree) refresh(ctx context.Context, tenantID string, r *CreatePlanTreeResult) {
	nodes, _, err := uc.tasks.GetSubtree(ctx, tenantID, r.Plan.ID, domain.DefaultMaxAncestorDepth)
	if err != nil {
		slog.WarnContext(ctx, "create_plan_tree: could not re-read committed tree", slog.String("plan_id", r.Plan.ID), slog.Any("error", err))
		return
	}
	byID := make(map[string]domain.Task, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	if p, ok := byID[r.Plan.ID]; ok {
		r.Plan = p
	}
	for i, t := range r.Phases {
		if n, ok := byID[t.ID]; ok {
			r.Phases[i] = n
		}
	}
	for i, t := range r.Tasks {
		if n, ok := byID[t.ID]; ok {
			r.Tasks[i] = n
		}
	}
}

// reread returns the request's existing active tree, phases and tasks in tree order.
func (uc *CreatePlanTree) reread(ctx context.Context, tenantID, requestID string) (CreatePlanTreeResult, error) {
	plan, found, err := findActivePlan(ctx, uc.tasks, tenantID, requestID)
	if err != nil {
		return CreatePlanTreeResult{}, err
	}
	if !found {
		return CreatePlanTreeResult{}, apperrors.New(apperrors.KindInternal, "TASK_PLAN_TREE_FAILED", "active plan vanished after a conflict", nil)
	}
	nodes, _, err := uc.tasks.GetSubtree(ctx, tenantID, plan.ID, domain.DefaultMaxAncestorDepth)
	if err != nil {
		return CreatePlanTreeResult{}, apperrors.New(apperrors.KindInternal, "TASK_PLAN_TREE_FAILED", "failed to read the existing plan tree", err)
	}
	res := CreatePlanTreeResult{Plan: plan, AlreadyExists: true}
	for _, n := range nodes {
		switch {
		case n.ID == plan.ID:
			res.Plan = n
		case n.Type == domain.TypePhase:
			res.Phases = append(res.Phases, n)
		default:
			res.Tasks = append(res.Tasks, n)
		}
	}
	return res, nil
}
