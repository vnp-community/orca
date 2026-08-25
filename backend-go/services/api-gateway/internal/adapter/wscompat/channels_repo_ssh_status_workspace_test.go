package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeProjectClient is a minimal test double for
// projectv1.ProjectServiceClient — embeds the (nil) interface so it
// satisfies every method, overriding only what registerRepoChannels' tests
// exercise. Kept local to this file (not channels_test.go's fakes) to avoid
// merge conflicts with the parallel git.*/files.* pass — see this file's
// sibling channels_repo_ssh_status_workspace.go's top doc comment.
type fakeProjectClient struct {
	projectv1.ProjectServiceClient

	addRepoFunc      func(ctx context.Context, in *projectv1.AddRepoRequest) (*projectv1.AddRepoResponse, error)
	listReposFunc    func(ctx context.Context, in *projectv1.ListReposRequest) (*projectv1.ListReposResponse, error)
	reorderReposFunc func(ctx context.Context, in *projectv1.ReorderReposRequest) (*projectv1.ReorderReposResponse, error)
	removeRepoFunc   func(ctx context.Context, in *projectv1.RemoveRepoRequest) (*projectv1.RemoveRepoResponse, error)
	updateRepoFunc   func(ctx context.Context, in *projectv1.UpdateRepoRequest) (*projectv1.UpdateRepoResponse, error)
}

func (f *fakeProjectClient) AddRepo(ctx context.Context, in *projectv1.AddRepoRequest, _ ...grpc.CallOption) (*projectv1.AddRepoResponse, error) {
	return f.addRepoFunc(ctx, in)
}
func (f *fakeProjectClient) ListRepos(ctx context.Context, in *projectv1.ListReposRequest, _ ...grpc.CallOption) (*projectv1.ListReposResponse, error) {
	return f.listReposFunc(ctx, in)
}
func (f *fakeProjectClient) ReorderRepos(ctx context.Context, in *projectv1.ReorderReposRequest, _ ...grpc.CallOption) (*projectv1.ReorderReposResponse, error) {
	return f.reorderReposFunc(ctx, in)
}
func (f *fakeProjectClient) RemoveRepo(ctx context.Context, in *projectv1.RemoveRepoRequest, _ ...grpc.CallOption) (*projectv1.RemoveRepoResponse, error) {
	return f.removeRepoFunc(ctx, in)
}
func (f *fakeProjectClient) UpdateRepo(ctx context.Context, in *projectv1.UpdateRepoRequest, _ ...grpc.CallOption) (*projectv1.UpdateRepoResponse, error) {
	return f.updateRepoFunc(ctx, in)
}

// fakeGitGatewayClient is a minimal test double for
// gitgatewayv1.GitGatewayServiceClient covering only the 8 repo.*-owned
// RPCs this file's channels call.
type fakeGitGatewayClient struct {
	gitgatewayv1.GitGatewayServiceClient

	cloneFunc                  func(ctx context.Context, in *gitgatewayv1.CloneRequest) (*gitgatewayv1.CloneResponse, error)
	baseRefDefaultFunc         func(ctx context.Context, in *gitgatewayv1.BaseRefDefaultRequest) (*gitgatewayv1.BaseRefDefaultResponse, error)
	searchRefsFunc             func(ctx context.Context, in *gitgatewayv1.SearchRefsRequest) (*gitgatewayv1.SearchRefsResponse, error)
	initRepoFunc               func(ctx context.Context, in *gitgatewayv1.InitRepoRequest) (*gitgatewayv1.InitRepoResponse, error)
	checkHooksFunc             func(ctx context.Context, in *gitgatewayv1.CheckHooksRequest) (*gitgatewayv1.CheckHooksResponse, error)
	readIssueCommandFunc       func(ctx context.Context, in *gitgatewayv1.ReadIssueCommandRequest) (*gitgatewayv1.ReadIssueCommandResponse, error)
	writeIssueCommandFunc      func(ctx context.Context, in *gitgatewayv1.WriteIssueCommandRequest) (*emptypb.Empty, error)
	scanSetupScriptImportsFunc func(ctx context.Context, in *gitgatewayv1.ScanSetupScriptImportsRequest) (*gitgatewayv1.ScanSetupScriptImportsResponse, error)
}

func (f *fakeGitGatewayClient) Clone(ctx context.Context, in *gitgatewayv1.CloneRequest, _ ...grpc.CallOption) (*gitgatewayv1.CloneResponse, error) {
	return f.cloneFunc(ctx, in)
}
func (f *fakeGitGatewayClient) BaseRefDefault(ctx context.Context, in *gitgatewayv1.BaseRefDefaultRequest, _ ...grpc.CallOption) (*gitgatewayv1.BaseRefDefaultResponse, error) {
	return f.baseRefDefaultFunc(ctx, in)
}
func (f *fakeGitGatewayClient) SearchRefs(ctx context.Context, in *gitgatewayv1.SearchRefsRequest, _ ...grpc.CallOption) (*gitgatewayv1.SearchRefsResponse, error) {
	return f.searchRefsFunc(ctx, in)
}
func (f *fakeGitGatewayClient) InitRepo(ctx context.Context, in *gitgatewayv1.InitRepoRequest, _ ...grpc.CallOption) (*gitgatewayv1.InitRepoResponse, error) {
	return f.initRepoFunc(ctx, in)
}
func (f *fakeGitGatewayClient) CheckHooks(ctx context.Context, in *gitgatewayv1.CheckHooksRequest, _ ...grpc.CallOption) (*gitgatewayv1.CheckHooksResponse, error) {
	return f.checkHooksFunc(ctx, in)
}
func (f *fakeGitGatewayClient) ReadIssueCommand(ctx context.Context, in *gitgatewayv1.ReadIssueCommandRequest, _ ...grpc.CallOption) (*gitgatewayv1.ReadIssueCommandResponse, error) {
	return f.readIssueCommandFunc(ctx, in)
}

// WriteIssueCommand returns google.protobuf.Empty on the wire; this fake
// ignores that and reports via error only, since none of this file's tests
// need to inspect a successful Empty response's fields.
func (f *fakeGitGatewayClient) WriteIssueCommand(ctx context.Context, in *gitgatewayv1.WriteIssueCommandRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.writeIssueCommandFunc(ctx, in)
}
func (f *fakeGitGatewayClient) ScanSetupScriptImports(ctx context.Context, in *gitgatewayv1.ScanSetupScriptImportsRequest, _ ...grpc.CallOption) (*gitgatewayv1.ScanSetupScriptImportsResponse, error) {
	return f.scanSetupScriptImportsFunc(ctx, in)
}

// fakeRepoSshStatusWorkspaceInfraFleetClient is a minimal test double for
// infrafleetv1.InfraFleetServiceClient covering ssh.*/workspacePorts.*'s
// backing RPCs — kept separate from channels_test.go's fakeInfraFleetClient
// per this file's merge-conflict-avoidance rationale.
type fakeRepoSshStatusWorkspaceInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	listSshTargetsFunc      func(ctx context.Context, in *infrafleetv1.ListSshTargetsRequest) (*infrafleetv1.ListSshTargetsResponse, error)
	getSshStateFunc         func(ctx context.Context, in *infrafleetv1.GetSshStateRequest) (*infrafleetv1.GetSshStateResponse, error)
	establishConnectionFunc func(ctx context.Context, in *infrafleetv1.EstablishConnectionRequest) (*infrafleetv1.Connection, error)
	scanWorkspacePortsFunc  func(ctx context.Context, in *infrafleetv1.ScanWorkspacePortsRequest) (*infrafleetv1.ScanWorkspacePortsResponse, error)
	killWorkspacePortFunc   func(ctx context.Context, in *infrafleetv1.KillWorkspacePortRequest) (*infrafleetv1.KillWorkspacePortResponse, error)
}

func (f *fakeRepoSshStatusWorkspaceInfraFleetClient) ListSshTargets(ctx context.Context, in *infrafleetv1.ListSshTargetsRequest, _ ...grpc.CallOption) (*infrafleetv1.ListSshTargetsResponse, error) {
	return f.listSshTargetsFunc(ctx, in)
}
func (f *fakeRepoSshStatusWorkspaceInfraFleetClient) GetSshState(ctx context.Context, in *infrafleetv1.GetSshStateRequest, _ ...grpc.CallOption) (*infrafleetv1.GetSshStateResponse, error) {
	return f.getSshStateFunc(ctx, in)
}
func (f *fakeRepoSshStatusWorkspaceInfraFleetClient) EstablishConnection(ctx context.Context, in *infrafleetv1.EstablishConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.Connection, error) {
	return f.establishConnectionFunc(ctx, in)
}
func (f *fakeRepoSshStatusWorkspaceInfraFleetClient) ScanWorkspacePorts(ctx context.Context, in *infrafleetv1.ScanWorkspacePortsRequest, _ ...grpc.CallOption) (*infrafleetv1.ScanWorkspacePortsResponse, error) {
	return f.scanWorkspacePortsFunc(ctx, in)
}
func (f *fakeRepoSshStatusWorkspaceInfraFleetClient) KillWorkspacePort(ctx context.Context, in *infrafleetv1.KillWorkspacePortRequest, _ ...grpc.CallOption) (*infrafleetv1.KillWorkspacePortResponse, error) {
	return f.killWorkspacePortFunc(ctx, in)
}

// ── repo.* ───────────────────────────────────────────────────────────────

func TestRegisterRepoChannels_AddListReorderRmUpdate(t *testing.T) {
	fake := &fakeProjectClient{}
	r := NewRegistry()
	registerRepoChannels(r, fake, &fakeGitGatewayClient{})

	t.Run("repo.add", func(t *testing.T) {
		var gotReq *projectv1.AddRepoRequest
		fake.addRepoFunc = func(ctx context.Context, in *projectv1.AddRepoRequest) (*projectv1.AddRepoResponse, error) {
			gotReq = in
			return &projectv1.AddRepoResponse{Repo: &projectv1.Repo{Id: "r1"}}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.add",
			argsJSON(t, map[string]any{"projectId": "p1", "url": "https://x", "displayName": "X"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetProjectId() != "p1" || gotReq.GetUrl() != "https://x" || gotReq.GetDisplayName() != "X" {
			t.Errorf("unexpected AddRepoRequest: %+v", gotReq)
		}
	})

	t.Run("repo.list", func(t *testing.T) {
		var gotReq *projectv1.ListReposRequest
		fake.listReposFunc = func(ctx context.Context, in *projectv1.ListReposRequest) (*projectv1.ListReposResponse, error) {
			gotReq = in
			return &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "r1"}}}, nil
		}
		result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.list",
			argsJSON(t, map[string]any{"projectId": "p1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetProjectId() != "p1" {
			t.Errorf("unexpected ListReposRequest: %+v", gotReq)
		}
		repos, ok := result.([]*projectv1.Repo)
		if !ok || len(repos) != 1 {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	t.Run("repo.reorder", func(t *testing.T) {
		var gotReq *projectv1.ReorderReposRequest
		fake.reorderReposFunc = func(ctx context.Context, in *projectv1.ReorderReposRequest) (*projectv1.ReorderReposResponse, error) {
			gotReq = in
			return &projectv1.ReorderReposResponse{}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.reorder",
			argsJSON(t, map[string]any{"projectId": "p1", "repoIdsInOrder": []string{"r2", "r1"}}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetProjectId() != "p1" || len(gotReq.GetRepoIdsInOrder()) != 2 {
			t.Errorf("unexpected ReorderReposRequest: %+v", gotReq)
		}
	})

	t.Run("repo.rm", func(t *testing.T) {
		var gotReq *projectv1.RemoveRepoRequest
		fake.removeRepoFunc = func(ctx context.Context, in *projectv1.RemoveRepoRequest) (*projectv1.RemoveRepoResponse, error) {
			gotReq = in
			return &projectv1.RemoveRepoResponse{}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.rm",
			argsJSON(t, map[string]any{"repoId": "r1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetRepoId() != "r1" {
			t.Errorf("unexpected RemoveRepoRequest: %+v", gotReq)
		}
	})

	t.Run("repo.update", func(t *testing.T) {
		var gotReq *projectv1.UpdateRepoRequest
		fake.updateRepoFunc = func(ctx context.Context, in *projectv1.UpdateRepoRequest) (*projectv1.UpdateRepoResponse, error) {
			gotReq = in
			return &projectv1.UpdateRepoResponse{Repo: &projectv1.Repo{Id: "r1", Url: "https://new"}}, nil
		}
		result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.update",
			argsJSON(t, map[string]any{"repoId": "r1", "url": "https://new"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetRepoId() != "r1" || gotReq.GetUrl() != "https://new" {
			t.Errorf("unexpected UpdateRepoRequest: %+v", gotReq)
		}
		repo, ok := result.(*projectv1.Repo)
		if !ok || repo.GetUrl() != "https://new" {
			t.Errorf("unexpected result: %+v", result)
		}
	})
}

func TestRegisterRepoChannels_GitGatewayOwnedMethods(t *testing.T) {
	git := &fakeGitGatewayClient{}
	r := NewRegistry()
	registerRepoChannels(r, &fakeProjectClient{}, git)

	t.Run("repo.clone", func(t *testing.T) {
		var gotReq *gitgatewayv1.CloneRequest
		git.cloneFunc = func(ctx context.Context, in *gitgatewayv1.CloneRequest) (*gitgatewayv1.CloneResponse, error) {
			gotReq = in
			return &gitgatewayv1.CloneResponse{WorktreePath: "/repo", DefaultBranch: "main"}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.clone",
			argsJSON(t, map[string]any{"devServerId": "ds1", "url": "https://x", "destPath": "/repo"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetDevServerId() != "ds1" || gotReq.GetUrl() != "https://x" || gotReq.GetDestPath() != "/repo" {
			t.Errorf("unexpected CloneRequest: %+v", gotReq)
		}
	})

	t.Run("repo.baseRefDefault", func(t *testing.T) {
		var gotReq *gitgatewayv1.BaseRefDefaultRequest
		git.baseRefDefaultFunc = func(ctx context.Context, in *gitgatewayv1.BaseRefDefaultRequest) (*gitgatewayv1.BaseRefDefaultResponse, error) {
			gotReq = in
			return &gitgatewayv1.BaseRefDefaultResponse{Ref: "main"}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.baseRefDefault",
			argsJSON(t, map[string]any{"worktreeId": "wt1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetWorktreeId() != "wt1" {
			t.Errorf("unexpected BaseRefDefaultRequest: %+v", gotReq)
		}
	})

	t.Run("repo.searchRefs", func(t *testing.T) {
		var gotReq *gitgatewayv1.SearchRefsRequest
		git.searchRefsFunc = func(ctx context.Context, in *gitgatewayv1.SearchRefsRequest) (*gitgatewayv1.SearchRefsResponse, error) {
			gotReq = in
			return &gitgatewayv1.SearchRefsResponse{Refs: []string{"main"}}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.searchRefs",
			argsJSON(t, map[string]any{"worktreeId": "wt1", "query": "mai"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetWorktreeId() != "wt1" || gotReq.GetQuery() != "mai" {
			t.Errorf("unexpected SearchRefsRequest: %+v", gotReq)
		}
	})

	t.Run("repo.create", func(t *testing.T) {
		var gotReq *gitgatewayv1.InitRepoRequest
		git.initRepoFunc = func(ctx context.Context, in *gitgatewayv1.InitRepoRequest) (*gitgatewayv1.InitRepoResponse, error) {
			gotReq = in
			return &gitgatewayv1.InitRepoResponse{Path: "/repo", DefaultBranch: "main"}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.create",
			argsJSON(t, map[string]any{"devServerId": "ds1", "destPath": "/repo", "defaultBranch": "main"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetDevServerId() != "ds1" || gotReq.GetDestPath() != "/repo" || gotReq.GetDefaultBranch() != "main" {
			t.Errorf("unexpected InitRepoRequest: %+v", gotReq)
		}
	})

	t.Run("repo.hooksCheck", func(t *testing.T) {
		var gotReq *gitgatewayv1.CheckHooksRequest
		git.checkHooksFunc = func(ctx context.Context, in *gitgatewayv1.CheckHooksRequest) (*gitgatewayv1.CheckHooksResponse, error) {
			gotReq = in
			return &gitgatewayv1.CheckHooksResponse{OrcaHooksCurrent: true}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.hooksCheck",
			argsJSON(t, map[string]any{"worktreeId": "wt1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetWorktreeId() != "wt1" {
			t.Errorf("unexpected CheckHooksRequest: %+v", gotReq)
		}
	})

	t.Run("repo.issueCommandRead", func(t *testing.T) {
		var gotReq *gitgatewayv1.ReadIssueCommandRequest
		git.readIssueCommandFunc = func(ctx context.Context, in *gitgatewayv1.ReadIssueCommandRequest) (*gitgatewayv1.ReadIssueCommandResponse, error) {
			gotReq = in
			return &gitgatewayv1.ReadIssueCommandResponse{Content: "x", Exists: true}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.issueCommandRead",
			argsJSON(t, map[string]any{"worktreeId": "wt1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetWorktreeId() != "wt1" {
			t.Errorf("unexpected ReadIssueCommandRequest: %+v", gotReq)
		}
	})

	t.Run("repo.issueCommandWrite", func(t *testing.T) {
		var gotReq *gitgatewayv1.WriteIssueCommandRequest
		git.writeIssueCommandFunc = func(ctx context.Context, in *gitgatewayv1.WriteIssueCommandRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.issueCommandWrite",
			argsJSON(t, map[string]any{"worktreeId": "wt1", "content": "x"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetWorktreeId() != "wt1" || gotReq.GetContent() != "x" {
			t.Errorf("unexpected WriteIssueCommandRequest: %+v", gotReq)
		}
	})

	t.Run("repo.setupScriptImports", func(t *testing.T) {
		var gotReq *gitgatewayv1.ScanSetupScriptImportsRequest
		git.scanSetupScriptImportsFunc = func(ctx context.Context, in *gitgatewayv1.ScanSetupScriptImportsRequest) (*gitgatewayv1.ScanSetupScriptImportsResponse, error) {
			gotReq = in
			return &gitgatewayv1.ScanSetupScriptImportsResponse{ImportedPaths: []string{"a"}}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "repo.setupScriptImports",
			argsJSON(t, map[string]any{"worktreeId": "wt1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetWorktreeId() != "wt1" {
			t.Errorf("unexpected ScanSetupScriptImportsRequest: %+v", gotReq)
		}
	})
}

func TestRegisterRepoChannels_RegistrationCoverage(t *testing.T) {
	r := NewRegistry()
	registerRepoChannels(r, &fakeProjectClient{}, &fakeGitGatewayClient{})

	want := []string{
		"repo.add", "repo.list", "repo.reorder", "repo.rm", "repo.update",
		"repo.clone", "repo.baseRefDefault", "repo.searchRefs", "repo.create",
		"repo.hooksCheck", "repo.issueCommandRead", "repo.issueCommandWrite",
		"repo.setupScriptImports",
	}
	for _, channel := range want {
		if _, ok := r.handlers[channel]; !ok {
			t.Errorf("expected channel %q to be registered", channel)
		}
	}
}

// ── ssh.* ────────────────────────────────────────────────────────────────

func TestRegisterSshChannels(t *testing.T) {
	fake := &fakeRepoSshStatusWorkspaceInfraFleetClient{}
	r := NewRegistry()
	registerSshChannels(r, fake)

	t.Run("ssh.listTargets", func(t *testing.T) {
		fake.listSshTargetsFunc = func(ctx context.Context, in *infrafleetv1.ListSshTargetsRequest) (*infrafleetv1.ListSshTargetsResponse, error) {
			return &infrafleetv1.ListSshTargetsResponse{SshTargets: []*infrafleetv1.SshTarget{{Id: "s1", User: "root"}}}, nil
		}
		result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "ssh.listTargets", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		targets, ok := result.([]*infrafleetv1.SshTarget)
		if !ok || len(targets) != 1 || targets[0].GetId() != "s1" {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	t.Run("ssh.getUserAccount derives from ListSshTargets, filters by id", func(t *testing.T) {
		var listCalled bool
		fake.listSshTargetsFunc = func(ctx context.Context, in *infrafleetv1.ListSshTargetsRequest) (*infrafleetv1.ListSshTargetsResponse, error) {
			listCalled = true
			return &infrafleetv1.ListSshTargetsResponse{SshTargets: []*infrafleetv1.SshTarget{
				{Id: "s1", User: "root"},
				{Id: "s2", User: "ubuntu"},
			}}, nil
		}
		result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "ssh.getUserAccount",
			argsJSON(t, map[string]any{"sshTargetId": "s2"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !listCalled {
			t.Error("expected ssh.getUserAccount to call ListSshTargets, not a dedicated RPC")
		}
		account, ok := result.(map[string]string)
		if !ok || account["username"] != "ubuntu" {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	t.Run("ssh.getUserAccount not found", func(t *testing.T) {
		fake.listSshTargetsFunc = func(ctx context.Context, in *infrafleetv1.ListSshTargetsRequest) (*infrafleetv1.ListSshTargetsResponse, error) {
			return &infrafleetv1.ListSshTargetsResponse{}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "ssh.getUserAccount",
			argsJSON(t, map[string]any{"sshTargetId": "missing"}))
		if err == nil {
			t.Fatal("expected error for unknown ssh target")
		}
	})

	t.Run("ssh.getState", func(t *testing.T) {
		var gotReq *infrafleetv1.GetSshStateRequest
		fake.getSshStateFunc = func(ctx context.Context, in *infrafleetv1.GetSshStateRequest) (*infrafleetv1.GetSshStateResponse, error) {
			gotReq = in
			return &infrafleetv1.GetSshStateResponse{Connected: true, ConnectionId: "c1"}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "ssh.getState",
			argsJSON(t, map[string]any{"sshTargetId": "s1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetSshTargetId() != "s1" {
			t.Errorf("unexpected GetSshStateRequest: %+v", gotReq)
		}
	})

	t.Run("ssh.connect", func(t *testing.T) {
		var gotReq *infrafleetv1.EstablishConnectionRequest
		fake.establishConnectionFunc = func(ctx context.Context, in *infrafleetv1.EstablishConnectionRequest) (*infrafleetv1.Connection, error) {
			gotReq = in
			return &infrafleetv1.Connection{Id: "c1", Status: "established"}, nil
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "ssh.connect",
			argsJSON(t, map[string]any{"sshTargetId": "s1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetSshTargetId() != "s1" {
			t.Errorf("unexpected EstablishConnectionRequest: %+v", gotReq)
		}
	})

	t.Run("ssh.connect propagates error", func(t *testing.T) {
		wantErr := errors.New("ssh handshake failed")
		fake.establishConnectionFunc = func(ctx context.Context, in *infrafleetv1.EstablishConnectionRequest) (*infrafleetv1.Connection, error) {
			return nil, wantErr
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "ssh.connect",
			argsJSON(t, map[string]any{"sshTargetId": "s1"}))
		if !errors.Is(err, wantErr) {
			t.Fatalf("want error %v, got %v", wantErr, err)
		}
	})
}

// ── status.get ───────────────────────────────────────────────────────────

func TestStatusGet_ReturnsHostPlatformAndHonestZeroValues(t *testing.T) {
	r := NewRegistry()
	registerStatusChannels(r)
	result, err := r.Dispatch(context.Background(), Identity{}, "status.get", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if m["hostPlatform"] == "" {
		t.Error("want non-empty hostPlatform")
	}
	if m["liveTabCount"] != 0 || m["authoritativeWindowId"] != nil {
		t.Error("want honest zero-values for window-graph fields, not fabricated data")
	}
	if _, ok := r.handlers["status.get"]; !ok {
		t.Error("expected status.get to be registered")
	}
}

// ── workspacePorts.* ─────────────────────────────────────────────────────

func TestRegisterWorkspacePortsChannels(t *testing.T) {
	fake := &fakeRepoSshStatusWorkspaceInfraFleetClient{}
	r := NewRegistry()
	registerWorkspacePortsChannels(r, fake)

	t.Run("workspacePorts.scan", func(t *testing.T) {
		var gotReq *infrafleetv1.ScanWorkspacePortsRequest
		fake.scanWorkspacePortsFunc = func(ctx context.Context, in *infrafleetv1.ScanWorkspacePortsRequest) (*infrafleetv1.ScanWorkspacePortsResponse, error) {
			gotReq = in
			return &infrafleetv1.ScanWorkspacePortsResponse{OpenPorts: []int32{3000, 8080}}, nil
		}
		result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "workspacePorts.scan",
			argsJSON(t, map[string]any{"connectionId": "c1", "worktreeId": "wt1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetConnectionId() != "c1" || gotReq.GetWorktreeId() != "wt1" {
			t.Errorf("unexpected ScanWorkspacePortsRequest: %+v", gotReq)
		}
		view, ok := result.(map[string]any)
		if !ok || view["platform"] != "unknown" {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	t.Run("workspacePorts.kill success", func(t *testing.T) {
		var gotReq *infrafleetv1.KillWorkspacePortRequest
		fake.killWorkspacePortFunc = func(ctx context.Context, in *infrafleetv1.KillWorkspacePortRequest) (*infrafleetv1.KillWorkspacePortResponse, error) {
			gotReq = in
			return &infrafleetv1.KillWorkspacePortResponse{Ok: true}, nil
		}
		result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "workspacePorts.kill",
			argsJSON(t, map[string]any{"connectionId": "c1", "worktreeId": "wt1", "pid": 123, "port": 8080}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotReq.GetConnectionId() != "c1" || gotReq.GetPid() != 123 || gotReq.GetPort() != 8080 {
			t.Errorf("unexpected KillWorkspacePortRequest: %+v", gotReq)
		}
		m, ok := result.(map[string]any)
		if !ok || m["ok"] != true {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	t.Run("workspacePorts.kill failure passes through reason", func(t *testing.T) {
		fake.killWorkspacePortFunc = func(ctx context.Context, in *infrafleetv1.KillWorkspacePortRequest) (*infrafleetv1.KillWorkspacePortResponse, error) {
			return &infrafleetv1.KillWorkspacePortResponse{Ok: false, Reason: "not implemented"}, nil
		}
		result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "workspacePorts.kill",
			argsJSON(t, map[string]any{"connectionId": "", "worktreeId": "wt1"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		m, ok := result.(map[string]any)
		if !ok || m["ok"] != false || m["reason"] != "not implemented" {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	if _, ok := r.handlers["workspacePorts.scan"]; !ok {
		t.Error("expected workspacePorts.scan to be registered")
	}
	if _, ok := r.handlers["workspacePorts.kill"]; !ok {
		t.Error("expected workspacePorts.kill to be registered")
	}
}

// ── umbrella wiring ──────────────────────────────────────────────────────

func TestRegisterRepoSshStatusWorkspaceChannels_RegistersEverything(t *testing.T) {
	r := NewRegistry()
	registerRepoSshStatusWorkspaceChannels(r, &fakeProjectClient{}, &fakeGitGatewayClient{}, &fakeRepoSshStatusWorkspaceInfraFleetClient{})

	want := []string{
		"repo.add", "repo.list", "repo.reorder", "repo.rm", "repo.update",
		"repo.clone", "repo.baseRefDefault", "repo.searchRefs", "repo.create",
		"repo.hooksCheck", "repo.issueCommandRead", "repo.issueCommandWrite",
		"repo.setupScriptImports",
		"ssh.listTargets", "ssh.getUserAccount", "ssh.getState", "ssh.connect",
		"status.get",
		"workspacePorts.scan", "workspacePorts.kill",
	}
	for _, channel := range want {
		if _, ok := r.handlers[channel]; !ok {
			t.Errorf("expected channel %q to be registered", channel)
		}
	}
}
