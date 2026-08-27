// Package grpc implements the generated projectv1.ProjectServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
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
	updateProject   *usecase.UpdateProject
	deleteProject   *usecase.DeleteProject

	listMembers      *usecase.ListMembers
	removeMember     *usecase.RemoveMember
	updateMemberRole *usecase.UpdateMemberRole

	addRepo      *usecase.AddRepo
	listRepos    *usecase.ListRepos
	reorderRepos *usecase.ReorderRepos
	removeRepo   *usecase.RemoveRepo
	updateRepo   *usecase.UpdateRepo

	recordWorktreeCreated *usecase.RecordWorktreeCreated
	recordWorktreeRemoved *usecase.RecordWorktreeRemoved
	listWorktrees         *usecase.ListWorktrees
	setWorktreeActivation *usecase.SetWorktreeActivation
	renameWorktree        *usecase.RenameWorktree

	createProjectGroup *usecase.CreateProjectGroup
	updateProjectGroup *usecase.UpdateProjectGroup
	deleteProjectGroup *usecase.DeleteProjectGroup
	listProjectGroups  *usecase.ListProjectGroups

	folderWorkspaces *usecase.FolderWorkspaceUseCase
	moveProject        *usecase.MoveProject
	scanNested         *usecase.ScanNested
	importNested       *usecase.ImportNested

	createHostSetup     *usecase.CreateHostSetup
	listHostSetups      *usecase.ListHostSetups
	updateHostSetup     *usecase.UpdateHostSetup
	deleteHostSetup     *usecase.DeleteHostSetup
	setupExistingFolder *usecase.SetupExistingFolder

	getMobileWorktreeStatus *usecase.GetMobileWorktreeStatus
}

// Deps groups every usecase Server needs — a plain constructor with 20
// positional *usecase.X params would be unreadable and error-prone to call
// correctly at the composition root, so New takes this struct instead.
type Deps struct {
	CreateProject   *usecase.CreateProject
	GetProject      *usecase.GetProject
	ListProjects    *usecase.ListProjects
	AddMember       *usecase.AddMember
	RebindDevServer *usecase.RebindDevServer
	UpdateProject   *usecase.UpdateProject
	DeleteProject   *usecase.DeleteProject

	ListMembers      *usecase.ListMembers
	RemoveMember     *usecase.RemoveMember
	UpdateMemberRole *usecase.UpdateMemberRole

	AddRepo      *usecase.AddRepo
	ListRepos    *usecase.ListRepos
	ReorderRepos *usecase.ReorderRepos
	RemoveRepo   *usecase.RemoveRepo
	UpdateRepo   *usecase.UpdateRepo

	RecordWorktreeCreated *usecase.RecordWorktreeCreated
	RecordWorktreeRemoved *usecase.RecordWorktreeRemoved
	ListWorktrees         *usecase.ListWorktrees
	SetWorktreeActivation *usecase.SetWorktreeActivation
	RenameWorktree        *usecase.RenameWorktree

	CreateProjectGroup *usecase.CreateProjectGroup
	UpdateProjectGroup *usecase.UpdateProjectGroup
	DeleteProjectGroup *usecase.DeleteProjectGroup
	ListProjectGroups  *usecase.ListProjectGroups

	FolderWorkspaces *usecase.FolderWorkspaceUseCase
	MoveProject        *usecase.MoveProject
	ScanNested         *usecase.ScanNested
	ImportNested       *usecase.ImportNested

	CreateHostSetup     *usecase.CreateHostSetup
	ListHostSetups      *usecase.ListHostSetups
	UpdateHostSetup     *usecase.UpdateHostSetup
	DeleteHostSetup     *usecase.DeleteHostSetup
	SetupExistingFolder *usecase.SetupExistingFolder

	GetMobileWorktreeStatus *usecase.GetMobileWorktreeStatus
}

func New(deps Deps) *Server {
	return &Server{
		createProject:   deps.CreateProject,
		getProject:      deps.GetProject,
		listProjects:    deps.ListProjects,
		addMember:       deps.AddMember,
		rebindDevServer: deps.RebindDevServer,
		updateProject:   deps.UpdateProject,
		deleteProject:   deps.DeleteProject,

		listMembers:      deps.ListMembers,
		removeMember:     deps.RemoveMember,
		updateMemberRole: deps.UpdateMemberRole,

		addRepo:      deps.AddRepo,
		listRepos:    deps.ListRepos,
		reorderRepos: deps.ReorderRepos,
		removeRepo:   deps.RemoveRepo,
		updateRepo:   deps.UpdateRepo,

		recordWorktreeCreated: deps.RecordWorktreeCreated,
		recordWorktreeRemoved: deps.RecordWorktreeRemoved,
		listWorktrees:         deps.ListWorktrees,
		setWorktreeActivation: deps.SetWorktreeActivation,
		renameWorktree:        deps.RenameWorktree,

		createProjectGroup: deps.CreateProjectGroup,
		updateProjectGroup: deps.UpdateProjectGroup,
		deleteProjectGroup: deps.DeleteProjectGroup,
		listProjectGroups:  deps.ListProjectGroups,

		folderWorkspaces: deps.FolderWorkspaces,
		moveProject:        deps.MoveProject,
		scanNested:         deps.ScanNested,
		importNested:       deps.ImportNested,

		createHostSetup:     deps.CreateHostSetup,
		listHostSetups:      deps.ListHostSetups,
		updateHostSetup:     deps.UpdateHostSetup,
		deleteHostSetup:     deps.DeleteHostSetup,
		setupExistingFolder: deps.SetupExistingFolder,

		getMobileWorktreeStatus: deps.GetMobileWorktreeStatus,
	}
}

// CreateProject ignores req.GetTenantId() — TenantID is pulled from the
// authenticated request context (see common/tenant), never trusted from the
// request body, per architecture/05-data-architecture.md's tenant-isolation
// rule. The wire field exists for forward compatibility with a future
// admin/cross-tenant path, not used here.
func (s *Server) CreateProject(ctx context.Context, req *projectv1.CreateProjectRequest) (*projectv1.CreateProjectResponse, error) {
	project, err := s.createProject.Execute(ctx, usecase.CreateProjectInput{
		Name:          req.GetName(),
		Description:   req.GetDescription(),
		DefaultBranch: req.GetDefaultBranch(),
		Visibility:    req.GetVisibility(),
	})
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

func (s *Server) ListMembers(ctx context.Context, req *projectv1.ListMembersRequest) (*projectv1.ListMembersResponse, error) {
	members, err := s.listMembers.Execute(ctx, usecase.ListMembersInput{ProjectID: req.GetProjectId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.Member, 0, len(members))
	for _, m := range members {
		out = append(out, &projectv1.Member{UserId: m.UserID, Role: toProtoRole(m.Role)})
	}
	return &projectv1.ListMembersResponse{Members: out}, nil
}

func (s *Server) RemoveMember(ctx context.Context, req *projectv1.RemoveMemberRequest) (*projectv1.RemoveMemberResponse, error) {
	err := s.removeMember.Execute(ctx, usecase.RemoveMemberInput{
		ProjectID: req.GetProjectId(), UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.RemoveMemberResponse{}, nil
}

func (s *Server) UpdateMemberRole(ctx context.Context, req *projectv1.UpdateMemberRoleRequest) (*projectv1.UpdateMemberRoleResponse, error) {
	member, err := s.updateMemberRole.Execute(ctx, usecase.UpdateMemberRoleInput{
		ProjectID: req.GetProjectId(), UserID: req.GetUserId(), Role: toDomainRole(req.GetRole()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateMemberRoleResponse{Member: &projectv1.Member{UserId: member.UserID, Role: toProtoRole(member.Role)}}, nil
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

func (s *Server) UpdateProject(ctx context.Context, req *projectv1.UpdateProjectRequest) (*projectv1.UpdateProjectResponse, error) {
	project, err := s.updateProject.Execute(ctx, usecase.UpdateProjectInput{
		ProjectID:     req.GetProjectId(),
		Name:          req.GetName(),
		Description:   req.GetDescription(),
		DefaultBranch: req.GetDefaultBranch(),
		Visibility:    req.GetVisibility(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateProjectResponse{Project: toProtoProject(project)}, nil
}

func (s *Server) DeleteProject(ctx context.Context, req *projectv1.DeleteProjectRequest) (*projectv1.DeleteProjectResponse, error) {
	if err := s.deleteProject.Execute(ctx, usecase.DeleteProjectInput{ProjectID: req.GetProjectId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.DeleteProjectResponse{}, nil
}

func (s *Server) AddRepo(ctx context.Context, req *projectv1.AddRepoRequest) (*projectv1.AddRepoResponse, error) {
	repo, err := s.addRepo.Execute(ctx, usecase.AddRepoInput{
		ProjectID:   req.GetProjectId(),
		URL:         req.GetUrl(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.AddRepoResponse{Repo: toProtoRepo(repo)}, nil
}

func (s *Server) ListRepos(ctx context.Context, req *projectv1.ListReposRequest) (*projectv1.ListReposResponse, error) {
	repos, err := s.listRepos.Execute(ctx, usecase.ListReposInput{ProjectID: req.GetProjectId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.Repo, 0, len(repos))
	for _, r := range repos {
		out = append(out, toProtoRepo(r))
	}
	return &projectv1.ListReposResponse{Repos: out}, nil
}

func (s *Server) ReorderRepos(ctx context.Context, req *projectv1.ReorderReposRequest) (*projectv1.ReorderReposResponse, error) {
	if err := s.reorderRepos.Execute(ctx, usecase.ReorderReposInput{
		ProjectID:      req.GetProjectId(),
		RepoIDsInOrder: req.GetRepoIdsInOrder(),
	}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.ReorderReposResponse{}, nil
}

func (s *Server) RemoveRepo(ctx context.Context, req *projectv1.RemoveRepoRequest) (*projectv1.RemoveRepoResponse, error) {
	if err := s.removeRepo.Execute(ctx, usecase.RemoveRepoInput{RepoID: req.GetRepoId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.RemoveRepoResponse{}, nil
}

func (s *Server) UpdateRepo(ctx context.Context, req *projectv1.UpdateRepoRequest) (*projectv1.UpdateRepoResponse, error) {
	repo, err := s.updateRepo.Execute(ctx, usecase.UpdateRepoInput{
		RepoID:      req.GetRepoId(),
		URL:         req.GetUrl(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateRepoResponse{Repo: toProtoRepo(repo)}, nil
}

func (s *Server) RecordWorktreeCreated(ctx context.Context, req *projectv1.RecordWorktreeCreatedRequest) (*projectv1.RecordWorktreeCreatedResponse, error) {
	wt, err := s.recordWorktreeCreated.Execute(ctx, usecase.RecordWorktreeCreatedInput{
		ProjectID: req.GetProjectId(),
		RepoID:    req.GetRepoId(),
		Path:      req.GetPath(),
		Branch:    req.GetBranch(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.RecordWorktreeCreatedResponse{Worktree: toProtoWorktree(wt)}, nil
}

func (s *Server) RecordWorktreeRemoved(ctx context.Context, req *projectv1.RecordWorktreeRemovedRequest) (*projectv1.RecordWorktreeRemovedResponse, error) {
	if err := s.recordWorktreeRemoved.Execute(ctx, usecase.RecordWorktreeRemovedInput{WorktreeID: req.GetWorktreeId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.RecordWorktreeRemovedResponse{}, nil
}

func (s *Server) ListWorktrees(ctx context.Context, req *projectv1.ListWorktreesRequest) (*projectv1.ListWorktreesResponse, error) {
	worktrees, err := s.listWorktrees.Execute(ctx, usecase.ListWorktreesInput{ProjectID: req.GetProjectId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.Worktree, 0, len(worktrees))
	for _, wt := range worktrees {
		out = append(out, toProtoWorktree(wt))
	}
	return &projectv1.ListWorktreesResponse{Worktrees: out}, nil
}

func (s *Server) SetWorktreeActivation(ctx context.Context, req *projectv1.SetWorktreeActivationRequest) (*projectv1.SetWorktreeActivationResponse, error) {
	wt, err := s.setWorktreeActivation.Execute(ctx, usecase.SetWorktreeActivationInput{
		WorktreeID: req.GetWorktreeId(),
		Active:     req.GetActive(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.SetWorktreeActivationResponse{Worktree: toProtoWorktree(wt)}, nil
}

func (s *Server) RenameWorktree(ctx context.Context, req *projectv1.RenameWorktreeRequest) (*projectv1.RenameWorktreeResponse, error) {
	wt, err := s.renameWorktree.Execute(ctx, usecase.RenameWorktreeInput{
		WorktreeID: req.GetWorktreeId(),
		Branch:     req.GetBranch(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.RenameWorktreeResponse{Worktree: toProtoWorktree(wt)}, nil
}

func (s *Server) CreateProjectGroup(ctx context.Context, req *projectv1.CreateProjectGroupRequest) (*projectv1.CreateProjectGroupResponse, error) {
	group, err := s.createProjectGroup.Execute(ctx, usecase.CreateProjectGroupInput{
		Name:          req.GetName(),
		ParentGroupID: req.GetParentGroupId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.CreateProjectGroupResponse{Group: toProtoProjectGroup(group)}, nil
}

func (s *Server) UpdateProjectGroup(ctx context.Context, req *projectv1.UpdateProjectGroupRequest) (*projectv1.UpdateProjectGroupResponse, error) {
	group, err := s.updateProjectGroup.Execute(ctx, usecase.UpdateProjectGroupInput{
		GroupID: req.GetGroupId(),
		Name:    req.GetName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateProjectGroupResponse{Group: toProtoProjectGroup(group)}, nil
}

func (s *Server) DeleteProjectGroup(ctx context.Context, req *projectv1.DeleteProjectGroupRequest) (*projectv1.DeleteProjectGroupResponse, error) {
	if err := s.deleteProjectGroup.Execute(ctx, usecase.DeleteProjectGroupInput{GroupID: req.GetGroupId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.DeleteProjectGroupResponse{}, nil
}

func (s *Server) ListProjectGroups(ctx context.Context, _ *projectv1.ListProjectGroupsRequest) (*projectv1.ListProjectGroupsResponse, error) {
	groups, err := s.listProjectGroups.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.ProjectGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, toProtoProjectGroup(g))
	}
	return &projectv1.ListProjectGroupsResponse{Groups: out}, nil
}

func (s *Server) MoveProject(ctx context.Context, req *projectv1.MoveProjectRequest) (*projectv1.MoveProjectResponse, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err))
	}
	group, err := s.moveProject.Execute(ctx, tenantID, usecase.MoveProjectInput{
		ProjectID: req.GetProjectId(), TargetParentGroupID: req.GetTargetParentGroupId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.MoveProjectResponse{Group: toProtoProjectGroup(group)}, nil
}

func (s *Server) ScanNested(ctx context.Context, req *projectv1.ScanNestedRequest) (*projectv1.ScanNestedResponse, error) {
	candidates, err := s.scanNested.Execute(ctx, usecase.ScanNestedInput{
		DevServerID: req.GetDevServerId(), RootPath: req.GetRootPath(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.NestedRepoCandidate, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, &projectv1.NestedRepoCandidate{Path: c.Path, SuggestedName: c.SuggestedName, IsGitRepo: c.IsGitRepo})
	}
	return &projectv1.ScanNestedResponse{Candidates: out}, nil
}

func (s *Server) ImportNested(ctx context.Context, req *projectv1.ImportNestedRequest) (*projectv1.ImportNestedResponse, error) {
	selected := make([]domain.NestedRepoCandidate, 0, len(req.GetSelected()))
	for _, c := range req.GetSelected() {
		selected = append(selected, domain.NestedRepoCandidate{Path: c.GetPath(), SuggestedName: c.GetSuggestedName(), IsGitRepo: c.GetIsGitRepo()})
	}
	groups, projects, err := s.importNested.Execute(ctx, usecase.ImportNestedInput{
		DevServerID: req.GetDevServerId(), ParentGroupID: req.GetParentGroupId(), Selected: selected,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	outGroups := make([]*projectv1.ProjectGroup, 0, len(groups))
	for _, g := range groups {
		outGroups = append(outGroups, toProtoProjectGroup(g))
	}
	outProjects := make([]*projectv1.Project, 0, len(projects))
	for _, p := range projects {
		outProjects = append(outProjects, toProtoProject(p))
	}
	return &projectv1.ImportNestedResponse{CreatedGroups: outGroups, CreatedProjects: outProjects}, nil
}

func (s *Server) CreateHostSetup(ctx context.Context, req *projectv1.CreateHostSetupRequest) (*projectv1.CreateHostSetupResponse, error) {
	setup, err := s.createHostSetup.Execute(ctx, usecase.CreateHostSetupInput{
		DevServerID: req.GetDevServerId(), FolderPath: req.GetFolderPath(), DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.CreateHostSetupResponse{Setup: toProtoHostSetup(setup)}, nil
}

func (s *Server) ListHostSetups(ctx context.Context, _ *projectv1.ListHostSetupsRequest) (*projectv1.ListHostSetupsResponse, error) {
	setups, err := s.listHostSetups.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.HostSetup, 0, len(setups))
	for _, setup := range setups {
		out = append(out, toProtoHostSetup(setup))
	}
	return &projectv1.ListHostSetupsResponse{Setups: out}, nil
}

func (s *Server) UpdateHostSetup(ctx context.Context, req *projectv1.UpdateHostSetupRequest) (*projectv1.UpdateHostSetupResponse, error) {
	setup, err := s.updateHostSetup.Execute(ctx, usecase.UpdateHostSetupInput{
		ID:    req.GetId(),
		Patch: domain.HostSetupPatch{FolderPath: req.GetFolderPath(), DisplayName: req.GetDisplayName()},
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateHostSetupResponse{Setup: toProtoHostSetup(setup)}, nil
}

func (s *Server) DeleteHostSetup(ctx context.Context, req *projectv1.DeleteHostSetupRequest) (*projectv1.DeleteHostSetupResponse, error) {
	if err := s.deleteHostSetup.Execute(ctx, usecase.DeleteHostSetupInput{ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.DeleteHostSetupResponse{}, nil
}

func (s *Server) SetupExistingFolder(ctx context.Context, req *projectv1.SetupExistingFolderRequest) (*projectv1.SetupExistingFolderResponse, error) {
	setup, project, err := s.setupExistingFolder.Execute(ctx, usecase.SetupExistingFolderInput{ID: req.GetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &projectv1.SetupExistingFolderResponse{Setup: toProtoHostSetup(setup)}
	if setup.Status == domain.HostSetupCompleted {
		resp.Project = toProtoProject(project)
	}
	return resp, nil
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

// toProtoRole is toDomainRole's inverse, for ListMembers/UpdateMemberRole's
// response mapping.
func toProtoRole(r domain.ProjectRole) projectv1.ProjectRole {
	switch r {
	case domain.ProjectRoleOwner:
		return projectv1.ProjectRole_PROJECT_ROLE_OWNER
	case domain.ProjectRoleMember:
		return projectv1.ProjectRole_PROJECT_ROLE_MEMBER
	default:
		return projectv1.ProjectRole_PROJECT_ROLE_UNSPECIFIED
	}
}

func toProtoProject(p domain.Project) *projectv1.Project {
	out := &projectv1.Project{
		Id:            p.ID,
		TenantId:      p.TenantID,
		Name:          p.Name,
		DevServerId:   p.DevServerID,
		Description:   p.Description,
		DefaultBranch: p.DefaultBranch,
		Visibility:    p.Visibility,
		CreatedBy:     p.CreatedBy,
	}
	if !p.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(p.CreatedAt)
	}
	if !p.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(p.UpdatedAt)
	}
	return out
}

func toProtoRepo(r domain.Repo) *projectv1.Repo {
	return &projectv1.Repo{
		Id:          r.ID,
		ProjectId:   r.ProjectID,
		Url:         r.URL,
		DisplayName: r.DisplayName,
		Position:    r.Position,
	}
}

func toProtoWorktree(wt domain.Worktree) *projectv1.Worktree {
	return &projectv1.Worktree{
		Id:        wt.ID,
		ProjectId: wt.ProjectID,
		RepoId:    wt.RepoID,
		Path:      wt.Path,
		Branch:    wt.Branch,
		Active:    wt.Active,
	}
}

func toProtoProjectGroup(g domain.ProjectGroup) *projectv1.ProjectGroup {
	return &projectv1.ProjectGroup{
		Id:            g.ID,
		TenantId:      g.TenantID,
		Name:          g.Name,
		ParentGroupId: g.ParentGroupID,
		ProjectId:     g.ProjectID,
	}
}

func toProtoHostSetup(s domain.HostSetup) *projectv1.HostSetup {
	return &projectv1.HostSetup{
		Id: s.ID, TenantId: s.TenantID, DevServerId: s.DevServerID, FolderPath: s.FolderPath,
		DisplayName: s.DisplayName, Status: string(s.Status), ProjectId: s.ProjectID,
	}
}

func (s *Server) CreateFolderWorkspace(ctx context.Context, req *projectv1.CreateFolderWorkspaceRequest) (*projectv1.CreateFolderWorkspaceResponse, error) {
	fw, err := s.folderWorkspaces.Create(ctx, usecase.CreateFolderWorkspaceInput{
		DevServerID: req.GetDevServerId(),
		Path:        req.GetPath(),
		Name:        req.GetName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.CreateFolderWorkspaceResponse{FolderWorkspace: toProtoFolderWorkspace(fw)}, nil
}

func (s *Server) UpdateFolderWorkspace(ctx context.Context, req *projectv1.UpdateFolderWorkspaceRequest) (*projectv1.UpdateFolderWorkspaceResponse, error) {
	fw, err := s.folderWorkspaces.Update(ctx, req.GetId(), req.GetName())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateFolderWorkspaceResponse{FolderWorkspace: toProtoFolderWorkspace(fw)}, nil
}

func (s *Server) DeleteFolderWorkspace(ctx context.Context, req *projectv1.DeleteFolderWorkspaceRequest) (*projectv1.DeleteFolderWorkspaceResponse, error) {
	if err := s.folderWorkspaces.Delete(ctx, req.GetId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.DeleteFolderWorkspaceResponse{}, nil
}

func (s *Server) ListFolderWorkspaces(ctx context.Context, _ *projectv1.ListFolderWorkspacesRequest) (*projectv1.ListFolderWorkspacesResponse, error) {
	list, err := s.folderWorkspaces.List(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.FolderWorkspace, 0, len(list))
	for _, fw := range list {
		out = append(out, toProtoFolderWorkspace(fw))
	}
	return &projectv1.ListFolderWorkspacesResponse{FolderWorkspaces: out}, nil
}

func (s *Server) GetFolderWorkspacePathStatus(ctx context.Context, req *projectv1.GetFolderWorkspacePathStatusRequest) (*projectv1.GetFolderWorkspacePathStatusResponse, error) {
	result, err := s.folderWorkspaces.GetPathStatus(ctx, req.GetDevServerId(), req.GetPath())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.GetFolderWorkspacePathStatusResponse{
		Status:                    result.Status,
		ExistingFolderWorkspaceId: result.ExistingID,
	}, nil
}

// GetMobileWorktreeStatus is BL-MB-04's ONE composed-read call — tenant_id/
// user_id come from request-context metadata (see GetMobileWorktreeStatusRequest's
// proto doc comment), not the (empty) request message.
func (s *Server) GetMobileWorktreeStatus(ctx context.Context, req *projectv1.GetMobileWorktreeStatusRequest) (*projectv1.GetMobileWorktreeStatusResponse, error) {
	result, err := s.getMobileWorktreeStatus.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.MobileWorktreeStatus, 0, len(result.Worktrees))
	for _, wt := range result.Worktrees {
		out = append(out, &projectv1.MobileWorktreeStatus{
			Id: wt.ID, Name: wt.Name, Agent: wt.Agent, Status: wt.Status,
			DurationMs: wt.DurationMs, LastOutput: wt.LastOutput,
		})
	}
	return &projectv1.GetMobileWorktreeStatusResponse{
		Worktrees:         out,
		GeneratedAtUnixMs: result.GeneratedAt.UnixMilli(),
	}, nil
}

func toProtoFolderWorkspace(fw domain.FolderWorkspace) *projectv1.FolderWorkspace {
	out := &projectv1.FolderWorkspace{
		Id:          fw.ID,
		DevServerId: fw.DevServerID,
		Path:        fw.Path,
		Name:        fw.Name,
		AddedBy:     fw.AddedBy,
	}
	if !fw.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(fw.CreatedAt)
	}
	return out
}
