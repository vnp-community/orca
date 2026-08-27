# TASK-CR-05-03: Implement `Draft` support in each provider's `CreatePullRequest` (BR-CR-20)

**From Solution:** SOL-CR-05
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/adapter/github/client.go`, `.../gitlab/client.go`, `.../gitea/client.go`, `.../azuredevops/client.go`, `.../bitbucket/client.go`
**Depends on:** TASK-CR-05-02
**Status:** `[ ]` TODO

---

## Context

Draft-PR support varies by provider. Four providers have a native draft
field; Bitbucket Cloud's PR API has no draft concept at all, so it must
return `domain.ErrCapabilityUnsupported` (wrapped) when `Draft=true` is
requested, rather than silently creating a ready-for-review PR — which
would violate BR-CR-20's intent for a caller that explicitly asked for
draft.

## Changes to make

### GitHub (`github/client.go`) — native `draft` field

```go
func (c *Client) CreatePullRequest(ctx context.Context, cred usecase.Credential, repo string, input usecase.CreatePullRequestInput) (domain.PullRequest, error) {
	body, err := json.Marshal(struct {
		Title string `json:"title"`
		Body  string `json:"body,omitempty"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Draft bool   `json:"draft,omitempty"` // NEW
	}{Title: input.Title, Body: input.Body, Head: input.HeadBranch, Base: input.BaseBranch, Draft: input.Draft})
	// ... rest unchanged ...
```

Also thread `draft` through `toDomainPullRequest`'s source struct
(`githubPullRequest`) and the resulting `domain.PullRequest{Draft: ...}` so
the echoed response reflects GitHub's actual `draft` field.

### GitLab (`gitlab/client.go`) — native `draft` field on the create-MR payload

```go
	body, err := json.Marshal(struct {
		Title        string `json:"title"`
		Description  string `json:"description,omitempty"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Draft        bool   `json:"draft,omitempty"` // NEW
	}{Title: input.Title, Description: input.Body, SourceBranch: input.HeadBranch, TargetBranch: input.BaseBranch, Draft: input.Draft})
```

Thread `draft` through the response decode/`toDomainPullRequest` the same
way as GitHub.

### Gitea (`gitea/client.go`) — `draft` field (Gitea ≥1.16)

```go
	body, err := json.Marshal(struct {
		Title string `json:"title"`
		Body  string `json:"body,omitempty"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Draft bool   `json:"draft,omitempty"` // NEW — requires Gitea >= 1.16
	}{Title: input.Title, Body: input.Body, Head: input.HeadBranch, Base: input.BaseBranch, Draft: input.Draft})
```

Add a doc-comment note on the method: "requires Gitea >= 1.16 for the
`draft` field to be honored; older instances silently ignore it (not this
adapter's problem to detect — no version-negotiation primitive exists in
this client today)."

### Azure DevOps (`azuredevops/client.go`) — `isDraft` on the PR resource

```go
	body, err := json.Marshal(struct {
		Title         string `json:"title"`
		Description   string `json:"description,omitempty"`
		SourceRefName string `json:"sourceRefName"`
		TargetRefName string `json:"targetRefName"`
		IsDraft       bool   `json:"isDraft,omitempty"` // NEW
	}{
		Title:         input.Title,
		Description:   input.Body,
		SourceRefName: "refs/heads/" + input.HeadBranch,
		TargetRefName: "refs/heads/" + input.BaseBranch,
		IsDraft:       input.Draft,
	})
```

Thread `isDraft` through the response decode/`toDomainPullRequest`.

### Bitbucket (`bitbucket/client.go`) — unsupported

Add a check at the top of `CreatePullRequest`, before building the request
body:

```go
func (c *Client) CreatePullRequest(ctx context.Context, cred usecase.Credential, repo string, input usecase.CreatePullRequestInput) (domain.PullRequest, error) {
	if input.Draft {
		// Bitbucket Cloud's PR API has no draft concept — see
		// ErrCapabilityUnsupported's doc comment above. Wrapping this
		// package's own sentinel with domain.ErrCapabilityUnsupported lets
		// usecase/create_pull_request.go (TASK-CR-05-07) detect this via
		// errors.Is without importing this package.
		return domain.PullRequest{}, fmt.Errorf("bitbucket: draft pull requests are not supported: %w", domain.ErrCapabilityUnsupported)
	}
	body, err := json.Marshal(struct {
		// ... unchanged ...
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./internal/adapter/...
go test ./internal/adapter/github/... -run TestCreatePullRequest -v
go test ./internal/adapter/gitlab/... -run TestCreatePullRequest -v
go test ./internal/adapter/gitea/... -run TestCreatePullRequest -v
go test ./internal/adapter/azuredevops/... -run TestCreatePullRequest -v
go test ./internal/adapter/bitbucket/... -run TestCreatePullRequest -v
```

Add a test per adapter (GitHub/GitLab/Gitea/Azure DevOps) asserting
`Draft: true` serializes into the correct payload field. Add a Bitbucket
test asserting `Draft: true` returns an error satisfying
`errors.Is(err, domain.ErrCapabilityUnsupported)` without attempting the
HTTP call (assert the fake/mock HTTP transport records zero requests).
