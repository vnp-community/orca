# TASK-PI-03-05: `CreatePullRequest`/`MergePullRequest` enqueue `pr.created`/`pr.merged`

**From Solution:** SOL-PI-03
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/create_pull_request.go`, `backend-go/services/scm-integration-service/internal/usecase/merge_pull_request.go`
**Depends on:** TASK-PI-03-01, TASK-PI-03-04
**Status:** `[ ]` TODO

---

## Context

BUG-PI-03: neither PR usecase publishes anything today. This is the
Orca-initiated half of PR-merge detection — the externally-merged half is
`ReceiveWebhook` (TASK-PI-03-06). Enqueue failure must not fail the RPC:
the provider-side mutation already succeeded, per BR-PI-09's non-blocking
posture applied to the publisher side too.

## Changes to make

```go
// internal/usecase/create_pull_request.go — after the existing provider call succeeds
func (uc *CreatePullRequest) Execute(ctx context.Context, in CreatePullRequestParams) (domain.PullRequest, error) {
	pr, err := /* existing provider.CreatePullRequest(...) call, unchanged */
	if err != nil {
		return domain.PullRequest{}, err
	}
	if uc.outbox != nil {
		payload, mErr := json.Marshal(prLifecycleEventPayload{
			Provider: string(in.Provider), Repo: in.Repo, PrNumber: pr.Number,
			LinkedIssueProvider: in.LinkedIssueProvider, LinkedIssueRef: in.LinkedIssueRef,
		})
		if mErr == nil {
			event := domain.OutboxEvent{
				ID: uuid.NewString(), Subject: "orca.scm.pull_request.created",
				OccurredAt: time.Now().UTC(), PayloadJSON: payload,
			}
			if err := uc.outbox.Enqueue(ctx, in.TenantID, event); err != nil {
				uc.logger.WarnContext(ctx, "failed to enqueue pr.created event", "error", err, "pr", pr.ID)
			}
		}
	}
	return pr, nil
}
```

`MergePullRequest` gets the identical addition, publishing
`orca.scm.pull_request.merged` after a successful merge (`merged == true`
branch only — an unmerged/conflicted result publishes nothing).

Add `outbox OutboxEnqueuer` and `logger *slog.Logger` fields to both
usecase structs and their constructors, following `LinkIssue`'s existing
`OutboxEnqueuer` field shape (`link_issue.go`).

`LinkedIssueProvider`/`LinkedIssueRef` on `CreatePullRequestParams`/
`MergePullRequestParams`: resolved from the PR body's closing-keyword
reference (e.g. "Fixes #123") if present, else empty — parsing this is a
small addition to whichever code already builds the request body; if no
such parsing exists yet, leave both fields empty for this task (the
consumer already no-ops on an empty `linked_issue_ref`) rather than
inventing closing-keyword parsing here.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/scm-integration-service/...
go vet ./services/scm-integration-service/...
```
