package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeGitGatewayServiceClient/fakeProjectServiceClient are minimal test
// doubles — embed the (nil) interface, per fakeInfraFleetClient's
// precedent in channels_test.go, and override only the methods this
// file's channel handlers actually call.
type fakeGitGatewayServiceClient struct {
	gitgatewayv1.GitGatewayServiceClient

	createWorktreeFunc          func(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest) (*gitgatewayv1.CreateWorktreeResponse, error)
	createWorktreeFromIssueFunc func(ctx context.Context, in *gitgatewayv1.CreateWorktreeFromIssueRequest) (*gitgatewayv1.CreateWorktreeFromIssueResponse, error)
	removeWorktreeFunc     func(ctx context.Context, in *gitgatewayv1.RemoveWorktreeRequest) (*emptypb.Empty, error)
	forceDeleteBranchFunc  func(ctx context.Context, in *gitgatewayv1.ForceDeleteBranchRequest) (*emptypb.Empty, error)
	prefetchCreateBaseFunc func(ctx context.Context, in *gitgatewayv1.PrefetchCreateBaseRequest) (*gitgatewayv1.PrefetchCreateBaseResponse, error)
	resolvePrBaseFunc      func(ctx context.Context, in *gitgatewayv1.ResolvePrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error)
	resolveMrBaseFunc      func(ctx context.Context, in *gitgatewayv1.ResolveMrBaseRequest) (*gitgatewayv1.ResolveBaseResponse, error)
	detectWorktreesFunc    func(ctx context.Context, in *gitgatewayv1.DetectWorktreesRequest) (*gitgatewayv1.DetectWorktreesResponse, error)

	calledCreateWorktree          bool
	calledCreateWorktreeFromIssue bool
	calledRemoveWorktree          bool
	calledForceDeleteBranch  bool
	calledPrefetchCreateBase bool
	calledResolvePrBase      bool
	calledResolveMrBase      bool
	calledDetectWorktrees    bool
}

func (f *fakeGitGatewayServiceClient) CreateWorktree(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	f.calledCreateWorktree = true
	return f.createWorktreeFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) CreateWorktreeFromIssue(ctx context.Context, in *gitgatewayv1.CreateWorktreeFromIssueRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeFromIssueResponse, error) {
	f.calledCreateWorktreeFromIssue = true
	return f.createWorktreeFromIssueFunc(ctx, in)
}

func (f *fakeGitGatewayServiceClient) RemoveWorktree(ctx context.Context, in *gitgatewayv1.RemoveWorktreeRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.calledRemoveWorktree = true
	return f.removeWorktreeFunc(ctx, in)
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
	registerWorktreeChannels(r, git, project)

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

func TestWorktreeCreateFromIssueChannel_ScmShapeDecodesCorrectly(t *testing.T) {
	var gotReq *gitgatewayv1.CreateWorktreeFromIssueRequest
	git := &fakeGitGatewayServiceClient{
		createWorktreeFromIssueFunc: func(_ context.Context, in *gitgatewayv1.CreateWorktreeFromIssueRequest) (*gitgatewayv1.CreateWorktreeFromIssueResponse, error) {
			gotReq = in
			return &gitgatewayv1.CreateWorktreeFromIssueResponse{WorktreeId: "wt-1", BranchName: "fix/bug-42"}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.createFromIssue",
		argsJSON(t, map[string]any{
			"projectId": "proj-1", "repoId": "repo-1", "baseRef": "main",
			"provider": "github", "repo": "o/r", "number": 42,
		}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scmIssue := gotReq.GetScmIssue()
	if scmIssue == nil {
		t.Fatalf("expected ScmIssue oneof branch, got %+v", gotReq)
	}
	if scmIssue.GetProvider() != "github" || scmIssue.GetRepo() != "o/r" || scmIssue.GetNumber() != 42 {
		t.Errorf("unexpected ScmIssue: %+v", scmIssue)
	}
	resp, ok := result.(*gitgatewayv1.CreateWorktreeFromIssueResponse)
	if !ok || resp.GetWorktreeId() != "wt-1" || resp.GetBranchName() != "fix/bug-42" {
		t.Errorf("expected response to be returned unmodified, got %+v", result)
	}
}

func TestWorktreeCreateFromIssueChannel_TrackerShapeDecodesCorrectly(t *testing.T) {
	var gotReq *gitgatewayv1.CreateWorktreeFromIssueRequest
	git := &fakeGitGatewayServiceClient{
		createWorktreeFromIssueFunc: func(_ context.Context, in *gitgatewayv1.CreateWorktreeFromIssueRequest) (*gitgatewayv1.CreateWorktreeFromIssueResponse, error) {
			gotReq = in
			return &gitgatewayv1.CreateWorktreeFromIssueResponse{WorktreeId: "wt-2"}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.createFromIssue",
		argsJSON(t, map[string]any{
			"projectId": "proj-1", "repoId": "repo-1", "baseRef": "main",
			"provider": "linear", "issueRef": "ENG-123",
		}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	trackerIssue := gotReq.GetTrackerIssue()
	if trackerIssue == nil {
		t.Fatalf("expected TrackerIssue oneof branch, got %+v", gotReq)
	}
	if trackerIssue.GetProvider() != "linear" || trackerIssue.GetIssueRef() != "ENG-123" {
		t.Errorf("unexpected TrackerIssue: %+v", trackerIssue)
	}
}

func TestWorktreeCreateFromIssueChannel_UnknownProviderRejectedBeforeRPC(t *testing.T) {
	git := &fakeGitGatewayServiceClient{}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "worktree.createFromIssue",
		argsJSON(t, map[string]any{"projectId": "proj-1", "repoId": "repo-1", "provider": "bitbucket"}))
	if err == nil {
		t.Fatal("expected an error for an unknown provider")
	}
	if git.calledCreateWorktreeFromIssue {
		t.Error("expected no gRPC call for an unknown provider")
	}
}

func TestWorktreeCreateFromIssueChannel_ScmProviderMissingRepoOrNumberRejectedBeforeRPC(t *testing.T) {
	git := &fakeGitGatewayServiceClient{}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project)

	// Missing "number".
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "worktree.createFromIssue",
		argsJSON(t, map[string]any{"projectId": "proj-1", "repoId": "repo-1", "provider": "github", "repo": "o/r"}))
	if err == nil {
		t.Fatal("expected an error when repo is set but number is missing")
	}
	if git.calledCreateWorktreeFromIssue {
		t.Error("expected no gRPC call for a malformed scm issue ref")
	}
}

func TestWorktreeCreateFromIssueChannel_TrackerProviderMissingIssueRefRejectedBeforeRPC(t *testing.T) {
	git := &fakeGitGatewayServiceClient{}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "worktree.createFromIssue",
		argsJSON(t, map[string]any{"projectId": "proj-1", "repoId": "repo-1", "provider": "jira"}))
	if err == nil {
		t.Fatal("expected an error when issueRef is missing for a tracker provider")
	}
	if git.calledCreateWorktreeFromIssue {
		t.Error("expected no gRPC call for a malformed tracker issue ref")
	}
}

func TestWorktreeRmChannel_Success(t *testing.T) {
	var gotReq *gitgatewayv1.RemoveWorktreeRequest
	git := &fakeGitGatewayServiceClient{
		removeWorktreeFunc: func(_ context.Context, in *gitgatewayv1.RemoveWorktreeRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	project := &fakeProjectServiceClient{}
	r := NewRegistry()
	registerWorktreeChannels(r, git, project)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "worktree.rm",
		argsJSON(t, map[string]any{"worktreeId": "wt-1", "force": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetWorktreeId() != "wt-1" || !gotReq.GetForce() {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	if v, ok := result.(map[string]bool); !ok || !v["ok"] {
		t.Errorf("expected {ok:true}, got %+v", result)
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
	registerWorktreeChannels(r, git, project)

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
	registerWorktreeChannels(r, git, project)

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
	registerWorktreeChannels(r, git, project)

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
	registerWorktreeChannels(r, git, project)

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
	registerWorktreeChannels(r, git, project)

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
	registerWorktreeChannels(r, git, project)

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
	registerWorktreeChannels(r, git, project)

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
	registerWorktreeChannels(r, git, project)

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
	registerWorktreeChannels(r, git, project)

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
