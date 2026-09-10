# TASK-TG-003-02: Owner grant on task creation (`CreateTask` gains `CreatorID` + best-effort `Grant` insert)

**From Solution:** BE-SOL-003
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/create_task.go`, `backend-go/services/task-service/internal/adapter/grpc/server.go` (`CreateTask` handler), `backend-go/proto/orca/task/v1/task.proto` (`CreateTaskRequest.creator_id`)
**Depends on:** TASK-TG-001-02 (no new `Task` field needed for this — `Task.OwnerID` is informational/display-only per BE-SOL-003 §1, NOT read by authorization; this task only needs `GrantRepository` wired into `CreateTask`, which already exists as a port — see Context)
**Status:** `[x]` DONE

---

## Context

Verified directly (`internal/usecase/create_task.go:1-62`, read in full):
`CreateTask` today takes `{ID, Title, ParentID, ProjectID}` and calls only
`uc.repo.Create` — no `GrantRepository` dependency, no `CreatorID` field.
`GrantRepository` already exists as a port (`internal/usecase/ports.go:72-79`:
`Grant(ctx, tenantID, grant domain.Grant) error`) and already has a real
usecase consumer, `usecase.Grant` (`internal/usecase/grant.go:1-50`, read in
full) — this task reuses that exact port, it does not add a new one.

Per BE-SOL-003 §1's correction: this is **not** an `OwnerID`-based
authorization short-circuit. `Task.OwnerID` (added by TASK-TG-001-01's
migration/TASK-TG-001-02's struct widening) is informational/display-only —
"who created this," shown in UI — and is never read by `ResolveGrant`.
Instead, `CreateTask` inserts a real `Grant{Level: GrantLevelOwner}` row for
the creator, reusing the grant model exactly as designed (`task_grant.rego`'s
`level_actions["owner"]`, confirmed at `backend-go/policy/orca-authz/task_grant.rego:25-31`
— NOT lines 16-24 as BE-SOL-003's own citation states; the actual
`level_actions` map starts at line 25, after a 16-line file-header comment
block — direct read confirms `{"owner": {"read", "write", "execute",
"admin"}, ...}` at lines 25-31).

`CreateTaskRequest` currently has 4 fields (`tenant_id=1, title=2,
parent_id=3, project_id=4` — confirmed, `task.proto:61-66`); `creator_id`
is the next free field number, `5`.

## Changes to make

**1. `internal/usecase/create_task.go`** — add `CreatorID`, best-effort
grant insert:

```go
type CreateTaskInput struct {
	ID        string
	Title     string
	ParentID  string
	ProjectID string
	CreatorID string // NEW — follows ResolvePermissionRequest.UserID's existing
	                  // convention of passing caller identity explicitly on the
	                  // wire; task-service has no auth-context user-id
	                  // extractor today (confirmed, 0 hits for a
	                  // RequireUserID-style helper in common/).
}

type CreateTask struct {
	repo   TaskRepository
	grants GrantRepository // NEW
}

func NewCreateTask(repo TaskRepository, grants GrantRepository) *CreateTask {
	return &CreateTask{repo: repo, grants: grants}
}

func (uc *CreateTask) Execute(ctx context.Context, in CreateTaskInput) (domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	// ...unchanged validation + uc.repo.Create(...)...

	created, err := uc.repo.Create(ctx, task)
	if err != nil {
		return domain.Task{}, apperrors.New(apperrors.KindInternal, "TASK_CREATE_FAILED", "failed to persist task", err)
	}

	if in.CreatorID != "" {
		if err := uc.grants.Grant(ctx, tenantID, domain.Grant{
			TaskID: created.ID, SubjectID: in.CreatorID, Level: domain.GrantLevelOwner, ApplyTree: true,
		}); err != nil {
			// Best-effort: a failed owner-grant insert must not fail task
			// creation outright in v1 — log and continue. A task with no
			// owner grant still resolves via any OTHER matching grant
			// (e.g. a parent's inherited grant); it just has no intrinsic
			// owner until re-granted. See "Not in scope" below for why
			// this isn't wrapped in one transaction with the Create call.
			log.Error("create_task: failed to insert owner grant", "taskID", created.ID, "creatorID", in.CreatorID, "err", err)
		}
	}
	return created, nil
}
```

**Grounding note — `NewCreateTask`'s signature change ripples further than
this snippet.** `NewCreateTask(repo)` is called from `AIApply`'s `RunInTx`
closure too (`internal/usecase/ai_apply.go:60`, per TASK-TG-001-04/
TASK-TG-002-02's citations: `createTask := NewCreateTask(tasks)`). Once
`NewCreateTask` takes a second `grants GrantRepository` parameter, that call
site needs a `GrantRepository` scoped the same way `tasks`/`edges` are
inside `RunInTx` — but `usecase.TxRunner.RunInTx`'s signature
(`ports.go:182-183`) only hands `fn` a `TaskRepository`/`EdgeRepository`
pair today, no `GrantRepository`. **Decide explicitly, don't default
silently**: either (a) widen `TxRunner.RunInTx`'s `fn` signature to also
carry a tx-scoped `GrantRepository` (touches `postgres.Repository.RunInTx`
too, since it would need to implement `GrantRepository` inside the
transaction as well — check whether `Repository` already does, since one
struct backs all these interfaces), or (b) give `AIApply`'s call site an
untyped/nil `GrantRepository` (AI-generated subtasks arguably shouldn't
auto-grant their creator anyway, since there's no human "creator" in that
flow) by having `AIApply` construct `NewCreateTask(tasks, nil)` and making
`CreateTask.Execute` tolerate a nil `grants` port by skipping the grant
step entirely when `in.CreatorID == ""` (already true above) AND
`uc.grants == nil`. Option (b) is smaller and matches BE-SOL-003's own
framing that this is a human-task-creation feature, not a implicit
apply-a-batch feature — pick it unless product explicitly wants AI-created
subtasks owner-granted to whoever triggered `AIApply`.

**2. `internal/adapter/grpc/server.go`** — `CreateTask` handler
(`internal/adapter/grpc/server.go:70-81`, confirmed by direct read) passes
`req.GetCreatorId()` through:

```go
task, err := s.createTask.Execute(ctx, usecase.CreateTaskInput{
	Title: req.GetTitle(), ParentID: req.GetParentId(), ProjectID: req.GetProjectId(),
	CreatorID: req.GetCreatorId(), // NEW
})
```

**3. `task.proto`** — add `string creator_id = 5;` to `CreateTaskRequest`.

**4. `cmd/server/main.go`** — update `NewCreateTask` wiring to pass the
existing `GrantRepository` (the same `*postgres.Repository` instance
already implements it, per `usecase.Grant`'s existing wiring).

## Not in scope (per BE-SOL-003)

- Wrapping `CreateTask` + owner-grant insert in one transaction — needs
  `TxRunner` wiring `CreateTask` doesn't have today (see the grounding note
  above for why this is even more true after this task's own
  `NewCreateTask` signature change); tracked as a follow-up.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run "TestCreateTask|TestAIApply" -v
```

Expected: clean build; `CreateTask` with `CreatorID` set → creator
immediately resolves `GrantLevelOwner` via `ResolvePermission` (integration
test, not a special-cased assertion on `Task.OwnerID`); a grant-insert
failure (simulated via a fake `GrantRepository` returning an error) still
returns the created task successfully (best-effort); `AIApply`'s existing
tests still pass unchanged regardless of which option ((a) or (b) above)
was chosen for its `NewCreateTask` call site.

## Execution notes (2026-09-09)

Chose option (b) exactly as the task's own Context section recommends:
`AIApply`'s `RunInTx` closure now constructs `NewCreateTask(tasks, nil)`,
and `CreateTask.Execute`'s grant step is skipped whenever `in.CreatorID ==
""` OR `uc.grants == nil` — AIApply never sets `CreatorID`, so this is
never actually reached from that call site, not a latent nil-pointer risk
(covered by a dedicated regression test,
`TestCreateTask_NilGrantRepository_SkipsGrantStep`). Implemented
`CreateTaskInput.CreatorID`, the best-effort `Grant` insert (using
`log/slog`'s `slog.ErrorContext`, the exact logging convention already used
elsewhere in this codebase for "best-effort, log and continue" — e.g.
`workflow-service/internal/usecase/execute.go` — no prior logging call
existed in this specific package before this task), `task.proto`'s
`creator_id = 5` field (confirmed still the next-free number — field count
unchanged at 4 since the task file was written, despite line numbers
having drifted from `task.proto:61-66` to 125-130 due to this same agent
run's earlier proto edits for other tasks in this series), the `server.go`
handler passthrough, and `main.go`'s `NewCreateTask(repo, repo)` wiring
(one `*postgres.Repository` instance already implements both
`TaskRepository` and `GrantRepository`).

Fixed test fallout beyond the task's own file list: `create_task_test.go`
rewritten with a `GrantRepository` fake at every call site;
`server_test.go`'s 2 `NewCreateTask` call sites updated (both pass `tasks`
twice — that package's `fakeTaskRepository` already implements both ports).
Added exactly the cases the task's Verify section names plus a few more:
`TestCreateTask_CreatorID_ThenResolvePermission_ResolvesOwner` (the named
integration-style test — `CreateTask` then `ResolvePermission` in the same
test, real `domain.ResolveGrant` BFS walk, not a special-cased assertion),
`TestCreateTask_GrantInsertFailure_StillReturnsCreatedTask` (best-effort),
`TestCreateTask_CreatorID_InsertsOwnerGrant`/`_NoCreatorID_NoGrantInserted`/
`_NilGrantRepository_SkipsGrantStep` (the 3 branches of the new guard
condition).

Verify: `go build`/`go vet ./services/task-service/...` both clean; `go
test .../usecase/... -run "TestCreateTask|TestAIApply"` — all 15 cases
pass (7 new `TestCreateTask_*` cases plus the unchanged `TestAIApply_*`
suite, confirming option (b) didn't disturb `AIApply`'s existing
transactional tests); full `go test ./services/task-service/...` passes
with no regressions.
