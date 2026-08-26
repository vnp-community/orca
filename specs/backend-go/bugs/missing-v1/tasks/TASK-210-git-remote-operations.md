# TASK-210: Add Group D — remote RPCs to `git-gateway-service` (3 methods)

**From Solution:** SOL-032 (Part 2, Group D)
**Priority:** P2 — read-heavy, lower urgency than Groups A/B per SOL-032's phased rollout
**Service:** `git-gateway-service`
**File:** `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`, `internal/usecase/ports.go`, `internal/usecase/fetch.go`, `internal/usecase/remote_commit_url.go`, `internal/usecase/remote_file_url.go` (new), `internal/adapter/localgit/executor.go`, `internal/adapter/grpcclient/relay_executor.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** [TASK-227](./TASK-227-expose-git-status-diff-commit-on-agent-part-a.md) (agent-side reachability for `fetch`) — `remoteCommitUrl`/`remoteFileUrl` have no agent-side dependency at all (pure local computation, see correction section below). Also still touches the same shared files as TASK-207/208/209/211 — rebase onto whichever has already merged.
**Status:** `[partial]` `remoteCommitUrl`/`remoteFileUrl` fully implemented — proto, usecases, `localgit.Executor` (real `remoteWebBaseURL` host pattern-matching for GitHub/GitLab/Bitbucket, tested against HTTPS and SSH remote URL forms), `RelayExecutor`, gRPC adapter, `main.go` wiring; `go build`/`go vet`/`go test` clean. Test coverage verified/closed this pass: `localgit.executor_test.go` (GitHub-HTTPS/GitHub-SSH/Bitbucket URL forms), `grpcclient_test.go` (`RelayExecutor.RemoteCommitURL`/`RemoteFileURL` wire-shape tests added now — worktreePath/sha and worktreePath/path/ref params, URL passthrough), `grpc/server_test.go` (`RemoteCommitUrl`/`RemoteFileUrl` translation tests already present). `fetch` deliberately NOT implemented at all (not just its `RelayExecutor` leg) — BLOCKED on TASK-227 (reachability) AND the unresolved `pushTarget` redesign (SOL-032 §0 open question #1), per this task's own Contract correction section.

---

## ⚠️ Contract correction (read before implementing)

Per [SOL-032 §0](../solutions/SOL-032-git-channels.md#0--correction-pass-read-before-implementing-anything-below-real-agent-contract-vs-this-docs-original-design):

- **`remoteCommitUrl` / `remoteFileUrl` — no fix needed.** These are pure
  local string-construction (resolve `origin`'s URL, pattern-match the
  host, build a permalink) with **no agent method on either Part A or
  Part B**. This task's Step 4/5 design (implemented in `localgit.Executor`,
  a passthrough attempt in `RelayExecutor`) was already correctly designed
  as local-only from the start. Nobody needs to "fix" these two — they are
  fine as written below.
- **`fetch` — ❌ BLOCKED, do not implement Step 5's `RelayExecutor.Fetch` as
  written below without resolving this first.** Two separate problems:
  1. Not reachable on the agent's Part A dispatcher today — needs
     [TASK-227](./TASK-227-expose-git-status-diff-commit-on-agent-part-a.md).
  2. Even once reachable, the real `git.fetch` takes `worktreePath,
     pushTarget?` and **always prunes** (`git fetch --prune [remote]`) —
     there is no separate `prune` bool to send, and the wire param key is
     `worktreePath`, not `repoPath`. This is the same `pushTarget` concept
     as `push`/`pull`/`fastForward` — **SOL-032 §0's open design question
     #1**, not yet resolved. Do not invent a `PushTargetInput` shape here;
     that decision needs `git-handler-push-target.ts` read in full, which
     this task does not have loaded. See the inline `// ⚠️ BLOCKED` comment
     on `RelayExecutor.Fetch` below.

## Context

`fetch` is a real git network operation needing host dispatch like every
other group. `remoteCommitUrl`/`remoteFileUrl` are pure string construction
from the worktree's configured remote URL — SOL-032 recommends keeping them
in `git-gateway-service` (option (a) in the solution doc) since this
service already resolves "what does this worktree's remote look like" for
every other method, rather than duplicating GitHub/GitLab/Bitbucket URL
templates client-side. This task implements option (a).

## Changes to make

### Step 1: Proto

Add to the `GitGatewayService` service block:

```protobuf
  rpc Fetch(FetchRequest) returns (FetchResponse);
  rpc RemoteCommitUrl(RemoteCommitUrlRequest) returns (RemoteUrlResponse);
  rpc RemoteFileUrl(RemoteFileUrlRequest) returns (RemoteUrlResponse);
```

Append messages:

```protobuf
message FetchRequest {
  string worktree_id = 1;
  string remote = 2; // empty = default remote
  bool prune = 3;
}
message FetchResponse {
  bool success = 1;
}

message RemoteCommitUrlRequest { string worktree_id = 1; string sha = 2; }
message RemoteFileUrlRequest   { string worktree_id = 1; string path = 2; string ref = 3; }
message RemoteUrlResponse      { string url = 1; }
```

### Step 2: Extend `GitExecutor` — `internal/usecase/ports.go`

```go
	Fetch(ctx context.Context, repoPath, remote string, prune bool) (domain.SimpleResult, error)
	RemoteCommitURL(ctx context.Context, repoPath, sha string) (string, error)
	RemoteFileURL(ctx context.Context, repoPath, path, ref string) (string, error)
```

### Step 3: Usecases — `internal/usecase/`

`fetch.go` (dispatch shape, identical to `commit.go`):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type FetchInput struct {
	WorktreeID string
	Remote     string
	Prune      bool
}

type Fetch struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewFetch(resolver ConnectionResolver, local, relay GitExecutor) *Fetch {
	return &Fetch{resolver: resolver, local: local, relay: relay}
}

func (uc *Fetch) Execute(ctx context.Context, in FetchInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.Fetch(ctx, repoPath, in.Remote, in.Prune)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_FETCH_FAILED", "failed to fetch", err)
	}
	return result, nil
}
```

`remote_commit_url.go` / `remote_file_url.go` follow the same
validate/dispatch/call/wrap shape, returning a plain `string`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type RemoteCommitURLInput struct {
	WorktreeID string
	SHA        string
}

type RemoteCommitURL struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewRemoteCommitURL(resolver ConnectionResolver, local, relay GitExecutor) *RemoteCommitURL {
	return &RemoteCommitURL{resolver: resolver, local: local, relay: relay}
}

func (uc *RemoteCommitURL) Execute(ctx context.Context, in RemoteCommitURLInput) (string, error) {
	if in.WorktreeID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.SHA == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_SHA", "sha is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	url, err := executor.RemoteCommitURL(ctx, repoPath, in.SHA)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_REMOTE_URL_FAILED", "failed to resolve remote commit url", err)
	}
	return url, nil
}
```

`remote_file_url.go` mirrors this with `Path`/`Ref` fields instead of `SHA`.

### Step 4: `localgit.Executor`

```go
// Fetch runs `git fetch [remote] [--prune]`.
func (e *Executor) Fetch(ctx context.Context, repoPath, remote string, prune bool) (domain.SimpleResult, error) {
	args := []string{"fetch"}
	if remote != "" {
		args = append(args, remote)
	}
	if prune {
		args = append(args, "--prune")
	}
	if _, err := e.run(ctx, repoPath, args...); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// RemoteCommitURL resolves origin's URL and pattern-matches the host
// (github.com/gitlab.com/bitbucket.org) to build a commit permalink.
func (e *Executor) RemoteCommitURL(ctx context.Context, repoPath, sha string) (string, error) {
	base, err := e.remoteWebBaseURL(ctx, repoPath)
	if err != nil {
		return "", err
	}
	return base + "/commit/" + sha, nil
}

// RemoteFileURL resolves origin's URL and builds a file-at-ref permalink.
// GitHub/GitLab both use "/blob/<ref>/<path>"; Bitbucket uses
// "/src/<ref>/<path>" — branch on host.
func (e *Executor) RemoteFileURL(ctx context.Context, repoPath, path, ref string) (string, error) {
	base, err := e.remoteWebBaseURL(ctx, repoPath)
	if err != nil {
		return "", err
	}
	if strings.Contains(base, "bitbucket.org") {
		return base + "/src/" + ref + "/" + path, nil
	}
	return base + "/blob/" + ref + "/" + path, nil
}

// remoteWebBaseURL converts `git remote get-url origin`'s SSH or HTTPS form
// into a browsable https://<host>/<org>/<repo> base URL.
func (e *Executor) remoteWebBaseURL(ctx context.Context, repoPath string) (string, error) {
	raw, err := e.run(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(raw)
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "git@") {
		// git@host:org/repo -> https://host/org/repo
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
		url = "https://" + url
	}
	return url, nil
}
```

Add `"strings"` to imports if not already present (it already is, per the
existing file's imports).

### Step 5: `RelayExecutor`

```go
func (r *RelayExecutor) Fetch(ctx context.Context, repoPath, remote string, prune bool) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	// ⚠️ BLOCKED — see this task's Contract correction section and
	// SOL-032 §0 open question #1. This param shape is wrong on two
	// counts, neither of which this task resolves: (1) wire key must be
	// "worktreePath", not "repoPath"; (2) the real agent takes
	// worktreePath + an optional structured pushTarget and ALWAYS prunes
	// (git fetch --prune [remote]) — there is no separate "prune" bool to
	// send. Do not ship this as-is; needs the pushTarget design first.
	err := r.relay(ctx, repoPath, "git.fetch", map[string]any{
		"repoPath": repoPath, "remote": remote, "prune": prune,
	}, &result)
	return result, err
}

func (r *RelayExecutor) RemoteCommitURL(ctx context.Context, repoPath, sha string) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	err := r.relay(ctx, repoPath, "git.remoteCommitUrl", map[string]any{
		"repoPath": repoPath, "sha": sha,
	}, &result)
	return result.URL, err
}

func (r *RelayExecutor) RemoteFileURL(ctx context.Context, repoPath, path, ref string) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	err := r.relay(ctx, repoPath, "git.remoteFileUrl", map[string]any{
		"repoPath": repoPath, "path": path, "ref": ref,
	}, &result)
	return result.URL, err
}
```

`remoteCommitUrl`/`remoteFileUrl` are not among BUG-032's 8
confirmed-agent-handler methods — same "relay and see" caveat as
`upstreamStatus` in TASK-209 applies here too, but lower-risk in practice
since `RelayExecutor` only takes this path when the worktree is on a
connected remote host (`Connected=true`); a local-only worktree always uses
`localgit.Executor.remoteWebBaseURL` above regardless.

### Step 6: gRPC adapter — `internal/adapter/grpc/server.go`

```go
func (s *Server) Fetch(ctx context.Context, req *gitgatewayv1.FetchRequest) (*gitgatewayv1.FetchResponse, error) {
	result, err := s.fetch.Execute(ctx, usecase.FetchInput{
		WorktreeID: req.GetWorktreeId(), Remote: req.GetRemote(), Prune: req.GetPrune(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.FetchResponse{Success: result.Success}, nil
}

func (s *Server) RemoteCommitUrl(ctx context.Context, req *gitgatewayv1.RemoteCommitUrlRequest) (*gitgatewayv1.RemoteUrlResponse, error) {
	url, err := s.remoteCommitURL.Execute(ctx, usecase.RemoteCommitURLInput{
		WorktreeID: req.GetWorktreeId(), SHA: req.GetSha(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.RemoteUrlResponse{Url: url}, nil
}

func (s *Server) RemoteFileUrl(ctx context.Context, req *gitgatewayv1.RemoteFileUrlRequest) (*gitgatewayv1.RemoteUrlResponse, error) {
	url, err := s.remoteFileURL.Execute(ctx, usecase.RemoteFileURLInput{
		WorktreeID: req.GetWorktreeId(), Path: req.GetPath(), Ref: req.GetRef(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.RemoteUrlResponse{Url: url}, nil
}
```

Add `fetch *usecase.Fetch`, `remoteCommitURL *usecase.RemoteCommitURL`,
`remoteFileURL *usecase.RemoteFileURL` fields to `Server` and 3 params to
`New`.

### Step 7: Composition root — `cmd/server/main.go`

```go
	fetchUC := usecase.NewFetch(resolver, local, relay)
	remoteCommitURLUC := usecase.NewRemoteCommitURL(resolver, local, relay)
	remoteFileURLUC := usecase.NewRemoteFileURL(resolver, local, relay)
```

Extend `gitgatewaygrpc.New(...)` with all 3.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
cd services/git-gateway-service
go build ./... && go vet ./...
```

Build/vet passing here only proves the Go compiles. `git.fetch` won't
produce a working result against a real relay-connected worktree until
TASK-227 lands (agent reachability) AND SOL-032 §0's open design question
#1 (`pushTarget`) is resolved — treat `Fetch` as "code complete,
runtime-blocked" until both close. `remoteCommitUrl`/`remoteFileUrl` have
no such caveat — they're local-only computation and work today regardless
of TASK-227.
