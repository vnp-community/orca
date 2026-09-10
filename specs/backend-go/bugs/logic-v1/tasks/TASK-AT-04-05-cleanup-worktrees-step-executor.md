# TASK-AT-04-05: `CleanupWorktreesStepExecutor` — bulk policy delete + dry-run (BR-AT-13)

**From Solution:** SOL-AT-04
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/usecase/cleanup_worktrees_step_executor.go` (new)
**Depends on:** TASK-AT-04-02, TASK-AT-04-03, TASK-AT-04-04
**Status:** `[x]` DONE — usecase/cleanup_worktrees_step_executor.go registered as workflow-service's 6th StepExecutor (STEP_TYPE_CLEANUP_WORKTREES); new adapter/serviceclients dials project/git-gateway/automation-service; force=false,allow_open_pr=false unconditional; unit-tested (mixed batch, dry-run, empty-run-id, audit-failure-is-best-effort).

---

## Context

A new, sixth `StepExecutor` implementation running natively in-process in
`workflow-service` (no execution-plane relay needed) — lists candidate
worktrees via `project-service`'s filtered `ListWorktrees`, deletes each via
`git-gateway-service.RemoveWorktree` with `force=false, allow_open_pr=false`
unconditionally (an automated bulk delete must never bypass either safety
rule), and writes an audit report.

## Changes to make

Create `backend-go/services/workflow-service/internal/usecase/cleanup_worktrees_step_executor.go`:

```go
package usecase

type CleanupWorktreesConfig struct {
	ProjectID string
	StatusIn  []string
	OlderThan time.Duration
	DryRun    bool // BR-AT-13
}

type CleanupEntry struct {
	WorktreeID string
	Action     string // "deleted" | "skipped" | "would_delete"
	Reason     string
}

type CleanupWorktreesStepExecutor struct {
	projects ProjectClient      // ListWorktrees(status_in, older_than)
	gitgw    GitGatewayClient   // RemoveWorktree — the ONE call site for the actual delete
	auditLog CleanupAuditWriter // automation-service's WriteCleanupReport RPC client
}

func (e *CleanupWorktreesStepExecutor) Execute(ctx context.Context, step Step, execCtx ExecutionContext) (StepOutput, error) {
	cfg, err := parseCleanupConfig(step.Config)
	if err != nil {
		return StepOutput{}, err
	}
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
		// force=false, allow_open_pr=false — UNCONDITIONALLY. An automated
		// bulk delete must never silently bypass either safety rule.
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

	if err := e.auditLog.WriteCleanupReport(ctx, execCtx.TenantID, execCtx.ExecutionID, report); err != nil {
		// Best-effort, mirrors this codebase's existing bookkeeping-failure
		// posture — a report-write failure must not fail the step itself.
	}
	return StepOutput{Status: "completed", OutputJSON: mustJSON(cleanupSummary{Deleted: deleted, Skipped: skipped, Entries: report})}, nil
}
```

Register `CleanupWorktreesStepExecutor` in `workflow-service`'s
`StepExecutor` dispatch table, keyed on `STEP_TYPE_CLEANUP_WORKTREES`
(check the existing dispatch table's registration pattern — likely a map or
switch in a central wiring file, e.g. `internal/usecase/step_executor.go` or
`cmd/server/main.go`).

Add the `GitGatewayClient.RemoveWorktree` and `CleanupAuditWriter` port
interfaces to this package if not already present, backed by the gRPC
clients from TASK-AT-04-03 and TASK-AT-04-04 respectively.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run TestCleanupWorktreesStepExecutor
```

Expected: a batch of 5 candidates (2 clean/eligible, 1 dirty, 1 open-PR, 1
removal-error) → `deleted=2, skipped=3`, report has correct `Action`/
`Reason` per entry; `DryRun=true` → zero `RemoveWorktree` calls, every entry
`would_delete`.
