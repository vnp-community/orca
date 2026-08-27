package wscompat

import (
	"context"
	"errors"
	"testing"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// newWorkspaceTestRegistry wires only workspace.refreshFileTree, against
// the fakeGitGatewayClient/fakeProjectServiceClient already established by
// channels_git_test.go/channels_worktree_test.go (same package).
func newWorkspaceTestRegistry(git *fakeGitGatewayClient, project *fakeProjectServiceClient) *Registry {
	r := NewRegistry()
	registerWorkspaceChannels(r, git, project)
	return r
}

func TestWorkspaceRefreshFileTree_ResolvesActiveWorktreeAndReshapesEntries(t *testing.T) {
	project := &fakeProjectServiceClient{
		listWorktreesFunc: func(ctx context.Context, in *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
			if in.GetProjectId() != "proj-1" {
				t.Fatalf("expected projectId=proj-1, got %q", in.GetProjectId())
			}
			return &projectv1.ListWorktreesResponse{Worktrees: []*projectv1.Worktree{
				{Id: "wt-inactive", Active: false},
				{Id: "wt-active", Active: true},
			}}, nil
		},
	}
	git := &fakeGitGatewayClient{
		readDirFunc: func(ctx context.Context, in *gitgatewayv1.ReadDirRequest) (*gitgatewayv1.ReadDirResponse, error) {
			if in.GetWorktreeId() != "wt-active" {
				t.Fatalf("expected the active worktree to be resolved, got %q", in.GetWorktreeId())
			}
			if in.GetPath() != "src" {
				t.Fatalf("expected path=src, got %q", in.GetPath())
			}
			return &gitgatewayv1.ReadDirResponse{Entries: []*gitgatewayv1.DirEntry{
				{Name: "main.go", IsDirectory: false},
				{Name: "internal", IsDirectory: true},
			}}, nil
		},
	}
	r := newWorkspaceTestRegistry(git, project)

	got, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "workspace.refreshFileTree",
		argsJSON(t, map[string]any{"projectId": "proj-1", "path": "src"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes, ok := got.([]backendFileTreeNode)
	if !ok {
		t.Fatalf("expected []backendFileTreeNode, got %T", got)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(nodes))
	}
	if nodes[0].Name != "main.go" || nodes[0].Path != "src/main.go" || nodes[0].IsDir {
		t.Errorf("unexpected first entry: %+v", nodes[0])
	}
	if nodes[1].Name != "internal" || nodes[1].Path != "src/internal" || !nodes[1].IsDir {
		t.Errorf("unexpected second entry: %+v", nodes[1])
	}
}

func TestWorkspaceRefreshFileTree_DefaultsPathToDotAndFallsBackToFirstWorktree(t *testing.T) {
	project := &fakeProjectServiceClient{
		listWorktreesFunc: func(ctx context.Context, in *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
			// No worktree marked active -> fall back to the first one.
			return &projectv1.ListWorktreesResponse{Worktrees: []*projectv1.Worktree{{Id: "wt-only"}}}, nil
		},
	}
	git := &fakeGitGatewayClient{
		readDirFunc: func(ctx context.Context, in *gitgatewayv1.ReadDirRequest) (*gitgatewayv1.ReadDirResponse, error) {
			if in.GetWorktreeId() != "wt-only" {
				t.Fatalf("expected fallback to the only worktree, got %q", in.GetWorktreeId())
			}
			if in.GetPath() != "." {
				t.Fatalf("expected path to default to '.', got %q", in.GetPath())
			}
			return &gitgatewayv1.ReadDirResponse{}, nil
		},
	}
	r := newWorkspaceTestRegistry(git, project)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "workspace.refreshFileTree",
		argsJSON(t, map[string]any{"projectId": "proj-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkspaceRefreshFileTree_MissingProjectID_ReturnsError(t *testing.T) {
	r := newWorkspaceTestRegistry(&fakeGitGatewayClient{}, &fakeProjectServiceClient{})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "workspace.refreshFileTree",
		argsJSON(t, map[string]any{"path": "."}))
	if err == nil {
		t.Fatal("expected an error for missing projectId")
	}
}

func TestWorkspaceRefreshFileTree_NoWorktreesForProject_ReturnsError(t *testing.T) {
	project := &fakeProjectServiceClient{
		listWorktreesFunc: func(ctx context.Context, in *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
			return &projectv1.ListWorktreesResponse{}, nil
		},
	}
	r := newWorkspaceTestRegistry(&fakeGitGatewayClient{}, project)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "workspace.refreshFileTree",
		argsJSON(t, map[string]any{"projectId": "proj-1"}))
	if err == nil {
		t.Fatal("expected an error when the project has no worktrees")
	}
}

func TestWorkspaceRefreshFileTree_PropagatesReadDirFailure(t *testing.T) {
	project := &fakeProjectServiceClient{
		listWorktreesFunc: func(ctx context.Context, in *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
			return &projectv1.ListWorktreesResponse{Worktrees: []*projectv1.Worktree{{Id: "wt-1", Active: true}}}, nil
		},
	}
	git := &fakeGitGatewayClient{
		readDirFunc: func(ctx context.Context, in *gitgatewayv1.ReadDirRequest) (*gitgatewayv1.ReadDirResponse, error) {
			return nil, errors.New("boom")
		},
	}
	r := newWorkspaceTestRegistry(git, project)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "workspace.refreshFileTree",
		argsJSON(t, map[string]any{"projectId": "proj-1"}))
	if err == nil {
		t.Fatal("expected ReadDir failure to propagate")
	}
}
