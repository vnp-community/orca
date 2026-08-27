# BUG-PI-03: No event-driven issue-status sync exists (worktree/PR lifecycle → Linear/GitHub status)

**Business Logic:** [BL-PI-03](../../../../docs/logic/project-integration/BL-PI-03-update-issue-status.md) — Cập nhật Trạng thái Issue/Task tự động
**Priority (per spec):** P2
**Status:** NOT_IMPLEMENTED
**Severity:** Medium
**Symptom:** A Linear issue or GitHub issue never automatically moves to "In Progress"/"In Review"/"Done"/"Cancelled" as Maya's Orca workflow progresses (worktree created, PR opened, PR merged, worktree deleted) — every one of those transitions requires manual action on the provider's own site today; backend-go has no code path that observes any of those four Orca events and calls the corresponding issue-update RPC.

---

## Spec summary

BL-PI-03 maps four Orca workflow events to issue-tracker status transitions: worktree created from issue → Linear "In Progress" / GitHub label "in-progress"; PR created → "In Review" / linked PR; PR merged → "Done" / issue closed; worktree deleted with no PR → "Cancelled". Business rules require this be disableable per-project (BR-PI-07), retried up to 3 times on API failure before giving up (BR-PI-08), and never block the main workflow (BR-PI-09) — implying an async/best-effort event-driven design.

## What backend-go has

- The individual mutation RPCs this flow would need to call already exist and work: `UpdateIssue` on both `ScmIntegrationService` (GitHub issue update incl. labels — `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:37,273-284`) and `IssueTrackingService` (Jira/Linear issue update incl. `workflow_state_id` — `backend-go/proto/orca/issuetracking/v1/issuetracking.proto:28,218-229`), each with a real usecase (`backend-go/services/scm-integration-service/internal/usecase/update_issue.go`, `backend-go/services/issue-tracking-service/internal/usecase/update_issue.go`).
- `IssueTrackingService.LinkIssue` publishes a durable outbox event `orca.issuetracking.link.created` (`backend-go/services/issue-tracking-service/internal/usecase/link_issue.go:19-83`) when an issue is linked to a task — proof the codebase does have an outbox/event mechanism this feature could be built on (`common/outbox.Relay`, referenced in that file's doc comment).
- Backend-go's whole event surface was enumerated: only three outbox `Subject` constants exist anywhere in the codebase — `orca.usage.session.recorded` (`backend-go/services/usage-service/internal/usecase/record_usage_session.go:21`), `orca.issuetracking.link.created` (`link_issue.go:19`), and `orca.tenant.profile.invalidated` (`backend-go/services/tenant-service/internal/adapter/eventbus/publisher.go:32`).

## What's missing

- **No `worktree.created`/`worktree.deleted` event is ever published.** `CreateWorktree` (`backend-go/services/git-gateway-service/internal/usecase/create_worktree.go`) and the worktree-removal usecase call `project-service`'s `RecordWorktreeCreated`/deletion RPCs directly and return — neither publishes an outbox event a status-sync consumer could react to. A repo-wide grep for `worktree.created`/`worktree\.deleted` as event subjects returns zero hits.
- **No `pr.created`/`pull_request.created` event exists.** `CreatePullRequest` (`scm-integration-service`) and `hostedReview.create` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:741-765`) return the created `PullRequest` synchronously to the caller with no outbox publish.
- **No `pr.merged` event or merge-detection mechanism exists.** `MergePullRequest` (`scmintegration.proto:33`) is a direct, synchronous mutation RPC — there is no webhook receiver, polling job, or event that fires when a PR merges (whether merged through Orca or externally on the provider's site), so even the "In Review → Done" half of the mapping table has no trigger.
- **No consumer subscribes to any of these events to call `UpdateIssue`.** A repo-wide grep for `UpdateIssueStatus`, `SyncIssueStatus`, `AutoUpdateIssue`, `IssueStatusSync` returns zero hits across all of `backend-go/`.
- **No per-project opt-out (BR-PI-07)** — since there is no sync mechanism to disable, there is naturally no toggle for it either; no `project.proto` field or setting resembling "issue status sync enabled" exists.
- **No retry-with-give-up-after-3 policy (BR-PI-08)** and **no non-blocking guarantee (BR-PI-09)** for this specific flow — moot in the absence of the flow itself, but worth noting neither pattern exists in a form this feature could reuse (the outbox `Relay` gives at-least-once delivery, which is a different guarantee than "retry the provider API call 3x then give up").

## See also

- None — this is a wholly new gap not covered by the `missing-v1`/`api-v1` bug sets, which focus on unwired WS channels rather than event-driven cross-service workflows.

## References

- `backend-go/services/issue-tracking-service/internal/usecase/link_issue.go:1-83` — the one real outbox-publishing usecase in this domain, and its `LinkCreatedSubject` constant
- `backend-go/services/scm-integration-service/internal/usecase/update_issue.go` — real GitHub `UpdateIssue` usecase (has no caller besides the direct WS channel)
- `backend-go/services/issue-tracking-service/internal/usecase/update_issue.go` — real Jira/Linear `UpdateIssue` usecase (same)
- `backend-go/services/git-gateway-service/internal/usecase/create_worktree.go:29-70` — no event publish after successful creation
- `backend-go/services/usage-service/internal/usecase/record_usage_session.go:21`, `backend-go/services/tenant-service/internal/adapter/eventbus/publisher.go:32` — the only other two outbox subjects in the whole codebase, for scale/comparison
- `backend-go/services/orchestration-service/internal/usecase/` — no issue-status-sync logic anywhere
- `docs/logic/project-integration/BL-PI-03-update-issue-status.md`
