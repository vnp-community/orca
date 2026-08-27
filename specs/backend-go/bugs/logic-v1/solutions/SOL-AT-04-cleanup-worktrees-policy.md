# SOL-AT-04: `cleanup_worktrees` action, worktree status tracking, and safety checks for BOTH the single-delete and policy-bulk paths

**Resolves:** [BUG-AT-04](../BUG-AT-04-cleanup-worktrees-policy-not-implemented.md)
**Service:** `workflow-service` (new in-process `StepExecutor`) +
`project-service` (new `Status` column + query) + `git-gateway-service`
(safety checks added to `RemoveWorktree`) + `automation-service` (audit log
table)
**Affected files (proposed):**
- `backend-go/proto/orca/workflow/v1/workflow.proto` (`StepType` +
  `CLEANUP_WORKTREES`)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
  (`RemoveWorktreeRequest` gains `allow_open_pr`)
- `backend-go/proto/orca/project/v1/project.proto` (`Worktree` gains
  `status`; new `ListWorktreesRequest` filters)
- `backend-go/services/workflow-service/internal/usecase/cleanup_worktrees_step_executor.go` (new)
- `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go`
- `backend-go/services/project-service/internal/domain/worktree.go`
- `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go` (+ migration: `status` column)
- `backend-go/services/automation-service/internal/adapter/postgres/` (+ migration: `worktree_cleanup_log` table)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

**Where the new action executes.** `workflow-service.md:30-46`'s bounded
context draws the exact line this action needs: `webhook`/`condition`
"run entirely in-process" while `agent`/`shell`/`notification` "are
dispatched here but executed on the Dev Server Agent execution plane."
`cleanup_worktrees` is neither — it needs no execution-plane relay at all,
only orchestrated calls to `project-service` (worktree metadata + status
filter), `git-gateway-service` (uncommitted-changes check + the actual
`RemoveWorktree`), and `scm-integration-service` (open-PR check). This is
structurally identical to `WebhookStepExecutor`'s shape — a sixth
`StepExecutor` implementation (`workflow-service.md:129-142` documents five;
this adds one), running natively in `workflow-service`'s own process, never
touching `infra-fleet-service` or the Dev Server Agent.

**Safety checks belong in `git-gateway-service.RemoveWorktree`, not
duplicated in the new executor.** BUG-AT-04 is explicit that BR-AT-11/BR-
AT-12 are unenforced "even for the single-delete path"
(`remove_worktree.go:33-45`, taking `force` straight from the caller with
no check). Per `project-service.md:26-40`'s coordination/execution split
("resolve target + check access here, relay the actual work elsewhere"),
the safety check belongs at the one place that actually issues the delete
— fixing it there fixes it for *every* caller (the WS `worktree.rm` manual
path AND this solution's new bulk `cleanup_worktrees` path), rather than
building a second, parallel safety check inside `workflow-service`'s
executor that could drift from the single-delete path's behavior.

**Open-PR check reuses an existing RPC, no new SCM surface needed.**
`scm-integration-service`'s already-defined `GetPullRequestForBranch(tenant_id,
provider, repo, head_branch) → {pull_request, found}` (confirmed by direct
proto read, `scmintegration.proto:286-296`) is exactly BR-AT-12's check —
"does this worktree's branch have an open PR" — with no proto changes
needed on that service's side at all.

**Status tracking is a genuine schema gap, not a design choice.**
`project-service.md:176-180`'s documented `Worktree` domain model already
lists only `activation_state (active/asleep)` — no `completed`/`error`/
`stopped` concept exists anywhere in that document either, confirming
BUG-AT-04's finding is a real gap in the TDD's own domain model, not just
the implementation. This solution adds `Status` as a new, additional field
alongside the existing `activation_state`/`Active` (not a replacement) —
see "Design — project-service" below for why they're orthogonal.

**Audit log ownership.** BUG-AT-04 correctly notes the two existing
audit-log implementations (`auth-service`, `credential-broker-service`)
don't cover worktrees and shouldn't be forced to. Per
`automation-service.md:33-35`'s table — "Run bookkeeping (status,
timestamps, outcome, usage) | Yes — Postgres row" is already this service's
job for every other kind of automation-run outcome — a cleanup run's
per-worktree audit trail is bookkeeping about *what an automation run did*,
which belongs beside `automation_runs`, not invented as a third,
unrelated audit subsystem.

---

## Design — proto

```protobuf
// workflow.proto
enum StepType {
  // ... existing 5 values ...
  STEP_TYPE_CLEANUP_WORKTREES = 6; // NEW — BL-AT-04
}
```

```protobuf
// project.proto — Worktree message
message Worktree {
  // ... existing fields ...
  string status = N; // NEW — "active" | "completed" | "error" | "stopped"; orthogonal to activation_state (see below)
}

message ListWorktreesRequest {
  string project_id = 1;
  // NEW — both optional; unset = no filtering, existing behavior unchanged.
  repeated string status_in = 2;      // e.g. ["completed","error","stopped"]
  google.protobuf.Timestamp older_than = 3; // created_at (or a future last-activity field) cutoff
}
```

```protobuf
// gitgateway.proto — RemoveWorktreeRequest
message RemoveWorktreeRequest {
  string worktree_id = 1;
  bool   force = 2;          // unchanged meaning: override the uncommitted-changes check (BR-AT-11), mirrors `git worktree remove --force`
  bool   allow_open_pr = 3;  // NEW — separate, explicit override for BR-AT-12; NEVER set true by the cleanup_worktrees path
}
```

`force` and `allow_open_pr` are deliberately two separate booleans, not one
combined flag — BR-AT-11 and BR-AT-12 are two independent safety rules with
independent override semantics (a caller might have deliberately committed
to overriding "has uncommitted changes" for a scratch worktree while still
wanting the open-PR guard to hold, or vice versa); collapsing them into one
`force` would make an accidental PR-bypass a side effect of an unrelated
uncommitted-changes override.

## Design — `project-service`: `Status` (new, orthogonal to `Active`)

```go
// internal/domain/worktree.go
type WorktreeStatus string

const (
	WorktreeStatusActive    WorktreeStatus = "active"
	WorktreeStatusCompleted WorktreeStatus = "completed"
	WorktreeStatusError     WorktreeStatus = "error"
	WorktreeStatusStopped   WorktreeStatus = "stopped"
)
```

`Active bool` (`worktree.go:34`, project-service.md's `activation_state`)
already means "is this worktree currently the one shown/usable in the UI" —
a UI-level sleep/wake toggle, set by `SetWorktreeActivation`. `Status`
answers a different question — "what did the work in this worktree end up
doing" (completed/errored/stopped) — set by whichever service tracks
task/agent-run completion against a worktree (out of this solution's file
list to fully wire: flagged as a follow-up needing a new
`SetWorktreeStatus`-shaped RPC called by `task-service`/`workflow-service`
on run completion, the same class of cross-service follow-up
[SOL-AT-03](./SOL-AT-03-event-trigger.md) already flags for its missing
event publishers). Both fields can be true/either independently: a
`completed` worktree can still be `active` (visible, just done) until a
human or this cleanup policy removes it.

`ListWorktrees` gains the `status_in`/`older_than` filter (BL-AT-04's
`status: [...]`/`older_than` schema, quoted directly in BUG-AT-04) —
`WHERE ($2::text[] IS NULL OR status = ANY($2)) AND ($3::timestamptz IS
NULL OR created_at < $3)`, both optional so every existing caller (which
passes neither) is unaffected.

## Design — `git-gateway-service.RemoveWorktree`: mandatory safety checks

```go
// internal/usecase/remove_worktree.go
func (uc *RemoveWorktree) Execute(ctx context.Context, worktreeID string, force, allowOpenPR bool) error {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-AT-11 — checked server-side regardless of `force`'s downstream
	// effect on the git binary itself; produces a typed, catchable error
	// (WORKTREE_HAS_UNCOMMITTED_CHANGES) the caller can distinguish from a
	// generic git failure, which the bulk cleanup path (below) needs to
	// record a proper skip reason rather than surface a raw git error.
	status, err := executor.GetStatus(ctx, repoPath)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_STATUS_CHECK_FAILED", "failed to check worktree status before removal", err)
	}
	if len(status.Files) > 0 && !force {
		return apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_UNCOMMITTED_CHANGES", "worktree has uncommitted changes", nil)
	}

	// BR-AT-12 — independent of force; requires the SEPARATE allow_open_pr
	// override (never set by cleanup_worktrees, see below).
	branch, err := uc.currentBranch(ctx, executor, repoPath) // reuses existing GetStatus/branch info
	if err == nil && branch != "" {
		pr, found, err := uc.scm.GetPullRequestForBranch(ctx, uc.tenantIDFromCtx(ctx), branch)
		if err == nil && found && pr.State == "open" && !allowOpenPR {
			return apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_OPEN_PR", "worktree's branch has an open pull request", nil)
		}
		// A GetPullRequestForBranch error (e.g. SCM not connected for this
		// repo) does NOT block deletion — fails open on this specific check
		// only, since a repo with no SCM integration configured has no way
		// to ever answer "is there an open PR", and BR-AT-12 shouldn't make
		// every such repo's worktrees permanently undeletable. Contrast with
		// BR-AT-11 above, which fails closed on its own check error — a
		// git-status failure is a signal something is actually wrong with
		// the worktree, not "the check doesn't apply here".
	}

	if err := executor.RemoveWorktree(ctx, repoPath, force); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, worktreeID); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return nil
}
```

`SCMClient` (new dependency for this usecase, or reuse the existing one if
already injected elsewhere in this service — `git-gateway-service`
already has an `SCMClient` port per `ports.go:347-364` for PR/MR base-branch
lookups, extended here with `GetPullRequestForBranch`).

## Design — `workflow-service`: `CleanupWorktreesStepExecutor`

```go
// internal/usecase/cleanup_worktrees_step_executor.go (new)
type CleanupWorktreesConfig struct {
	ProjectID  string
	StatusIn   []string      // e.g. ["completed","error"]
	OlderThan  time.Duration // e.g. 7*24h for completed/error, 3*24h for stopped per BL-AT-04
	DryRun     bool          // BR-AT-13
}

type CleanupWorktreesStepExecutor struct {
	projects ProjectClient        // ListWorktrees(status_in, older_than), RemoveWorktree is NOT called here — see below
	gitgw    GitGatewayClient     // RemoveWorktree — the ONE call site for the actual delete, force=false, allow_open_pr=false, always
	auditLog CleanupAuditWriter   // automation-service's new worktree_cleanup_log
}

func (e *CleanupWorktreesStepExecutor) Execute(ctx context.Context, step Step, execCtx ExecutionContext) (StepOutput, error) {
	cfg := parseCleanupConfig(step.Config) // step_config_json -> CleanupWorktreesConfig
	candidates, err := e.projects.ListWorktrees(ctx, cfg.ProjectID, cfg.StatusIn, time.Now().Add(-cfg.OlderThan))
	if err != nil {
		return StepOutput{}, err
	}

	var deleted, skipped int
	report := make([]CleanupEntry, 0, len(candidates))
	for _, wt := range candidates {
		if cfg.DryRun {
			report = append(report, CleanupEntry{WorktreeID: wt.ID, Action: "would_delete"})
			continue
		}
		// force=false, allow_open_pr=false — UNCONDITIONALLY, for every
		// candidate in an unattended policy run. BR-AT-11/BR-AT-12 have NO
		// override path here, unlike the manual worktree.rm channel — an
		// automated bulk delete must never silently bypass either safety
		// rule (this is the one place in this design where "force" is
		// hardcoded, not caller-configurable, by deliberate choice).
		err := e.gitgw.RemoveWorktree(ctx, wt.ID, false, false)
		switch {
		case err == nil:
			deleted++
			report = append(report, CleanupEntry{WorktreeID: wt.ID, Action: "deleted"})
		case isFailedPrecondition(err, "WORKTREE_HAS_UNCOMMITTED_CHANGES", "WORKTREE_HAS_OPEN_PR"):
			skipped++
			report = append(report, CleanupEntry{WorktreeID: wt.ID, Action: "skipped", Reason: err.Error()})
		default:
			skipped++
			report = append(report, CleanupEntry{WorktreeID: wt.ID, Action: "skipped", Reason: "removal failed: " + err.Error()})
		}
	}

	_ = e.auditLog.WriteCleanupReport(ctx, execCtx.TenantID, execCtx.ExecutionID, report) // BR-AT-14, best-effort per this doc's existing bookkeeping-failure posture
	return StepOutput{Status: "completed", OutputJSON: mustJSON(cleanupSummary{Deleted: deleted, Skipped: skipped, Entries: report})}, nil
}
```

Registered in `workflow-service`'s `StepExecutor` dispatch table
(`workflow-service.md:129-142`) alongside the existing five, keyed on
`STEP_TYPE_CLEANUP_WORKTREES`. `automation-service`'s `RunNow` (per
[SOL-AT-01](./SOL-AT-01-config-cap-cycle-chain.md)) dispatches to it exactly
like any other action — no special-casing needed in `automation-service`
itself, since `ExecuteAdHocStep`'s `step_type`/`step_config_json` shape
already carries whatever this action needs.

## Design — BR-AT-13 (dry-run) and BR-AT-14 (audit log)

Dry-run is the `cfg.DryRun` branch above — no `RemoveWorktree` call at all,
only the report. Audit log:

```sql
-- automation-service migration, new table
CREATE TABLE automation.worktree_cleanup_log (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  run_id         UUID NOT NULL REFERENCES automation.automation_runs (id) ON DELETE CASCADE,
  worktree_id    TEXT NOT NULL,
  action         TEXT NOT NULL CHECK (action IN ('deleted','skipped','would_delete')),
  reason         TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_worktree_cleanup_log_run ON automation.worktree_cleanup_log (run_id);
```

One row per worktree per cleanup run — queryable independent of
`automation_runs.output_json`'s free-form blob (which still holds the
summary counts for quick display), giving BR-AT-14 a real per-worktree,
per-reason audit trail rather than only an aggregate count.
`CleanupAuditWriter` is implemented in `automation-service`'s
`adapter/postgres/` and exposed to `workflow-service` via a small new gRPC
method (`automation-service` already owns run bookkeeping, so this is a
natural reverse-direction call: `workflow-service` → `automation-service`,
flagged as a new dependency edge not in `02-microservices-decomposition.md`'s
existing graph — narrowly scoped to this one write, following the same
"flag a scope addition" discipline as SOL-009's `git-gateway-service`
extension).

## Test plan

- `git-gateway-service/internal/usecase/remove_worktree_test.go`: fake
  `GitExecutor.GetStatus` returns dirty files, `force=false` →
  `WORKTREE_HAS_UNCOMMITTED_CHANGES`, `executor.RemoveWorktree` never
  called (assert zero calls); `force=true` → proceeds. Fake `SCMClient`
  reports an open PR, `allow_open_pr=false` → `WORKTREE_HAS_OPEN_PR`;
  `allow_open_pr=true` → proceeds. `SCMClient` errors (no integration
  configured) → deletion proceeds (fail-open, per this check's documented
  exception).
- `project-service/internal/adapter/postgres/worktree_repository_test.go`:
  `ListWorktrees` with `status_in`/`older_than` filters returns exactly the
  matching subset; both unset → identical results to today's unfiltered
  call (regression guard).
- `workflow-service/internal/usecase/cleanup_worktrees_step_executor_test.go`:
  a batch of 5 candidates (2 clean/eligible, 1 dirty, 1 open-PR, 1
  removal-error) → `deleted=2, skipped=3`, report has the correct
  `Action`/`Reason` per entry; `DryRun=true` → zero `RemoveWorktree` calls,
  every entry `would_delete`.
- `automation-service/internal/adapter/postgres/`: `WriteCleanupReport`
  round-trip — N entries in, N rows queryable by `run_id`.
- Regression: existing `worktree.rm` WS-channel callers (manual delete, no
  `allow_open_pr` in their JSON args) default to `allow_open_pr=false` —
  a manual delete of a worktree with an open PR now correctly fails where
  it previously silently succeeded (this is the intended behavior change;
  document in cutover notes, same discipline `project-service.md:383-388`
  applies to its own `RebindDevServer` behavior change).

## References

- `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go:1-45` — the single-delete path this solution adds checks to
- `backend-go/services/git-gateway-service/internal/usecase/ports.go:61-64` (`GitExecutor.GetStatus`, reused for BR-AT-11), `:347-364` (`SCMClient` port, extended for BR-AT-12)
- `backend-go/services/project-service/internal/domain/worktree.go:28-50` — `Worktree` struct, no `Status` field
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:126-138` (`GetStatusResponse`/`FileStatus` — BR-AT-11's data source), `:629-632` (`RemoveWorktreeRequest`, gains `allow_open_pr`)
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:286-296` — `GetPullRequestForBranch`, the existing RPC this solution reuses verbatim for BR-AT-12
- `backend-go/proto/orca/workflow/v1/workflow.proto:53-60` — `StepType` enum this solution extends
- `specs/backend-go/tdd/services/workflow-service.md:30-46` (bounded-context split determining where this executor lives), `:129-142` (`StepExecutor` five-implementation precedent)
- `specs/backend-go/tdd/services/project-service.md:26-40` (coordination/execution split rationale), `:176-180` (`Worktree` domain model, no status today), `:383-388` (precedent for documenting an intentional behavior change in cutover notes)
- `specs/backend-go/tdd/services/automation-service.md:33-35` — run-bookkeeping ownership rationale for the new audit table
- `docs/logic/automation/BL-AT-04-cleanup-worktrees.md` — spec
