package usecase

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// CreateTaskInput mirrors the CreateTask RPC request 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable. TenantID is NOT part of
// this struct: it's pulled from context (common/tenant), never trusted from
// the request body, per architecture/05-data-architecture.md. ID is
// optional: the CreateTaskRequest proto message has no id field (the
// service owns ID assignment), but tests may supply one directly for
// determinism.
type CreateTaskInput struct {
	ID        string
	Title     string
	ParentID  string
	ProjectID string

	// The remaining fields are optional, client-settable widened fields
	// (TASK-TG-01-01's proto/domain widen) — AIApply (TASK-TG-02-05) is the
	// first caller to set any of them, carrying a SubtaskProposal's fields
	// straight through to the created subtask.
	Description    string
	Type           string
	Priority       string
	AssigneeID     string
	EstimatedHours *float64
	PromptTemplate string
	AIContext      string
	Visibility     string
	// CreatorID, when non-empty, mints an owner-level Grant for the caller
	// after the task is created (TASK-TG-003-02) — follows
	// ResolvePermissionRequest.UserID's existing convention of passing
	// caller identity explicitly on the wire; task-service has no
	// auth-context user-id extractor today. Task.OwnerID is set below as a
	// SEPARATE, informational/display-only field (BE-SOL-003 §1) — but see
	// ResolvePermission's owner-intrinsic short-circuit, which DOES read
	// OwnerID; this Grant row is nonetheless kept as the persisted,
	// individually-revocable counterpart the short-circuit itself isn't.
	CreatorID string
}

// CreateTask persists a new task and, when CreatorID is set, best-effort
// grants that caller GrantLevelOwner on it (TASK-TG-003-02) — see
// CreateTaskInput.CreatorID's doc comment for why this is a real Grant row,
// not Task.OwnerID. grants may be nil (see NewCreateTask's doc comment for
// when/why) — Execute skips the grant step entirely in that case, the same
// as when CreatorID is empty.
type CreateTask struct {
	repo   TaskRepository
	grants GrantRepository
}

// NewCreateTask's grants parameter may be nil — AIApply's RunInTx closure
// (internal/usecase/ai_apply.go) constructs CreateTask with a nil
// GrantRepository rather than threading a transaction-scoped one through
// usecase.TxRunner's fn signature (which only carries TaskRepository/
// EdgeRepository today, per ports.go's TxRunner doc comment). This is a
// deliberate scope choice (option (b) in this task's own Context section):
// AI-generated subtasks have no human "creator" to owner-grant, so AIApply
// never sets CreatorID either — Execute's nil-grants + empty-CreatorID
// guard means this is simply never reached from that call site, not a
// latent nil-pointer risk.
func NewCreateTask(repo TaskRepository, grants GrantRepository) *CreateTask {
	return &CreateTask{repo: repo, grants: grants}
}

func (uc *CreateTask) Execute(ctx context.Context, in CreateTaskInput) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	id := in.ID
	if id == "" {
		id = uuid.NewString()
	}
	task, err := domain.NewTask(id, tenantID, in.Title, domain.StatusOpen, in.ParentID, in.ProjectID)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_INVALID", err.Error(), err)
	}
	// OwnerID = the creating user — see TASK-TG-03-01's "first-caller
	// bootstrap" note: without this, Grant's new manage-access check
	// (grant.go) would lock every new task's creator out of granting
	// anyone else access, since a brand-new task has no grant rows at all
	// yet. ResolvePermission's owner-intrinsic short-circuit
	// (resolve_permission.go) treats this field as an implicit Owner grant
	// with zero stored rows.
	callerID, _ := tenant.UserID(ctx)
	task.OwnerID = callerID
	// Widened fields are set directly (not part of NewTask's signature,
	// which stays backward compatible per TASK-TG-01-03) — every one of
	// them is optional, zero-value-valid.
	task.Description = in.Description
	task.Type = in.Type
	task.Priority = in.Priority
	task.AssigneeID = in.AssigneeID
	task.EstimatedHours = in.EstimatedHours
	task.PromptTemplate = in.PromptTemplate
	task.AIContext = in.AIContext
	task.Visibility = in.Visibility

	if task.ParentID != "" {
		if _, err := uc.repo.Get(ctx, tenantID, task.ParentID); err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_PARENT_NOT_FOUND", "parent task does not exist", err)
		}
	}

	created, err := uc.repo.Create(ctx, task)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_CREATE_FAILED", "failed to persist task", err)
	}

	if in.CreatorID != "" && uc.grants != nil {
		if _, err := uc.grants.Grant(ctx, tenantID, domain.Grant{
			TaskID: created.ID, SubjectID: in.CreatorID, Level: domain.GrantLevelOwner, ApplyTree: true,
		}); err != nil {
			// Best-effort: a failed owner-grant insert must not fail task
			// creation outright in v1 — log and continue. A task with no
			// owner grant still resolves via any OTHER matching grant
			// (e.g. a parent's inherited grant); it just has no intrinsic
			// owner until re-granted. Not wrapped in one transaction with
			// the Create call above — see this task's "Not in scope" note.
			slog.ErrorContext(ctx, "create_task: failed to insert owner grant", slog.String("task_id", created.ID), slog.String("creator_id", in.CreatorID), slog.Any("error", err))
		}
	}
	return created, nil
}
