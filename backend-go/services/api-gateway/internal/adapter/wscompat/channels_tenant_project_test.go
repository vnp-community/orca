package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// fakeTenantServiceClient2 is a minimal test double for
// tenantv1.TenantServiceClient — embeds the nil interface, overrides only
// the methods this file's channel handlers call, same pattern as
// fakeInfraFleetClient (channels_test.go).
type fakeTenantServiceClient2 struct {
	tenantv1.TenantServiceClient

	getResolvedProfileFunc func(ctx context.Context, in *tenantv1.GetResolvedProfileRequest) (*tenantv1.GetResolvedProfileResponse, error)
	getUserProfileFunc     func(ctx context.Context, in *tenantv1.GetUserProfileRequest) (*tenantv1.GetUserProfileResponse, error)
	listDepartmentsFunc    func(ctx context.Context, in *tenantv1.ListDepartmentsRequest) (*tenantv1.ListDepartmentsResponse, error)
	updateCompanyFunc      func(ctx context.Context, in *tenantv1.UpdateCompanyRequest) (*tenantv1.UpdateCompanyResponse, error)
	updateDepartmentFunc   func(ctx context.Context, in *tenantv1.UpdateDepartmentRequest) (*tenantv1.UpdateDepartmentResponse, error)
	updateUserProfileFunc  func(ctx context.Context, in *tenantv1.UpdateUserProfileRequest) (*tenantv1.UpdateUserProfileResponse, error)
}

func (f *fakeTenantServiceClient2) GetResolvedProfile(ctx context.Context, in *tenantv1.GetResolvedProfileRequest, _ ...grpc.CallOption) (*tenantv1.GetResolvedProfileResponse, error) {
	return f.getResolvedProfileFunc(ctx, in)
}
func (f *fakeTenantServiceClient2) GetUserProfile(ctx context.Context, in *tenantv1.GetUserProfileRequest, _ ...grpc.CallOption) (*tenantv1.GetUserProfileResponse, error) {
	return f.getUserProfileFunc(ctx, in)
}
func (f *fakeTenantServiceClient2) ListDepartments(ctx context.Context, in *tenantv1.ListDepartmentsRequest, _ ...grpc.CallOption) (*tenantv1.ListDepartmentsResponse, error) {
	return f.listDepartmentsFunc(ctx, in)
}
func (f *fakeTenantServiceClient2) UpdateCompany(ctx context.Context, in *tenantv1.UpdateCompanyRequest, _ ...grpc.CallOption) (*tenantv1.UpdateCompanyResponse, error) {
	return f.updateCompanyFunc(ctx, in)
}
func (f *fakeTenantServiceClient2) UpdateDepartment(ctx context.Context, in *tenantv1.UpdateDepartmentRequest, _ ...grpc.CallOption) (*tenantv1.UpdateDepartmentResponse, error) {
	return f.updateDepartmentFunc(ctx, in)
}
func (f *fakeTenantServiceClient2) UpdateUserProfile(ctx context.Context, in *tenantv1.UpdateUserProfileRequest, _ ...grpc.CallOption) (*tenantv1.UpdateUserProfileResponse, error) {
	return f.updateUserProfileFunc(ctx, in)
}

// fakeProjectServiceClient2 is a minimal test double for
// projectv1.ProjectServiceClient scoped to this file's tests — named with a
// "2" suffix to avoid colliding with httpgateway package's own
// fakeProjectServiceClient (different package, but keeps grep unambiguous
// within this package if another wscompat test file adds one later).
type fakeProjectServiceClient2 struct {
	projectv1.ProjectServiceClient

	createProjectFunc       func(ctx context.Context, in *projectv1.CreateProjectRequest) (*projectv1.CreateProjectResponse, error)
	getProjectFunc          func(ctx context.Context, in *projectv1.GetProjectRequest) (*projectv1.GetProjectResponse, error)
	listProjectsFunc        func(ctx context.Context, in *projectv1.ListProjectsRequest) (*projectv1.ListProjectsResponse, error)
	updateProjectFunc       func(ctx context.Context, in *projectv1.UpdateProjectRequest) (*projectv1.UpdateProjectResponse, error)
	listMembersFunc         func(ctx context.Context, in *projectv1.ListMembersRequest) (*projectv1.ListMembersResponse, error)
	removeMemberFunc        func(ctx context.Context, in *projectv1.RemoveMemberRequest) (*projectv1.RemoveMemberResponse, error)
	updateMemberRoleFunc    func(ctx context.Context, in *projectv1.UpdateMemberRoleRequest) (*projectv1.UpdateMemberRoleResponse, error)
	createProjectGroupFunc  func(ctx context.Context, in *projectv1.CreateProjectGroupRequest) (*projectv1.CreateProjectGroupResponse, error)
	updateProjectGroupFunc  func(ctx context.Context, in *projectv1.UpdateProjectGroupRequest) (*projectv1.UpdateProjectGroupResponse, error)
	deleteProjectGroupFunc  func(ctx context.Context, in *projectv1.DeleteProjectGroupRequest) (*projectv1.DeleteProjectGroupResponse, error)
	listProjectGroupsFunc   func(ctx context.Context, in *projectv1.ListProjectGroupsRequest) (*projectv1.ListProjectGroupsResponse, error)
	moveProjectFunc         func(ctx context.Context, in *projectv1.MoveProjectRequest) (*projectv1.MoveProjectResponse, error)
	scanNestedFunc          func(ctx context.Context, in *projectv1.ScanNestedRequest) (*projectv1.ScanNestedResponse, error)
	importNestedFunc        func(ctx context.Context, in *projectv1.ImportNestedRequest) (*projectv1.ImportNestedResponse, error)
	createHostSetupFunc     func(ctx context.Context, in *projectv1.CreateHostSetupRequest) (*projectv1.CreateHostSetupResponse, error)
	listHostSetupsFunc      func(ctx context.Context, in *projectv1.ListHostSetupsRequest) (*projectv1.ListHostSetupsResponse, error)
	updateHostSetupFunc     func(ctx context.Context, in *projectv1.UpdateHostSetupRequest) (*projectv1.UpdateHostSetupResponse, error)
	deleteHostSetupFunc     func(ctx context.Context, in *projectv1.DeleteHostSetupRequest) (*projectv1.DeleteHostSetupResponse, error)
	setupExistingFolderFunc func(ctx context.Context, in *projectv1.SetupExistingFolderRequest) (*projectv1.SetupExistingFolderResponse, error)
}

func (f *fakeProjectServiceClient2) CreateProject(ctx context.Context, in *projectv1.CreateProjectRequest, _ ...grpc.CallOption) (*projectv1.CreateProjectResponse, error) {
	return f.createProjectFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) GetProject(ctx context.Context, in *projectv1.GetProjectRequest, _ ...grpc.CallOption) (*projectv1.GetProjectResponse, error) {
	return f.getProjectFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) ListProjects(ctx context.Context, in *projectv1.ListProjectsRequest, _ ...grpc.CallOption) (*projectv1.ListProjectsResponse, error) {
	return f.listProjectsFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) UpdateProject(ctx context.Context, in *projectv1.UpdateProjectRequest, _ ...grpc.CallOption) (*projectv1.UpdateProjectResponse, error) {
	return f.updateProjectFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) ListMembers(ctx context.Context, in *projectv1.ListMembersRequest, _ ...grpc.CallOption) (*projectv1.ListMembersResponse, error) {
	return f.listMembersFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) RemoveMember(ctx context.Context, in *projectv1.RemoveMemberRequest, _ ...grpc.CallOption) (*projectv1.RemoveMemberResponse, error) {
	return f.removeMemberFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) UpdateMemberRole(ctx context.Context, in *projectv1.UpdateMemberRoleRequest, _ ...grpc.CallOption) (*projectv1.UpdateMemberRoleResponse, error) {
	return f.updateMemberRoleFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) CreateProjectGroup(ctx context.Context, in *projectv1.CreateProjectGroupRequest, _ ...grpc.CallOption) (*projectv1.CreateProjectGroupResponse, error) {
	return f.createProjectGroupFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) UpdateProjectGroup(ctx context.Context, in *projectv1.UpdateProjectGroupRequest, _ ...grpc.CallOption) (*projectv1.UpdateProjectGroupResponse, error) {
	return f.updateProjectGroupFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) DeleteProjectGroup(ctx context.Context, in *projectv1.DeleteProjectGroupRequest, _ ...grpc.CallOption) (*projectv1.DeleteProjectGroupResponse, error) {
	return f.deleteProjectGroupFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) ListProjectGroups(ctx context.Context, in *projectv1.ListProjectGroupsRequest, _ ...grpc.CallOption) (*projectv1.ListProjectGroupsResponse, error) {
	return f.listProjectGroupsFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) MoveProject(ctx context.Context, in *projectv1.MoveProjectRequest, _ ...grpc.CallOption) (*projectv1.MoveProjectResponse, error) {
	return f.moveProjectFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) ScanNested(ctx context.Context, in *projectv1.ScanNestedRequest, _ ...grpc.CallOption) (*projectv1.ScanNestedResponse, error) {
	return f.scanNestedFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) ImportNested(ctx context.Context, in *projectv1.ImportNestedRequest, _ ...grpc.CallOption) (*projectv1.ImportNestedResponse, error) {
	return f.importNestedFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) CreateHostSetup(ctx context.Context, in *projectv1.CreateHostSetupRequest, _ ...grpc.CallOption) (*projectv1.CreateHostSetupResponse, error) {
	return f.createHostSetupFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) ListHostSetups(ctx context.Context, in *projectv1.ListHostSetupsRequest, _ ...grpc.CallOption) (*projectv1.ListHostSetupsResponse, error) {
	return f.listHostSetupsFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) UpdateHostSetup(ctx context.Context, in *projectv1.UpdateHostSetupRequest, _ ...grpc.CallOption) (*projectv1.UpdateHostSetupResponse, error) {
	return f.updateHostSetupFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) DeleteHostSetup(ctx context.Context, in *projectv1.DeleteHostSetupRequest, _ ...grpc.CallOption) (*projectv1.DeleteHostSetupResponse, error) {
	return f.deleteHostSetupFunc(ctx, in)
}
func (f *fakeProjectServiceClient2) SetupExistingFolder(ctx context.Context, in *projectv1.SetupExistingFolderRequest, _ ...grpc.CallOption) (*projectv1.SetupExistingFolderResponse, error) {
	return f.setupExistingFolderFunc(ctx, in)
}

// ── profile.* ──────────────────────────────────────────────────────────

func TestProfileGetResolvedChannel_Success(t *testing.T) {
	var gotCtx context.Context
	fake := &fakeTenantServiceClient2{
		getResolvedProfileFunc: func(ctx context.Context, in *tenantv1.GetResolvedProfileRequest) (*tenantv1.GetResolvedProfileResponse, error) {
			gotCtx = ctx
			if in.GetUserId() != "user-1" {
				t.Errorf("want userId user-1, got %q", in.GetUserId())
			}
			return &tenantv1.GetResolvedProfileResponse{ResolvedSettingsJson: `{"theme":"dark"}`}, nil
		},
	}
	r := NewRegistry()
	registerProfileChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "profile.getResolved", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*tenantv1.GetResolvedProfileResponse)
	if !ok || resp.GetResolvedSettingsJson() != `{"theme":"dark"}` {
		t.Fatalf("unexpected result: %+v", result)
	}
	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "tenant-1" || user != "user-1" {
		t.Errorf("AttachIdentity not applied: tenant=%q user=%q", tenant, user)
	}
}

func TestProfileGetUserProfileChannel_Success(t *testing.T) {
	fake := &fakeTenantServiceClient2{
		getUserProfileFunc: func(ctx context.Context, in *tenantv1.GetUserProfileRequest) (*tenantv1.GetUserProfileResponse, error) {
			if in.GetUserId() != "user-2" {
				t.Errorf("want userId user-2, got %q", in.GetUserId())
			}
			return &tenantv1.GetUserProfileResponse{Profile: &tenantv1.UserProfile{UserId: "user-2", CompanyId: "co-1"}}, nil
		},
	}
	r := NewRegistry()
	registerProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{"userId": "user-2"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-2"}, "profile.getUserProfile", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	profile, ok := result.(*tenantv1.UserProfile)
	if !ok || profile.GetUserId() != "user-2" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProfileListDeptsChannel_Success(t *testing.T) {
	fake := &fakeTenantServiceClient2{
		listDepartmentsFunc: func(ctx context.Context, in *tenantv1.ListDepartmentsRequest) (*tenantv1.ListDepartmentsResponse, error) {
			if in.GetCompanyId() != "co-1" {
				t.Errorf("want companyId co-1, got %q", in.GetCompanyId())
			}
			return &tenantv1.ListDepartmentsResponse{Departments: []*tenantv1.Department{{Id: "d1"}, {Id: "d2"}}}, nil
		},
	}
	r := NewRegistry()
	registerProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{"companyId": "co-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "profile.listDepts", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	depts, ok := result.([]*tenantv1.Department)
	if !ok || len(depts) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProfileUpdateCompanyChannel_Success(t *testing.T) {
	var gotReq *tenantv1.UpdateCompanyRequest
	fake := &fakeTenantServiceClient2{
		updateCompanyFunc: func(ctx context.Context, in *tenantv1.UpdateCompanyRequest) (*tenantv1.UpdateCompanyResponse, error) {
			gotReq = in
			return &tenantv1.UpdateCompanyResponse{Company: &tenantv1.Company{Id: in.GetId(), Name: in.GetName()}}, nil
		},
	}
	r := NewRegistry()
	registerProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "co-1", "name": "Acme", "settingsJson": `{"x":1}`})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "profile.updateCompany", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetId() != "co-1" || gotReq.GetName() != "Acme" || gotReq.GetSettingsJson() != `{"x":1}` {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	company, ok := result.(*tenantv1.Company)
	if !ok || company.GetName() != "Acme" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProfileUpdateDeptChannel_Success(t *testing.T) {
	var gotReq *tenantv1.UpdateDepartmentRequest
	fake := &fakeTenantServiceClient2{
		updateDepartmentFunc: func(ctx context.Context, in *tenantv1.UpdateDepartmentRequest) (*tenantv1.UpdateDepartmentResponse, error) {
			gotReq = in
			return &tenantv1.UpdateDepartmentResponse{Department: &tenantv1.Department{Id: in.GetId(), Name: in.GetName()}}, nil
		},
	}
	r := NewRegistry()
	registerProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "d1", "name": "Eng"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "profile.updateDept", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetId() != "d1" || gotReq.GetName() != "Eng" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	dept, ok := result.(*tenantv1.Department)
	if !ok || dept.GetName() != "Eng" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProfileUpdateUserChannel_Success(t *testing.T) {
	var gotReq *tenantv1.UpdateUserProfileRequest
	fake := &fakeTenantServiceClient2{
		updateUserProfileFunc: func(ctx context.Context, in *tenantv1.UpdateUserProfileRequest) (*tenantv1.UpdateUserProfileResponse, error) {
			gotReq = in
			return &tenantv1.UpdateUserProfileResponse{Profile: &tenantv1.UserProfile{UserId: in.GetUserId()}}, nil
		},
	}
	r := NewRegistry()
	registerProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"userId": "user-3", "departmentId": "", "clearDepartment": true, "settingsJson": `{"a":true}`,
	})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "profile.updateUser", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotReq.GetClearDepartment() {
		t.Errorf("want clearDepartment=true to map through, got false")
	}
	if gotReq.GetSettingsJson() != `{"a":true}` {
		t.Errorf("want settingsJson to map through, got %q", gotReq.GetSettingsJson())
	}
}

func TestProfileChannels_PropagateErrors(t *testing.T) {
	wantErr := errors.New("tenant-service unavailable")
	fake := &fakeTenantServiceClient2{
		getResolvedProfileFunc: func(ctx context.Context, in *tenantv1.GetResolvedProfileRequest) (*tenantv1.GetResolvedProfileResponse, error) {
			return nil, wantErr
		},
		getUserProfileFunc: func(ctx context.Context, in *tenantv1.GetUserProfileRequest) (*tenantv1.GetUserProfileResponse, error) {
			return nil, wantErr
		},
		listDepartmentsFunc: func(ctx context.Context, in *tenantv1.ListDepartmentsRequest) (*tenantv1.ListDepartmentsResponse, error) {
			return nil, wantErr
		},
		updateCompanyFunc: func(ctx context.Context, in *tenantv1.UpdateCompanyRequest) (*tenantv1.UpdateCompanyResponse, error) {
			return nil, wantErr
		},
		updateDepartmentFunc: func(ctx context.Context, in *tenantv1.UpdateDepartmentRequest) (*tenantv1.UpdateDepartmentResponse, error) {
			return nil, wantErr
		},
		updateUserProfileFunc: func(ctx context.Context, in *tenantv1.UpdateUserProfileRequest) (*tenantv1.UpdateUserProfileResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerProfileChannels(r, fake)

	cases := []struct {
		channel string
		args    []byte
	}{
		{"profile.getResolved", nil},
		{"profile.getUserProfile", []byte(`{"userId":"u"}`)},
		{"profile.listDepts", []byte(`{"companyId":"c"}`)},
		{"profile.updateCompany", []byte(`{"id":"c"}`)},
		{"profile.updateDept", []byte(`{"id":"d"}`)},
		{"profile.updateUser", []byte(`{"userId":"u"}`)},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			var argsList []json.RawMessage
			if tc.args != nil {
				argsList = []json.RawMessage{tc.args}
			}
			_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, tc.channel, argsList)
			if !errors.Is(err, wantErr) {
				t.Fatalf("channel %s: want error %v, got %v", tc.channel, wantErr, err)
			}
		})
	}
}

func TestProfileChannels_AttachIdentity(t *testing.T) {
	var gotCtx context.Context
	fake := &fakeTenantServiceClient2{
		getResolvedProfileFunc: func(ctx context.Context, in *tenantv1.GetResolvedProfileRequest) (*tenantv1.GetResolvedProfileResponse, error) {
			gotCtx = ctx
			return &tenantv1.GetResolvedProfileResponse{}, nil
		},
	}
	r := NewRegistry()
	registerProfileChannels(r, fake)

	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-9", UserID: "user-9"}, "profile.getResolved", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "tenant-9" || user != "user-9" {
		t.Errorf("AttachIdentity not applied: tenant=%q user=%q", tenant, user)
	}
}

// ── project.* ──────────────────────────────────────────────────────────

func TestProjectCreateChannel_Success(t *testing.T) {
	var gotReq *projectv1.CreateProjectRequest
	fake := &fakeProjectServiceClient2{
		createProjectFunc: func(ctx context.Context, in *projectv1.CreateProjectRequest) (*projectv1.CreateProjectResponse, error) {
			gotReq = in
			return &projectv1.CreateProjectResponse{Project: &projectv1.Project{Id: "p1", TenantId: in.GetTenantId()}}, nil
		},
	}
	r := NewRegistry()
	registerProjectChannels(r, fake)

	args := argsJSON(t, map[string]any{"name": "My Project"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "project.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTenantId() != "tenant-1" {
		t.Errorf("want TenantId set from Identity, got %q", gotReq.GetTenantId())
	}
	proj, ok := result.(*projectv1.Project)
	if !ok || proj.GetId() != "p1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectGetChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		getProjectFunc: func(ctx context.Context, in *projectv1.GetProjectRequest) (*projectv1.GetProjectResponse, error) {
			if in.GetId() != "p1" {
				t.Errorf("want id p1, got %q", in.GetId())
			}
			return &projectv1.GetProjectResponse{Project: &projectv1.Project{Id: "p1"}}, nil
		},
	}
	r := NewRegistry()
	registerProjectChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "p1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "project.get", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj, ok := result.(*projectv1.Project); !ok || proj.GetId() != "p1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectListChannel_Success(t *testing.T) {
	var gotReq *projectv1.ListProjectsRequest
	fake := &fakeProjectServiceClient2{
		listProjectsFunc: func(ctx context.Context, in *projectv1.ListProjectsRequest) (*projectv1.ListProjectsResponse, error) {
			gotReq = in
			return &projectv1.ListProjectsResponse{Projects: []*projectv1.Project{{Id: "p1"}}}, nil
		},
	}
	r := NewRegistry()
	registerProjectChannels(r, fake)

	// Missing args (empty slice) must not panic — defaults applied.
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "project.list", []json.RawMessage{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTenantId() != "tenant-1" {
		t.Errorf("want TenantId set from Identity, got %q", gotReq.GetTenantId())
	}
	if projs, ok := result.([]*projectv1.Project); !ok || len(projs) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectUpdateChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		updateProjectFunc: func(ctx context.Context, in *projectv1.UpdateProjectRequest) (*projectv1.UpdateProjectResponse, error) {
			if in.GetProjectId() != "p1" {
				t.Errorf("want ProjectId p1, got %q", in.GetProjectId())
			}
			return &projectv1.UpdateProjectResponse{Project: &projectv1.Project{Id: "p1", Name: in.GetName()}}, nil
		},
	}
	r := NewRegistry()
	registerProjectChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "p1", "name": "Renamed"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "project.update", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj, ok := result.(*projectv1.Project); !ok || proj.GetName() != "Renamed" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectGetMembersChannel_Success(t *testing.T) {
	var called bool
	fake := &fakeProjectServiceClient2{
		listMembersFunc: func(ctx context.Context, in *projectv1.ListMembersRequest) (*projectv1.ListMembersResponse, error) {
			called = true
			if in.GetProjectId() != "p1" {
				t.Errorf("want ProjectId p1, got %q", in.GetProjectId())
			}
			return &projectv1.ListMembersResponse{Members: []*projectv1.Member{{UserId: "u1"}}}, nil
		},
	}
	r := NewRegistry()
	registerProjectChannels(r, fake)

	args := argsJSON(t, map[string]any{"projectId": "p1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "project.getMembers", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("project.getMembers did not map onto client.ListMembers")
	}
	if members, ok := result.([]*projectv1.Member); !ok || len(members) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectRemoveMemberChannel_Success(t *testing.T) {
	var gotReq *projectv1.RemoveMemberRequest
	fake := &fakeProjectServiceClient2{
		removeMemberFunc: func(ctx context.Context, in *projectv1.RemoveMemberRequest) (*projectv1.RemoveMemberResponse, error) {
			gotReq = in
			return &projectv1.RemoveMemberResponse{}, nil
		},
	}
	r := NewRegistry()
	registerProjectChannels(r, fake)

	args := argsJSON(t, map[string]any{"projectId": "p1", "userId": "u1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "project.removeMember", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetProjectId() != "p1" || gotReq.GetUserId() != "u1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	ok, isMap := result.(map[string]bool)
	if !isMap || !ok["ok"] {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectUpdateMemberRoleChannel_Success(t *testing.T) {
	var gotReq *projectv1.UpdateMemberRoleRequest
	fake := &fakeProjectServiceClient2{
		updateMemberRoleFunc: func(ctx context.Context, in *projectv1.UpdateMemberRoleRequest) (*projectv1.UpdateMemberRoleResponse, error) {
			gotReq = in
			return &projectv1.UpdateMemberRoleResponse{Member: &projectv1.Member{UserId: in.GetUserId(), Role: in.GetRole()}}, nil
		},
	}
	r := NewRegistry()
	registerProjectChannels(r, fake)

	args := argsJSON(t, map[string]any{"projectId": "p1", "userId": "u1", "role": "owner"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "project.updateMemberRole", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetRole() != projectv1.ProjectRole_PROJECT_ROLE_OWNER {
		t.Errorf("want role mapped to PROJECT_ROLE_OWNER, got %v", gotReq.GetRole())
	}
	if member, ok := result.(*projectv1.Member); !ok || member.GetRole() != projectv1.ProjectRole_PROJECT_ROLE_OWNER {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectChannels_PropagateErrors(t *testing.T) {
	wantErr := errors.New("project-service unavailable")
	fake := &fakeProjectServiceClient2{
		createProjectFunc: func(ctx context.Context, in *projectv1.CreateProjectRequest) (*projectv1.CreateProjectResponse, error) {
			return nil, wantErr
		},
		getProjectFunc: func(ctx context.Context, in *projectv1.GetProjectRequest) (*projectv1.GetProjectResponse, error) {
			return nil, wantErr
		},
		listProjectsFunc: func(ctx context.Context, in *projectv1.ListProjectsRequest) (*projectv1.ListProjectsResponse, error) {
			return nil, wantErr
		},
		updateProjectFunc: func(ctx context.Context, in *projectv1.UpdateProjectRequest) (*projectv1.UpdateProjectResponse, error) {
			return nil, wantErr
		},
		listMembersFunc: func(ctx context.Context, in *projectv1.ListMembersRequest) (*projectv1.ListMembersResponse, error) {
			return nil, wantErr
		},
		removeMemberFunc: func(ctx context.Context, in *projectv1.RemoveMemberRequest) (*projectv1.RemoveMemberResponse, error) {
			return nil, wantErr
		},
		updateMemberRoleFunc: func(ctx context.Context, in *projectv1.UpdateMemberRoleRequest) (*projectv1.UpdateMemberRoleResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerProjectChannels(r, fake)

	cases := []struct {
		channel string
		args    []byte
	}{
		{"project.create", []byte(`{}`)},
		{"project.get", []byte(`{"id":"p1"}`)},
		{"project.list", []byte(`{}`)},
		{"project.update", []byte(`{"id":"p1"}`)},
		{"project.getMembers", []byte(`{"projectId":"p1"}`)},
		{"project.removeMember", []byte(`{"projectId":"p1","userId":"u1"}`)},
		{"project.updateMemberRole", []byte(`{"projectId":"p1","userId":"u1","role":"owner"}`)},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, tc.channel, []json.RawMessage{tc.args})
			if !errors.Is(err, wantErr) {
				t.Fatalf("channel %s: want error %v, got %v", tc.channel, wantErr, err)
			}
		})
	}
}

// ── projectGroup.* ─────────────────────────────────────────────────────

func TestProjectGroupCreateChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		createProjectGroupFunc: func(ctx context.Context, in *projectv1.CreateProjectGroupRequest) (*projectv1.CreateProjectGroupResponse, error) {
			return &projectv1.CreateProjectGroupResponse{Group: &projectv1.ProjectGroup{Id: "g1", Name: in.GetName()}}, nil
		},
	}
	r := NewRegistry()
	registerProjectGroupChannels(r, fake)

	args := argsJSON(t, map[string]any{"name": "Group A"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectGroup.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g, ok := result.(*projectv1.ProjectGroup); !ok || g.GetName() != "Group A" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectGroupUpdateChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		updateProjectGroupFunc: func(ctx context.Context, in *projectv1.UpdateProjectGroupRequest) (*projectv1.UpdateProjectGroupResponse, error) {
			return &projectv1.UpdateProjectGroupResponse{Group: &projectv1.ProjectGroup{Id: in.GetGroupId(), Name: in.GetName()}}, nil
		},
	}
	r := NewRegistry()
	registerProjectGroupChannels(r, fake)

	args := argsJSON(t, map[string]any{"groupId": "g1", "name": "Renamed"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectGroup.update", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g, ok := result.(*projectv1.ProjectGroup); !ok || g.GetName() != "Renamed" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectGroupDeleteChannel_Success(t *testing.T) {
	var gotReq *projectv1.DeleteProjectGroupRequest
	fake := &fakeProjectServiceClient2{
		deleteProjectGroupFunc: func(ctx context.Context, in *projectv1.DeleteProjectGroupRequest) (*projectv1.DeleteProjectGroupResponse, error) {
			gotReq = in
			return &projectv1.DeleteProjectGroupResponse{}, nil
		},
	}
	r := NewRegistry()
	registerProjectGroupChannels(r, fake)

	args := argsJSON(t, map[string]any{"groupId": "g1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectGroup.delete", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetGroupId() != "g1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	ok, isMap := result.(map[string]bool)
	if !isMap || !ok["ok"] {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectGroupListChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		listProjectGroupsFunc: func(ctx context.Context, in *projectv1.ListProjectGroupsRequest) (*projectv1.ListProjectGroupsResponse, error) {
			return &projectv1.ListProjectGroupsResponse{Groups: []*projectv1.ProjectGroup{{Id: "g1"}}}, nil
		},
	}
	r := NewRegistry()
	registerProjectGroupChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectGroup.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if groups, ok := result.([]*projectv1.ProjectGroup); !ok || len(groups) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectGroupMoveProjectChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		moveProjectFunc: func(ctx context.Context, in *projectv1.MoveProjectRequest) (*projectv1.MoveProjectResponse, error) {
			return &projectv1.MoveProjectResponse{Group: &projectv1.ProjectGroup{Id: "g1", ProjectId: in.GetProjectId()}}, nil
		},
	}
	r := NewRegistry()
	registerProjectGroupChannels(r, fake)

	args := argsJSON(t, map[string]any{"projectId": "p1", "targetParentGroupId": "g0"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectGroup.moveProject", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g, ok := result.(*projectv1.ProjectGroup); !ok || g.GetProjectId() != "p1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectGroupScanNestedChannel_UsesLongerTimeout(t *testing.T) {
	var gotDeadline time.Time
	var hasDeadline bool
	fake := &fakeProjectServiceClient2{
		scanNestedFunc: func(ctx context.Context, in *projectv1.ScanNestedRequest) (*projectv1.ScanNestedResponse, error) {
			gotDeadline, hasDeadline = ctx.Deadline()
			return &projectv1.ScanNestedResponse{}, nil
		},
	}
	r := NewRegistry()
	registerProjectGroupChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "ds1", "rootPath": "/repos"})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectGroup.scanNested", args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDeadline {
		t.Fatalf("expected a deadline on the outbound context")
	}
	remaining := time.Until(gotDeadline)
	if remaining <= rpcTimeout {
		t.Errorf("want deadline > rpcTimeout (%v), got %v remaining", rpcTimeout, remaining)
	}
}

func TestProjectGroupImportNestedChannel_Success(t *testing.T) {
	var gotReq *projectv1.ImportNestedRequest
	fake := &fakeProjectServiceClient2{
		importNestedFunc: func(ctx context.Context, in *projectv1.ImportNestedRequest) (*projectv1.ImportNestedResponse, error) {
			gotReq = in
			return &projectv1.ImportNestedResponse{}, nil
		},
	}
	r := NewRegistry()
	registerProjectGroupChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"devServerId":   "ds1",
		"parentGroupId": "g1",
		"selected": []map[string]any{
			{"path": "/repos/a", "suggestedName": "a", "isGitRepo": true},
		},
	})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectGroup.importNested", args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotReq.GetSelected()) != 1 {
		t.Fatalf("want 1 selected candidate, got %d", len(gotReq.GetSelected()))
	}
	c := gotReq.GetSelected()[0]
	if c.GetPath() != "/repos/a" || c.GetSuggestedName() != "a" || !c.GetIsGitRepo() {
		t.Errorf("unexpected candidate mapping: %+v", c)
	}
}

// ── projectHostSetup.* ─────────────────────────────────────────────────

func TestProjectHostSetupCreateChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		createHostSetupFunc: func(ctx context.Context, in *projectv1.CreateHostSetupRequest) (*projectv1.CreateHostSetupResponse, error) {
			return &projectv1.CreateHostSetupResponse{Setup: &projectv1.HostSetup{Id: "hs1", DevServerId: in.GetDevServerId()}}, nil
		},
	}
	r := NewRegistry()
	registerProjectHostSetupChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "ds1", "folderPath": "/repos/a", "displayName": "A"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectHostSetup.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := result.(*projectv1.HostSetup); !ok || s.GetDevServerId() != "ds1" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectHostSetupListChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		listHostSetupsFunc: func(ctx context.Context, in *projectv1.ListHostSetupsRequest) (*projectv1.ListHostSetupsResponse, error) {
			return &projectv1.ListHostSetupsResponse{Setups: []*projectv1.HostSetup{{Id: "hs1"}}}, nil
		},
	}
	r := NewRegistry()
	registerProjectHostSetupChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectHostSetup.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setups, ok := result.([]*projectv1.HostSetup); !ok || len(setups) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectHostSetupUpdateChannel_Success(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		updateHostSetupFunc: func(ctx context.Context, in *projectv1.UpdateHostSetupRequest) (*projectv1.UpdateHostSetupResponse, error) {
			return &projectv1.UpdateHostSetupResponse{Setup: &projectv1.HostSetup{Id: in.GetId(), FolderPath: in.GetFolderPath()}}, nil
		},
	}
	r := NewRegistry()
	registerProjectHostSetupChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "hs1", "folderPath": "/repos/b"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectHostSetup.update", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := result.(*projectv1.HostSetup); !ok || s.GetFolderPath() != "/repos/b" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectHostSetupDeleteChannel_Success(t *testing.T) {
	var gotReq *projectv1.DeleteHostSetupRequest
	fake := &fakeProjectServiceClient2{
		deleteHostSetupFunc: func(ctx context.Context, in *projectv1.DeleteHostSetupRequest) (*projectv1.DeleteHostSetupResponse, error) {
			gotReq = in
			return &projectv1.DeleteHostSetupResponse{}, nil
		},
	}
	r := NewRegistry()
	registerProjectHostSetupChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "hs1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectHostSetup.delete", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetId() != "hs1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	ok, isMap := result.(map[string]bool)
	if !isMap || !ok["ok"] {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProjectHostSetupSetupExistingFolderChannel_UsesLongerTimeout(t *testing.T) {
	var gotDeadline time.Time
	var hasDeadline bool
	fake := &fakeProjectServiceClient2{
		setupExistingFolderFunc: func(ctx context.Context, in *projectv1.SetupExistingFolderRequest) (*projectv1.SetupExistingFolderResponse, error) {
			gotDeadline, hasDeadline = ctx.Deadline()
			return &projectv1.SetupExistingFolderResponse{Setup: &projectv1.HostSetup{Status: "completed"}}, nil
		},
	}
	r := NewRegistry()
	registerProjectHostSetupChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "hs1"})
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectHostSetup.setupExistingFolder", args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasDeadline {
		t.Fatalf("expected a deadline on the outbound context")
	}
	remaining := time.Until(gotDeadline)
	if remaining <= rpcTimeout {
		t.Errorf("want deadline > rpcTimeout (%v), got %v remaining", rpcTimeout, remaining)
	}
}

func TestProjectHostSetupSetupExistingFolderChannel_ReturnsProjectOnlyOnSuccess(t *testing.T) {
	fake := &fakeProjectServiceClient2{
		setupExistingFolderFunc: func(ctx context.Context, in *projectv1.SetupExistingFolderRequest) (*projectv1.SetupExistingFolderResponse, error) {
			// Failed case: Setup.Status == "failed", Project left nil — the
			// handler does no status branching itself, that's the gRPC
			// layer's job (TASK-143 Step 9); assert raw passthrough.
			return &projectv1.SetupExistingFolderResponse{Setup: &projectv1.HostSetup{Status: "failed"}}, nil
		},
	}
	r := NewRegistry()
	registerProjectHostSetupChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "hs1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "projectHostSetup.setupExistingFolder", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*projectv1.SetupExistingFolderResponse)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if resp.GetSetup().GetStatus() != "failed" {
		t.Errorf("want status failed, got %q", resp.GetSetup().GetStatus())
	}
	if resp.GetProject() != nil {
		t.Errorf("want nil Project on failure passthrough, got %+v", resp.GetProject())
	}
}
