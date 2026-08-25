package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// fakeGitGatewayClient implements gitgatewayv1.GitGatewayServiceClient over
// per-method hook functions — embeds the (nil) interface so unset methods
// panic loudly if a test reaches one it didn't mean to exercise, mirroring
// this package's existing fakeInfraFleetClient convention (channels_test.go).
type fakeGitGatewayClient struct {
	gitgatewayv1.GitGatewayServiceClient

	getDiffFunc                        func(ctx context.Context, in *gitgatewayv1.GetDiffRequest) (*gitgatewayv1.GetDiffResponse, error)
	commitFunc                         func(ctx context.Context, in *gitgatewayv1.CommitRequest) (*gitgatewayv1.CommitResponse, error)
	stageFunc                          func(ctx context.Context, in *gitgatewayv1.StageRequest) (*gitgatewayv1.StageResponse, error)
	historyFunc                        func(ctx context.Context, in *gitgatewayv1.HistoryRequest) (*gitgatewayv1.HistoryResponse, error)
	checkIgnoredFunc                   func(ctx context.Context, in *gitgatewayv1.CheckIgnoredRequest) (*gitgatewayv1.CheckIgnoredResponse, error)
	forkSyncFunc                       func(ctx context.Context, in *gitgatewayv1.ForkSyncRequest) (*gitgatewayv1.ForkSyncResponse, error)
	remoteCommitURLFunc                func(ctx context.Context, in *gitgatewayv1.RemoteCommitUrlRequest) (*gitgatewayv1.RemoteUrlResponse, error)
	generatePullRequestFieldsFunc      func(ctx context.Context, in *gitgatewayv1.GeneratePullRequestFieldsRequest) (*gitgatewayv1.GeneratePullRequestFieldsResponse, error)
	discoverCommitMessageModelsFunc    func(ctx context.Context, in *gitgatewayv1.DiscoverCommitMessageModelsRequest) (*gitgatewayv1.DiscoverCommitMessageModelsResponse, error)
	readFileFunc                       func(ctx context.Context, in *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error)
	writeFileFunc                      func(ctx context.Context, in *gitgatewayv1.WriteFileRequest) (*gitgatewayv1.WriteFileResponse, error)
	renameFileFunc                     func(ctx context.Context, in *gitgatewayv1.RenameFileRequest) (*gitgatewayv1.RenameFileResponse, error)
}

func (f *fakeGitGatewayClient) GetDiff(ctx context.Context, in *gitgatewayv1.GetDiffRequest, _ ...grpc.CallOption) (*gitgatewayv1.GetDiffResponse, error) {
	return f.getDiffFunc(ctx, in)
}
func (f *fakeGitGatewayClient) Commit(ctx context.Context, in *gitgatewayv1.CommitRequest, _ ...grpc.CallOption) (*gitgatewayv1.CommitResponse, error) {
	return f.commitFunc(ctx, in)
}
func (f *fakeGitGatewayClient) Stage(ctx context.Context, in *gitgatewayv1.StageRequest, _ ...grpc.CallOption) (*gitgatewayv1.StageResponse, error) {
	return f.stageFunc(ctx, in)
}
func (f *fakeGitGatewayClient) History(ctx context.Context, in *gitgatewayv1.HistoryRequest, _ ...grpc.CallOption) (*gitgatewayv1.HistoryResponse, error) {
	return f.historyFunc(ctx, in)
}
func (f *fakeGitGatewayClient) CheckIgnored(ctx context.Context, in *gitgatewayv1.CheckIgnoredRequest, _ ...grpc.CallOption) (*gitgatewayv1.CheckIgnoredResponse, error) {
	return f.checkIgnoredFunc(ctx, in)
}
func (f *fakeGitGatewayClient) ForkSync(ctx context.Context, in *gitgatewayv1.ForkSyncRequest, _ ...grpc.CallOption) (*gitgatewayv1.ForkSyncResponse, error) {
	return f.forkSyncFunc(ctx, in)
}
func (f *fakeGitGatewayClient) RemoteCommitUrl(ctx context.Context, in *gitgatewayv1.RemoteCommitUrlRequest, _ ...grpc.CallOption) (*gitgatewayv1.RemoteUrlResponse, error) {
	return f.remoteCommitURLFunc(ctx, in)
}
func (f *fakeGitGatewayClient) GeneratePullRequestFields(ctx context.Context, in *gitgatewayv1.GeneratePullRequestFieldsRequest, _ ...grpc.CallOption) (*gitgatewayv1.GeneratePullRequestFieldsResponse, error) {
	return f.generatePullRequestFieldsFunc(ctx, in)
}
func (f *fakeGitGatewayClient) DiscoverCommitMessageModels(ctx context.Context, in *gitgatewayv1.DiscoverCommitMessageModelsRequest, _ ...grpc.CallOption) (*gitgatewayv1.DiscoverCommitMessageModelsResponse, error) {
	return f.discoverCommitMessageModelsFunc(ctx, in)
}
func (f *fakeGitGatewayClient) ReadFile(ctx context.Context, in *gitgatewayv1.ReadFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.ReadFileResponse, error) {
	return f.readFileFunc(ctx, in)
}
func (f *fakeGitGatewayClient) WriteFile(ctx context.Context, in *gitgatewayv1.WriteFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.WriteFileResponse, error) {
	return f.writeFileFunc(ctx, in)
}
func (f *fakeGitGatewayClient) RenameFile(ctx context.Context, in *gitgatewayv1.RenameFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.RenameFileResponse, error) {
	return f.renameFileFunc(ctx, in)
}

func TestGitDiffChannel_ThreadsFilePathThrough(t *testing.T) {
	var got *gitgatewayv1.GetDiffRequest
	fake := &fakeGitGatewayClient{
		getDiffFunc: func(ctx context.Context, in *gitgatewayv1.GetDiffRequest) (*gitgatewayv1.GetDiffResponse, error) {
			got = in
			return &gitgatewayv1.GetDiffResponse{UnifiedDiff: "diff"}, nil
		},
	}
	r := NewRegistry()
	registerGitDeepChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "git.diff",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "filePath": "a.txt", "staged": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetWorktreeId() != "wt-1" || got.GetFilePath() != "a.txt" || !got.GetStaged() {
		t.Errorf("unexpected request: %+v", got)
	}
}

func TestGitCommitChannel_Success(t *testing.T) {
	fake := &fakeGitGatewayClient{
		commitFunc: func(ctx context.Context, in *gitgatewayv1.CommitRequest) (*gitgatewayv1.CommitResponse, error) {
			return &gitgatewayv1.CommitResponse{CommitSha: "abc123"}, nil
		},
	}
	r := NewRegistry()
	registerGitDeepChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "git.commit",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "message": "fix"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*gitgatewayv1.CommitResponse)
	if !ok || resp.GetCommitSha() != "abc123" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestGitStageAndBulkStageChannels_ShareOneHandler(t *testing.T) {
	var callCount int
	fake := &fakeGitGatewayClient{
		stageFunc: func(ctx context.Context, in *gitgatewayv1.StageRequest) (*gitgatewayv1.StageResponse, error) {
			callCount++
			return &gitgatewayv1.StageResponse{Success: true}, nil
		},
	}
	r := NewRegistry()
	registerGitDeepChannels(r, fake)

	for _, channel := range []string{"git.stage", "git.bulkStage"} {
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, channel,
			argsJSON(t, map[string]any{"worktreeId": "wt-1", "paths": []string{"a.txt"}}))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", channel, err)
		}
	}
	if callCount != 2 {
		t.Errorf("expected both channels to reach Stage, got %d calls", callCount)
	}
}

func TestGitHistoryChannel_CorrectedShape(t *testing.T) {
	var got *gitgatewayv1.HistoryRequest
	fake := &fakeGitGatewayClient{
		historyFunc: func(ctx context.Context, in *gitgatewayv1.HistoryRequest) (*gitgatewayv1.HistoryResponse, error) {
			got = in
			return &gitgatewayv1.HistoryResponse{}, nil
		},
	}
	r := NewRegistry()
	registerGitDeepChannels(r, fake)

	// baseRef (not "ref"), no cursor — matches TASK-209's corrected shape.
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "git.history",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "baseRef": "main", "limit": 5}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetBaseRef() != "main" || got.GetLimit() != 5 {
		t.Errorf("unexpected request: %+v", got)
	}
}

func TestGitDiscoverCommitMessageModelsChannel_UsesIdentityNotArgs(t *testing.T) {
	var got *gitgatewayv1.DiscoverCommitMessageModelsRequest
	fake := &fakeGitGatewayClient{
		discoverCommitMessageModelsFunc: func(ctx context.Context, in *gitgatewayv1.DiscoverCommitMessageModelsRequest) (*gitgatewayv1.DiscoverCommitMessageModelsResponse, error) {
			got = in
			return &gitgatewayv1.DiscoverCommitMessageModelsResponse{}, nil
		},
	}
	r := NewRegistry()
	registerGitDeepChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "git.discoverCommitMessageModels", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetTenantId() != "tenant-1" || got.GetUserId() != "user-1" {
		t.Errorf("expected tenant/user sourced from Identity (never a client-supplied arg), got %+v", got)
	}
}

func TestFilesReadChannel_Success(t *testing.T) {
	fake := &fakeGitGatewayClient{
		readFileFunc: func(ctx context.Context, in *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error) {
			return &gitgatewayv1.ReadFileResponse{Content: []byte("hi"), Encoding: "utf8"}, nil
		},
	}
	r := NewRegistry()
	registerFilesChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "files.read",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "path": "a.txt"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*gitgatewayv1.ReadFileResponse)
	if !ok || string(resp.GetContent()) != "hi" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestFilesWriteBase64Channel_DecodesContent(t *testing.T) {
	var got *gitgatewayv1.WriteFileRequest
	fake := &fakeGitGatewayClient{
		writeFileFunc: func(ctx context.Context, in *gitgatewayv1.WriteFileRequest) (*gitgatewayv1.WriteFileResponse, error) {
			got = in
			return &gitgatewayv1.WriteFileResponse{BytesWritten: int64(len(in.GetContent()))}, nil
		},
	}
	r := NewRegistry()
	registerFilesChannels(r, fake)

	// base64("hi") == "aGk="
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "files.writeBase64",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "path": "a.txt", "content": "aGk=", "base64": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got.GetContent()) != "hi" {
		t.Errorf("expected decoded content %q, got %q", "hi", got.GetContent())
	}
}

func TestFilesRenameChannel_KnownGapErrorPassesThrough(t *testing.T) {
	wantErr := errors.New("FAILED_PRECONDITION: rename/copy are not supported over a relay connection")
	fake := &fakeGitGatewayClient{
		renameFileFunc: func(ctx context.Context, in *gitgatewayv1.RenameFileRequest) (*gitgatewayv1.RenameFileResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerFilesChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "files.rename",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "fromPath": "a.txt", "toPath": "b.txt"}))
	if err == nil {
		t.Fatal("expected the known-gap error to surface as-is, not be swallowed")
	}
}

func TestFilesCommitUploadChannel_LocalNoOpAck(t *testing.T) {
	r := NewRegistry()
	registerFilesChannels(r, &fakeGitGatewayClient{})

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "files.commitUpload", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok, isMap := result.(map[string]bool); !isMap || !ok["ok"] {
		t.Errorf("expected local no-op ack {ok:true}, got %+v", result)
	}
}
