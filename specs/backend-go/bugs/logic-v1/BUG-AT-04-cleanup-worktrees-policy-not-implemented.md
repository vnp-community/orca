# BUG-AT-04: No policy-based worktree cleanup exists — only a single, unsafe, caller-decided delete

**Business Logic:** [BL-AT-04](../../../../docs/logic/automation/BL-AT-04-cleanup-worktrees.md) — Cleanup Worktrees Theo Retention Policy
**Priority (per spec):** P2
**Status:** NOT_IMPLEMENTED
**Severity:** High
**Symptom:** There is no way to configure "delete completed/error worktrees older than 7 days, stopped ones older than 3 days, except ones with uncommitted changes or an open PR" anywhere in backend-go — no automation action type for it, no query that could even evaluate the filter (worktree bookkeeping doesn't track a status or PR-link at all), and no dry-run/audit trail. The one worktree-delete primitive that does exist (`worktree.rm` → `git-gateway-service.RemoveWorktree`) takes a bare `force: bool` from whoever calls it, with **no server-side check for uncommitted changes or an open PR** — the exact two safety conditions BR-AT-11/BR-AT-12 require are enforced nowhere, not even for a single manual delete, let alone a bulk policy run.

---

## Spec summary

BL-AT-04 describes a `cleanup_worktrees` automation action that queries worktrees matching age/status filters, previews them in `dry_run` mode, and — for real runs — deletes each matching worktree after a per-worktree safety check (never deleting one with uncommitted changes or a link to an open PR), then reports counts deleted/skipped. Four business rules: never delete a worktree with uncommitted changes (BR-AT-11), never delete one linked to an open PR (BR-AT-12), a dry-run preview mode must exist (BR-AT-13), and every deletion must be audit-logged (BR-AT-14).

## What backend-go has

- A single-worktree delete primitive exists and is wired end-to-end: `worktree.rm` (WS channel, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:76-89`) calls `git-gateway-service.RemoveWorktree`, whose usecase (`backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go:33-45`) resolves the owning host, runs `git worktree remove [--force]`, and best-effort records the removal via `project-service.RecordWorktreeRemoved`. This is a real, callable RPC — not a stub.
- `Automation`'s `step_type` enum (`backend-go/proto/orca/workflow/v1/workflow.proto:53-59`) is a fixed, closed set: `AGENT`, `SHELL`, `NOTIFICATION`, `WEBHOOK`, `CONDITION`. No step type maps to "cleanup" or "cleanup_worktrees" in any form.

## What's missing

- **No `cleanup_worktrees` action type at all.** `workflow.proto`'s `StepType` enum has no entry for it (`workflow.proto:53-59`), so an automation cannot even be configured to perform this action — there is nothing for `RunNow`/`ExecuteAdHocStep` to dispatch to.
- **No filter query capability.** BL-AT-04's filters need `status: [completed, error, stopped]` and `older_than`, but `project-service`'s `Worktree` domain struct (`backend-go/services/project-service/internal/domain/worktree.go:28-50`) has no `Status` field at all — only `ID`, `ProjectID`, `RepoID`, `Path`, `Branch`, `Active`, `CreatedAt`, and lineage-capture fields. There is no way to query "worktrees with status=completed older than 7 days" because the status concept the filter depends on isn't tracked anywhere in backend-go.
- **BR-AT-11 (never delete with uncommitted changes) is unenforced even for the single-delete path.** `RemoveWorktree.Execute` (`backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go:33-45`) takes `force bool` straight from the caller (`channels_worktree.go:80`, `WorktreeID`/`Force` decoded directly from the WS args) and passes it to `executor.RemoveWorktree(ctx, repoPath, force)` with no server-side check of working-tree cleanliness beforehand — the caller (client) decides `force`, the server never independently verifies "does this worktree have uncommitted changes."
- **BR-AT-12 (never delete linked to an open PR) is unenforced.** No PR-link lookup exists in `RemoveWorktree.Execute` or anywhere in `git-gateway-service`/`project-service` — confirmed by grep for PR-link/open-PR/safety-check patterns across both services, which returns no matches relevant to worktree deletion.
- **BR-AT-13 (dry_run mode) does not exist.** A repo-wide grep for `dry_run`/`dryRun`/`DryRun` across all of `backend-go/services` returns zero matches — there is no preview-only mode for any deletion path.
- **BR-AT-14 (audit log of all deleted worktrees) does not exist for this domain.** `backend-go` does have an audit-log concept, but it belongs to `auth-service` (`internal/adapter/postgres/audit_repository.go`) and `credential-broker-service` (`internal/domain/access_audit_entry.go`) — neither is wired to worktree removal. `RemoveWorktree.Execute`'s only bookkeeping is `RecordWorktreeRemoved` (marks the metadata row gone), which is not an audit trail (no reason/actor/policy-match record, no dedicated audit table).
- **"Report: X deleted, Y skipped"**: no bulk-operation entry point exists to report from — since there is no `cleanup_worktrees` action, there is no batch runner that could aggregate a deleted/skipped count in the first place.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-031-worktree-channels-not-implemented.md` — originally found `worktree.rm` entirely unregistered; per this audit `worktree.rm` and `git-gateway-service.RemoveWorktree` now exist and are wired (`channels_worktree.go:76-89`, `remove_worktree.go`). That specific gap is resolved, but as this bug shows, the single-delete path it unblocked still has no safety checks — resolving BUG-031 did not touch BR-AT-11/BR-AT-12 at all.

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:76-89` — `worktree.rm`, `force` taken directly from caller
- `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go:1-45` — `RemoveWorktree.Execute`, no uncommitted-changes or PR-link check
- `backend-go/services/project-service/internal/domain/worktree.go:28-50` — `Worktree` struct, no `Status` field
- `backend-go/proto/orca/workflow/v1/workflow.proto:53-59` — `StepType` enum, no cleanup action
- `backend-go/services/auth-service/internal/adapter/postgres/audit_repository.go`, `backend-go/services/credential-broker-service/internal/domain/access_audit_entry.go` — the only audit-log implementations in backend-go, neither covering worktrees
- `docs/logic/automation/BL-AT-04-cleanup-worktrees.md` — spec
