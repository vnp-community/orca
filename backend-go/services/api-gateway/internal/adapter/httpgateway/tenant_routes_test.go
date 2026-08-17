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

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// fakeTenantServiceClient implements tenantv1.TenantServiceClient with
// per-method canned responses/errors, configurable per test — mirroring
// fakeTaskServiceClient's shape in task_routes_test.go.
type fakeTenantServiceClient struct {
	createCompanyResp *tenantv1.CreateCompanyResponse
	createCompanyErr  error

	validateTenantResp *tenantv1.ValidateTenantResponse
	validateTenantErr  error

	createDepartmentResp *tenantv1.CreateDepartmentResponse
	createDepartmentErr  error

	setUserDepartmentResp *tenantv1.SetUserDepartmentResponse
	setUserDepartmentErr  error

	getResolvedProfileResp *tenantv1.GetResolvedProfileResponse
	getResolvedProfileErr  error

	createTeamResp *tenantv1.CreateTeamResponse
	createTeamErr  error
	createTeamReq  *tenantv1.CreateTeamRequest // captures the last request for assertions

	addTeamMemberResp *tenantv1.AddTeamMemberResponse
	addTeamMemberErr  error

	listTeamMembersResp *tenantv1.ListTeamMembersResponse
	listTeamMembersErr  error
}

func (f *fakeTenantServiceClient) CreateCompany(_ context.Context, _ *tenantv1.CreateCompanyRequest, _ ...grpc.CallOption) (*tenantv1.CreateCompanyResponse, error) {
	if f.createCompanyErr != nil {
		return nil, f.createCompanyErr
	}
	return f.createCompanyResp, nil
}

func (f *fakeTenantServiceClient) ValidateTenant(_ context.Context, _ *tenantv1.ValidateTenantRequest, _ ...grpc.CallOption) (*tenantv1.ValidateTenantResponse, error) {
	if f.validateTenantErr != nil {
		return nil, f.validateTenantErr
	}
	return f.validateTenantResp, nil
}

func (f *fakeTenantServiceClient) CreateDepartment(_ context.Context, _ *tenantv1.CreateDepartmentRequest, _ ...grpc.CallOption) (*tenantv1.CreateDepartmentResponse, error) {
	if f.createDepartmentErr != nil {
		return nil, f.createDepartmentErr
	}
	return f.createDepartmentResp, nil
}

func (f *fakeTenantServiceClient) SetUserDepartment(_ context.Context, _ *tenantv1.SetUserDepartmentRequest, _ ...grpc.CallOption) (*tenantv1.SetUserDepartmentResponse, error) {
	if f.setUserDepartmentErr != nil {
		return nil, f.setUserDepartmentErr
	}
	return f.setUserDepartmentResp, nil
}

func (f *fakeTenantServiceClient) GetResolvedProfile(_ context.Context, _ *tenantv1.GetResolvedProfileRequest, _ ...grpc.CallOption) (*tenantv1.GetResolvedProfileResponse, error) {
	if f.getResolvedProfileErr != nil {
		return nil, f.getResolvedProfileErr
	}
	return f.getResolvedProfileResp, nil
}

func (f *fakeTenantServiceClient) CreateTeam(_ context.Context, in *tenantv1.CreateTeamRequest, _ ...grpc.CallOption) (*tenantv1.CreateTeamResponse, error) {
	f.createTeamReq = in
	if f.createTeamErr != nil {
		return nil, f.createTeamErr
	}
	return f.createTeamResp, nil
}

func (f *fakeTenantServiceClient) AddTeamMember(_ context.Context, _ *tenantv1.AddTeamMemberRequest, _ ...grpc.CallOption) (*tenantv1.AddTeamMemberResponse, error) {
	if f.addTeamMemberErr != nil {
		return nil, f.addTeamMemberErr
	}
	return f.addTeamMemberResp, nil
}

func (f *fakeTenantServiceClient) ListTeamMembers(_ context.Context, _ *tenantv1.ListTeamMembersRequest, _ ...grpc.CallOption) (*tenantv1.ListTeamMembersResponse, error) {
	if f.listTeamMembersErr != nil {
		return nil, f.listTeamMembersErr
	}
	return f.listTeamMembersResp, nil
}

// tenantTestRouter mounts mountTenantRoutes standalone (router.go isn't
// touched by this package's tests, per task instructions) and injects an
// identity into the request context the same way authMiddleware would.
func tenantTestRouter(client tenantv1.TenantServiceClient) chi.Router {
	r := chi.NewRouter()
	mountTenantRoutes(r, client)
	return r
}

func TestHandleCreateTeam_SuccessRoundTrip(t *testing.T) {
	fake := &fakeTenantServiceClient{
		createTeamResp: &tenantv1.CreateTeamResponse{
			Team: &tenantv1.Team{
				Id:           "team-1",
				CompanyId:    "company-1",
				Name:         "Platform",
				SettingsJson: `{"theme":"dark"}`,
			},
		},
	}
	router := tenantTestRouter(fake)

	body, _ := json.Marshal(createTeamRequestBody{
		CompanyID:    "company-1",
		Name:         "Platform",
		SettingsJSON: `{"theme":"dark"}`,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/tenants/teams", bytes.NewReader(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var gotTeam tenantv1.Team
	if err := json.Unmarshal(rec.Body.Bytes(), &gotTeam); err != nil {
		t.Fatalf("response body is not the expected Team JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if gotTeam.GetId() != "team-1" || gotTeam.GetName() != "Platform" {
		t.Fatalf("unexpected team in response: %+v", &gotTeam)
	}

	// settings_json must round-trip from the REST body into the gRPC
	// request sent upstream.
	if fake.createTeamReq == nil {
		t.Fatal("CreateTeam was never called")
	}
	if fake.createTeamReq.GetSettingsJson() != `{"theme":"dark"}` {
		t.Fatalf("SettingsJson sent upstream = %q, want %q", fake.createTeamReq.GetSettingsJson(), `{"theme":"dark"}`)
	}
	if fake.createTeamReq.GetCompanyId() != "company-1" {
		t.Fatalf("CompanyId sent upstream = %q, want %q", fake.createTeamReq.GetCompanyId(), "company-1")
	}
}

func TestHandleGetResolvedProfile_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	fake := &fakeTenantServiceClient{
		getResolvedProfileErr: status.Error(codes.NotFound, "profile not found"),
	}
	router := tenantTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/tenants/profile", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the expected error JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Code != codes.NotFound.String() {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, codes.NotFound.String())
	}
}
