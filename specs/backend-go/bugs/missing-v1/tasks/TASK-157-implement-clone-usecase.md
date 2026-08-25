# TASK-157: Add `Clone`/`InitRepo` to `GitExecutor`, a `DevServerReachability` port, and the `Clone`/`InitRepo` usecases (Bucket 3)

**From Solution:** SOL-023 (Bucket 3)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `services/git-gateway-service/internal/usecase/ports.go`, `services/git-gateway-service/internal/usecase/clone.go` (new), `services/git-gateway-service/internal/usecase/init_repo.go` (new), `services/git-gateway-service/internal/adapter/localgit/executor.go`, `services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`, `services/git-gateway-service/internal/adapter/grpcclient/resolver.go`
**Depends on:** TASK-156 (needs generated `CloneRequest`/`InitRepoRequest` stubs)
**Status:** `[needs merge with git.*/files.* group]` — `Clone`/`InitRepo` added to `GitExecutor`, `DevServerReachability` port + `grpcclient.DevServerReachability` adapter added, `Clone`/`InitRepo` usecases added, `localgit.Executor.Clone`/`InitRepo` (real `git clone`/`git init` via os/exec) and `grpcclient.RelayExecutor.Clone`/`InitRepo` implemented. `go build`/`go vet`/`go test` all green in this worktree. Touches `internal/adapter/grpc/server.go` and `cmd/server/main.go`, same files the parallel git.*/files.* group is editing — needs merge, see TASK-156.

---

## Context

Every existing usecase in this package (`get_status.go`, `commit.go`, ...)
resolves via `ConnectionResolver.ResolveConnection(ctx, worktreeID)`
(`ports.go:91`'s `dispatchExecutor` helper) — but `Clone` and `InitRepo`
create a worktree that doesn't exist yet, so there is no `worktreeID` (=
`connectionId` in this scaffold, see `grpcclient/resolver.go`'s doc
comment) to resolve through. Both instead carry `dev_server_id`
(`CloneRequest`/`InitRepoRequest`, TASK-156).

There is no existing infra-fleet-service RPC that resolves reachability by
`dev_server_id` directly (`ResolveConnection` only takes a
`connection_id`). This task adds a small new port,
`DevServerReachability`, backed by `GetFleetHealth` (the closest existing
read — `infra-fleet-service.md` §8's per-dev-server `Reachable` sample) —
this is the pragmatic resolution to an ambiguity SOL-023 flagged but did
not fully resolve; if a real `ResolveDevServer`-style RPC lands on
`infra-fleet-service` later, swap this port's implementation, not its
usecase-facing shape.

## Changes to make

### `internal/usecase/ports.go` — extend `GitExecutor`, add `DevServerReachability`

Add to the `GitExecutor` interface:

```go
	// Clone and InitRepo create a worktree that doesn't exist yet — unlike
	// every other GitExecutor method, they are not called with a repoPath
	// resolved from an existing worktreeId/connectionId. See DevServerReachability.
	Clone(ctx context.Context, url, destPath string) (worktreePath, defaultBranch string, err error)
	InitRepo(ctx context.Context, destPath, defaultBranch string) (path, resolvedDefaultBranch string, err error)
```

Add a new port below `ConnectionResolver`:

```go
// DevServerReachability resolves whether devServerID is a live,
// agent-reachable remote host (relay branch) or this service should
// operate on its own filesystem (local branch) — used only by Clone/
// InitRepo, which have no worktree/connectionId yet to resolve through
// ConnectionResolver (a repo doesn't exist until one of these two calls
// creates it). Backed by infra-fleet-service's GetFleetHealth (per-dev-server
// reachability) — the closest existing read to "is this host up" without
// inventing a new infra-fleet-service RPC.
type DevServerReachability interface {
	IsReachable(ctx context.Context, devServerID string) (bool, error)
}
```

### New file `internal/usecase/clone.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// CloneInput mirrors the gRPC request 1:1.
type CloneInput struct {
	DevServerID string
	URL         string
	DestPath    string
}

type CloneResult struct {
	WorktreePath  string
	DefaultBranch string
}

// Clone dispatches to whichever GitExecutor answers for DevServerID —
// same resolve-then-dispatch shape as every other usecase in this package,
// keyed by DevServerReachability instead of ConnectionResolver since no
// worktree/connectionId exists yet (see ports.go's doc comment on this
// port).
type Clone struct {
	reachability DevServerReachability
	local        GitExecutor
	relay        GitExecutor
}

func NewClone(reachability DevServerReachability, local, relay GitExecutor) *Clone {
	return &Clone{reachability: reachability, local: local, relay: relay}
}

func (uc *Clone) Execute(ctx context.Context, in CloneInput) (CloneResult, error) {
	if in.DevServerID == "" {
		return CloneResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_DEV_SERVER_ID", "dev_server_id is required", nil)
	}

	reachable, err := uc.reachability.IsReachable(ctx, in.DevServerID)
	if err != nil {
		return CloneResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve dev server reachability", err)
	}

	executor := uc.local
	if reachable {
		executor = uc.relay
	}
	worktreePath, defaultBranch, err := executor.Clone(ctx, in.URL, in.DestPath)
	if err != nil {
		return CloneResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CLONE_FAILED", "failed to clone repository", err)
	}
	return CloneResult{WorktreePath: worktreePath, DefaultBranch: defaultBranch}, nil
}
```

### New file `internal/usecase/init_repo.go`

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// InitRepoInput mirrors the gRPC request 1:1.
type InitRepoInput struct {
	DevServerID   string
	DestPath      string
	DefaultBranch string
}

type InitRepoResult struct {
	Path          string
	DefaultBranch string
}

// InitRepo runs `git init` at DestPath on whichever host DevServerID
// resolves to — same dispatch shape as Clone. The caller (project-service
// context, per CloneRequest's own doc comment in gitgateway.proto) is
// responsible for then calling ProjectService.AddRepo with the returned
// path — this usecase only performs the git operation.
type InitRepo struct {
	reachability DevServerReachability
	local        GitExecutor
	relay        GitExecutor
}

func NewInitRepo(reachability DevServerReachability, local, relay GitExecutor) *InitRepo {
	return &InitRepo{reachability: reachability, local: local, relay: relay}
}

func (uc *InitRepo) Execute(ctx context.Context, in InitRepoInput) (InitRepoResult, error) {
	if in.DevServerID == "" {
		return InitRepoResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_DEV_SERVER_ID", "dev_server_id is required", nil)
	}

	reachable, err := uc.reachability.IsReachable(ctx, in.DevServerID)
	if err != nil {
		return InitRepoResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve dev server reachability", err)
	}

	executor := uc.local
	if reachable {
		executor = uc.relay
	}
	path, defaultBranch, err := executor.InitRepo(ctx, in.DestPath, in.DefaultBranch)
	if err != nil {
		return InitRepoResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_INIT_REPO_FAILED", "failed to init repository", err)
	}
	return InitRepoResult{Path: path, DefaultBranch: defaultBranch}, nil
}
```

### `internal/adapter/localgit/executor.go` — implement `Clone`/`InitRepo`

Add, following `run`'s existing helper (note `Clone`/`InitRepo` run with no
prior `repoPath` to `cd` into — `run` takes `repoPath` as the working
directory, so these two need a direct `exec.CommandContext` call, or a
`run`-variant that takes an explicit working directory instead of assuming
one already exists):

```go
// Clone runs `git clone <url> <destPath>` and reads back the resulting
// default branch with `git symbolic-ref --short HEAD`.
func (e *Executor) Clone(ctx context.Context, url, destPath string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", "clone", url, destPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	branch, err := e.run(ctx, destPath, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return destPath, "", err
	}
	return destPath, strings.TrimSpace(branch), nil
}

// InitRepo runs `git init` (optionally with -b <defaultBranch>, Git 2.28+;
// falls back to a plain `git init` + `git symbolic-ref` rename for older
// Git per docs/reference/git-compatibility.md's 2.25 baseline) at destPath.
func (e *Executor) InitRepo(ctx context.Context, destPath, defaultBranch string) (string, string, error) {
	args := []string{"init"}
	if defaultBranch != "" {
		args = append(args, "-b", defaultBranch)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = destPath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("git init: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	branch, err := e.run(ctx, destPath, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return destPath, defaultBranch, nil // best-effort: init succeeded even if branch read fails
	}
	return destPath, strings.TrimSpace(branch), nil
}
```

`cmd.Dir = destPath` on a not-yet-existing directory fails — the caller
(usecase or an earlier step) must `os.MkdirAll(destPath, 0o755)` before
invoking `InitRepo`; add that inside this method, before the `exec.CommandContext` call, guarded with `os.MkdirAll`.

### `internal/adapter/grpcclient/relay_executor.go` — implement `Clone`/`InitRepo`

Follow the exact `relay(...)` helper every other method already uses.
Unlike those methods, there is no `repoPath`/`connectionId` to key the
relay on yet — pass `destPath` as the best available identifier,
consistent with this file's existing "repoPath doubles as connectionId"
convention (see its own doc comment's "Known gap"):

```go
func (r *RelayExecutor) Clone(ctx context.Context, url, destPath string) (string, string, error) {
	var result struct {
		WorktreePath  string `json:"worktreePath"`
		DefaultBranch string `json:"defaultBranch"`
	}
	err := r.relay(ctx, destPath, "git.clone", map[string]any{
		"url": url, "destPath": destPath,
	}, &result)
	return result.WorktreePath, result.DefaultBranch, err
}

func (r *RelayExecutor) InitRepo(ctx context.Context, destPath, defaultBranch string) (string, string, error) {
	var result struct {
		Path          string `json:"path"`
		DefaultBranch string `json:"defaultBranch"`
	}
	err := r.relay(ctx, destPath, "git.init", map[string]any{
		"destPath": destPath, "defaultBranch": defaultBranch,
	}, &result)
	return result.Path, result.DefaultBranch, err
}
```

### New adapter method for `DevServerReachability`

Add to `internal/adapter/grpcclient/resolver.go` (or a new
`reachability.go` in the same package):

```go
// DevServerReachability implements usecase.DevServerReachability by reading
// infra-fleet-service's GetFleetHealth and checking the sample for
// devServerID — see usecase/ports.go's doc comment for why this port
// exists instead of a worktree-keyed ConnectionResolver lookup.
type DevServerReachability struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewDevServerReachability(client infrafleetv1.InfraFleetServiceClient) *DevServerReachability {
	return &DevServerReachability{client: client}
}

func (d *DevServerReachability) IsReachable(ctx context.Context, devServerID string) (bool, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return false, err
	}
	resp, err := d.client.GetFleetHealth(ctx, &infrafleetv1.GetFleetHealthRequest{})
	if err != nil {
		return false, fmt.Errorf("grpcclient: GetFleetHealth: %w", err)
	}
	for _, h := range resp.GetDevServers() {
		if h.GetDevServerId() == devServerID {
			return h.GetReachable(), nil
		}
	}
	return false, nil // no sample yet for this dev server — treat as not reachable, not an error
}
```

Verify `GetFleetHealthResponse`'s actual field name for the health-sample
list (`GetDevServers()` above is illustrative — match the generated
accessor) before using it.

### `cmd/server/main.go` — wire the new usecases

Construct `usecase.NewClone(devServerReachability, localExecutor,
relayExecutor)` and `usecase.NewInitRepo(...)` next to this service's
existing usecase constructors, and a
`grpcclient.NewDevServerReachability(infraFleetClient)` next to
`grpcclient.NewConnectionResolver(...)`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./... && go vet ./...
```
