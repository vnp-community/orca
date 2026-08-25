package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeProjectServiceClient implements projectv1.ProjectServiceClient entirely
// in memory, capturing the last request of each kind it saw so tests can
// assert what actually crossed the REST->gRPC boundary — same shape as
// fakeAnnotationServiceClient (annotation_routes_test.go).
type fakeProjectServiceClient struct {
	lastCreateProjectReq *projectv1.CreateProjectRequest
	createProjectResp    *projectv1.CreateProjectResponse
	createProjectErr     error

	lastGetProjectReq *projectv1.GetProjectRequest
	getProjectResp    *projectv1.GetProjectResponse
	getProjectErr     error

	lastListProjectsReq *projectv1.ListProjectsRequest
	listProjectsResp    *projectv1.ListProjectsResponse
	listProjectsErr     error

	lastAddMemberReq *projectv1.AddMemberRequest
	addMemberResp    *projectv1.AddMemberResponse
	addMemberErr     error

	lastRebindDevServerReq *projectv1.RebindDevServerRequest
	rebindDevServerResp    *projectv1.RebindDevServerResponse
	rebindDevServerErr     error

	lastUpdateProjectReq *projectv1.UpdateProjectRequest
	updateProjectResp    *projectv1.UpdateProjectResponse
	updateProjectErr     error

	lastDeleteProjectReq *projectv1.DeleteProjectRequest
	deleteProjectResp    *projectv1.DeleteProjectResponse
	deleteProjectErr     error

	lastAddRepoReq *projectv1.AddRepoRequest
	addRepoResp    *projectv1.AddRepoResponse
	addRepoErr     error

	lastUpdateRepoReq *projectv1.UpdateRepoRequest
	updateRepoResp    *projectv1.UpdateRepoResponse
	updateRepoErr     error

	lastListReposReq *projectv1.ListReposRequest
	listReposResp    *projectv1.ListReposResponse
	listReposErr     error

	lastReorderReposReq *projectv1.ReorderReposRequest
	reorderReposResp    *projectv1.ReorderReposResponse
	reorderReposErr     error

	lastRemoveRepoReq *projectv1.RemoveRepoRequest
	removeRepoResp    *projectv1.RemoveRepoResponse
	removeRepoErr     error

	lastRecordWorktreeCreatedReq *projectv1.RecordWorktreeCreatedRequest
	recordWorktreeCreatedResp    *projectv1.RecordWorktreeCreatedResponse
	recordWorktreeCreatedErr     error

	lastRecordWorktreeRemovedReq *projectv1.RecordWorktreeRemovedRequest
	recordWorktreeRemovedResp    *projectv1.RecordWorktreeRemovedResponse
	recordWorktreeRemovedErr     error

	lastListWorktreesReq *projectv1.ListWorktreesRequest
	listWorktreesResp    *projectv1.ListWorktreesResponse
	listWorktreesErr     error

	lastSetWorktreeActivationReq *projectv1.SetWorktreeActivationRequest
	setWorktreeActivationResp    *projectv1.SetWorktreeActivationResponse
	setWorktreeActivationErr     error

	lastRenameWorktreeReq *projectv1.RenameWorktreeRequest
	renameWorktreeResp    *projectv1.RenameWorktreeResponse
	renameWorktreeErr     error

	lastCreateProjectGroupReq *projectv1.CreateProjectGroupRequest
	createProjectGroupResp    *projectv1.CreateProjectGroupResponse
	createProjectGroupErr     error

	lastUpdateProjectGroupReq *projectv1.UpdateProjectGroupRequest
	updateProjectGroupResp    *projectv1.UpdateProjectGroupResponse
	updateProjectGroupErr     error

	lastDeleteProjectGroupReq *projectv1.DeleteProjectGroupRequest
	deleteProjectGroupResp    *projectv1.DeleteProjectGroupResponse
	deleteProjectGroupErr     error

	lastListProjectGroupsReq *projectv1.ListProjectGroupsRequest
	listProjectGroupsResp    *projectv1.ListProjectGroupsResponse
	listProjectGroupsErr     error
}

func (f *fakeProjectServiceClient) CreateProject(_ context.Context, in *projectv1.CreateProjectRequest, _ ...grpc.CallOption) (*projectv1.CreateProjectResponse, error) {
	f.lastCreateProjectReq = in
	if f.createProjectErr != nil {
		return nil, f.createProjectErr
	}
	return f.createProjectResp, nil
}

func (f *fakeProjectServiceClient) GetProject(_ context.Context, in *projectv1.GetProjectRequest, _ ...grpc.CallOption) (*projectv1.GetProjectResponse, error) {
	f.lastGetProjectReq = in
	if f.getProjectErr != nil {
		return nil, f.getProjectErr
	}
	return f.getProjectResp, nil
}

func (f *fakeProjectServiceClient) ListProjects(_ context.Context, in *projectv1.ListProjectsRequest, _ ...grpc.CallOption) (*projectv1.ListProjectsResponse, error) {
	f.lastListProjectsReq = in
	if f.listProjectsErr != nil {
		return nil, f.listProjectsErr
	}
	return f.listProjectsResp, nil
}

func (f *fakeProjectServiceClient) AddMember(_ context.Context, in *projectv1.AddMemberRequest, _ ...grpc.CallOption) (*projectv1.AddMemberResponse, error) {
	f.lastAddMemberReq = in
	if f.addMemberErr != nil {
		return nil, f.addMemberErr
	}
	return f.addMemberResp, nil
}

func (f *fakeProjectServiceClient) RebindDevServer(_ context.Context, in *projectv1.RebindDevServerRequest, _ ...grpc.CallOption) (*projectv1.RebindDevServerResponse, error) {
	f.lastRebindDevServerReq = in
	if f.rebindDevServerErr != nil {
		return nil, f.rebindDevServerErr
	}
	return f.rebindDevServerResp, nil
}

func (f *fakeProjectServiceClient) UpdateProject(_ context.Context, in *projectv1.UpdateProjectRequest, _ ...grpc.CallOption) (*projectv1.UpdateProjectResponse, error) {
	f.lastUpdateProjectReq = in
	if f.updateProjectErr != nil {
		return nil, f.updateProjectErr
	}
	return f.updateProjectResp, nil
}

func (f *fakeProjectServiceClient) DeleteProject(_ context.Context, in *projectv1.DeleteProjectRequest, _ ...grpc.CallOption) (*projectv1.DeleteProjectResponse, error) {
	f.lastDeleteProjectReq = in
	if f.deleteProjectErr != nil {
		return nil, f.deleteProjectErr
	}
	return f.deleteProjectResp, nil
}

func (f *fakeProjectServiceClient) AddRepo(_ context.Context, in *projectv1.AddRepoRequest, _ ...grpc.CallOption) (*projectv1.AddRepoResponse, error) {
	f.lastAddRepoReq = in
	if f.addRepoErr != nil {
		return nil, f.addRepoErr
	}
	return f.addRepoResp, nil
}

func (f *fakeProjectServiceClient) ListRepos(_ context.Context, in *projectv1.ListReposRequest, _ ...grpc.CallOption) (*projectv1.ListReposResponse, error) {
	f.lastListReposReq = in
	if f.listReposErr != nil {
		return nil, f.listReposErr
	}
	return f.listReposResp, nil
}

func (f *fakeProjectServiceClient) ReorderRepos(_ context.Context, in *projectv1.ReorderReposRequest, _ ...grpc.CallOption) (*projectv1.ReorderReposResponse, error) {
	f.lastReorderReposReq = in
	if f.reorderReposErr != nil {
		return nil, f.reorderReposErr
	}
	return f.reorderReposResp, nil
}

func (f *fakeProjectServiceClient) RemoveRepo(_ context.Context, in *projectv1.RemoveRepoRequest, _ ...grpc.CallOption) (*projectv1.RemoveRepoResponse, error) {
	f.lastRemoveRepoReq = in
	if f.removeRepoErr != nil {
		return nil, f.removeRepoErr
	}
	return f.removeRepoResp, nil
}

func (f *fakeProjectServiceClient) UpdateRepo(_ context.Context, in *projectv1.UpdateRepoRequest, _ ...grpc.CallOption) (*projectv1.UpdateRepoResponse, error) {
	f.lastUpdateRepoReq = in
	if f.updateRepoErr != nil {
		return nil, f.updateRepoErr
	}
	return f.updateRepoResp, nil
}

func (f *fakeProjectServiceClient) RecordWorktreeCreated(_ context.Context, in *projectv1.RecordWorktreeCreatedRequest, _ ...grpc.CallOption) (*projectv1.RecordWorktreeCreatedResponse, error) {
	f.lastRecordWorktreeCreatedReq = in
	if f.recordWorktreeCreatedErr != nil {
		return nil, f.recordWorktreeCreatedErr
	}
	return f.recordWorktreeCreatedResp, nil
}

func (f *fakeProjectServiceClient) RecordWorktreeRemoved(_ context.Context, in *projectv1.RecordWorktreeRemovedRequest, _ ...grpc.CallOption) (*projectv1.RecordWorktreeRemovedResponse, error) {
	f.lastRecordWorktreeRemovedReq = in
	if f.recordWorktreeRemovedErr != nil {
		return nil, f.recordWorktreeRemovedErr
	}
	return f.recordWorktreeRemovedResp, nil
}

func (f *fakeProjectServiceClient) ListWorktrees(_ context.Context, in *projectv1.ListWorktreesRequest, _ ...grpc.CallOption) (*projectv1.ListWorktreesResponse, error) {
	f.lastListWorktreesReq = in
	if f.listWorktreesErr != nil {
		return nil, f.listWorktreesErr
	}
	return f.listWorktreesResp, nil
}

func (f *fakeProjectServiceClient) SetWorktreeActivation(_ context.Context, in *projectv1.SetWorktreeActivationRequest, _ ...grpc.CallOption) (*projectv1.SetWorktreeActivationResponse, error) {
	f.lastSetWorktreeActivationReq = in
	if f.setWorktreeActivationErr != nil {
		return nil, f.setWorktreeActivationErr
	}
	return f.setWorktreeActivationResp, nil
}

func (f *fakeProjectServiceClient) RenameWorktree(_ context.Context, in *projectv1.RenameWorktreeRequest, _ ...grpc.CallOption) (*projectv1.RenameWorktreeResponse, error) {
	f.lastRenameWorktreeReq = in
	if f.renameWorktreeErr != nil {
		return nil, f.renameWorktreeErr
	}
	return f.renameWorktreeResp, nil
}

func (f *fakeProjectServiceClient) CreateProjectGroup(_ context.Context, in *projectv1.CreateProjectGroupRequest, _ ...grpc.CallOption) (*projectv1.CreateProjectGroupResponse, error) {
	f.lastCreateProjectGroupReq = in
	if f.createProjectGroupErr != nil {
		return nil, f.createProjectGroupErr
	}
	return f.createProjectGroupResp, nil
}

func (f *fakeProjectServiceClient) UpdateProjectGroup(_ context.Context, in *projectv1.UpdateProjectGroupRequest, _ ...grpc.CallOption) (*projectv1.UpdateProjectGroupResponse, error) {
	f.lastUpdateProjectGroupReq = in
	if f.updateProjectGroupErr != nil {
		return nil, f.updateProjectGroupErr
	}
	return f.updateProjectGroupResp, nil
}

func (f *fakeProjectServiceClient) DeleteProjectGroup(_ context.Context, in *projectv1.DeleteProjectGroupRequest, _ ...grpc.CallOption) (*projectv1.DeleteProjectGroupResponse, error) {
	f.lastDeleteProjectGroupReq = in
	if f.deleteProjectGroupErr != nil {
		return nil, f.deleteProjectGroupErr
	}
	return f.deleteProjectGroupResp, nil
}

func (f *fakeProjectServiceClient) ListProjectGroups(_ context.Context, in *projectv1.ListProjectGroupsRequest, _ ...grpc.CallOption) (*projectv1.ListProjectGroupsResponse, error) {
	f.lastListProjectGroupsReq = in
	if f.listProjectGroupsErr != nil {
		return nil, f.listProjectGroupsErr
	}
	return f.listProjectGroupsResp, nil
}

var _ projectv1.ProjectServiceClient = (*fakeProjectServiceClient)(nil)

// projectTestRouter mounts mountProjectRoutes standalone (router.go isn't
// touched by this package's tests, per task instructions).
func projectTestRouter(client projectv1.ProjectServiceClient) chi.Router {
	r := chi.NewRouter()
	mountProjectRoutes(r, client)
	return r
}

// withTestIdentity is shared across this package's route tests —
// git_routes_test.go defines it.

func TestHandleCreateProject_SuccessRoundTrip(t *testing.T) {
	fake := &fakeProjectServiceClient{
		createProjectResp: &projectv1.CreateProjectResponse{
			Project: &projectv1.Project{
				Id:       "proj-1",
				TenantId: "tenant-1",
				Name:     "orca",
			},
		},
	}
	router := projectTestRouter(fake)

	body := `{"name":"orca","description":"the app","default_branch":"main","visibility":"team"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", bytes.NewBufferString(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if fake.lastCreateProjectReq == nil {
		t.Fatal("expected CreateProject to be called")
	}
	if fake.lastCreateProjectReq.GetName() != "orca" || fake.lastCreateProjectReq.GetDefaultBranch() != "main" || fake.lastCreateProjectReq.GetVisibility() != "team" {
		t.Fatalf("CreateProjectRequest = %+v, unexpected", fake.lastCreateProjectReq)
	}

	var got projectv1.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshaling response: %v; body=%s", err, rec.Body.String())
	}
	if got.Id != "proj-1" {
		t.Fatalf("Id = %q, want %q", got.Id, "proj-1")
	}
}

// TestHandleCreateProject_IdentityComesFromContextNotBody proves the
// gateway never trusts a caller-supplied tenant_id: the request body here
// carries no tenant_id field at all, so the outbound gRPC request's
// TenantId must come solely from the resolved Identity (via
// identityFromContext + gatewaygrpc.AttachIdentity), not the JSON body.
func TestHandleCreateProject_IdentityComesFromContextNotBody(t *testing.T) {
	fake := &fakeProjectServiceClient{
		createProjectResp: &projectv1.CreateProjectResponse{
			Project: &projectv1.Project{Id: "proj-1", TenantId: "tenant-from-identity"},
		},
	}
	router := projectTestRouter(fake)

	body := `{"name":"orca"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", bytes.NewBufferString(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-from-identity", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if fake.lastCreateProjectReq == nil {
		t.Fatal("expected CreateProject to be called")
	}
	if fake.lastCreateProjectReq.GetTenantId() != "tenant-from-identity" {
		t.Fatalf("TenantId = %q, want %q (from identity, not body)", fake.lastCreateProjectReq.GetTenantId(), "tenant-from-identity")
	}
}

// TestHandleGetProject_NotFoundMapsTo404 covers the gRPC-error->HTTP-status
// mapping (writeGRPCError / grpcCodeToHTTPStatus, usage_routes.go) for this
// service's routes.
func TestHandleGetProject_NotFoundMapsTo404(t *testing.T) {
	fake := &fakeProjectServiceClient{
		getProjectErr: status.Error(codes.NotFound, "project not found"),
	}
	router := projectTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/projects/missing", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	var errBody errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("unmarshaling error body: %v; body=%s", err, rec.Body.String())
	}
	if errBody.Error.Code != codes.NotFound.String() {
		t.Fatalf("error.code = %q, want %q", errBody.Error.Code, codes.NotFound.String())
	}
	if fake.lastGetProjectReq == nil || fake.lastGetProjectReq.GetId() != "missing" {
		t.Fatalf("lastGetProjectReq = %+v, want Id \"missing\"", fake.lastGetProjectReq)
	}
}

func TestHandleListProjectGroups_Success(t *testing.T) {
	fake := &fakeProjectServiceClient{
		listProjectGroupsResp: &projectv1.ListProjectGroupsResponse{
			Groups: []*projectv1.ProjectGroup{
				{Id: "grp-1", TenantId: "tenant-1", Name: "backend"},
			},
		},
	}
	router := projectTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/project-groups", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.lastListProjectGroupsReq == nil {
		t.Fatal("expected ListProjectGroups to be called")
	}
}
