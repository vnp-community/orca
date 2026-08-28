# TASK-PI-01-08: Tests for issue filters, cache, backoff, comments, linked PRs

**From Solution:** SOL-PI-01
**Priority:** P1
**Service:** `scm-integration-service` + `api-gateway`
**File:** `services/scm-integration-service/internal/usecase/list_issues_test.go`, `list_issue_comments_test.go` (new), `get_linked_pull_requests_for_issue_test.go` (new), `services/api-gateway/internal/adapter/httpgateway/scm_routes_test.go`, `services/api-gateway/internal/adapter/wscompat/channels_scm_test.go`
**Depends on:** TASK-PI-01-03, TASK-PI-01-04, TASK-PI-01-05, TASK-PI-01-06, TASK-PI-01-07
**Status:** `[x] DONE — list_issues_test.go (filter regression guard, cache hit/miss/expired, force_refresh bypass, cache read/write errors non-fatal, backoff retry + non-transient short-circuit), list_issue_comments_test.go, get_linked_pull_requests_for_issue_test.go all new; scm_routes_test.go/channels_scm_test.go extended for filters+refresh+new comments route/channel.`

---

## Tests to add

### `list_issues_test.go`

- Regression guard for the exact bug this solution fixes: fake
  `ScmProvider` records the `IssueFilter` it was called with; assert it
  equals `in.Filter`, not `IssueFilter{}`.
- Cache hit: a `IssueListCache.Get` fake returning a fresh (< 5 min) entry
  short-circuits — assert the provider's `ListIssues` is never called.
- Cache miss/expired: provider is called, result then `Put` into the cache.
- `force_refresh=true` always bypasses the cache regardless of freshness.
- Cache read/write error does not fail `Execute` — provider call still
  happens, a result is still returned.
- Backoff: fake provider fails transiently N times then succeeds; assert
  `BackoffExecutor.Do` retried until success. A non-transient (4xx-mapped)
  error is not retried — assert exactly 1 call.

### `list_issue_comments_test.go`

- Empty `item_slug` returns `SCM_EMPTY_SLUG` before any provider call.
- Happy path: fake provider's `ListIssueCommentsBySlug` result passed
  through unchanged.

### `get_linked_pull_requests_for_issue_test.go`

- GitHub-like fake provider returns real results — response has
  `CapabilityUnsupported=false`.
- A provider fake returning `capabilitySupported=false` maps to
  `CapabilityUnsupported: true` with an empty list, not an error — assert
  `Execute` returns `nil` error.

### `scm_routes_test.go` / `channels_scm_test.go`

- `state`/`assignee`/`label` (repeated)/`milestone` query params and WS args
  correctly populate `IssueFilter`.
- `refresh=true` maps to `force_refresh`.
- New `GET /v1/scm/issues/{number}/comments` route and `github.issueComments`
  channel each round-trip through a fake `ScmIntegrationServiceClient`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/scm-integration-service/internal/usecase/... -run "ListIssues|ListIssueComments|GetLinkedPullRequestsForIssue" -v
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestScmRoutes -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestChannelsScm -v
go build ./... && go vet ./...
```
