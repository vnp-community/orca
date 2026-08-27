# BUG-CR-05: PR creation has no draft option, no CODEOWNERS-based reviewer suggestion, and no automatic linked-issue update

**Business Logic:** [BL-CR-05](../../../../docs/logic/code-review/BL-CR-05-tao-pull-request.md) — Tạo Pull Request với AI-generated Description
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A user can generate an AI title/description and open a real PR against GitHub/GitLab/Bitbucket/Azure DevOps/Gitea. But they cannot open it as a draft (every PR is created ready-for-review), the system never suggests reviewers from the repo's `CODEOWNERS` file (a caller must already know who to request), and after the PR is created the linked issue/ticket is never automatically flipped to "In Review" — a separate manual step (or no step at all) is required.

---

## Spec summary

After committing, "Create PR" gathers the branch's base, all its commits, changed-file stats, and any linked issue; AI-generates a title and description; the user edits and confirms; the system suggests reviewers based on code ownership; submits the PR via the provider's API; updates the linked issue's status to "In Review"; and shows the PR link in Orca. Business rules require: PR only after the branch is pushed (BR-CR-17), CODEOWNERS-based reviewer suggestion (BR-CR-18), automatic linked-issue status update on success (BR-CR-19), and a draft-PR option that does not default to ready-for-review (BR-CR-20).

## What backend-go has

- **AI field generation is real**: `GeneratePullRequestFields` relays the worktree's full diff to `ai.complete` and parses a title+description from the response (`backend-go/services/git-gateway-service/internal/usecase/generate_pull_request_fields.go:36-72`), reachable via WS `git.generatePullRequestFields` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:643-664`).
- **PR creation is real, multi-provider**: `scm-integration-service`'s `CreatePullRequest` usecase resolves the tenant's per-provider credential and delegates to the concrete provider adapter (`backend-go/services/scm-integration-service/internal/usecase/create_pull_request.go:36-67`), backed by real GitHub/GitLab/Bitbucket/Azure DevOps/Gitea client adapters. Reachable via REST `POST /v1/scm/pull-requests` (`backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go:25,68-96`) and WS `hostedReview.create` (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:741-767`).
- **Reviewer assignment exists as a manual primitive**: `RequestPullRequestReviewers` usecase (`backend-go/services/scm-integration-service/internal/usecase/request_pull_request_reviewers.go:19-48`), wired via WS `github.pr.requestReviewers` (`channels_scm.go:~109`) — it applies an explicit `reviewer_logins`/`team_slugs` list the caller supplies.
- **Linked-issue update capability exists as a separate primitive**: `UpdateIssue` RPC, wired via WS (`channels_scm.go:181`) — can flip an issue's state, but only as its own independent call.
- **PR-existence lookup exists**: `hostedReview.forBranch` / `GetPullRequestForBranch` (`channels_scm.go:772-796`) and `hostedReview.getCreationEligibility` / `CheckHostedReviewEligibility` (`channels_scm.go:798-820`) — the latter is the natural home for a "branch pushed?" precondition check, though its actual eligibility criteria were not verified in this pass.

## What's missing

- **No `draft` field on PR creation.** `CreatePullRequestRequest` (`backend-go/proto/orca/scmintegration/v1/scmintegration.proto:122-129`) has no `draft` bool — there is no way to create a draft PR through this RPC at all, so BR-CR-20 ("draft PR option must exist, must not default to ready-for-review") cannot be satisfied. (A `draft` field does exist on GitLab's `MergeRequest` response message at `scmintegration.proto:508`, but that's a read-only reported state, not a create-time option.)
- **No CODEOWNERS-based reviewer suggestion.** A repo-wide grep for `CODEOWNERS` across `backend-go/` returns zero matches. `RequestPullRequestReviewers` only applies an already-decided reviewer list (`request_pull_request_reviewers.go:10-17`) — nothing parses a repo's `CODEOWNERS` file or maps changed files to owners, so BR-CR-18 has no backing implementation.
- **No automatic linked-issue status update after PR creation.** `CreatePullRequest`/`hostedReview.create` does not call `UpdateIssue` (or anything else) on success (`create_pull_request.go:36-67`, `channels_scm.go:741-767`) — BR-CR-19's "linked issue status must be updated after successful PR creation" would require the caller to remember to make a second, separate call; nothing orchestrates it.
- **No explicit branch-pushed precondition inside `CreatePullRequest` itself.** The usecase validates `tenant_id`/`repo`/`title` only (`create_pull_request.go:37-45`) and otherwise delegates straight to the provider API — a not-yet-pushed branch is left to fail at the provider's own API call rather than being checked proactively per BR-CR-17. (`CheckHostedReviewEligibility` may cover this from the caller side, but that is a separate, optional call the client must remember to make first — it is not enforced inside `CreatePullRequest`.)

## See also

None — no existing missing-v1/api-v1 bug documents scm-integration-service's PR-creation gaps; BUG-032 covers `git-gateway-service`'s `git.*` channel wiring, a different service.

## References

- `backend-go/services/git-gateway-service/internal/usecase/generate_pull_request_fields.go:1-72`
- `backend-go/services/scm-integration-service/internal/usecase/create_pull_request.go:1-67`
- `backend-go/services/scm-integration-service/internal/usecase/request_pull_request_reviewers.go:1-48`
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:14,122-142,505-517`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:740-821`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/scm_routes.go:22-97`
- `docs/logic/code-review/BL-CR-05-tao-pull-request.md:21-49`
