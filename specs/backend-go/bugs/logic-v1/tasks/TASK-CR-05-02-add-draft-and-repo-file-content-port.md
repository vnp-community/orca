# TASK-CR-05-02: Add `Draft`/`ErrCapabilityUnsupported`/`GetRepoFileContent` to `usecase/ports.go` and `domain`

**From Solution:** SOL-CR-05
**Priority:** P0
**Service:** `scm-integration-service`
**File:** `backend-go/services/scm-integration-service/internal/usecase/ports.go`, `backend-go/services/scm-integration-service/internal/domain/scm.go`
**Depends on:** TASK-CR-05-01
**Status:** `[x]` DONE — domain.ErrCapabilityUnsupported + PullRequest.Draft added; ports.go CreatePullRequestInput.Draft + ScmProvider.GetRepoFileContent added; domain+usecase packages compile in isolation

---

## Context

`ports.go`'s package doc comment states usecase/ code "never imports a
provider package (github, gitlab, ...) directly — it depends only on the
`ScmProvider` interface". SOL-CR-05's own sketch checks
`errors.Is(err, domain.ErrCapabilityUnsupported)` in `create_pull_request.go`,
but no such shared sentinel exists today — only each provider adapter's own
package-level `ErrCapabilityUnsupported` (`bitbucket.ErrCapabilityUnsupported`,
`azuredevops.ErrCapabilityUnsupported`, etc.), which the usecase layer
cannot import without violating that rule. This task adds a shared
`domain.ErrCapabilityUnsupported` sentinel that an adapter wraps its own
package-level error with when the capability genuinely isn't supported —
correcting SOL-CR-05's sketch to something that actually compiles under
this service's own architecture rule.

## Changes to make

### 1. `internal/domain/scm.go`

```go
var (
	ErrInvalidProvider = errors.New("domain: invalid scm provider")
	ErrEmptyRepo       = errors.New("domain: repo is required")
	ErrEmptyTitle      = errors.New("domain: title is required")
	// ErrCapabilityUnsupported is a shared sentinel a provider adapter wraps
	// its own package-level "not supported" error with (via %w), so
	// usecase/ code can detect the condition via errors.Is without
	// importing the adapter package directly — see
	// scm-integration-service.md §4's ErrCapabilityUnsupported degrade
	// pattern and this file's package doc comment on why usecase/ never
	// imports adapter packages.
	ErrCapabilityUnsupported = errors.New("domain: capability not supported by this provider")
)
```

Add `Draft` to `domain.PullRequest`:

```go
type PullRequest struct {
	ID         string
	Provider   ScmProvider
	Repo       string
	Title      string
	State      string
	URL        string
	HeadBranch string
	BaseBranch string
	Number     int32
	Draft      bool // NEW — BR-CR-20
}
```

### 2. `internal/usecase/ports.go`

```go
// CreatePullRequestInput — add:
type CreatePullRequestInput struct {
	Title      string
	Body       string
	HeadBranch string
	BaseBranch string
	Draft      bool // NEW — BR-CR-20
}
```

```go
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

## Verify

```bash
cd /opt/repos/orca/backend-go/services/scm-integration-service
go build ./...
```

Expected: this alone will NOT build clean yet — every `ScmProvider`
implementation (github/gitlab/bitbucket/azuredevops/gitea) must add
`GetRepoFileContent` before `go build ./...` passes. That's TASK-CR-05-04.
Run `go build ./internal/domain/... ./internal/usecase/...` here to confirm
just this task's two files compile in isolation.
