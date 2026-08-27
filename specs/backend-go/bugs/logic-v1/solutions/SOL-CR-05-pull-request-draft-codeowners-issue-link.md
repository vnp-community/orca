# SOL-CR-05: Draft-PR option, CODEOWNERS reviewer suggestion, branch-pushed precondition, and linked-issue auto-update on `CreatePullRequest`

**Resolves:** [BUG-CR-05](../BUG-CR-05-tao-pull-request-partial.md)
**Service:** `scm-integration-service`
**Affected files (proposed):**
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto`
- `backend-go/services/scm-integration-service/internal/usecase/create_pull_request.go`
- `backend-go/services/scm-integration-service/internal/usecase/suggest_pull_request_reviewers.go` (new)
- `backend-go/services/scm-integration-service/internal/usecase/codeowners.go` (new — pure parsing/matching)
- `backend-go/services/scm-integration-service/internal/usecase/codeowners_test.go` (new)
- `backend-go/services/scm-integration-service/internal/usecase/ports.go`
- `backend-go/services/scm-integration-service/internal/adapter/external/{github,gitlab,bitbucket,azuredevops,gitea}/` (draft field + `GetRepoFileContent`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go` (`hostedReview.create`, new `hostedReview.suggestReviewers`)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`scm-integration-service.md` §4 already establishes the mechanism this
solution needs for all three of BUG-CR-05's provider-facing gaps: "A
provider that doesn't support an operation... returns typed
`ErrCapabilityUnsupported` — checked explicitly by `usecase/` code so
[capability] can degrade per provider instead of failing uniformly"
(`scm-integration-service.md:139-142`). Draft-PR support and CODEOWNERS-file
availability both vary by provider (see per-gap notes below), so both use
this exact, already-specified degrade path — no new error-handling pattern
invented.

`ports.go`'s existing `ScmProvider` interface already has the precedent
this solution's two additions follow structurally:

- `BranchExists(ctx, cred, repo, branch) (bool, error)` — "a plain
  existence read every provider's REST API supports uniformly... backs
  `CheckHostedReviewEligibility`'s step 2" (`ports.go:77-80`). BR-CR-17's
  "branch not pushed" precondition is answered by this exact method,
  already implemented for all 5 providers — this solution's job is only to
  call it one more place (inside `CreatePullRequest`, not just
  `CheckHostedReviewEligibility`), not to build a new check.
- `CreatePullRequestInput` (`ports.go:39-44`) is the natural home for a new
  `Draft bool` field — same shape as `Title`/`Body`/`HeadBranch`, no new
  method needed on `ScmProvider`.

CODEOWNERS-file fetching is the one place this solution adds a new
`ScmProvider` method (`GetRepoFileContent`) rather than reusing something
existing — grounded in §1's bounded-context statement that this service
owns "everything that isn't raw git object transfer" for the 5 providers
(`scm-integration-service.md:12-14`); fetching one file's content via each
provider's REST "contents"/"raw file" endpoint is standard provider-API
surface (the same category as `ListIssues`/`GetPullRequestForBranch`), not
git object transfer — it does **not** need a `git-gateway-service`
dependency, keeping the dependency graph's `scm --> cred` edge unchanged
(`02-microservices-decomposition.md:159`) rather than adding an
undocumented new one.

**Linked-issue auto-update** (BR-CR-19) composes two usecases that already
live in the same service and package — `CreatePullRequest` and the existing
`UpdateIssue` (`update_issue.go:1-45`) — the same in-process
usecase-composition pattern `GenerateCommitMessage` already uses for
`getStatus`/`getDiff`/`history` in `git-gateway-service`
([SOL-CR-04](./SOL-CR-04-commit-message-context-and-fallback.md)). No new
service call, no new dependency edge: both usecases already share
`credentials`/`providers`.

## Design — proto additions

```protobuf
message CreatePullRequestRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  string title = 4;
  string body = 5;
  string head_branch = 6;
  string base_branch = 7;
  string request_id = 8;
  bool draft = 9;                          // NEW — BR-CR-20
  optional int32 linked_issue_number = 10; // NEW — BR-CR-19
}

message CreatePullRequestResponse {
  PullRequest pull_request = 1;
  // NEW — set only when linked_issue_number was provided and the PR was
  // created successfully but the issue update itself failed. The PR is
  // NOT rolled back for this — see usecase section.
  string linked_issue_update_error = 2;
}

message PullRequest {
  string id = 1;
  string url = 2;
  string state = 3;
  int32 number = 4;
  bool draft = 5; // NEW — echoes provider's actual draft state, since a
                   // provider without draft support (Bitbucket) never
                   // returns true even when requested (see below)
}

// NEW RPC — BR-CR-18, called before CreatePullRequest so the client can
// display/edit suggestions per BL-CR-05 flow step 5 ("suggest reviewers"
// happens after AI title/description review, before submit).
rpc SuggestPullRequestReviewers(SuggestPullRequestReviewersRequest) returns (SuggestPullRequestReviewersResponse);

message SuggestPullRequestReviewersRequest {
  string tenant_id = 1;
  ScmProvider provider = 2;
  string repo = 3;
  string base_ref = 4;          // CODEOWNERS is read from base_ref, matching GitHub's own resolution rule
  repeated string changed_files = 5; // caller-supplied — see wiring section for why
}
message SuggestPullRequestReviewersResponse {
  repeated string reviewer_logins = 1;
  repeated string team_slugs = 2;
  bool codeowners_found = 3; // false = no CODEOWNERS file at any canonical path; empty suggestion is not an error
}
```

`draft` defaults to `false` (proto3 zero value) — this is a deliberate,
backward-compatible default for existing callers, not a violation of
BR-CR-20: the rule requires the *option* to exist so a caller isn't
structurally stuck always creating ready-for-review PRs, not that the
default itself must be draft. `hostedReview.create`'s wscompat handler
threads a real `draft` argument from the client so the option is actually
reachable end-to-end (see wiring section).

## Design — `usecase/ports.go` and provider adapters

```go
// CreatePullRequestInput — add:
type CreatePullRequestInput struct {
    Title      string
    Body       string
    HeadBranch string
    BaseBranch string
    Draft      bool // NEW
}

// ScmProvider — add:
type ScmProvider interface {
    // ... existing methods unchanged ...

    // GetRepoFileContent fetches one file's raw content at ref via the
    // provider's own contents/raw-file REST endpoint. found=false (not an
    // error) when the path doesn't exist at ref — the expected case for
    // "no CODEOWNERS file" on most repos.
    GetRepoFileContent(ctx context.Context, cred Credential, repo, path, ref string) (content string, found bool, err error)
}
```

**Draft support per provider** (all via `CreatePullRequestInput.Draft`,
each adapter's own `CreatePullRequest` implementation):

| Provider | Mechanism | Unsupported? |
|---|---|---|
| GitHub | `draft: true` in the create-PR REST/GraphQL payload — native field | No |
| GitLab | `draft: true` in the create-MR payload (modern API; older instances via `Draft: ` title prefix as a documented fallback) | No |
| Gitea | `draft` field on create-PR API (Gitea ≥1.16) | No, with a version floor to note in the adapter |
| Azure DevOps | `isDraft: true` on the pull request resource | No |
| Bitbucket | Bitbucket Cloud's PR API has no draft concept | **Yes** — returns `domain.ErrCapabilityUnsupported` wrapped when `Draft=true` is requested; `CreatePullRequest`'s usecase (below) surfaces this as a clear, typed error rather than silently creating a ready-for-review PR, which would violate BR-CR-20's intent for a caller that explicitly asked for draft |

`GetRepoFileContent` is implemented against each provider's existing
authenticated HTTP client (already present in each adapter for every other
method) hitting the well-known single-file endpoint (GitHub `GET
/repos/{owner}/{repo}/contents/{path}?ref=`, GitLab `GET
/projects/:id/repository/files/:file_path/raw?ref=`, Bitbucket `GET
/2.0/repositories/{workspace}/{repo}/src/{ref}/{path}`, Azure DevOps Items
API, Gitea `GET /repos/{owner}/{repo}/raw/{path}?ref=`) — a 404 maps to
`found=false, err=nil`, not an error, consistent with `BranchExists`'s own
"not found is a valid answer, not a failure" convention already in this
package.

## Design — `usecase/codeowners.go` (new, pure)

```go
// ParseCodeowners parses a CODEOWNERS file's content into ordered
// (pattern, owners) rules — gitignore-style patterns, last-match-wins per
// GitHub/GitLab's own documented CODEOWNERS semantics. Pure function, no
// I/O, unit-testable without any port.
func ParseCodeowners(content string) []CodeownersRule { /* ... */ }

// MatchOwners returns the union of owners for every changedFile, applying
// last-match-wins per file the same way GitHub resolves overlapping
// patterns.
func MatchOwners(rules []CodeownersRule, changedFiles []string) (logins, teams []string) { /* ... */ }
```

```go
// suggest_pull_request_reviewers.go
type SuggestPullRequestReviewers struct {
    credentials CredentialResolver
    providers   ProviderRegistry
}

// codeownersPaths mirrors GitHub's own documented lookup order.
var codeownersPaths = []string{"CODEOWNERS", ".github/CODEOWNERS", ".gitlab/CODEOWNERS", "docs/CODEOWNERS"}

func (uc *SuggestPullRequestReviewers) Execute(ctx context.Context, in SuggestPullRequestReviewersParams) (SuggestedReviewers, error) {
    // ... tenant/repo validation, credential+provider resolution, same shape as CreatePullRequest.Execute ...
    for _, path := range codeownersPaths {
        content, found, err := provider.GetRepoFileContent(ctx, cred, in.Repo, path, in.BaseRef)
        if err != nil {
            return SuggestedReviewers{}, apperrors.New(apperrors.KindInternal, "SCM_CODEOWNERS_FETCH_FAILED", "failed to fetch CODEOWNERS", err)
        }
        if !found {
            continue
        }
        logins, teams := MatchOwners(ParseCodeowners(content), in.ChangedFiles)
        return SuggestedReviewers{ReviewerLogins: logins, TeamSlugs: teams, Found: true}, nil
    }
    return SuggestedReviewers{Found: false}, nil // no CODEOWNERS anywhere — not an error, BR-CR-18 says "if present"
}
```

## Design — `create_pull_request.go` (BR-CR-17, BR-CR-19)

```go
type CreatePullRequest struct {
    credentials CredentialResolver
    providers   ProviderRegistry
    updateIssue *UpdateIssue // NEW — in-process composition, mirrors GenerateCommitMessage's pattern
}

func (uc *CreatePullRequest) Execute(ctx context.Context, in CreatePullRequestParams) (CreatePullRequestResult, error) {
    // ... existing tenant/repo/title validation, credential+provider resolution unchanged ...

    // BR-CR-17 — reuse BranchExists (already implemented, already used by
    // CheckHostedReviewEligibility) as an explicit precondition inside
    // CreatePullRequest itself, not left to CheckHostedReviewEligibility
    // being a separate, optional caller-side step.
    exists, err := provider.BranchExists(ctx, cred, in.Repo, in.HeadBranch)
    if err != nil {
        return CreatePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_BRANCH_EXISTS_CHECK_FAILED", "failed to verify branch was pushed", err)
    }
    if !exists {
        return CreatePullRequestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "SCM_BRANCH_NOT_PUSHED", "branch must be pushed to the remote before a pull request can be created", nil)
    }

    pr, err := provider.CreatePullRequest(ctx, cred, in.Repo, CreatePullRequestInput{
        Title: in.Title, Body: in.Body, HeadBranch: in.HeadBranch, BaseBranch: in.BaseBranch,
        Draft: in.Draft, // BR-CR-20
    })
    if err != nil {
        if errors.Is(err, domain.ErrCapabilityUnsupported) && in.Draft {
            return CreatePullRequestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "SCM_DRAFT_UNSUPPORTED", "this provider does not support draft pull requests", err)
        }
        return CreatePullRequestResult{}, apperrors.New(apperrors.KindInternal, "SCM_CREATE_PULL_REQUEST_FAILED", "failed to create pull request", err)
    }

    result := CreatePullRequestResult{PullRequest: pr}
    // BR-CR-19 — best-effort: the PR is already real at this point; a
    // failed issue update must not look like a failed PR creation to the
    // caller, so this error is carried in the result, not returned as the
    // call's error.
    if in.LinkedIssueNumber != 0 {
        state := "in_review" // provider-appropriate mapping is UpdateIssue/the provider adapter's concern, unchanged by this solution
        if _, err := uc.updateIssue.Execute(ctx, UpdateIssueParams{
            TenantID: in.TenantID, Provider: in.Provider, Repo: in.Repo,
            Number: in.LinkedIssueNumber, Patch: IssuePatch{State: &state},
        }); err != nil {
            result.LinkedIssueUpdateError = err.Error()
        }
    }
    return result, nil
}
```

## Design — wiring (`channels_scm.go`)

`hostedReview.create` (`channels_scm.go:741-767`) gains `draft` and
`linkedIssueNumber` args, threaded into `CreatePullRequestRequest`. A new
`hostedReview.suggestReviewers` channel calls
`SuggestPullRequestReviewers`; its `changedFiles` argument is supplied by
the client from the same changed-file list the AI-description flow
(`generate_pull_request_fields.go`) already gathers client-side via
`git.diff`/`git.status` before calling `git.generatePullRequestFields` —
no new `git-gateway-service` call is added by this solution, reusing data
the frontend flow already has at that point in BL-CR-05's own sequence
(step 2c "Changed files" is already gathered before step 3's AI generation,
per `BL-CR-05-tao-pull-request.md:25-31`).

## Test plan

- `codeowners_test.go`: `ParseCodeowners` handles glob patterns, comments,
  blank lines; `MatchOwners` applies last-match-wins for overlapping
  patterns (standard CODEOWNERS semantics fixture).
- `suggest_pull_request_reviewers_test.go`: tries all 4 canonical paths in
  order, stops at first `found=true`; returns `Found: false` (not an error)
  when none exist; fake provider's `GetRepoFileContent` call count asserted
  to stop early once found.
- `create_pull_request_test.go`:
  - `BranchExists=false` → `SCM_BRANCH_NOT_PUSHED`, `provider.CreatePullRequest`
    never called (assert zero calls) — regression guard for BR-CR-17.
  - `Draft=true` against a fake provider returning `ErrCapabilityUnsupported`
    → `SCM_DRAFT_UNSUPPORTED`, not a generic internal error.
  - `LinkedIssueNumber` set, `UpdateIssue`'s fake returns an error → result
    still has the created `PullRequest` and a non-empty
    `LinkedIssueUpdateError`, method itself returns `err == nil` —
    regression guard against BR-CR-19's failure mode rolling back or
    masking a successful PR creation.
  - `LinkedIssueNumber` unset (0) → `updateIssue` never called.
- Provider adapter tests (GitHub/GitLab/Gitea/Azure DevOps): `Draft: true`
  serializes into the correct payload field per provider. Bitbucket adapter
  test: `Draft: true` returns `ErrCapabilityUnsupported` without attempting
  the API call.
- `channels_scm_test.go`: `hostedReview.create` forwards `draft`/
  `linkedIssueNumber`; `hostedReview.suggestReviewers` wired and forwards
  `changedFiles`.

## References

- `specs/backend-go/tdd/services/scm-integration-service.md:12-14` (§1
  bounded context — why CODEOWNERS fetch is in-scope without a new
  dependency), `:128-142` (§4 `ScmProvider` interface,
  `ErrCapabilityUnsupported` degrade pattern this solution reuses twice)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:159`
  (`scm --> cred` — the only edge this service has; unchanged by this
  solution)
- [SOL-CR-04](./SOL-CR-04-commit-message-context-and-fallback.md) — the
  in-process usecase-composition precedent this solution's
  `CreatePullRequest`+`UpdateIssue` composition follows
- `backend-go/services/scm-integration-service/internal/usecase/create_pull_request.go:1-67`
- `backend-go/services/scm-integration-service/internal/usecase/update_issue.go:1-45`
- `backend-go/services/scm-integration-service/internal/usecase/check_hosted_review_eligibility.go:75-82`
  (`BranchExists` precedent this solution's BR-CR-17 check reuses)
- `backend-go/services/scm-integration-service/internal/usecase/ports.go:37-81`
  (`CreatePullRequestInput`, `ScmProvider`, `IssuePatch`)
- `backend-go/proto/orca/scmintegration/v1/scmintegration.proto:122-144,563-583`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_scm.go:740-821`
- `docs/logic/code-review/BL-CR-05-tao-pull-request.md:21-49`
