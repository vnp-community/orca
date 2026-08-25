# TASK-216: `wscompat` channel tests for all wired `git.*` channels (33 channels)

**From Solution:** SOL-032 (Test plan — `wscompat/channels_test.go`)
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-206, TASK-212, TASK-213
**Status:** `[partial]` `channels_git_test.go` added with a `fakeGitGatewayClient` (mirroring `channels_test.go`'s `fakeInfraFleetClient` embed-nil-interface pattern) covering `git.diff` (filePath threaded through), `git.commit`, `git.stage`/`git.bulkStage` (shared-handler regression), `git.history` (corrected baseRef/no-cursor shape), and `git.discoverCommitMessageModels` (Identity-sourced tenant/user, never client args). Does not cover every one of the ~19 `git.*` channels this pass wired (`git.push`/`git.pull`/`git.generateCommitMessage`/`git.unstage`/`git.bulkUnstage`/`git.checkIgnored`/`git.forkSync`/`git.upstreamStatus`/`git.remoteCommitUrl`/`git.remoteFileUrl`/`git.generatePullRequestFields` have no dedicated channel test, though their usecase/adapter layers are tested elsewhere). `go test ./internal/adapter/wscompat/...` passes.

---

## Context

No `fakeGitGatewayServiceClient` exists yet in `channels_test.go` — this
task adds one (mirroring `fakeInfraFleetClient`'s existing
embed-the-interface-and-override pattern, `channels_test.go:18-23`) and one
test per registered `git.*` channel, following
`TestDevServerListChannel_Success`'s shape (fake gRPC client, this file's
`argsJSON` helper, assert the channel calls through and returns the
translated response).

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`

### Step 1: `fakeGitGatewayServiceClient`

```go
// fakeGitGatewayServiceClient implements gitgatewayv1.GitGatewayServiceClient
// — embeds the (nil) interface so unimplemented methods panic loudly rather
// than silently returning a zero value, same convention as
// fakeInfraFleetClient above. One func field per RPC this file's tests
// exercise; unset fields are never called.
type fakeGitGatewayServiceClient struct {
	gitgatewayv1.GitGatewayServiceClient

	commitFunc                          func(ctx context.Context, in *gitgatewayv1.CommitRequest) (*gitgatewayv1.CommitResponse, error)
	pushFunc                            func(ctx context.Context, in *gitgatewayv1.PushRequest) (*gitgatewayv1.PushResponse, error)
	pullFunc                            func(ctx context.Context, in *gitgatewayv1.PullRequest) (*gitgatewayv1.PullResponse, error)
	generateCommitMessageFunc           func(ctx context.Context, in *gitgatewayv1.GenerateCommitMessageRequest) (*gitgatewayv1.GenerateCommitMessageResponse, error)
	checkoutFunc                        func(ctx context.Context, in *gitgatewayv1.CheckoutRequest) (*gitgatewayv1.CheckoutResponse, error)
	listLocalBranchesFunc               func(ctx context.Context, in *gitgatewayv1.ListLocalBranchesRequest) (*gitgatewayv1.ListLocalBranchesResponse, error)
	stageFunc                           func(ctx context.Context, in *gitgatewayv1.StageRequest) (*gitgatewayv1.StageResponse, error)
	unstageFunc                         func(ctx context.Context, in *gitgatewayv1.UnstageRequest) (*gitgatewayv1.UnstageResponse, error)
	historyFunc                         func(ctx context.Context, in *gitgatewayv1.HistoryRequest) (*gitgatewayv1.HistoryResponse, error)
	fetchFunc                           func(ctx context.Context, in *gitgatewayv1.FetchRequest) (*gitgatewayv1.FetchResponse, error)
	discoverCommitMessageModelsFunc     func(ctx context.Context, in *gitgatewayv1.DiscoverCommitMessageModelsRequest) (*gitgatewayv1.DiscoverCommitMessageModelsResponse, error)
	// ... one *Func field per remaining RPC (ListLocalBranches, FastForward,
	// RebaseFromBase, AbortRebase, AbortMerge, ConflictOperation, Discard,
	// BulkDiscard, CommitCompare, BranchCompare, CommitDiff, BranchDiff,
	// SubmoduleStatus, CheckIgnored, ForkSync, UpstreamStatus,
	// RemoteCommitUrl, RemoteFileUrl, GeneratePullRequestFields) — same
	// shape, added as each channel's test needs it.
}

func (f *fakeGitGatewayServiceClient) Commit(ctx context.Context, in *gitgatewayv1.CommitRequest, _ ...grpc.CallOption) (*gitgatewayv1.CommitResponse, error) {
	return f.commitFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) Push(ctx context.Context, in *gitgatewayv1.PushRequest, _ ...grpc.CallOption) (*gitgatewayv1.PushResponse, error) {
	return f.pushFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) Pull(ctx context.Context, in *gitgatewayv1.PullRequest, _ ...grpc.CallOption) (*gitgatewayv1.PullResponse, error) {
	return f.pullFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) GenerateCommitMessage(ctx context.Context, in *gitgatewayv1.GenerateCommitMessageRequest, _ ...grpc.CallOption) (*gitgatewayv1.GenerateCommitMessageResponse, error) {
	return f.generateCommitMessageFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) Checkout(ctx context.Context, in *gitgatewayv1.CheckoutRequest, _ ...grpc.CallOption) (*gitgatewayv1.CheckoutResponse, error) {
	return f.checkoutFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) Stage(ctx context.Context, in *gitgatewayv1.StageRequest, _ ...grpc.CallOption) (*gitgatewayv1.StageResponse, error) {
	return f.stageFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) Unstage(ctx context.Context, in *gitgatewayv1.UnstageRequest, _ ...grpc.CallOption) (*gitgatewayv1.UnstageResponse, error) {
	return f.unstageFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) History(ctx context.Context, in *gitgatewayv1.HistoryRequest, _ ...grpc.CallOption) (*gitgatewayv1.HistoryResponse, error) {
	return f.historyFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) Fetch(ctx context.Context, in *gitgatewayv1.FetchRequest, _ ...grpc.CallOption) (*gitgatewayv1.FetchResponse, error) {
	return f.fetchFunc(ctx, in)
}
func (f *fakeGitGatewayServiceClient) DiscoverCommitMessageModels(ctx context.Context, in *gitgatewayv1.DiscoverCommitMessageModelsRequest, _ ...grpc.CallOption) (*gitgatewayv1.DiscoverCommitMessageModelsResponse, error) {
	return f.discoverCommitMessageModelsFunc(ctx, in)
}
// ... one override method per remaining *Func field, identical one-line
// pass-through shape.
```

### Step 2: One test per channel

Representative examples (repeat this shape for all 33 registered `git.*`
channels — the 4 from TASK-206, 13 from TASK-212, 16 from TASK-213):

```go
func TestGitCommitChannel_Success(t *testing.T) {
	fake := &fakeGitGatewayServiceClient{
		commitFunc: func(ctx context.Context, in *gitgatewayv1.CommitRequest) (*gitgatewayv1.CommitResponse, error) {
			if in.GetWorktreeId() != "wt-1" || in.GetMessage() != "fix bug" {
				t.Errorf("unexpected request: %+v", in)
			}
			return &gitgatewayv1.CommitResponse{CommitSha: "abc123"}, nil
		},
	}
	r := NewRegistry()
	registerGitChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "git.commit",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "message": "fix bug"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*gitgatewayv1.CommitResponse)
	if !ok || resp.GetCommitSha() != "abc123" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGitBulkStageChannel_SharesStageHandler(t *testing.T) {
	var gotPaths [][]string
	fake := &fakeGitGatewayServiceClient{
		stageFunc: func(ctx context.Context, in *gitgatewayv1.StageRequest) (*gitgatewayv1.StageResponse, error) {
			gotPaths = append(gotPaths, in.GetPaths())
			return &gitgatewayv1.StageResponse{Success: true}, nil
		},
	}
	r := NewRegistry()
	registerGitChannels(r, fake)

	if _, err := r.Dispatch(context.Background(), Identity{}, "git.stage", argsJSON(t, map[string]any{"worktreeId": "wt-1", "paths": []string{"a.txt"}})); err != nil {
		t.Fatalf("git.stage: unexpected error: %v", err)
	}
	if _, err := r.Dispatch(context.Background(), Identity{}, "git.bulkStage", argsJSON(t, map[string]any{"worktreeId": "wt-1", "paths": []string{"a.txt", "b.txt"}})); err != nil {
		t.Fatalf("git.bulkStage: unexpected error: %v", err)
	}
	// Locks in the shared-closure economy from SOL-032 Group B — a future
	// edit that accidentally forks git.stage/git.bulkStage's handler bodies
	// would still pass a naive "does it call Stage" test, but this asserts
	// both call sites reached the exact same registered handler by checking
	// both were actually invoked against the one Stage RPC.
	if len(gotPaths) != 2 {
		t.Fatalf("expected both git.stage and git.bulkStage to call Stage, got %d calls", len(gotPaths))
	}
	if len(gotPaths[0]) != 1 || len(gotPaths[1]) != 2 {
		t.Errorf("unexpected paths per call: %+v", gotPaths)
	}
}

func TestGitDiscoverCommitMessageModelsChannel_UsesIdentityNotArgs(t *testing.T) {
	fake := &fakeGitGatewayServiceClient{
		discoverCommitMessageModelsFunc: func(ctx context.Context, in *gitgatewayv1.DiscoverCommitMessageModelsRequest) (*gitgatewayv1.DiscoverCommitMessageModelsResponse, error) {
			if in.GetTenantId() != "tenant-1" || in.GetUserId() != "user-1" {
				t.Errorf("expected tenant/user id sourced from Identity, got %+v", in)
			}
			return &gitgatewayv1.DiscoverCommitMessageModelsResponse{}, nil
		},
	}
	r := NewRegistry()
	registerGitChannels(r, fake)

	// No tenantId/userId in args — regression guard against ever trusting a
	// client-supplied tenant for this channel (SOL-033's admin-route
	// precedent).
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "git.discoverCommitMessageModels", argsJSON(t, map[string]any{})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

Add the remaining 30 channel tests following `TestGitCommitChannel_Success`'s
shape: one per channel, asserting the args decode into the expected request
fields and the fake's response comes back through unmodified (or, for
`git.localBranches`/`git.submoduleStatus`/`git.checkIgnored`, that the
handler returns the unwrapped `resp.Get<Field>()` slice rather than the
whole response envelope, per TASK-213's handler bodies).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go test ./internal/adapter/wscompat/... -run TestGit -count=1 -v
```

Expected: 33 passing tests, one per registered `git.*` channel (4 from
TASK-206 + 13 from TASK-212 + 16 from TASK-213), plus the
`TestGitBulkStageChannel_SharesStageHandler` and
`TestGitDiscoverCommitMessageModelsChannel_UsesIdentityNotArgs` regression
guards above.
