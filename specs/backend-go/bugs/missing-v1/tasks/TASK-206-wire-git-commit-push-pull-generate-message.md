# TASK-206: Wire `git.commit`/`git.push`/`git.pull`/`git.generateCommitMessage` wscompat channels

**From Solution:** SOL-032 (Part 1)
**Priority:** P0 — zero new backend risk, ship first per SOL-032's phased-rollout recommendation
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** none for the wscompat wiring itself — **but see BUG-036 /
[TASK-227](./TASK-227-expose-git-status-diff-commit-on-agent-part-a.md) /
[TASK-228](./TASK-228-fix-existing-relay-param-names-and-diff-shape.md)**:
for a *connected* (relay-required) worktree, `commit`/`push`/`pull` call
`RelayExecutor`, which (a) targets agent methods not currently reachable
from backend-go's transport (TASK-227 fixes) and (b) sends the wrong
param key (`repoPath` instead of `worktreePath`) even once reachable
(TASK-228 fixes). `push`/`pull` additionally need a `pushTarget` redesign
per `SOL-032` §0 that neither TASK-227 nor TASK-228 solves — see SOL-032's
open design question #1 before considering `git.push`/`git.pull`
production-ready even after both land. `generateCommitMessage` (via
`ai.complete`) has none of these problems — ship its channel immediately;
treat `commit`/`push`/`pull`'s channels as "code complete, runtime-blocked"
in the meantime. Local (unconnected) worktrees are unaffected by any of
this.
**Status:** `[x]` DONE — `git.commit`/`git.push`/`git.pull`/`git.generateCommitMessage` registered, but in a NEW file `channels_git.go`'s `registerGitDeepChannels` rather than directly inside `channels.go`'s `registerGitChannels` as this task's own sketch shows — editing `channels.go` directly was off-limits for this pass (other agents register other namespaces there in parallel worktrees). `go build`/`go vet`/`go test` clean for `api-gateway`. Still needs the integration pass to add one `registerGitDeepChannels(r, gitClient)` call into `RegisterRealChannels` (after the existing `registerGitChannels` call, so this file's corrected `git.diff` registration wins — see `channels_git.go`'s package doc comment).

---

## Context

Per BUG-032, `Commit`/`Push`/`Pull`/`GenerateCommitMessage` already have real
usecases, gRPC methods, and REST routes (`git_routes.go`) — only the
`wscompat` channel registration is missing, so the frontend's
`git.commit`/`git.push`/`git.pull`/`git.generateCommitMessage` calls fall
into `notImplementedHandler`. This is a pure wrapper addition, following
`registerGitChannels`'s existing `git.status`/`git.diff` pattern exactly
(`channels.go:221-252`).

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Add the 4 channel registrations to `registerGitChannels`

Add immediately after the existing `git.diff` registration (inside
`registerGitChannels`, before its closing `}`):

```go
	r.Register("git.commit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commitArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Message    string   `json:"message"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[commitArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Commit(ctx, &gitgatewayv1.CommitRequest{
			WorktreeId: in.WorktreeID, Message: in.Message, Paths: in.Paths,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.push", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type pushArgs struct {
			WorktreeID string `json:"worktreeId"`
			Remote     string `json:"remote"`
			Branch     string `json:"branch"`
		}
		in, err := decodeArg[pushArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Push(ctx, &gitgatewayv1.PushRequest{
			WorktreeId: in.WorktreeID, Remote: in.Remote, Branch: in.Branch,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.pull", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type pullArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[pullArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Pull(ctx, &gitgatewayv1.PullRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.generateCommitMessage", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type genArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[genArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GenerateCommitMessage(ctx, &gitgatewayv1.GenerateCommitMessageRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

No `gatewaygrpc.AttachIdentity` call — `gitgateway.proto`'s messages carry
no tenant field, and every existing `git.*`/`task.*`/`automation.*` channel
in this file calls its client with the bare inbound `ctx` (the one
documented exception, `devServer.*`, does not apply here).

### Step 2: Update the file's channel-count doc comment

Find the comment above `registerGitChannels`:

```go
// ── git.* (subset: status/diff — the two ops git-gateway-service
// implements for real against the local git binary; commit/push/pull relay
// to the Dev Server Agent, still a stub) ────────────────────────────────
```

Replace with:

```go
// ── git.* (6 of 34 methods: status/diff (local-only) plus
// commit/push/pull/generateCommitMessage, all backed by real usecases per
// BUG-032 Part 1. The remaining ~28 methods — branch/ref, staging,
// history/compare, remote, and AI-assist operations — are tracked in
// SOL-032 Part 2 / TASK-207 through TASK-216) ───────────────────────────
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./internal/adapter/wscompat/...
go test ./internal/adapter/wscompat/... -run TestGit -v
```

Expected: clean build; existing `git.status`/`git.diff` channel tests still
pass (no new tests required by this task — see TASK-216 for `git.*` channel
test coverage of the newly wired channels).
