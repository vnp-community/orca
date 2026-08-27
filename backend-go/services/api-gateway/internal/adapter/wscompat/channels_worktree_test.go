package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// fakeGitGatewayServiceClient/fakeProjectServiceClient are minimal test
// doubles — embed the (nil) interface, per fakeInfraFleetClient's
// precedent in channels_test.go, and override only the methods this
// file's channel handlers actually call.
type fakeGitGatewayServiceClient struct {
	gitgatewayv1.GitGatewayServiceClient

	createWorktreeFunc            func(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error)
	removeWorktreeFunc            func(ctx context.Context, in *gitgatewayv1.RemoveWorktreeRequest) (*gitgatewayv1.RemoveWorktreeResponse, error)
	forceDeleteBranchFunc         func(ctx context.Context, in *gitgatewayv1.ForceDeleteBranchRequest) (*emptypb.Empty, error)
	prefetchCreateBaseFunc        func(ctx context.Context, in *gitgatewayv1.PrefetchCreateBaseRequest) (*gitgatewayv1.PrefetchCreateBaseResponse, error)
	resolvePrBaseFunc             func(ctx context.Context, in *gitgatewayv1.ResolvePrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error)
	resolveMrBaseFunc             func(ctx context.Context, in *gitgatewayv1.ResolveMrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error)
	detectWorktreesFunc           func(ctx context.Context, in *gitgatewayv1.DetectWorktreesRequest) (*gitgatewayv1.DetectWorktreesResponse, error)
	compareWorktreesFunc          func(ctx context.Context, in *gitgatewayv1.CompareWorktreesRequest) (*gitgatewayv1.CompareWorktreesResponse, error)
	checkWorktreeDeleteSafetyFunc func(ctx context.Context, in *gitgatewayv1.CheckWorktreeDeleteSafetyRequest) (*gitgatewayv1.CheckWorktreeDeleteSafetyResponse, error)
	mergeBranchFunc               func(ctx context.Context, in *gitgatewayv1.MergeBranchRequest) (*gitgatewayv1.MergeBranchResponse, error)

	calledCreateWorktree            bool
	calledRemoveWorktree            bool
	calledForceDeleteBranch         bool
	calledPrefetchCreateBase        bool
	calledResolvePrBase             bool
	calledResolveMrBase             bool
	calledDetectWorktrees           bool
	calledCompareWorktrees          bool
	calledCheckWorktreeDeleteSafety bool
	calledMergeBranch               bool
	removeWorktreeCalls             []*gitgatewayv1.RemoveWorktreeRequest
}

func (f *fakeGitGatewayServiceClient) CompareWorktrees(ctx context.Context, in *gitgatewayv1.CompareWorktreesRequest, _ ...grpc.CallOption) (*gitgatewayv1.CompareWorktreesResponse, error) {
	f.calledCompareWorktrees = true
	return f.compareWorktreesFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) CreateWorktree(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	f.calledCreateWorktree = true
	return f.createWorktreeFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) RemoveWorktree(ctx context.Context, in *gitgatewayv1.RemoveWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.RemoveWorktreeResponse, error) {
	f.calledRemoveWorktree = true
	f.removeWorktreeCalls = append(f.removeWorktreeCalls, in)
	return f.removeWorktreeFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) CheckWorktreeDeleteSafety(ctx context.Context, in *gitgatewayv1.CheckWorktreeDeleteSafetyRequest, _ ...grpc.CallOption) (*gitgatewayv1.CheckWorktreeDeleteSafetyResponse, error) {
	f.calledCheckWorktreeDeleteSafety = true
	return f.checkWorktreeDeleteSafetyFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) MergeBranch(ctx context.Context, in *gitgatewayv1.MergeBranchRequest, _ ...grpc.CallOption) (*gitgatewayv1.MergeBranchResponse, error) {
	f.calledMergeBranch = true
	return f.mergeBranchFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) ForceDeleteBranch(ctx context.Context, in *gitgatewayv1.ForceDeleteBranchRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.calledForceDeleteBranch = true
	return f.forceDeleteBranchFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) PrefetchCreateBase(ctx context.Context, in *gitgatewayv1.PrefetchCreateBaseRequest, _ ...grpc.CallOption) (*gitgatewayv1.PrefetchCreateBaseResponse, error) {
	f.calledPrefetchCreateBase = true
	return f.prefetchCreateBaseFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) ResolvePrBase(ctx context.Context, in *gitgatewayv1.ResolvePrBaseRequest, _ ...grpc.CallOption) (*gitgatewayv1.ResolveBaseResponse, error) {
	f.calledResolvePrBase = true
	return f.resolvePrBaseFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) ResolveMrBase(ctx context.Context, in *gitgatewayv1.ResolveMrBaseRequest, _ ...grpc.CallOption) (*gitgatewayv1.ResolveBaseResponse, error) {
	f.calledResolveMrBase = true
	return f.resolveMrBaseFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) DetectWorktrees(ctx context.Context, in *gitgatewayv1.DetectWorktreesRequest, _ ...grpc.CallOption) (*gitgatewayv1.DetectWorktreesResponse, error) {
	f.calledDetectWorktrees = true
	return f.detectWorktreesFunc(ctx, in)
}

type fakeProjectServiceClient struct {
	projectv1.ProjectServiceClient

	listWorktreesFunc         func(ctx context.Context, in *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error)
	setWorktreeActivationFunc func(ctx context.Context, in *projectv1.SetWorktreeActivationRequest) (*projectv1.SetWorktreeActivationResponse, error)

	calledListWorktrees         bool
	calledSetWorktreeActivation bool
}

func (f *fakeProjectServiceClient) ListWorktrees(ctx context.Context, in *projectv1.ListWorktreesRequest, _ ...grpc.CallOption) (*projectv1.ListWorktreesResponse, error) {
	f.calledListWorktrees = true
	return f.listWorktreesFunc(ctx, in)
}

func (f *fakeProjectServiceClient) SetWorktreeActivation(ctx context.Context, in *projectv1.SetWorktreeActivationRequest, _ ...grpc.CallOption) (*projectv1.SetWorktreeActivationResponse, error) {
	f.calledSetWorktreeActivation = true
	return f.setWorktreeActivationFunc(ctx, in)
}

func TestWorktreeCreateChannel_Success(t *testing.T) {
	var gotReq *gitgatewayv1.CreateWorktreeRequest
	git := &fakeGitGatewayServiceClient{
		createWorktreeFunc: func(_ context.Context, in *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error) {
			gotReq = in
			return &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-1", Path: "/repo-feature", HeadSha: "sha1"}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.create",
		argsJSON(t, map[string]any{"projectId": "proj-1", "repoId": "repo-1", "branch": "feature", "baseRef": "main"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProjectId() != "proj-1" || gotReq.GetRepoId() != "repo-1" || gotReq.GetBranch() != "feature" || gotReq.GetBaseRef() != "main" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*gitgatewayv1.CreateWorktreeResponse)
	if !ok || resp.GetWorktreeId() != "wt-1" || resp.GetPath() != "/repo-feature" || resp.GetHeadSha() != "sha1" {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

func TestWorktreeRmChannel_Success(t *testing.T) {
	var gotReq *gitgatewayv1.RemoveWorktreeRequest
	git := &fakeGitGatewayServiceClient{
		removeWorktreeFunc: func(_ context.Context, in *gitgatewayv1.RemoveWorktreeRequest) (*gitgatewayv1.RemoveWorktreeResponse, error) {
			gotReq = in
			return &gitgatewayv1.RemoveWorktreeResponse{UncommittedFilesDiscarded: 3}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.rm",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "force": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetWorktreeId() != "wt-1" || !gotReq.GetForce() {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*gitgatewayv1.RemoveWorktreeResponse)
	if !ok || resp.GetUncommittedFilesDiscarded() != 3 {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

func TestWorktreeCompareChannel_Success(t *testing.T) {
	var gotReq *gitgatewayv1.CompareWorktreesRequest
	git := &fakeGitGatewayServiceClient{
		compareWorktreesFunc: func(_ context.Context, in *gitgatewayv1.CompareWorktreesRequest) (*gitgatewayv1.CompareWorktreesResponse, error) {
			gotReq = in
			return &gitgatewayv1.CompareWorktreesResponse{BaseRef: "main"}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.compare",
		argsJSON(t, map[string]any{"worktreeIds": []string{"wt-1", "wt-2"}}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotReq.GetWorktreeIds()) != 2 {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*gitgatewayv1.CompareWorktreesResponse)
	if !ok || resp.GetBaseRef() != "main" {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

func TestWorktreeForceDeleteBranchChannel_Success(t *testing.T) {
	var gotReq *gitgatewayv1.ForceDeleteBranchRequest
	git := &fakeGitGatewayServiceClient{
		forceDeleteBranchFunc: func(_ context.Context, in *gitgatewayv1.ForceDeleteBranchRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.forceDeleteBranch",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "branch": "feature"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetWorktreeId() != "wt-1" || gotReq.GetBranch() != "feature" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	if v, ok := result.(map[string]bool); !ok || !v["ok"] {
		t.Errorf("expected {ok:true}, got %+v", result)
	}
}

func TestWorktreePrefetchCreateBaseChannel_Success(t *testing.T) {
	var gotReq *gitgatewayv1.PrefetchCreateBaseRequest
	git := &fakeGitGatewayServiceClient{
		prefetchCreateBaseFunc: func(_ context.Context, in *gitgatewayv1.PrefetchCreateBaseRequest) (*gitgatewayv1.PrefetchCreateBaseResponse, error) {
			gotReq = in
			return &gitgatewayv1.PrefetchCreateBaseResponse{ResolvedSha: "sha-abc"}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.prefetchCreateBase",
		argsJSON(t, map[string]any{"repoId": "repo-1", "baseRef": "main"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetRepoId() != "repo-1" || gotReq.GetBaseRef() != "main" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*gitgatewayv1.PrefetchCreateBaseResponse)
	if !ok || resp.GetResolvedSha() != "sha-abc" {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

func TestWorktreeResolvePrBaseChannel_Success(t *testing.T) {
	var gotReq *gitgatewayv1.ResolvePrBaseRequest
	git := &fakeGitGatewayServiceClient{
		resolvePrBaseFunc: func(_ context.Context, in *gitgatewayv1.ResolvePrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error) {
			gotReq = in
			return &gitgatewayv1.ResolveBaseResponse{BaseBranch: "main", BaseSha: "sha-pr"}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.resolvePrBase",
		argsJSON(t, map[string]any{"repoId": "repo-1", "prNumber": 42}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetRepoId() != "repo-1" || gotReq.GetPrNumber() != 42 {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*gitgatewayv1.ResolveBaseResponse)
	if !ok || resp.GetBaseBranch() != "main" || resp.GetBaseSha() != "sha-pr" {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

func TestWorktreeResolveMrBaseChannel_Success(t *testing.T) {
	var gotReq *gitgatewayv1.ResolveMrBaseRequest
	git := &fakeGitGatewayServiceClient{
		resolveMrBaseFunc: func(_ context.Context, in *gitgatewayv1.ResolveMrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error) {
			gotReq = in
			return &gitgatewayv1.ResolveBaseResponse{BaseBranch: "main", BaseSha: "sha-mr"}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.resolveMrBase",
		argsJSON(t, map[string]any{"repoId": "repo-1", "mrNumber": 7}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetRepoId() != "repo-1" || gotReq.GetMrNumber() != 7 {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*gitgatewayv1.ResolveBaseResponse)
	if !ok || resp.GetBaseBranch() != "main" || resp.GetBaseSha() != "sha-mr" {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

// TestWorktreeListChannel_CallsProjectClientNotGitClient is a regression
// guard on BUG-031's "always-local bookkeeping, no git-gateway-service
// involvement" dispatch-model finding.
func TestWorktreeListChannel_CallsProjectClientNotGitClient(t *testing.T) {
	var gotReq *projectv1.ListWorktreesRequest
	git := &fakeGitGatewayServiceClient{}
	project := &fakeProjectServiceClient{
		listWorktreesFunc: func(_ context.Context, in *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
			gotReq = in
			return &projectv1.ListWorktreesResponse{Worktrees: []*projectv1.Worktree{{Id: "wt-1", Path: "/repo-feature"}}}, nil
		},
	}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.list",
		argsJSON(t, map[string]any{"projectId": "proj-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProjectId() != "proj-1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	if !project.calledListWorktrees {
		t.Error("expected projectClient.ListWorktrees to be called")
	}
	if git.calledCreateWorktree || git.calledRemoveWorktree || git.calledDetectWorktrees {
		t.Error("expected gitClient NOT to be involved in worktree.list")
	}
	worktrees, ok := result.([]*projectv1.Worktree)
	if !ok || len(worktrees) != 1 || worktrees[0].GetId() != "wt-1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// TestWorktreeSetChannel_CallsProjectClientNotGitClient mirrors the list
// channel's regression guard.
func TestWorktreeSetChannel_CallsProjectClientNotGitClient(t *testing.T) {
	var gotReq *projectv1.SetWorktreeActivationRequest
	git := &fakeGitGatewayServiceClient{}
	project := &fakeProjectServiceClient{
		setWorktreeActivationFunc: func(_ context.Context, in *projectv1.SetWorktreeActivationRequest) (*projectv1.SetWorktreeActivationResponse, error) {
			gotReq = in
			return &projectv1.SetWorktreeActivationResponse{Worktree: &projectv1.Worktree{Id: "wt-1", Active: true}}, nil
		},
	}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.set",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "active": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetWorktreeId() != "wt-1" || !gotReq.GetActive() {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	if !project.calledSetWorktreeActivation {
		t.Error("expected projectClient.SetWorktreeActivation to be called")
	}
	if git.calledCreateWorktree || git.calledRemoveWorktree || git.calledDetectWorktrees {
		t.Error("expected gitClient NOT to be involved in worktree.set")
	}
	wt, ok := result.(*projectv1.Worktree)
	if !ok || !wt.GetActive() {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestWorktreeDetectedListChannel_OrphanedPathNotInBookkeeping(t *testing.T) {
	git := &fakeGitGatewayServiceClient{
		detectWorktreesFunc: func(_ context.Context, in *gitgatewayv1.DetectWorktreesRequest) (*gitgatewayv1.DetectWorktreesResponse, error) {
			return &gitgatewayv1.DetectWorktreesResponse{OnDiskPaths: []string{"/repo-main", "/repo-orphan"}}, nil
		},
	}
	project := &fakeProjectServiceClient{
		listWorktreesFunc: func(_ context.Context, in *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
			return &projectv1.ListWorktreesResponse{Worktrees: []*projectv1.Worktree{{Id: "wt-1", Path: "/repo-main"}}}, nil
		},
	}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.detectedList",
		argsJSON(t, map[string]any{"projectId": "proj-1", "repoId": "repo-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	orphaned, ok := m["orphanedPaths"].([]string)
	if !ok {
		t.Fatalf("unexpected orphanedPaths type: %+v", m)
	}
	if len(orphaned) != 1 || orphaned[0] != "/repo-orphan" {
		t.Errorf("expected only /repo-orphan to be orphaned, got %v", orphaned)
	}
}

func TestWorktreeDetectedListChannel_GitGatewayErrors_WholeCallFails(t *testing.T) {
	git := &fakeGitGatewayServiceClient{
		detectWorktreesFunc: func(_ context.Context, _ *gitgatewayv1.DetectWorktreesRequest) (*gitgatewayv1.DetectWorktreesResponse, error) {
			return nil, errors.New("git-gateway-service unreachable")
		},
	}
	project := &fakeProjectServiceClient{
		listWorktreesFunc: func(_ context.Context, _ *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
			return &projectv1.ListWorktreesResponse{Worktrees: []*projectv1.Worktree{{Id: "wt-1", Path: "/repo-main"}}}, nil
		},
	}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.detectedList",
		argsJSON(t, map[string]any{"projectId": "proj-1", "repoId": "repo-1"}))
	if err == nil {
		t.Fatal("expected an error when git-gateway-service's DetectWorktrees fails")
	}
	if result != nil {
		t.Errorf("expected no partial result to leak through on error, got %+v", result)
	}
}

func TestWorktreeDetectedListChannel_BothEmpty_ReturnsEmptyNotError(t *testing.T) {
	git := &fakeGitGatewayServiceClient{
		detectWorktreesFunc: func(_ context.Context, _ *gitgatewayv1.DetectWorktreesRequest) (*gitgatewayv1.DetectWorktreesResponse, error) {
			return &gitgatewayv1.DetectWorktreesResponse{OnDiskPaths: nil}, nil
		},
	}
	project := &fakeProjectServiceClient{
		listWorktreesFunc: func(_ context.Context, _ *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
			return &projectv1.ListWorktreesResponse{Worktrees: nil}, nil
		},
	}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.detectedList",
		argsJSON(t, map[string]any{"projectId": "proj-1", "repoId": "repo-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	orphaned, ok := m["orphanedPaths"].([]string)
	if !ok {
		t.Fatalf("unexpected orphanedPaths type: %+v", m)
	}
	if orphaned == nil {
		t.Error("expected orphanedPaths to be a non-nil empty slice, not nil, when both sides are empty")
	}
	if len(orphaned) != 0 {
		t.Errorf("expected orphanedPaths to be empty, got %v", orphaned)
	}
}

// ── worktree.checkDeleteSafety / worktree.rm stopAgents threading ───────────

func TestWorktreeCheckDeleteSafety_HappyPath(t *testing.T) {
	var gotReq *gitgatewayv1.CheckWorktreeDeleteSafetyRequest
	git := &fakeGitGatewayServiceClient{
		checkWorktreeDeleteSafetyFunc: func(_ context.Context, in *gitgatewayv1.CheckWorktreeDeleteSafetyRequest) (*gitgatewayv1.CheckWorktreeDeleteSafetyResponse, error) {
			gotReq = in
			return &gitgatewayv1.CheckWorktreeDeleteSafetyResponse{
				UncommittedFiles: 2, UntrackedFiles: 1, AgentRunning: true,
				ActivePtyIds: []string{"pty-1"}, SafeToDelete: false,
			}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.checkDeleteSafety",
		argsJSON(t, map[string]any{"worktreeId": "wt-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetWorktreeId() != "wt-1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*gitgatewayv1.CheckWorktreeDeleteSafetyResponse)
	if !ok || resp.GetUncommittedFiles() != 2 || resp.GetUntrackedFiles() != 1 || !resp.GetAgentRunning() || resp.GetSafeToDelete() {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

func TestWorktreeRm_StopAgentsThreadsThroughToGRPCRequest(t *testing.T) {
	var gotReq *gitgatewayv1.RemoveWorktreeRequest
	git := &fakeGitGatewayServiceClient{
		removeWorktreeFunc: func(_ context.Context, in *gitgatewayv1.RemoveWorktreeRequest) (*gitgatewayv1.RemoveWorktreeResponse, error) {
			gotReq = in
			return &gitgatewayv1.RemoveWorktreeResponse{StoppedPtyIds: []string{"pty-1"}}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.rm",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "force": false, "stopAgents": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetWorktreeId() != "wt-1" || !gotReq.GetStopAgents() {
		t.Errorf("expected StopAgents to be threaded through to RemoveWorktreeRequest, got %+v", gotReq)
	}
	resp, ok := result.(*gitgatewayv1.RemoveWorktreeResponse)
	if !ok || len(resp.GetStoppedPtyIds()) != 1 || resp.GetStoppedPtyIds()[0] != "pty-1" {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

// ── worktree.merge (BR-WT-18 optional cleanup composition) ──────────────────

func TestWorktreeMerge_HappyPath(t *testing.T) {
	git := &fakeGitGatewayServiceClient{
		mergeBranchFunc: func(_ context.Context, _ *gitgatewayv1.MergeBranchRequest) (*gitgatewayv1.MergeBranchResponse, error) {
			return &gitgatewayv1.MergeBranchResponse{ResultSha: "sha-merged", HasConflicts: false}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.merge",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "baseBranch": "main", "strategy": "merge"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*gitgatewayv1.MergeBranchResponse)
	if !ok || resp.GetResultSha() != "sha-merged" {
		t.Errorf("expected the raw MergeBranchResponse to be returned unmodified, got %+v (%T)", result, result)
	}
	if git.calledRemoveWorktree {
		t.Error("expected RemoveWorktree NOT to be called when no cleanupWorktreeIds are given")
	}
}

func TestWorktreeMerge_ConflictedMerge_NeverCallsRemoveWorktree_EvenWithCleanupIDsSet(t *testing.T) {
	git := &fakeGitGatewayServiceClient{
		mergeBranchFunc: func(_ context.Context, _ *gitgatewayv1.MergeBranchRequest) (*gitgatewayv1.MergeBranchResponse, error) {
			return &gitgatewayv1.MergeBranchResponse{HasConflicts: true, ConflictedPaths: []string{"file.txt"}}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.merge",
		argsJSON(t, map[string]any{
			"worktreeId": "wt-1", "baseBranch": "main", "strategy": "merge",
			"cleanupWorktreeIds": []string{"wt-2", "wt-3"},
		}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*gitgatewayv1.MergeBranchResponse)
	if !ok || !resp.GetHasConflicts() {
		t.Errorf("expected the raw conflicted MergeBranchResponse to be returned, got %+v", result)
	}
	if git.calledRemoveWorktree {
		t.Error("expected RemoveWorktree to never be called on a conflicted merge, even with cleanupWorktreeIds set")
	}
}

func TestWorktreeMerge_CleanupOneFails_OthersStillRemoved_MergeResponseStillReturned(t *testing.T) {
	git := &fakeGitGatewayServiceClient{
		mergeBranchFunc: func(_ context.Context, _ *gitgatewayv1.MergeBranchRequest) (*gitgatewayv1.MergeBranchResponse, error) {
			return &gitgatewayv1.MergeBranchResponse{ResultSha: "sha-merged", HasConflicts: false}, nil
		},
		removeWorktreeFunc: func(_ context.Context, in *gitgatewayv1.RemoveWorktreeRequest) (*gitgatewayv1.RemoveWorktreeResponse, error) {
			if in.GetWorktreeId() == "wt-2" {
				return nil, errors.New("worktree has uncommitted changes")
			}
			return &gitgatewayv1.RemoveWorktreeResponse{}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.merge",
		argsJSON(t, map[string]any{
			"worktreeId": "wt-1", "baseBranch": "main", "strategy": "merge",
			"cleanupWorktreeIds": []string{"wt-2", "wt-3", "wt-4"},
		}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	mergeResp, ok := m["merge"].(*gitgatewayv1.MergeBranchResponse)
	if !ok || mergeResp.GetResultSha() != "sha-merged" {
		t.Fatalf("expected merge key to carry the successful merge response, got %+v", m["merge"])
	}
	cleanup, ok := m["cleanup"].(map[string]string)
	if !ok {
		t.Fatalf("unexpected cleanup type: %+v", m["cleanup"])
	}
	if cleanup["wt-3"] != "removed" || cleanup["wt-4"] != "removed" {
		t.Errorf("expected wt-3/wt-4 to be marked removed, got %+v", cleanup)
	}
	if cleanup["wt-2"] == "removed" || cleanup["wt-2"] == "" {
		t.Errorf("expected wt-2's failure to be surfaced as its error string, got %q", cleanup["wt-2"])
	}
	if len(git.removeWorktreeCalls) != 3 {
		t.Errorf("expected all 3 cleanup ids to be attempted despite one failing, got %d calls", len(git.removeWorktreeCalls))
	}
}

// ── worktree.fanOut ──────────────────────────────────────────────────────

// fakeWorktreeCreatorPort/fakeAgentSpawnerPort/fakePromptInjectorPort
// implement usecase.WorktreeCreator/AgentSpawner/PromptInjector directly —
// this file's own local fakes (kept separate from usecase package's
// unexported fan-out test fakes, which this package can't reach).
type fakeWorktreeCreatorPort struct {
	fn func(ctx context.Context, projectID, repoID, branch, baseRef string) (string, string, string, error)
}

func (f *fakeWorktreeCreatorPort) CreateWorktree(ctx context.Context, projectID, repoID, branch, baseRef string) (string, string, string, error) {
	return f.fn(ctx, projectID, repoID, branch, baseRef)
}

type fakeAgentSpawnerPort struct {
	fn func(ctx context.Context, projectID, worktreePath, agentType string) (string, string, error)
}

func (f *fakeAgentSpawnerPort) SpawnAgentTerminal(ctx context.Context, projectID, worktreePath, agentType string) (string, string, error) {
	return f.fn(ctx, projectID, worktreePath, agentType)
}

type fakePromptInjectorPort struct {
	fn func(ctx context.Context, connectionID, ptyID, prompt string) error
}

func (f *fakePromptInjectorPort) InjectPrompt(ctx context.Context, connectionID, ptyID, prompt string) error {
	if f.fn == nil {
		return nil
	}
	return f.fn(ctx, connectionID, ptyID, prompt)
}

func TestWorktreeFanOut_HappyPath(t *testing.T) {
	worktrees := &fakeWorktreeCreatorPort{fn: func(_ context.Context, _, _, branch, _ string) (string, string, string, error) {
		return "wt-" + branch, "/repo-" + branch, "sha-" + branch, nil
	}}
	agents := &fakeAgentSpawnerPort{fn: func(_ context.Context, _, worktreePath, _ string) (string, string, error) {
		return "pty-" + worktreePath, "conn-" + worktreePath, nil
	}}
	prompts := &fakePromptInjectorPort{}
	fanOutUseCase := usecase.NewFanOutCreateWorktrees(worktrees, agents, prompts)

	git := &fakeGitGatewayServiceClient{}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, fanOutUseCase)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.fanOut",
		argsJSON(t, map[string]any{
			"projectId": "proj-1", "repoId": "repo-1", "baseRef": "main",
			"branchPrefix": "feat", "prompt": "do it", "agentType": "claude", "n": 3,
		}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	items, ok := m["items"].([]usecase.FanOutItemResult)
	if !ok {
		t.Fatalf("unexpected items type: %+v", m["items"])
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	for _, it := range items {
		if it.Status != "ready" || it.WorktreeID == "" || it.Path == "" || it.PtyID == "" || it.ConnectionID == "" {
			t.Errorf("expected every item to be fully populated and ready, got %+v", it)
		}
	}
}

func TestWorktreeFanOut_NOutOfRange_ErrorSurfacesAsChannelError(t *testing.T) {
	worktrees := &fakeWorktreeCreatorPort{fn: func(_ context.Context, _, _, branch, _ string) (string, string, string, error) {
		t.Fatal("CreateWorktree must never be called when n is out of range")
		return "", "", "", nil
	}}
	agents := &fakeAgentSpawnerPort{fn: func(_ context.Context, _, _, _ string) (string, string, error) { return "", "", nil }}
	prompts := &fakePromptInjectorPort{}
	fanOutUseCase := usecase.NewFanOutCreateWorktrees(worktrees, agents, prompts)

	git := &fakeGitGatewayServiceClient{}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project, fanOutUseCase)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.fanOut",
		argsJSON(t, map[string]any{
			"projectId": "proj-1", "repoId": "repo-1", "baseRef": "main",
			"branchPrefix": "feat", "prompt": "do it", "agentType": "claude", "n": 11,
		}))
	if err == nil {
		t.Fatal("expected a non-nil error when n is out of range, not a 200 with an empty items array")
	}
	if result != nil {
		t.Errorf("expected no partial result to leak through on error, got %+v", result)
	}
}
