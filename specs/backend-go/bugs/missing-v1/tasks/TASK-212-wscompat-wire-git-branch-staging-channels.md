# TASK-212: Wire `wscompat` channels for Groups A+B (branch/ref + staging, 13 channels)

**From Solution:** SOL-032 (`wscompat` wiring section)
**Priority:** P1 — ships alongside Groups A/B per SOL-032's phased rollout
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** [TASK-207](./TASK-207-git-branch-ref-operations.md) (Group A RPCs), [TASK-208](./TASK-208-git-staging-operations.md) (Group B RPCs), and — for the RPCs flagged below — [TASK-227](./TASK-227-expose-git-status-diff-commit-on-agent-part-a.md) for agent reachability. Channels for `checkout`, `localBranches`, `fastForward`, and `conflictOperation` additionally depend on open design questions in [SOL-032 §0](../solutions/SOL-032-git-channels.md#0--correction-pass-read-before-implementing-anything-below-real-agent-contract-vs-this-docs-original-design) that TASK-207/208 alone do not resolve — see the correction note below.
**Status:** `[x]` DONE — Group A and Group B are both wired in `channels_git.go`'s `registerGitDeepChannels`, now that TASK-207 implements all 9 Group-A RPCs. Group B (staging): `git.stage`/`git.bulkStage`/`git.unstage`/`git.bulkUnstage`, sharing one handler each, exactly as originally designed. Group A (`git.checkout`/`git.localBranches`/`git.fastForward`/`git.rebaseFromBase`/`git.abortRebase`/`git.abortMerge`/`git.conflictOperation`/`git.discard`/`git.bulkDiscard`) is wired against TASK-207's real (not this doc's stale pre-correction) shapes: `checkout` sends `{worktreeId,branch}` (no `create`); `localBranches` returns the richer `BranchInfo[]` unwrapped; `fastForward` sends an optional structured `pushTarget` object instead of a bare `branch` string; `conflictOperation` sends `{worktreeId}` only and returns `{operation}` (a detector); a new `git.resolveConflict` channel (not part of this doc's original 9, additive) covers the per-file ours/theirs/markResolved op via the new `ResolveConflict` RPC, whose `FailedPrecondition` (relay-unsupported) error passes through unchanged. `go build`/`go vet`/`go test` clean.

---

## ⚠️ Contract correction (read before implementing)

This task wires `wscompat` channels — the Go/wiring code below is correct
regardless of the underlying RPC's readiness (it's a pure `decodeArg` →
`client.<RPC>(ctx, ...)` → return-response wrapper, same shape as every
other channel in this file). **But per TASK-207/208's own corrections,
several of the RPCs these channels call are blocked on open design
questions from [SOL-032 §0](../solutions/SOL-032-git-channels.md#0--correction-pass-read-before-implementing-anything-below-real-agent-contract-vs-this-docs-original-design),
not just on TASK-227/228 agent-reachability work.** Do not consider a
channel "done" just because it compiles and its RPC exists — check whether
the RPC itself is still blocked:

| Channel | Underlying RPC status |
|---|---|
| `git.checkout` | ❌ BLOCKED — real `git.checkout` has no create-branch (`-b`) semantics; TASK-207's `create` flag needs a redesign (compose as a separate call, or drop from this RPC's scope), see SOL-032 §0's `checkout` row |
| `git.localBranches` | ❌ BLOCKED — real agent response is `{current, branches[]}` (names only, no ahead/behind/upstream); TASK-207's richer `BranchInfo` shape doesn't match, see SOL-032 §0 open question #4 |
| `git.fastForward` | ❌ BLOCKED — same `pushTarget` redesign as `push`/`pull`/`fetch`, see SOL-032 §0 open question #1 |
| `git.conflictOperation` | ❌ BLOCKED — real `git.conflictOperation` is a detector only (`worktreePath` → `'merge'\|'rebase'\|'cherry-pick'\|'unknown'`), not a per-file ours/theirs/markResolved resolver; TASK-207's request shape conflates two different operations, see SOL-032 §0 open question #2 |
| `git.rebaseFromBase` | ⚠️ needs TASK-227 (reachability) — mechanical param rename only once reachable, no design question |
| `git.abortRebase` / `git.abortMerge` | ⚠️ needs TASK-227 (reachability) — param rename only, closest-to-fixed of the whole set |
| `git.discard` / `git.bulkDiscard` | ⚠️ needs TASK-227 (reachability) — param rename only |
| `git.stage` / `git.bulkStage` / `git.unstage` / `git.bulkUnstage` | ⚠️ needs TASK-227 (reachability); works once `RelayExecutor` always targets the bulk variant per SOL-032 §0's note — no design question |

Channels for the 4 ❌ BLOCKED rows above should not be considered
"done" until their underlying RPC's design is resolved in TASK-207,
even though the `wscompat` wiring code itself (this file) is fine as
written below.

## Context

Once TASK-207/TASK-208's RPCs exist, wiring follows the exact Part 1
pattern (TASK-206): `decodeArg` → `client.<RPC>(ctx, &...Request{...})` →
return response. This task wires all 13 frontend channel names this group
backs — 9 for Group A (1:1 with its 9 RPCs) and 4 for Group B (`git.stage`/
`git.bulkStage`/`git.unstage`/`git.bulkUnstage`, all calling the 2
`Stage`/`Unstage` RPCs per SOL-032's channel-vs-RPC collapse).

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

Add to `registerGitChannels`, after the channels TASK-206 added:

```go
	r.Register("git.checkout", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type checkoutArgs struct {
			WorktreeID string `json:"worktreeId"`
			Ref        string `json:"ref"`
			Create     bool   `json:"create"`
		}
		in, err := decodeArg[checkoutArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Checkout(ctx, &gitgatewayv1.CheckoutRequest{
			WorktreeId: in.WorktreeID, Ref: in.Ref, Create: in.Create,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.localBranches", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type localBranchesArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[localBranchesArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.ListLocalBranches(ctx, &gitgatewayv1.ListLocalBranchesRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp.GetBranches(), nil
	})

	r.Register("git.fastForward", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type fastForwardArgs struct {
			WorktreeID string `json:"worktreeId"`
			Branch     string `json:"branch"`
		}
		in, err := decodeArg[fastForwardArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.FastForward(ctx, &gitgatewayv1.FastForwardRequest{WorktreeId: in.WorktreeID, Branch: in.Branch})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.rebaseFromBase", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rebaseArgs struct {
			WorktreeID string `json:"worktreeId"`
			BaseBranch string `json:"baseBranch"`
		}
		in, err := decodeArg[rebaseArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.RebaseFromBase(ctx, &gitgatewayv1.RebaseFromBaseRequest{WorktreeId: in.WorktreeID, BaseBranch: in.BaseBranch})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.abortRebase", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type abortRebaseArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[abortRebaseArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.AbortRebase(ctx, &gitgatewayv1.AbortRebaseRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.abortMerge", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type abortMergeArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[abortMergeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.AbortMerge(ctx, &gitgatewayv1.AbortMergeRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.conflictOperation", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type conflictOpArgs struct {
			WorktreeID string `json:"worktreeId"`
			Path       string `json:"path"`
			Operation  string `json:"operation"`
		}
		in, err := decodeArg[conflictOpArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.ConflictOperation(ctx, &gitgatewayv1.ConflictOperationRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, Operation: in.Operation,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.discard", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type discardArgs struct {
			WorktreeID string `json:"worktreeId"`
			Path       string `json:"path"`
		}
		in, err := decodeArg[discardArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Discard(ctx, &gitgatewayv1.DiscardRequest{WorktreeId: in.WorktreeID, Path: in.Path})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.bulkDiscard", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type bulkDiscardArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[bulkDiscardArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.BulkDiscard(ctx, &gitgatewayv1.BulkDiscardRequest{WorktreeId: in.WorktreeID, Paths: in.Paths})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// git.stage/git.bulkStage share one handler (a closure, not duplicated
	// bodies) — both call the same StageRequest.Paths-typed RPC, differing
	// only in call-site path-selection granularity (SOL-032 Group B).
	stageHandler := func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type stageArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[stageArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Stage(ctx, &gitgatewayv1.StageRequest{WorktreeId: in.WorktreeID, Paths: in.Paths})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	r.Register("git.stage", stageHandler)
	r.Register("git.bulkStage", stageHandler)

	unstageHandler := func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type unstageArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[unstageArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Unstage(ctx, &gitgatewayv1.UnstageRequest{WorktreeId: in.WorktreeID, Paths: in.Paths})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	r.Register("git.unstage", unstageHandler)
	r.Register("git.bulkUnstage", unstageHandler)
```

Update the `git.*` channel-count doc comment added in TASK-206 from "6" to
"19" (6 + 9 Group A + 4 Group B channel names, even though Group B is only
2 RPCs).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./internal/adapter/wscompat/...
```

Build/vet passing here only proves the Go compiles — it won't produce a
working result against a relay-connected (SSH/remote) worktree until
TASK-227 lands for every channel in this group. Local (unconnected)
worktrees don't hit the agent at all, so TASK-227's reachability gap
doesn't affect them — but `git.checkout`/`git.localBranches`/
`git.fastForward`/`git.conflictOperation`'s *shape* is still an open
question in TASK-207/SOL-032 §0, and since `GitExecutor` is one Go
interface both `localgit.Executor` and `RelayExecutor` must implement
identically, those 4 channels' final request/response shape isn't settled
for local worktrees either until TASK-207 resolves the underlying design
question (see the Contract correction table above).
