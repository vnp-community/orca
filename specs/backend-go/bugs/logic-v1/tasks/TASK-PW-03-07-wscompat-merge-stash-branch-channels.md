# TASK-PW-03-07: Wire `git.merge`/`git.stash.push`/`git.stash.pop`/`git.branch.create`/`git.branch.delete` wscompat channels

**From Solution:** SOL-PW-03
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go`
**Depends on:** TASK-PW-03-06
**Status:** `[ ]` TODO

---

## Context

Five new unary channels in `registerGitDeepChannels`, identical shape to
the existing `git.checkout`/`git.abortMerge` entries in this file
(`channels_git.go:151-165`, `:242-255`). `BL-PW-03-remote-git-operations.md:19-44`
already names `git.branch.create`/`git.branch.delete`/`git.merge`/
`git.stash.push`/`git.stash.pop` as the exact frontend method names to
match.

## Changes to make

Add to `registerGitDeepChannels` in `channels_git.go`:

```go
r.Register("git.merge", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type mergeArgs struct {
		WorktreeID string `json:"worktreeId"`
		Branch     string `json:"branch"`
		NoFF       bool   `json:"noFf"`
	}
	in, err := decodeArg[mergeArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.MergeBranch(ctx, &gitgatewayv1.MergeBranchRequest{WorktreeId: in.WorktreeID, Branch: in.Branch, NoFf: in.NoFF})
	if err != nil {
		return nil, err // FAILED_PRECONDITION over relay-ssh surfaces as-is
	}
	return resp, nil
})

r.Register("git.stash.push", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type stashPushArgs struct {
		WorktreeID        string `json:"worktreeId"`
		Message           string `json:"message"`
		IncludeUntracked  bool   `json:"includeUntracked"`
	}
	in, err := decodeArg[stashPushArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.StashPush(ctx, &gitgatewayv1.StashPushRequest{
		WorktreeId: in.WorktreeID, Message: in.Message, IncludeUntracked: in.IncludeUntracked,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

r.Register("git.stash.pop", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type stashPopArgs struct {
		WorktreeID string `json:"worktreeId"`
		StashRef   string `json:"stashRef"`
	}
	in, err := decodeArg[stashPopArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.StashPop(ctx, &gitgatewayv1.StashPopRequest{WorktreeId: in.WorktreeID, StashRef: in.StashRef})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

r.Register("git.branch.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type createBranchArgs struct {
		WorktreeID string `json:"worktreeId"`
		Branch     string `json:"branch"`
		BaseRef    string `json:"baseRef"`
		Checkout   bool   `json:"checkout"`
	}
	in, err := decodeArg[createBranchArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.CreateBranch(ctx, &gitgatewayv1.CreateBranchRequest{
		WorktreeId: in.WorktreeID, Branch: in.Branch, BaseRef: in.BaseRef, Checkout: in.Checkout,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

r.Register("git.branch.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type deleteBranchArgs struct {
		WorktreeID string `json:"worktreeId"`
		Branch     string `json:"branch"`
	}
	in, err := decodeArg[deleteBranchArgs](args, 0)
	if err != nil {
		return nil, err
	}
	resp, err := client.DeleteBranch(ctx, &gitgatewayv1.DeleteBranchRequest{WorktreeId: in.WorktreeID, Branch: in.Branch})
	if err != nil {
		return nil, err
	}
	return resp, nil
})
```

Add one test per channel to `channels_git_test.go`; assert a
`FAILED_PRECONDITION` from the client surfaces unmodified to the WS
response (not swallowed into a generic error).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run 'TestGitMerge|TestGitStash|TestGitBranch' -v
```

Expected: clean build; five new tests pass.
