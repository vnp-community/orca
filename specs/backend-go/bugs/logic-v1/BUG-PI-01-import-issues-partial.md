# BUG-PI-01: GitHub/GitLab issue browsing has no filtering, no cache, and no detail-view comments

**Business Logic:** [BL-PI-01](../../../../docs/logic/project-integration/BL-PI-01-import-issues.md) — Import và Duyệt GitHub/GitLab Issues
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** Maya opens the GitHub panel and sees the full open-issue list and can authenticate via OAuth/PAT, but she cannot filter by status/assignee/label/milestone, every panel refresh re-hits the live GitHub/GitLab API (no 5-minute cache), and clicking into an issue's detail view shows title/body/state/url but never its existing comments (only add/update/delete comment RPCs exist, no list).

---

## Spec summary

BL-PI-01 lets a user browse GitHub/GitLab issues inside Orca without a browser: authenticate (OAuth/PAT), load issues with a default filter (open, assigned to me), filter by status/assignee/label/milestone, view issue detail (description, comments, linked PRs), and jump to BL-PI-02 ("Create Worktree from issue"). Business rules require a 5-minute issue cache with on-demand refresh (BR-PI-01), GitHub REST+GraphQL support (BR-PI-02), and rate-limit handling with exponential backoff and a visible remaining-quota indicator (BR-PI-03).

## What backend-go has

- `ScmIntegrationService.ListIssues` RPC exists and is provider-generic (`backend-go/proto/orca/scmintegration/v1/scmintegration.proto:13,112-120`), with real GitHub and GitLab HTTP client implementations: `backend-go/services/scm-integration-service/internal/adapter/github/client.go:82-112` (`GET /repos/{repo}/issues`, real OAuth Bearer auth, filters out PR-shaped issues) and `backend-go/services/scm-integration-service/internal/adapter/gitlab/client.go:91` (`ListIssues`).
- The usecase (`backend-go/services/scm-integration-service/internal/usecase/list_issues.go:16-58`) resolves the tenant's credential via `CredentialResolver` and dispatches to the right provider adapter — this is real, working end-to-end plumbing, reachable from the frontend via `GET /v1/scm/issues` (`backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go`, `RouteWired`).
- OAuth auth flow (BR's implicit auth requirement) is real: `StartOAuthFlow`/`CompleteOAuthFlow`/`RevokeAuth`/`GetAuthStatus` (`scmintegration.proto:25-28`), each with a working usecase (`start_oauth_flow.go`, `complete_oauth_flow.go`, `revoke_auth.go`, `get_auth_status.go`) and REST proxy (`scm_routes.go`).
- Rate-limit quota display (part of BR-PI-03) is real and even cached: `GetRateLimitStatus` (`scmintegration.proto:16`) reads through a 60-second freshness cache before falling back to a live provider call — `backend-go/services/scm-integration-service/internal/usecase/get_rate_limit_status.go:12-16,43-51`, backed by a real Postgres table (`backend-go/services/scm-integration-service/internal/adapter/postgres/rate_limit_cache.go`).
- Issue detail beyond the bare fields is partially covered for GitHub Projects items: `GetWorkItemDetailsBySlug` (`scmintegration.proto:58`) returns title/body/state/url/fields, and `AddIssueCommentBySlug`/`UpdateIssueCommentBySlug`/`DeleteIssueCommentBySlug` (`scmintegration.proto:65-67`) let a user mutate comments.

## What's missing

- **No filter support at all** — `ListIssuesRequest` (`scmintegration.proto:112-116`) carries only `tenant_id`, `provider`, `repo`; there is no status/assignee/label/milestone field. Server-side, `usecase.IssueFilter` does have a `State` field (`backend-go/services/scm-integration-service/internal/usecase/ports.go:33-35`), but `ListIssues.Execute` never populates it — it calls `provider.ListIssues(ctx, cred, in.Repo, IssueFilter{})` with a hardcoded empty filter (`backend-go/services/scm-integration-service/internal/usecase/list_issues.go:53`). Every one of the spec's step-5 filters (status, assignee, label, milestone) is unreachable end-to-end.
- **No 5-minute issue cache (BR-PI-01)** — unlike `GetRateLimitStatus`'s real Postgres-backed 60s cache, `ListIssues` has no cache layer of any kind; a repo-wide grep for issue-list caching (`5 * time.Minute`, `issueCache`, `IssueCache`, `ListIssuesCache`) returns zero hits anywhere in `backend-go/`. Every panel load/refresh is a live GitHub/GitLab API call.
- **No exponential backoff (BR-PI-03)** — a repo-wide grep for `backoff`/`Backoff`/`exponential` across `backend-go/services/scm-integration-service/` returns zero hits. Only the quota-display half of BR-PI-03 is covered (`GetRateLimitStatus`), not the retry/backoff half.
- **No GraphQL support (BR-PI-02)** — GitHub Projects v2 RPCs (`ListAccessibleProjects`, `ViewProjectTable`, etc., `scmintegration.proto:52-67`) use GraphQL for Projects-v2-specific concepts, but plain issue listing (`ListIssues`) is REST-only; BR-PI-02's "support both REST and GraphQL" is only half-true and only for a different feature (Projects, not issue browsing).
- **No comment-listing for issue detail (step 6)** — `AddIssueCommentBySlug`/`UpdateIssueCommentBySlug`/`DeleteIssueCommentBySlug` exist but there is no `ListIssueCommentsBySlug` (or equivalent) RPC anywhere in `scmintegration.proto` — a client can write a comment but can never read the existing comment thread through this service.
- Linked-PR lookup for a given issue (part of step 6's detail view) has no dedicated RPC; `GetPullRequestForBranch` (`scmintegration.proto:42`) resolves a PR by *branch*, not by the issue it's linked to — there's no issue↔PR association concept in the proto at all.

## References

- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:13,25-28,42,52-67,112-120,156-165` — RPC/message surface
- `backend-go/services/scm-integration-service/internal/usecase/list_issues.go:16-58` — hardcoded empty `IssueFilter{}`
- `backend-go/services/scm-integration-service/internal/usecase/ports.go:33-35` — unused `IssueFilter.State`
- `backend-go/services/scm-integration-service/internal/adapter/github/client.go:82-112` — real GitHub REST `ListIssues`
- `backend-go/services/scm-integration-service/internal/adapter/gitlab/client.go:91` — real GitLab `ListIssues`
- `backend-go/services/scm-integration-service/internal/usecase/get_rate_limit_status.go:12-16,43-51` — the one real cache (rate limit, not issues)
- `backend-go/services/scm-integration-service/internal/adapter/postgres/rate_limit_cache.go` — Postgres-backed rate-limit cache
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go` — `/v1/scm` REST proxy, `RouteWired`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:58-67` — `github.rateLimit` WS channel
