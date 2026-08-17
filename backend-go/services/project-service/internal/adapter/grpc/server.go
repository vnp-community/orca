// Package grpc implements the generated projectv1.ProjectServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
	"github.com/stablyai/orca-go/services/project-service/internal/usecase"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// Server implements projectv1.UnimplementedProjectServiceServer.
type Server struct {
	projectv1.UnimplementedProjectServiceServer

	createProject   *usecase.CreateProject
	getProject      *usecase.GetProject
	listProjects    *usecase.ListProjects
	addMember       *usecase.AddMember
	rebindDevServer *usecase.RebindDevServer
}

func New(
	create *usecase.CreateProject,
	get *usecase.GetProject,
	list *usecase.ListProjects,
	addMember *usecase.AddMember,
	rebind *usecase.RebindDevServer,
) *Server {
	return &Server{
		createProject:   create,
		getProject:      get,
		listProjects:    list,
		addMember:       addMember,
		rebindDevServer: rebind,
	}
}

// CreateProject ignores req.GetTenantId() — TenantID is pulled from the
// authenticated request context (see common/tenant), never trusted from the
// request body, per architecture/05-data-architecture.md's tenant-isolation
// rule. The wire field exists for forward compatibility with a future
// admin/cross-tenant path, not used here.
func (s *Server) CreateProject(ctx context.Context, req *projectv1.CreateProjectRequest) (*projectv1.CreateProjectResponse, error) {
	project, err := s.createProject.Execute(ctx, usecase.CreateProjectInput{Name: req.GetName()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.CreateProjectResponse{Project: toProtoProject(project)}, nil
}

func (s *Server) GetProject(ctx context.Context, req *projectv1.GetProjectRequest) (*projectv1.GetProjectResponse, error) {
	project, err := s.getProject.Execute(ctx, usecase.GetProjectInput{ID: req.GetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.GetProjectResponse{Project: toProtoProject(project)}, nil
}

// ListProjects ignores req.GetTenantId() for the same reason as
// CreateProject — the caller's own tenant, from context, always scopes the
// list.
func (s *Server) ListProjects(ctx context.Context, req *projectv1.ListProjectsRequest) (*projectv1.ListProjectsResponse, error) {
	out, err := s.listProjects.Execute(ctx, usecase.ListProjectsInput{
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	projects := make([]*projectv1.Project, 0, len(out.Projects))
	for _, p := range out.Projects {
		projects = append(projects, toProtoProject(p))
	}
	return &projectv1.ListProjectsResponse{Projects: projects, NextPageToken: out.NextPageToken}, nil
}

func (s *Server) AddMember(ctx context.Context, req *projectv1.AddMemberRequest) (*projectv1.AddMemberResponse, error) {
	_, err := s.addMember.Execute(ctx, usecase.AddMemberInput{
		ProjectID: req.GetProjectId(),
		UserID:    req.GetUserId(),
		Role:      toDomainRole(req.GetRole()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.AddMemberResponse{}, nil
}

func (s *Server) RebindDevServer(ctx context.Context, req *projectv1.RebindDevServerRequest) (*projectv1.RebindDevServerResponse, error) {
	project, err := s.rebindDevServer.Execute(ctx, usecase.RebindDevServerInput{
		ProjectID:      req.GetProjectId(),
		NewDevServerID: req.GetNewDevServerId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.RebindDevServerResponse{Project: toProtoProject(project)}, nil
}

func toDomainRole(r projectv1.ProjectRole) domain.ProjectRole {
	switch r {
	case projectv1.ProjectRole_PROJECT_ROLE_OWNER:
		return domain.ProjectRoleOwner
	case projectv1.ProjectRole_PROJECT_ROLE_MEMBER:
		return domain.ProjectRoleMember
	default:
		return ""
	}
}

func toProtoProject(p domain.Project) *projectv1.Project {
	return &projectv1.Project{
		Id:          p.ID,
		TenantId:    p.TenantID,
		Name:        p.Name,
		DevServerId: p.DevServerID,
	}
}
