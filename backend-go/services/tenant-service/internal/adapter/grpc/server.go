// Package grpc implements the generated tenantv1.TenantServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
	"github.com/stablyai/orca-go/services/tenant-service/internal/usecase"

	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// ResolvedProfileGetter is satisfied by both usecase.GetResolvedProfile and
// its usecase.CachedGetResolvedProfile decorator — the gRPC layer doesn't
// care which one the composition root (cmd/server/main.go) wires in
// (tenant-service.md §6).
type ResolvedProfileGetter interface {
	Execute(ctx context.Context, userID string) (domain.ResolvedProfile, error)
}

// Server implements tenantv1.UnimplementedTenantServiceServer.
type Server struct {
	tenantv1.UnimplementedTenantServiceServer

	createCompany      *usecase.CreateCompany
	validateTenant     *usecase.ValidateTenant
	createDepartment   *usecase.CreateDepartment
	setUserDepartment  *usecase.SetUserDepartment
	getResolvedProfile ResolvedProfileGetter
	createTeam         *usecase.CreateTeam
	addTeamMember      *usecase.AddTeamMember
	listTeamMembers    *usecase.ListTeamMembers
}

func New(
	createCompany *usecase.CreateCompany,
	validateTenant *usecase.ValidateTenant,
	createDepartment *usecase.CreateDepartment,
	setUserDepartment *usecase.SetUserDepartment,
	getResolvedProfile ResolvedProfileGetter,
	createTeam *usecase.CreateTeam,
	addTeamMember *usecase.AddTeamMember,
	listTeamMembers *usecase.ListTeamMembers,
) *Server {
	return &Server{
		createCompany:      createCompany,
		validateTenant:     validateTenant,
		createDepartment:   createDepartment,
		setUserDepartment:  setUserDepartment,
		getResolvedProfile: getResolvedProfile,
		createTeam:         createTeam,
		addTeamMember:      addTeamMember,
		listTeamMembers:    listTeamMembers,
	}
}

func (s *Server) CreateCompany(ctx context.Context, req *tenantv1.CreateCompanyRequest) (*tenantv1.CreateCompanyResponse, error) {
	company, err := s.createCompany.Execute(ctx, usecase.CreateCompanyInput{Name: req.GetName()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	proto, err := toProtoCompany(company)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.CreateCompanyResponse{Company: proto}, nil
}

func (s *Server) ValidateTenant(ctx context.Context, req *tenantv1.ValidateTenantRequest) (*tenantv1.ValidateTenantResponse, error) {
	exists, err := s.validateTenant.Execute(ctx, usecase.ValidateTenantInput{TenantID: req.GetTenantId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.ValidateTenantResponse{Exists: exists}, nil
}

func (s *Server) CreateDepartment(ctx context.Context, req *tenantv1.CreateDepartmentRequest) (*tenantv1.CreateDepartmentResponse, error) {
	department, err := s.createDepartment.Execute(ctx, usecase.CreateDepartmentInput{
		CompanyID: req.GetCompanyId(),
		Name:      req.GetName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	proto, err := toProtoDepartment(department)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.CreateDepartmentResponse{Department: proto}, nil
}

func (s *Server) SetUserDepartment(ctx context.Context, req *tenantv1.SetUserDepartmentRequest) (*tenantv1.SetUserDepartmentResponse, error) {
	err := s.setUserDepartment.Execute(ctx, usecase.SetUserDepartmentInput{
		UserID:       req.GetUserId(),
		DepartmentID: req.GetDepartmentId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.SetUserDepartmentResponse{}, nil
}

func (s *Server) GetResolvedProfile(ctx context.Context, req *tenantv1.GetResolvedProfileRequest) (*tenantv1.GetResolvedProfileResponse, error) {
	resolved, err := s.getResolvedProfile.Execute(ctx, req.GetUserId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	settingsJSON, err := marshalSettings(resolved.Settings)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInternal, "TENANT_MARSHAL_PROFILE_FAILED", "failed to marshal resolved profile", err))
	}
	return &tenantv1.GetResolvedProfileResponse{ResolvedSettingsJson: settingsJSON}, nil
}

func (s *Server) CreateTeam(ctx context.Context, req *tenantv1.CreateTeamRequest) (*tenantv1.CreateTeamResponse, error) {
	settings, err := unmarshalSettings(req.GetSettingsJson())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_TEAM_SETTINGS", "settings_json is not valid JSON", err))
	}
	team, err := s.createTeam.Execute(ctx, usecase.CreateTeamInput{
		CompanyID: req.GetCompanyId(),
		Name:      req.GetName(),
		Settings:  settings,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	proto, err := toProtoTeam(team)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.CreateTeamResponse{Team: proto}, nil
}

func (s *Server) AddTeamMember(ctx context.Context, req *tenantv1.AddTeamMemberRequest) (*tenantv1.AddTeamMemberResponse, error) {
	_, err := s.addTeamMember.Execute(ctx, usecase.AddTeamMemberInput{
		TeamID:   req.GetTeamId(),
		UserID:   req.GetUserId(),
		Priority: req.GetPriority(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.AddTeamMemberResponse{}, nil
}

func (s *Server) ListTeamMembers(ctx context.Context, req *tenantv1.ListTeamMembersRequest) (*tenantv1.ListTeamMembersResponse, error) {
	members, err := s.listTeamMembers.Execute(ctx, usecase.ListTeamMembersInput{TeamID: req.GetTeamId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*tenantv1.TeamMember, 0, len(members))
	for _, m := range members {
		out = append(out, &tenantv1.TeamMember{UserId: m.UserID, Priority: m.Priority})
	}
	return &tenantv1.ListTeamMembersResponse{Members: out}, nil
}

func toProtoCompany(c domain.Company) (*tenantv1.Company, error) {
	settingsJSON, err := marshalSettings(c.Settings)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_MARSHAL_COMPANY_FAILED", "failed to marshal company settings", err)
	}
	return &tenantv1.Company{Id: c.ID, Name: c.Name, SettingsJson: settingsJSON}, nil
}

func toProtoDepartment(d domain.Department) (*tenantv1.Department, error) {
	settingsJSON, err := marshalSettings(d.Settings)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_MARSHAL_DEPARTMENT_FAILED", "failed to marshal department settings", err)
	}
	return &tenantv1.Department{Id: d.ID, CompanyId: d.CompanyID, Name: d.Name, SettingsJson: settingsJSON}, nil
}

func toProtoTeam(t domain.Team) (*tenantv1.Team, error) {
	settingsJSON, err := marshalSettings(t.Settings)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_MARSHAL_TEAM_FAILED", "failed to marshal team settings", err)
	}
	return &tenantv1.Team{Id: t.ID, CompanyId: t.CompanyID, Name: t.Name, SettingsJson: settingsJSON}, nil
}
