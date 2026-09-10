package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeOrcaProjectSharingClient is a minimal test double for
// projectv1.ProjectServiceClient — embeds the nil interface, overrides
// only the methods channels_orca_project_sharing.go's handlers call, same
// pattern as fakeProjectServiceClient2/fakeTenantServiceClient2.
type fakeOrcaProjectSharingClient struct {
	projectv1.ProjectServiceClient

	listProjectsFunc         func(ctx context.Context, in *projectv1.ListProjectsRequest) (*projectv1.ListProjectsResponse, error)
	listSourceProjectsFunc   func(ctx context.Context, in *projectv1.ListSourceProjectsRequest) (*projectv1.ListSourceProjectsResponse, error)
	linkSourceProjectFunc    func(ctx context.Context, in *projectv1.LinkSourceProjectRequest) (*projectv1.LinkSourceProjectResponse, error)
	unlinkSourceProjectFunc  func(ctx context.Context, in *projectv1.UnlinkSourceProjectRequest) (*projectv1.UnlinkSourceProjectResponse, error)
	getSharedProjectDataFunc func(ctx context.Context, in *projectv1.GetSharedProjectDataRequest) (*projectv1.GetSharedProjectDataResponse, error)
}

func (f *fakeOrcaProjectSharingClient) ListProjects(ctx context.Context, in *projectv1.ListProjectsRequest, _ ...grpc.CallOption) (*projectv1.ListProjectsResponse, error) {
	return f.listProjectsFunc(ctx, in)
}
func (f *fakeOrcaProjectSharingClient) ListSourceProjects(ctx context.Context, in *projectv1.ListSourceProjectsRequest, _ ...grpc.CallOption) (*projectv1.ListSourceProjectsResponse, error) {
	return f.listSourceProjectsFunc(ctx, in)
}
func (f *fakeOrcaProjectSharingClient) LinkSourceProject(ctx context.Context, in *projectv1.LinkSourceProjectRequest, _ ...grpc.CallOption) (*projectv1.LinkSourceProjectResponse, error) {
	return f.linkSourceProjectFunc(ctx, in)
}
func (f *fakeOrcaProjectSharingClient) UnlinkSourceProject(ctx context.Context, in *projectv1.UnlinkSourceProjectRequest, _ ...grpc.CallOption) (*projectv1.UnlinkSourceProjectResponse, error) {
	return f.unlinkSourceProjectFunc(ctx, in)
}
func (f *fakeOrcaProjectSharingClient) GetSharedProjectData(ctx context.Context, in *projectv1.GetSharedProjectDataRequest, _ ...grpc.CallOption) (*projectv1.GetSharedProjectDataResponse, error) {
	return f.getSharedProjectDataFunc(ctx, in)
}

func TestOrcaProjectsListChannel_ReturnsEachProjectWithItsSourceProjects(t *testing.T) {
	fake := &fakeOrcaProjectSharingClient{
		listProjectsFunc: func(ctx context.Context, in *projectv1.ListProjectsRequest) (*projectv1.ListProjectsResponse, error) {
			if in.GetTenantId() != "tenant-1" {
				t.Errorf("expected TenantId from Identity, got %q", in.GetTenantId())
			}
			return &projectv1.ListProjectsResponse{Projects: []*projectv1.Project{{Id: "p1", Name: "Project One"}}}, nil
		},
		listSourceProjectsFunc: func(ctx context.Context, in *projectv1.ListSourceProjectsRequest) (*projectv1.ListSourceProjectsResponse, error) {
			if in.GetContainerProjectId() != "p1" {
				t.Errorf("expected ContainerProjectId=p1, got %q", in.GetContainerProjectId())
			}
			return &projectv1.ListSourceProjectsResponse{SourceProjects: []*projectv1.SourceProject{
				{SourceProjectId: "p2", LinkedBy: "u1"},
			}}, nil
		},
	}
	r := NewRegistry()
	registerOrcaProjectSharingChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "orcaProjects.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("result did not marshal to a JSON array: %v (%s)", err, raw)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d: %+v", len(items), items)
	}
	orcaProject, _ := items[0]["orcaProject"].(map[string]any)
	if orcaProject["id"] != "p1" {
		t.Errorf("expected orcaProject.id=p1, got %+v", orcaProject)
	}
	sourceProjects, _ := items[0]["sourceProjects"].([]any)
	if len(sourceProjects) != 1 {
		t.Fatalf("expected 1 source project, got %+v", sourceProjects)
	}
	ref, _ := sourceProjects[0].(map[string]any)
	if ref["projectId"] != "p2" || ref["ownerUserId"] != "u1" {
		t.Errorf("expected {projectId:p2, ownerUserId:u1}, got %+v", ref)
	}
}

func TestOrcaProjectsLinkSourceProjectChannel_Success(t *testing.T) {
	var gotReq *projectv1.LinkSourceProjectRequest
	fake := &fakeOrcaProjectSharingClient{
		linkSourceProjectFunc: func(ctx context.Context, in *projectv1.LinkSourceProjectRequest) (*projectv1.LinkSourceProjectResponse, error) {
			gotReq = in
			return &projectv1.LinkSourceProjectResponse{SourceProject: &projectv1.SourceProject{}}, nil
		},
	}
	r := NewRegistry()
	registerOrcaProjectSharingChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "u1"}, "orcaProjects.linkSourceProject",
		argsJSON(t, map[string]any{"orcaProjectId": "container-1", "projectId": "source-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetContainerProjectId() != "container-1" || gotReq.GetSourceProjectId() != "source-1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	assertJSONKeys(t, result, "success")
}

func TestOrcaProjectsUnlinkSourceProjectChannel_Success(t *testing.T) {
	var gotReq *projectv1.UnlinkSourceProjectRequest
	fake := &fakeOrcaProjectSharingClient{
		unlinkSourceProjectFunc: func(ctx context.Context, in *projectv1.UnlinkSourceProjectRequest) (*projectv1.UnlinkSourceProjectResponse, error) {
			gotReq = in
			return &projectv1.UnlinkSourceProjectResponse{}, nil
		},
	}
	r := NewRegistry()
	registerOrcaProjectSharingChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "u1"}, "orcaProjects.unlinkSourceProject",
		argsJSON(t, map[string]any{"orcaProjectId": "container-1", "projectId": "source-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetContainerProjectId() != "container-1" || gotReq.GetSourceProjectId() != "source-1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	assertJSONKeys(t, result, "success")
}

func TestOrcaProjectsGetProjectDataChannel_ReturnsProjectReposWorktrees(t *testing.T) {
	fake := &fakeOrcaProjectSharingClient{
		getSharedProjectDataFunc: func(ctx context.Context, in *projectv1.GetSharedProjectDataRequest) (*projectv1.GetSharedProjectDataResponse, error) {
			return &projectv1.GetSharedProjectDataResponse{
				Project: &projectv1.Project{Id: "source-1", Name: "Source"},
				Repos:   []*projectv1.Repo{{Id: "r1", ProjectId: "source-1"}},
				Worktrees: []*projectv1.Worktree{
					{Id: "w1", RepoId: "r1", Path: "/tmp/wt", Branch: "main"},
				},
			}, nil
		},
	}
	r := NewRegistry()
	registerOrcaProjectSharingChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "u1"}, "orcaProjects.getProjectData",
		argsJSON(t, map[string]any{"orcaProjectId": "container-1", "projectId": "source-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := assertJSONKeys(t, result, "project", "repos", "worktrees")
	project, _ := decoded["project"].(map[string]any)
	if project["id"] != "source-1" {
		t.Errorf("expected project.id=source-1, got %+v", project)
	}
}

// TestOrcaProjectsChannels_PropagateErrors matches this package's
// established convention (e.g. TestProfileChannels_PropagateErrors): a
// downstream gRPC error (project-service's requireProjectAccess denial,
// in real use) passes through unchanged — no special-casing needed here
// since project-service's own OPA gate is the sole authority.
func TestOrcaProjectsChannels_PropagateErrors(t *testing.T) {
	wantErr := errors.New("boom")
	fake := &fakeOrcaProjectSharingClient{
		listProjectsFunc: func(ctx context.Context, in *projectv1.ListProjectsRequest) (*projectv1.ListProjectsResponse, error) {
			return nil, wantErr
		},
		listSourceProjectsFunc: func(ctx context.Context, in *projectv1.ListSourceProjectsRequest) (*projectv1.ListSourceProjectsResponse, error) {
			return nil, wantErr
		},
		linkSourceProjectFunc: func(ctx context.Context, in *projectv1.LinkSourceProjectRequest) (*projectv1.LinkSourceProjectResponse, error) {
			return nil, wantErr
		},
		unlinkSourceProjectFunc: func(ctx context.Context, in *projectv1.UnlinkSourceProjectRequest) (*projectv1.UnlinkSourceProjectResponse, error) {
			return nil, wantErr
		},
		getSharedProjectDataFunc: func(ctx context.Context, in *projectv1.GetSharedProjectDataRequest) (*projectv1.GetSharedProjectDataResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerOrcaProjectSharingChannels(r, fake)

	linkArgs := argsJSON(t, map[string]any{"orcaProjectId": "c1", "projectId": "s1"})
	for _, channel := range []string{
		"orcaProjects.list",
		"orcaProjects.linkSourceProject",
		"orcaProjects.unlinkSourceProject",
		"orcaProjects.getProjectData",
	} {
		var args []json.RawMessage
		if channel != "orcaProjects.list" {
			args = linkArgs
		}
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "u1"}, channel, args)
		if !errors.Is(err, wantErr) {
			t.Errorf("%s: expected wantErr to propagate, got %v", channel, err)
		}
	}
}
