# TASK-193: Implement `CreateWorktree`/`RemoveWorktree` saga usecases + `DetectWorktrees`/`PrefetchCreateBase`/`ResolvePrBase`/`ResolveMrBase`

**From Solution:** SOL-031 (design part 2: "the saga/compensation coordination logic with project-service")
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `internal/usecase/ports.go`, `internal/usecase/create_worktree.go`, `remove_worktree.go`, `detect_worktrees.go`, `prefetch_create_base.go`, `resolve_pr_base.go`, `resolve_mr_base.go` (all new), `internal/adapter/localgit/executor.go`, `internal/adapter/grpcclient/relay_executor.go`, `internal/adapter/grpcclient/project_client.go` (new), `internal/adapter/scmclient/` (new package)
**Depends on:** TASK-192
**Status:** `[ ]` TODO

---

## Context

`05-data-architecture.md`'s "Cross-service data consistency" section offers
two patterns: outbox (async, no waiting caller) or a synchronous saga ("A
needs B to also succeed before A's operation can be considered complete,
and the caller is waiting"). Worktree create/remove is the second shape —
the frontend is waiting on a "creating worktree..." spinner. This task
implements the saga: `git-gateway-service` performs the real `git worktree
add`/`remove` (via `GitExecutor`, extended below), then calls
`project-service.RecordWorktreeCreated`/`RecordWorktreeRemoved` for
bookkeeping — with an explicit, best-effort compensating rollback if the
bookkeeping call fails after the git operation already succeeded.

**Source of truth, stated explicitly:** `git-gateway-service` (via the Dev
Server Agent or local exec) is authoritative for on-disk existence;
`project-service` is authoritative for bookkeeping metadata. Compensation
is best-effort, not guaranteed — a crash between the agent's `git worktree
add` succeeding and the compensating `git worktree remove` running leaves a
genuine orphan; `DetectWorktrees`/`worktree.detectedList` (TASK-195) is the
reconciliation safety net for exactly that failure window, not optional
polish.

`RemoveWorktree`'s failure-after-success case needs NO compensation, by
design: "worktree doesn't exist" is a safe terminal state to leave a stale
bookkeeping record pointing at (unlike a create failure, which leaves live,
unaccounted-for disk usage and a dangling branch).

## Changes to make

### Step 1 — `internal/usecase/ports.go`: extend `GitExecutor`, add `ProjectClient`/`SCMClient`

Find:

```go
type GitExecutor interface {
	GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error)
	GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error)
	Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error)
	Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error)
	Pull(ctx context.Context, repoPath string) (domain.PullResult, error)
}
```

Replace with (note: `ForceDeleteBranch` is deliberately NOT added here —
that is TASK-194's job, kept as its own task per this batch's assignment):

```go
type GitExecutor interface {
	GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error)
	GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error)
	Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error)
	Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error)
	Pull(ctx context.Context, repoPath string) (domain.PullResult, error)
	// New, SOL-031:
	CreateWorktree(ctx context.Context, repoPath, branch, baseRef string) (domain.WorktreeCreateResult, error)
	RemoveWorktree(ctx context.Context, worktreePath string, force bool) error
	// FetchAndResolveRef ensures ref is fetched/up to date in repoPath's
	// local clone and returns its resolved SHA — shared by
	// PrefetchCreateBase and ResolvePrBase/ResolveMrBase's "confirm the
	// platform's base branch actually exists locally" step.
	FetchAndResolveRef(ctx context.Context, repoPath, ref string) (sha string, err error)
}
```

Add `WorktreeCreateResult` to `internal/domain/domain.go`:

```go
// WorktreeCreateResult is what a successful `git worktree add` reports.
type WorktreeCreateResult struct {
	Path    string
	HeadSHA string
}
```

Add two new ports after `AICompleter`:

```go
// ProjectClient wraps project-service's worktree-bookkeeping RPCs — a new
// outbound call this service didn't previously need to make for writes
// (git-gateway-service.md §7 lists "Calls project-service" for reads only
// today). Implemented by internal/adapter/grpcclient against
// project-service's existing RecordWorktreeCreated/RecordWorktreeRemoved
// RPCs (no project-service proto change — those RPCs already exist).
type ProjectClient interface {
	GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error)
	RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch string) (domain.WorktreeRecord, error)
	RecordWorktreeRemoved(ctx context.Context, worktreeID string) error
}

// SCMClient wraps scm-integration-service's PR/MR base-branch lookups — a
// new outbound dependency edge git-gateway-service --> scm-integration-service
// that git-gateway-service.md §7's current dependency list (project-service,
// infra-fleet-service only) doesn't yet document, flagged here as a scope
// addition (SOL-031).
//
// KNOWN GAP: scm-integration-service's current proto
// (proto/orca/scmintegration/v1/scmintegration.proto) has NO RPC to fetch
// a single PR/MR's base branch by number — only ListPullRequests/
// CreatePullRequest/ListIssues exist. internal/adapter/scmclient (below)
// is therefore wired against a port that has no real backing RPC yet; its
// implementation returns apperrors.KindUnimplemented until
// scm-integration-service adds one (out of this task's scope — a
// follow-up proto task, not assumed here).
type SCMClient interface {
	GetPullRequestBase(ctx context.Context, repoID string, prNumber int32) (baseBranch, baseSHA string, err error)
	GetMergeRequestBase(ctx context.Context, repoID string, mrNumber int32) (baseBranch, baseSHA string, err error)
}
```

Add `RepoInfo`/`WorktreeRecord` to `internal/domain/domain.go`:

```go
// RepoInfo is project-service's answer to "what dev server and path
// template does this repo live under" — the minimal shape CreateWorktree
// needs to resolve where to run `git worktree add`.
type RepoInfo struct {
	ID           string
	DevServerID  string
	RepoPath     string
}

// WorktreeRecord mirrors project-service's Worktree message — the
// bookkeeping row RecordWorktreeCreated returns.
type WorktreeRecord struct {
	ID     string
	Path   string
	Branch string
}
```

Confirm `project.proto`'s actual `Repo`/`Worktree` message field names
(`backend-go/proto/orca/project/v1/project.proto`) before finalizing
`RepoInfo`/`WorktreeRecord` — the fields above are this task's best-effort
mapping, adjust if the real proto has different field names (e.g. no
`dev_server_id` on `Repo`, or a different path-resolution scheme) and note
the correction in the PR.

### Step 2 — `internal/adapter/localgit/executor.go`: implement `CreateWorktree`/`RemoveWorktree`/`FetchAndResolveRef`

Add after `Pull`:

```go
// CreateWorktree runs `git worktree add <path> -b <branch> <baseRef>` — a
// new worktree directory is created as a sibling of repoPath, named after
// the branch (mirrors the old TS backend's convention: worktree path =
// repo root's parent dir / branch name, sanitized). This scaffold uses
// repoPath + "-" + branch as the target path; adjust to match
// project-service.md's actual path-template convention if it specifies
// one more precisely — flagged as a best-effort default, not verified
// against a real path-template spec.
func (e *Executor) CreateWorktree(ctx context.Context, repoPath, branch, baseRef string) (domain.WorktreeCreateResult, error) {
	targetPath := repoPath + "-" + sanitizeBranchForPath(branch)
	if _, err := e.run(ctx, repoPath, "worktree", "add", targetPath, "-b", branch, baseRef); err != nil {
		return domain.WorktreeCreateResult{}, err
	}
	sha, err := e.run(ctx, targetPath, "rev-parse", "HEAD")
	if err != nil {
		return domain.WorktreeCreateResult{}, err
	}
	return domain.WorktreeCreateResult{Path: targetPath, HeadSHA: strings.TrimSpace(sha)}, nil
}

// RemoveWorktree runs `git worktree remove [--force] <worktreePath>`. Run
// from the MAIN repo's directory is not required — git worktree remove
// accepts an absolute path to the worktree itself.
func (e *Executor) RemoveWorktree(ctx context.Context, worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)
	_, err := e.run(ctx, worktreePath, args...)
	return err
}

// FetchAndResolveRef runs `git fetch origin <ref>` then resolves its local
// SHA via `git rev-parse FETCH_HEAD`.
func (e *Executor) FetchAndResolveRef(ctx context.Context, repoPath, ref string) (string, error) {
	if _, err := e.run(ctx, repoPath, "fetch", "origin", ref); err != nil {
		return "", err
	}
	sha, err := e.run(ctx, repoPath, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

// sanitizeBranchForPath replaces path-hostile characters ('/' from e.g.
// "feature/foo") so the worktree's directory name is filesystem-safe.
func sanitizeBranchForPath(branch string) string {
	return strings.ReplaceAll(branch, "/", "-")
}
```

All four commands used here (`worktree add`, `worktree remove`, `fetch`,
`rev-parse`) are available since Git 2.5 — well under the Git 2.25 baseline
per `docs/reference/git-compatibility.md`, consistent with this package's
existing doc comment.

### Step 3 — `internal/adapter/grpcclient/relay_executor.go`: implement `CreateWorktree`/`RemoveWorktree`/`FetchAndResolveRef`

Add after `Pull`, following this file's existing `relay(...)` helper
pattern exactly:

```go
func (r *RelayExecutor) CreateWorktree(ctx context.Context, repoPath, branch, baseRef string) (domain.WorktreeCreateResult, error) {
	var result domain.WorktreeCreateResult
	err := r.relay(ctx, repoPath, "git.worktreeAdd", map[string]any{
		"repoPath": repoPath, "branch": branch, "baseRef": baseRef,
	}, &result)
	return result, err
}

func (r *RelayExecutor) RemoveWorktree(ctx context.Context, worktreePath string, force bool) error {
	return r.relay(ctx, worktreePath, "git.worktreeRemove", map[string]any{
		"worktreePath": worktreePath, "force": force,
	}, nil)
}

func (r *RelayExecutor) FetchAndResolveRef(ctx context.Context, repoPath, ref string) (string, error) {
	var result struct {
		SHA string `json:"sha"`
	}
	err := r.relay(ctx, repoPath, "git.fetchRef", map[string]any{
		"repoPath": repoPath, "ref": ref,
	}, &result)
	return result.SHA, err
}
```

Same best-effort-param-shape caveat this file's doc comment already states
for `git.status`/`git.diff`/etc. applies here — `git.worktreeAdd`/
`git.worktreeRemove`/`git.fetchRef` are not verified against a real Dev
Server Agent handler; reconcile before removing this note.

### Step 4 — `internal/adapter/grpcclient/project_client.go` (new)

```go
// Package grpcclient — project_client.go implements usecase.ProjectClient
// against project-service's existing gRPC surface (no project-service
// proto change — RecordWorktreeCreated/RecordWorktreeRemoved/GetRepo
// already exist per project.proto).
package grpcclient

import (
	"context"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ProjectClient struct {
	client projectv1.ProjectServiceClient
}

func NewProjectClient(client projectv1.ProjectServiceClient) *ProjectClient {
	return &ProjectClient{client: client}
}

func (p *ProjectClient) GetRepo(ctx context.Context, repoID string) (domain.RepoInfo, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.RepoInfo{}, err
	}
	// Confirm the real RPC name/fields against project.proto before
	// finalizing — GetRepo is this task's assumed name; project.proto may
	// name it differently (e.g. a field on GetProjectResponse rather than
	// a dedicated RPC). Adjust this call accordingly.
	resp, err := p.client.GetRepo(ctx, &projectv1.GetRepoRequest{Id: repoID})
	if err != nil {
		return domain.RepoInfo{}, err
	}
	return domain.RepoInfo{ID: resp.GetId(), DevServerID: resp.GetDevServerId(), RepoPath: resp.GetPath()}, nil
}

func (p *ProjectClient) RecordWorktreeCreated(ctx context.Context, projectID, repoID, path, branch string) (domain.WorktreeRecord, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return domain.WorktreeRecord{}, err
	}
	resp, err := p.client.RecordWorktreeCreated(ctx, &projectv1.RecordWorktreeCreatedRequest{
		ProjectId: projectID, RepoId: repoID, Path: path, Branch: branch,
	})
	if err != nil {
		return domain.WorktreeRecord{}, err
	}
	wt := resp.GetWorktree()
	return domain.WorktreeRecord{ID: wt.GetId(), Path: wt.GetPath(), Branch: wt.GetBranch()}, nil
}

func (p *ProjectClient) RecordWorktreeRemoved(ctx context.Context, worktreeID string) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	_, err = p.client.RecordWorktreeRemoved(ctx, &projectv1.RecordWorktreeRemovedRequest{WorktreeId: worktreeID})
	return err
}
```

`withTenantMetadata` is the existing helper in
`internal/adapter/grpcclient/tenant_forwarding.go` — reuse it, do not
reimplement. Confirm `projectv1.ProjectServiceClient` has a `GetRepo` RPC
before finalizing (`project.proto` grep in TASK-192's investigation did
not confirm this specific RPC name — check
`backend-go/proto/orca/project/v1/project.proto`'s full RPC list; if no
such RPC exists, this is a genuine proto gap outside this task's scope —
flag it rather than inventing a request/response shape further).

### Step 5 — `internal/adapter/scmclient/` (new package)

```go
// Package scmclient implements usecase.SCMClient against
// scm-integration-service. KNOWN GAP (see usecase.SCMClient's doc
// comment): scm-integration-service's current proto has no RPC to fetch a
// single PR/MR's base branch by number, so both methods below return
// apperrors.KindUnimplemented until that RPC exists — this makes
// ResolvePrBase/ResolveMrBase fail cleanly (a typed, catchable error) at
// their call sites rather than blocking this task on an out-of-scope
// proto addition.
package scmclient

import (
	"context"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
	"github.com/stablyai/orca-go/common/apperrors"
)

type Client struct {
	client scmintegrationv1.ScmIntegrationServiceClient
}

func New(client scmintegrationv1.ScmIntegrationServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) GetPullRequestBase(ctx context.Context, repoID string, prNumber int32) (string, string, error) {
	return "", "", apperrors.New(apperrors.KindUnimplemented, "WORKTREE_SCM_GET_PR_BASE_UNIMPLEMENTED",
		"scm-integration-service has no RPC to resolve a single PR's base branch yet", nil)
}

func (c *Client) GetMergeRequestBase(ctx context.Context, repoID string, mrNumber int32) (string, string, error) {
	return "", "", apperrors.New(apperrors.KindUnimplemented, "WORKTREE_SCM_GET_MR_BASE_UNIMPLEMENTED",
		"scm-integration-service has no RPC to resolve a single MR's base branch yet", nil)
}
```

Confirm `apperrors.KindUnimplemented` exists in `common/apperrors` (check
alongside `KindNotFound`/`KindFailedPrecondition`); if this codebase's
`apperrors.Kind` enum has no such value, use `apperrors.KindInternal` with
a clear message instead and note the substitution.

### Step 6 — `internal/usecase/create_worktree.go` (new) — the saga

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CreateWorktreeInput struct {
	ProjectID, RepoID, Branch, BaseRef string
}

// CreateWorktree is the saga: resolve host, run `git worktree add`, then
// record bookkeeping via project-service. If bookkeeping fails AFTER the
// git operation succeeded, best-effort compensate by removing the
// just-created worktree — see this file's package doc comment (ports.go)
// and SOL-031 for the full rationale.
type CreateWorktree struct {
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewCreateWorktree(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *CreateWorktree {
	return &CreateWorktree{resolver: resolver, projects: projects, local: local, relay: relay}
}

func (uc *CreateWorktree) Execute(ctx context.Context, in CreateWorktreeInput) (domain.WorktreeResult, error) {
	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}

	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	result, err := executor.CreateWorktree(ctx, repoPath, in.Branch, in.BaseRef)
	if err != nil {
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_CREATE_FAILED", "git worktree add failed", err)
	}

	worktree, err := uc.projects.RecordWorktreeCreated(ctx, in.ProjectID, in.RepoID, result.Path, in.Branch)
	if err != nil {
		// Compensating step (05-data-architecture.md's saga pattern) — the
		// git op already succeeded; project-service has no record of it.
		if compErr := executor.RemoveWorktree(ctx, result.Path, true); compErr != nil {
			return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_FAILED",
				fmt.Sprintf("worktree created but bookkeeping failed (%v) and rollback also failed (%v) — orphaned at %s, will surface via worktree.detectedList", err, compErr, result.Path), err)
		}
		return domain.WorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_FAILED", "worktree created but bookkeeping failed; rolled back cleanly", err)
	}
	return domain.WorktreeResult{WorktreeID: worktree.ID, Path: result.Path, HeadSHA: result.HeadSHA}, nil
}
```

Add `WorktreeResult` to `internal/domain/domain.go`:

```go
type WorktreeResult struct {
	WorktreeID string
	Path       string
	HeadSHA    string
}
```

### Step 7 — `internal/usecase/remove_worktree.go` (new) — no compensation, by design

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// RemoveWorktree has no compensating action on bookkeeping failure — see
// this file's package doc comment (ports.go) for why "on-disk gone,
// bookkeeping stale" is a safe terminal state, unlike CreateWorktree's
// failure direction.
type RemoveWorktree struct {
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewRemoveWorktree(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *RemoveWorktree {
	return &RemoveWorktree{resolver: resolver, projects: projects, local: local, relay: relay}
}

func (uc *RemoveWorktree) Execute(ctx context.Context, worktreeID string, force bool) error {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	if err := executor.RemoveWorktree(ctx, repoPath, force); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, worktreeID); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return nil
}
```

### Step 8 — `internal/usecase/detect_worktrees.go` (new)

```go
package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// DetectWorktrees runs `git worktree list --porcelain` for repoID's local
// clone and returns the raw on-disk worktree paths — NO bookkeeping join
// here (that diff happens at api-gateway's edge layer, TASK-195, per
// 05-data-architecture.md's cross-service-aggregation rule: this service
// owns no project-service data to reach into).
type DetectWorktrees struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewDetectWorktrees(resolver ConnectionResolver, local, relay GitExecutor) *DetectWorktrees {
	return &DetectWorktrees{resolver: resolver, local: local, relay: relay}
}

func (uc *DetectWorktrees) Execute(ctx context.Context, repoID string) ([]string, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repoID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	paths, err := executor.(interface {
		ListWorktreePaths(ctx context.Context, repoPath string) ([]string, error)
	}).ListWorktreePaths(ctx, repoPath)
	_ = strings.TrimSpace // placeholder to keep import if ListWorktreePaths impl needs trimming — remove if unused after Step 9 lands
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "WORKTREE_DETECT_FAILED", "git worktree list failed", err)
	}
	return paths, nil
}
```

The type-assertion above is a placeholder — cleaner: add
`ListWorktreePaths(ctx context.Context, repoPath string) ([]string,
error)` directly to the `GitExecutor` interface (Step 1) instead of a
runtime assertion, and implement it in `localgit`/`relay_executor` the same
way `CreateWorktree`/`RemoveWorktree` were added in Steps 2-3 (parse `git
worktree list --porcelain`'s `worktree <path>` lines for `localgit`; relay
to `git.worktreeList` for the relay executor). Prefer that over the
type-assertion sketch above — this task's own design should not ship a
runtime type assertion where a compile-time interface method is available
one edit away. Revise Step 1/2/3 to include `ListWorktreePaths` before
finalizing this file.

### Step 9 — `internal/usecase/prefetch_create_base.go`, `resolve_pr_base.go`, `resolve_mr_base.go` (new)

```go
// internal/usecase/prefetch_create_base.go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type PrefetchCreateBase struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewPrefetchCreateBase(resolver ConnectionResolver, local, relay GitExecutor) *PrefetchCreateBase {
	return &PrefetchCreateBase{resolver: resolver, local: local, relay: relay}
}

func (uc *PrefetchCreateBase) Execute(ctx context.Context, repoID, baseRef string) (string, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repoID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	sha, err := executor.FetchAndResolveRef(ctx, repoPath, baseRef)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "WORKTREE_PREFETCH_FAILED", "failed to prefetch base ref", err)
	}
	return sha, nil
}
```

```go
// internal/usecase/resolve_pr_base.go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ResolvePrBase struct {
	scm      SCMClient
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewResolvePrBase(scm SCMClient, resolver ConnectionResolver, local, relay GitExecutor) *ResolvePrBase {
	return &ResolvePrBase{scm: scm, resolver: resolver, local: local, relay: relay}
}

func (uc *ResolvePrBase) Execute(ctx context.Context, repoID string, prNumber int32) (domain.ResolvedBase, error) {
	baseBranch, _, err := uc.scm.GetPullRequestBase(ctx, repoID, prNumber)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_PR_BASE_LOOKUP_FAILED", "failed to resolve PR base", err)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repoID)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	resolvedSHA, err := executor.FetchAndResolveRef(ctx, repoPath, baseBranch)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BASE_REF_UNRESOLVABLE", "PR base branch not resolvable in local repo", err)
	}
	return domain.ResolvedBase{Branch: baseBranch, SHA: resolvedSHA}, nil
}
```

```go
// internal/usecase/resolve_mr_base.go — identical shape to ResolvePrBase,
// via SCMClient.GetMergeRequestBase instead.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ResolveMrBase struct {
	scm      SCMClient
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewResolveMrBase(scm SCMClient, resolver ConnectionResolver, local, relay GitExecutor) *ResolveMrBase {
	return &ResolveMrBase{scm: scm, resolver: resolver, local: local, relay: relay}
}

func (uc *ResolveMrBase) Execute(ctx context.Context, repoID string, mrNumber int32) (domain.ResolvedBase, error) {
	baseBranch, _, err := uc.scm.GetMergeRequestBase(ctx, repoID, mrNumber)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_MR_BASE_LOOKUP_FAILED", "failed to resolve MR base", err)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repoID)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	resolvedSHA, err := executor.FetchAndResolveRef(ctx, repoPath, baseBranch)
	if err != nil {
		return domain.ResolvedBase{}, apperrors.New(apperrors.KindInternal, "WORKTREE_BASE_REF_UNRESOLVABLE", "MR base branch not resolvable in local repo", err)
	}
	return domain.ResolvedBase{Branch: baseBranch, SHA: resolvedSHA}, nil
}
```

Add `ResolvedBase` to `internal/domain/domain.go`:

```go
type ResolvedBase struct {
	Branch string
	SHA    string
}
```

`dispatchExecutor`'s existing signature
(`dispatchExecutor(ctx, resolver, local, relay, worktreeID string)`) is
reused as-is for `repoID` above — confirm this is semantically correct
(the resolver looks up by connection/worktree id, not repo id, per its own
doc comment) before finalizing; if `ConnectionResolver.ResolveConnection`
genuinely needs a worktree id rather than a repo id for these new
repo-scoped RPCs, this is a design gap this task's own author must resolve
(likely: `DetectWorktrees`/`PrefetchCreateBase`/`ResolvePrBase`/
`ResolveMrBase` need to resolve the DEV SERVER hosting `repoID` via
`ProjectClient.GetRepo` + `ConnectionResolver`, similar to
`CreateWorktree`'s own `uc.projects.GetRepo` call above, rather than
reusing `dispatchExecutor`'s worktree-id-shaped signature directly) — flag
and resolve this before implementation, do not silently pass a repo id
where a worktree id is expected.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./... && go vet ./...
```
