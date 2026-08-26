# TASK-213: Wire `wscompat` channels for Groups C+D+E (history/compare + remote + AI-assist, 14 channels)

**From Solution:** SOL-032 (`wscompat` wiring section)
**Priority:** P2 — read-heavy, lower urgency per SOL-032's phased rollout
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** [TASK-209](./TASK-209-git-history-compare-operations.md) (Group C RPCs), [TASK-210](./TASK-210-git-remote-operations.md) (Group D RPCs), [TASK-211](./TASK-211-git-ai-assist-operations.md) (Group E RPCs) — mixed readiness per channel, see the correction table below; not all 14 channels in this file share the same dependency.
**Status:** `[partial]` The 8 channels backed by RPCs this pass actually implemented are wired in `channels_git.go`'s `registerGitDeepChannels`: `git.history`/`git.checkIgnored`/`git.forkSync`/`git.upstreamStatus` (Group C shippable-now subset), `git.remoteCommitUrl`/`git.remoteFileUrl` (Group D shippable-now subset), `git.generatePullRequestFields`/`git.discoverCommitMessageModels` (Group E, fully shippable). Their request/response shapes match TASK-209/210/211's own Contract correction sections (e.g. `history` uses `baseRef`+no `cursor`, `checkIgnored` returns `ignoredPaths` only, `forkSync` requires `expectedUpstream`), NOT this task's own inline sketch — that sketch predates those corrections. NOT wired: `git.commitCompare`/`git.branchCompare`/`git.commitDiff`/`git.branchDiff`/`git.submoduleStatus`/`git.fetch` — none of their backing RPCs exist (all 5 history/compare methods are BLOCKED per TASK-209; `fetch` is BLOCKED per TASK-210). `go build`/`go vet`/`go test` clean.

---

## ⚠️ Contract correction (read before implementing)

The `wscompat` wiring code below is correct Go regardless of readiness —
but per TASK-209/210/211's own corrections (rolling up SOL-032 §0's
findings), this file mixes channels that are ready today with channels
still blocked on TASK-227 reachability and/or an open design question.
Per-channel status, not one blanket dependency line:

| Channel | Group | Status |
|---|---|---|
| `git.history` | C | ⚠️ needs TASK-227 (reachability); param fix once reachable — drop `cursor` (no pagination on real agent), rename `ref`→`baseRef`. Already reachable on Part A today per TASK-209's revised classification, so this is lower-risk than most of Group C. |
| `git.commitCompare` | C | ✅ reachable today (Part A) — but ❌ wrong shape: real op is one `commitId` vs. its own parent, not two arbitrary `baseSha`/`headSha`. Needs TASK-209's shape fix, not just param renames. |
| `git.branchCompare` | C | ✅ reachable today — but ❌ wrong shape: real op is HEAD-vs-single-`baseRef`, not two arbitrary branches. Needs TASK-209's shape fix. |
| `git.commitDiff` | C | ✅ reachable today — but ❌ wrong shape: real op requires a `filePath` (per-file, like `git.diff`); this channel's whole-commit `{worktreeId, sha}` shape is missing it. Needs TASK-209's shape fix. |
| `git.branchDiff` | C | ✅ reachable today — but ❌ wrong shape: two-sided + missing `filePath`, same class of issue as `branchCompare`/`commitDiff`. Needs TASK-209's shape fix. |
| `git.submoduleStatus` | C | ✅ reachable today — but ❌ wrong shape: real op is per-submodule (`worktreePath, submodulePath`), not a bulk list; blocked on SOL-032 §0 open question #3. |
| `git.checkIgnored` | C | ✅ reachable today — but ⚠️ response shape too rich: real agent returns only the ignored subset (`string[]`), not a `{path, ignored}` bool per input path. Mechanical fix per TASK-209. |
| `git.forkSync` | C | ✅ reachable today — but ⚠️ missing required `expectedUpstream` param. Mechanical fix per TASK-209. |
| `git.upstreamStatus` | C | ⚠️ needs TASK-227 (reachability); param rename + optional `pushTarget` once reachable — not otherwise blocked on a design question. |
| `git.fetch` | D | ❌ BLOCKED — needs TASK-227 (reachability) AND the same `pushTarget` redesign as `push`/`pull`/`fastForward` (SOL-032 §0 open question #1). See TASK-210's correction section. |
| `git.remoteCommitUrl` | D | ✅ unblocked, no agent dependency at all — pure local string construction, correctly designed as-is per TASK-210. |
| `git.remoteFileUrl` | D | ✅ unblocked, same as `remoteCommitUrl`. |
| `git.generatePullRequestFields` | E | ✅ unblocked — relays via `ai.complete`, already reachable on Part A. No correction needed per TASK-211. |
| `git.discoverCommitMessageModels` | E | ✅ unblocked — calls `ai-provider-service` directly, doesn't touch the agent. No correction needed per TASK-211. |

Bottom line: Group E's 2 channels are fully ready. Group D splits —
`remoteCommitUrl`/`remoteFileUrl` are ready, `fetch` is blocked on two
things. Group C is the most mixed: 6 of its 8 channels are reachable
today but carry shape bugs from this doc's original design (TASK-209
fixes these); only `history`/`upstreamStatus` need TASK-227 for
reachability, and `submoduleStatus` additionally needs an unresolved
design decision (open question #3).

## Context

Wires the remaining 14 `git.*` frontend channel names: 8 for Group C
(`history`/`commitCompare`/`branchCompare`/`commitDiff`/`branchDiff`/
`submoduleStatus`/`checkIgnored`/`forkSync`), 1 more for Group C
(`upstreamStatus`), 3 for Group D (`fetch`/`remoteCommitUrl`/
`remoteFileUrl`), and 2 for Group E (`generatePullRequestFields`/
`discoverCommitMessageModels`). `cancelGenerateCommitMessage`/
`cancelGeneratePullRequestFields` are **not** wired by this task — per
TASK-211's Context, they need no new RPC; wiring the WS envelope's own
cancellation signal into the existing `handleInvoke` context is a separate
`wscompat`-core change, out of scope here.

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

Add to `registerGitChannels`, after TASK-212's channels:

```go
	r.Register("git.history", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type historyArgs struct {
			WorktreeID string `json:"worktreeId"`
			Ref        string `json:"ref"`
			Limit      int32  `json:"limit"`
			Cursor     string `json:"cursor"`
		}
		in, err := decodeArg[historyArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.History(ctx, &gitgatewayv1.HistoryRequest{
			WorktreeId: in.WorktreeID, Ref: in.Ref, Limit: in.Limit, Cursor: in.Cursor,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.commitCompare", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commitCompareArgs struct {
			WorktreeID string `json:"worktreeId"`
			BaseSHA    string `json:"baseSha"`
			HeadSHA    string `json:"headSha"`
		}
		in, err := decodeArg[commitCompareArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CommitCompare(ctx, &gitgatewayv1.CommitCompareRequest{
			WorktreeId: in.WorktreeID, BaseSha: in.BaseSHA, HeadSha: in.HeadSHA,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.branchCompare", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type branchCompareArgs struct {
			WorktreeID string `json:"worktreeId"`
			BaseBranch string `json:"baseBranch"`
			HeadBranch string `json:"headBranch"`
		}
		in, err := decodeArg[branchCompareArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.BranchCompare(ctx, &gitgatewayv1.BranchCompareRequest{
			WorktreeId: in.WorktreeID, BaseBranch: in.BaseBranch, HeadBranch: in.HeadBranch,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.commitDiff", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commitDiffArgs struct {
			WorktreeID string `json:"worktreeId"`
			SHA        string `json:"sha"`
		}
		in, err := decodeArg[commitDiffArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CommitDiff(ctx, &gitgatewayv1.CommitDiffRequest{WorktreeId: in.WorktreeID, Sha: in.SHA})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.branchDiff", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type branchDiffArgs struct {
			WorktreeID string `json:"worktreeId"`
			BaseBranch string `json:"baseBranch"`
			HeadBranch string `json:"headBranch"`
		}
		in, err := decodeArg[branchDiffArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.BranchDiff(ctx, &gitgatewayv1.BranchDiffRequest{
			WorktreeId: in.WorktreeID, BaseBranch: in.BaseBranch, HeadBranch: in.HeadBranch,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.submoduleStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type submoduleStatusArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[submoduleStatusArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.SubmoduleStatus(ctx, &gitgatewayv1.SubmoduleStatusRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp.GetSubmodules(), nil
	})

	r.Register("git.checkIgnored", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type checkIgnoredArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[checkIgnoredArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CheckIgnored(ctx, &gitgatewayv1.CheckIgnoredRequest{WorktreeId: in.WorktreeID, Paths: in.Paths})
		if err != nil {
			return nil, err
		}
		return resp.GetResults(), nil
	})

	r.Register("git.forkSync", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type forkSyncArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[forkSyncArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.ForkSync(ctx, &gitgatewayv1.ForkSyncRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.upstreamStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type upstreamStatusArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[upstreamStatusArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.UpstreamStatus(ctx, &gitgatewayv1.UpstreamStatusRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.fetch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type fetchArgs struct {
			WorktreeID string `json:"worktreeId"`
			Remote     string `json:"remote"`
			Prune      bool   `json:"prune"`
		}
		in, err := decodeArg[fetchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Fetch(ctx, &gitgatewayv1.FetchRequest{WorktreeId: in.WorktreeID, Remote: in.Remote, Prune: in.Prune})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.remoteCommitUrl", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type remoteCommitURLArgs struct {
			WorktreeID string `json:"worktreeId"`
			SHA        string `json:"sha"`
		}
		in, err := decodeArg[remoteCommitURLArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.RemoteCommitUrl(ctx, &gitgatewayv1.RemoteCommitUrlRequest{WorktreeId: in.WorktreeID, Sha: in.SHA})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.remoteFileUrl", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type remoteFileURLArgs struct {
			WorktreeID string `json:"worktreeId"`
			Path       string `json:"path"`
			Ref        string `json:"ref"`
		}
		in, err := decodeArg[remoteFileURLArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.RemoteFileUrl(ctx, &gitgatewayv1.RemoteFileUrlRequest{WorktreeId: in.WorktreeID, Path: in.Path, Ref: in.Ref})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.generatePullRequestFields", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type genPRFieldsArgs struct {
			WorktreeID string `json:"worktreeId"`
			BaseBranch string `json:"baseBranch"`
		}
		in, err := decodeArg[genPRFieldsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GeneratePullRequestFields(ctx, &gitgatewayv1.GeneratePullRequestFieldsRequest{
			WorktreeId: in.WorktreeID, BaseBranch: in.BaseBranch,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.discoverCommitMessageModels", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		resp, err := client.DiscoverCommitMessageModels(ctx, &gitgatewayv1.DiscoverCommitMessageModelsRequest{
			TenantId: id.TenantID, UserId: id.UserID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetModels(), nil
	})
```

`git.discoverCommitMessageModels` is the one channel in this file's `git.*`
section that reads from `Identity` (`id.TenantID`/`id.UserID`) instead of
`args` — mirrors SOL-033's `automation.create` note on never trusting a
client-supplied tenant; there's no equivalent identity field to source
elsewhere for this channel.

Update the `git.*` channel-count doc comment from "19" (TASK-212) to "33"
(19 + 14). This leaves `cancelGenerateCommitMessage`/
`cancelGeneratePullRequestFields` as the 2 remaining unregistered names out
of the frontend's full 34-method `git.*` catalog — call this out explicitly
in the updated comment, e.g.:

```go
// ── git.* (33 of 34 methods wired; cancelGenerateCommitMessage/
// cancelGeneratePullRequestFields are deliberately unregistered — see
// TASK-211's Context: both operations are synchronous unary RPCs with no
// server-side job to cancel, and wiring the WS envelope's own cancellation
// signal into handleInvoke's context is a separate wscompat-core follow-up,
// not a new channel) ──────────────────────────────────────────────────────
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./internal/adapter/wscompat/...
```

Build/vet passing here only proves the Go compiles. Per the Contract
correction table above, this file's 14 channels won't all produce a
working result together: `git.generatePullRequestFields`,
`git.discoverCommitMessageModels`, `git.remoteCommitUrl`, and
`git.remoteFileUrl` are the only 4 channels safe to consider "done" once
this compiles. The rest need TASK-227 (reachability), a shape fix from
TASK-209 (`commitCompare`/`branchCompare`/`commitDiff`/`branchDiff`/
`checkIgnored`/`forkSync`/`submoduleStatus`), and/or an unresolved SOL-032
§0 open design question (`fetch`'s `pushTarget`, `submoduleStatus`'s
per-submodule granularity) before they're runtime-correct against a real
agent.
