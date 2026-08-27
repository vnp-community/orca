# TASK-PW-04-03: `UpdateTask` enqueues `orca.task.task.statuschanged`; `FindTaskByNumber` + `task_number` assignment

**From Solution:** SOL-PW-04
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/update_task.go`, `backend-go/services/task-service/internal/usecase/find_task_by_number.go` (new), `backend-go/services/task-service/internal/usecase/create_task.go`
**Depends on:** TASK-PW-04-02
**Status:** `[ ]` TODO

---

## Context

**Grounding correction versus SOL-PW-04's own prose**:
`notification-service` already has a real, wired `SubjectBinding` for
`{StreamName: "TASK", Subject: "orca.task.task.completed"}` and a
`subjectRules["orca.task.task.completed"]` translation rule
(`services/notification-service/internal/adapter/eventbus/consumer.go:45`,
`internal/domain/notification_event.go` — verified present, not assumed).
There is **no existing binding/rule for `orca.task.task.statuschanged`**.
To get both "the api-gateway workspace bridge needs every transition" and
"notification-service's already-built `task.completed` consumer starts
receiving real events without needing its own code changed", this task's
`UpdateTask` publishes **two** subjects from the same enqueue point:
`orca.task.task.statuschanged` always (on any status transition), and
additionally `orca.task.task.completed` only when the new status is
`domain.StatusDone`.

## Changes to make

`internal/usecase/update_task.go` — extend `Execute` to build an
`OutboxEvent` on a genuine status transition and pass it to the extended
`Update` port (TASK-PW-04-02):

```go
func (uc *UpdateTask) Execute(ctx context.Context, in UpdateTaskInput) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.ID == "" {
		return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_ID", "id is required", nil)
	}

	current, err := uc.repo.Get(ctx, tenantID, in.ID)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	previousStatus := current.Status
	if in.Title != nil {
		current.Title = *in.Title
	}
	if in.PRURL != nil {
		current.PRURL = *in.PRURL
	}
	if in.WorktreeID != nil {
		current.WorktreeID = *in.WorktreeID
	}
	if in.Status != nil {
		updated, err := current.SetStatus(*in.Status)
		if err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_INVALID_STATUS_TRANSITION", err.Error(), err)
		}
		current = updated
	}

	var events []domain.OutboxEvent
	if in.Status != nil && *in.Status != previousStatus {
		payload, err := json.Marshal(taskStatusChangedPayload{
			TaskID: current.ID, ProjectID: current.ProjectID, WorktreeID: current.WorktreeID,
			PreviousStatus: previousStatus, NewStatus: current.Status,
		})
		if err != nil {
			return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_MARSHAL_EVENT_FAILED", "failed to marshal status-changed event payload", err)
		}
		events = append(events, domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.task.task.statuschanged", OccurredAt: time.Now().UTC(), PayloadJSON: payload})
		if current.Status == domain.StatusDone {
			// Feeds notification-service's ALREADY-WIRED
			// "orca.task.task.completed" consumer/rule — verified present
			// in consumer.go/notification_event.go before assuming this
			// subject name; do not rename it to statuschanged's shape.
			events = append(events, domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.task.task.completed", OccurredAt: time.Now().UTC(), PayloadJSON: payload})
		}
	}

	if err := uc.repo.Update(ctx, tenantID, current, events); err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_UPDATE_FAILED", "failed to persist update", err)
	}
	return current, nil
}

type taskStatusChangedPayload struct {
	TaskID         string `json:"task_id"`
	ProjectID      string `json:"project_id"`
	WorktreeID     string `json:"worktree_id"`
	PreviousStatus string `json:"previous_status"`
	NewStatus      string `json:"new_status"`
}
```

`Update`'s signature in TASK-PW-04-02 took a single `*domain.OutboxEvent`;
this task needs up to two events from one call. Change that port's
signature to `events []domain.OutboxEvent` (nil/empty = none) instead —
flag this adjustment back onto TASK-PW-04-02's repository implementation
(loop over `events` inside the same transaction rather than a single
conditional insert) rather than silently diverging from what that task
shipped.

New `internal/usecase/find_task_by_number.go`:

```go
package usecase

type FindTaskByNumberInput struct {
	ProjectID  string
	TaskNumber int64
}

type FindTaskByNumber struct {
	repo TaskRepository
}

func NewFindTaskByNumber(repo TaskRepository) *FindTaskByNumber {
	return &FindTaskByNumber{repo: repo}
}

func (uc *FindTaskByNumber) Execute(ctx context.Context, in FindTaskByNumberInput) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.ProjectID == "" {
		return domain.Task{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_PROJECT_ID", "project_id is required", nil)
	}
	task, err := uc.repo.FindByNumber(ctx, tenantID, in.ProjectID, in.TaskNumber)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND_BY_NUMBER", "no task with this number in this project", err)
	}
	return task, nil
}
```

Add `TaskRepository.FindByNumber(ctx, tenantID, projectID string,
taskNumber int64) (domain.Task, error)` to `ports.go`, implemented against
`idx_tasks_project_task_number` (`WHERE tenant_id = $1 AND project_id =
$2 AND task_number = $3`) in `internal/adapter/postgres`.

In `internal/usecase/create_task.go`: assign `task_number` via
`nextval('task.task_number_seq')` inside the existing `Create` INSERT
(e.g. `INSERT INTO task.tasks (..., task_number) VALUES (..., nextval('task.task_number_seq'))`
in `internal/adapter/postgres/repository.go`'s `Create`, returning the
assigned value via `RETURNING task_number` so the usecase's response
`Task` carries it).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run 'TestUpdateTask|TestFindTaskByNumber|TestCreateTask' -v
```

Expected: a status-changing update enqueues exactly one
`orca.task.task.statuschanged` event; a transition into `done` additionally
enqueues `orca.task.task.completed`; a title-only update enqueues none;
`FindTaskByNumber` resolves within a project and returns NOT_FOUND across a
different project's matching number (regression guard for project-scoping);
`CreateTask` assigns a non-zero `task_number`.
