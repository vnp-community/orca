// Package grpc implements the generated issuetrackingv1.IssueTrackingServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"encoding/json"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// Server implements issuetrackingv1.UnimplementedIssueTrackingServiceServer.
type Server struct {
	issuetrackingv1.UnimplementedIssueTrackingServiceServer

	listIssues  *usecase.ListIssues
	createIssue *usecase.CreateIssue
	linkIssue   *usecase.LinkIssue

	connect             *usecase.Connect
	disconnect          *usecase.Disconnect
	selectWorkspace     *usecase.SelectWorkspace
	getConnectionStatus *usecase.GetConnectionStatus
	testConnection      *usecase.TestConnection

	searchIssues      *usecase.SearchIssues
	getIssue          *usecase.GetIssue
	updateIssue       *usecase.UpdateIssue
	addIssueComment   *usecase.AddIssueComment
	listIssueComments *usecase.ListIssueComments

	listProjects          *usecase.ListProjects
	listIssueTypes        *usecase.ListIssueTypes
	listCreateFields      *usecase.ListCreateFields
	listAssignableUsers   *usecase.ListAssignableUsers
	listPriorities        *usecase.ListPriorities
	listTransitions       *usecase.ListTransitions
	getProjectStatusOrder *usecase.GetProjectStatusOrder

	createProject *usecase.CreateProject
	getProject    *usecase.GetProject

	listTeams          *usecase.ListTeams
	listTeamLabels     *usecase.ListTeamLabels
	listTeamMembers    *usecase.ListTeamMembers
	getCustomView      *usecase.GetCustomView
	listWorkflowStates *usecase.ListWorkflowStates

	// credentials.* group (TASK-041) — SetIntegrationCredential/
	// GetIntegrationCredentialStatus/ListIntegrationCredentials/RevokeAuth.
	setIntegrationCredential       *usecase.SetIntegrationCredential
	getIntegrationCredentialStatus *usecase.GetIntegrationCredentialStatus
	listIntegrationCredentials     *usecase.ListIntegrationCredentials
	revokeAuth                     *usecase.RevokeAuth
}

// Deps bundles every usecase pointer New needs — kept as a struct (not 25
// positional args) for readability at the composition-root call site.
type Deps struct {
	ListIssues  *usecase.ListIssues
	CreateIssue *usecase.CreateIssue
	LinkIssue   *usecase.LinkIssue

	Connect             *usecase.Connect
	Disconnect          *usecase.Disconnect
	SelectWorkspace     *usecase.SelectWorkspace
	GetConnectionStatus *usecase.GetConnectionStatus
	TestConnection      *usecase.TestConnection

	SearchIssues      *usecase.SearchIssues
	GetIssue          *usecase.GetIssue
	UpdateIssue       *usecase.UpdateIssue
	AddIssueComment   *usecase.AddIssueComment
	ListIssueComments *usecase.ListIssueComments

	ListProjects          *usecase.ListProjects
	ListIssueTypes        *usecase.ListIssueTypes
	ListCreateFields      *usecase.ListCreateFields
	ListAssignableUsers   *usecase.ListAssignableUsers
	ListPriorities        *usecase.ListPriorities
	ListTransitions       *usecase.ListTransitions
	GetProjectStatusOrder *usecase.GetProjectStatusOrder

	CreateProject *usecase.CreateProject
	GetProject    *usecase.GetProject

	ListTeams          *usecase.ListTeams
	ListTeamLabels     *usecase.ListTeamLabels
	ListTeamMembers    *usecase.ListTeamMembers
	GetCustomView      *usecase.GetCustomView
	ListWorkflowStates *usecase.ListWorkflowStates

	SetIntegrationCredential       *usecase.SetIntegrationCredential
	GetIntegrationCredentialStatus *usecase.GetIntegrationCredentialStatus
	ListIntegrationCredentials     *usecase.ListIntegrationCredentials
	RevokeAuth                     *usecase.RevokeAuth
}

func New(d Deps) *Server {
	return &Server{
		listIssues:  d.ListIssues,
		createIssue: d.CreateIssue,
		linkIssue:   d.LinkIssue,

		connect:             d.Connect,
		disconnect:          d.Disconnect,
		selectWorkspace:     d.SelectWorkspace,
		getConnectionStatus: d.GetConnectionStatus,
		testConnection:      d.TestConnection,

		searchIssues:      d.SearchIssues,
		getIssue:          d.GetIssue,
		updateIssue:       d.UpdateIssue,
		addIssueComment:   d.AddIssueComment,
		listIssueComments: d.ListIssueComments,

		listProjects:          d.ListProjects,
		listIssueTypes:        d.ListIssueTypes,
		listCreateFields:      d.ListCreateFields,
		listAssignableUsers:   d.ListAssignableUsers,
		listPriorities:        d.ListPriorities,
		listTransitions:       d.ListTransitions,
		getProjectStatusOrder: d.GetProjectStatusOrder,

		createProject: d.CreateProject,
		getProject:    d.GetProject,

		listTeams:          d.ListTeams,
		listTeamLabels:     d.ListTeamLabels,
		listTeamMembers:    d.ListTeamMembers,
		getCustomView:      d.GetCustomView,
		listWorkflowStates: d.ListWorkflowStates,

		setIntegrationCredential:       d.SetIntegrationCredential,
		getIntegrationCredentialStatus: d.GetIntegrationCredentialStatus,
		listIntegrationCredentials:     d.ListIntegrationCredentials,
		revokeAuth:                     d.RevokeAuth,
	}
}

// ── ListIssues / CreateIssue / LinkIssue ────────────────────────────────

func (s *Server) ListIssues(ctx context.Context, req *issuetrackingv1.ListIssuesRequest) (*issuetrackingv1.ListIssuesResponse, error) {
	issues, err := s.listIssues.Execute(ctx, usecase.ListIssuesInput{
		Provider:    toDomainProvider(req.GetProvider()),
		ProjectKey:  req.GetProjectKey(),
		FilterJSON:  req.GetFilterJson(),
		Limit:       req.GetLimit(),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.Issue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, toProtoIssue(issue))
	}
	return &issuetrackingv1.ListIssuesResponse{Issues: out}, nil
}

func (s *Server) CreateIssue(ctx context.Context, req *issuetrackingv1.CreateIssueRequest) (*issuetrackingv1.CreateIssueResponse, error) {
	issue, err := s.createIssue.Execute(ctx, usecase.CreateIssueInput{
		Provider:         toDomainProvider(req.GetProvider()),
		ProjectKey:       req.GetProjectKey(),
		Title:            req.GetTitle(),
		Description:      req.GetDescription(),
		IssueTypeID:      req.GetIssueTypeId(),
		AssigneeID:       req.GetAssigneeId(),
		PriorityID:       req.GetPriorityId(),
		LabelIDs:         req.GetLabelIds(),
		ParentIssueID:    req.GetParentIssueId(),
		CustomFieldsJSON: req.GetCustomFieldsJson(),
		WorkspaceID:      req.GetWorkspaceId(),
		TeamID:           req.GetTeamId(),
		StateID:          req.GetStateId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.CreateIssueResponse{Issue: toProtoIssue(issue)}, nil
}

func (s *Server) LinkIssue(ctx context.Context, req *issuetrackingv1.LinkIssueRequest) (*issuetrackingv1.LinkIssueResponse, error) {
	err := s.linkIssue.Execute(ctx, usecase.LinkIssueInput{
		IssueID: req.GetIssueId(),
		TaskID:  req.GetTaskId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.LinkIssueResponse{}, nil
}

// ── connection/credential group ─────────────────────────────────────────

func (s *Server) Connect(ctx context.Context, req *issuetrackingv1.ConnectRequest) (*issuetrackingv1.ConnectionStatus, error) {
	status, err := s.connect.Execute(ctx, usecase.ConnectInput{
		Provider: toDomainProvider(req.GetProvider()),
		SiteURL:  req.GetSiteUrl(),
		Email:    req.GetEmail(),
		Token:    req.GetToken(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoConnectionStatus(status), nil
}

func (s *Server) Disconnect(ctx context.Context, req *issuetrackingv1.DisconnectRequest) (*emptypb.Empty, error) {
	err := s.disconnect.Execute(ctx, usecase.DisconnectInput{
		Provider:    toDomainProvider(req.GetProvider()),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) SelectWorkspace(ctx context.Context, req *issuetrackingv1.SelectWorkspaceRequest) (*issuetrackingv1.ConnectionStatus, error) {
	status, err := s.selectWorkspace.Execute(ctx, usecase.SelectWorkspaceInput{
		Provider:    toDomainProvider(req.GetProvider()),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoConnectionStatus(status), nil
}

func (s *Server) GetConnectionStatus(ctx context.Context, req *issuetrackingv1.GetConnectionStatusRequest) (*issuetrackingv1.ConnectionStatus, error) {
	status, err := s.getConnectionStatus.Execute(ctx, usecase.GetConnectionStatusInput{
		Provider: toDomainProvider(req.GetProvider()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoConnectionStatus(status), nil
}

func (s *Server) TestConnection(ctx context.Context, req *issuetrackingv1.TestConnectionRequest) (*issuetrackingv1.TestConnectionResult, error) {
	result, err := s.testConnection.Execute(ctx, usecase.TestConnectionInput{
		Provider:    toDomainProvider(req.GetProvider()),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.TestConnectionResult{Ok: result.OK, Error: result.Error}, nil
}

// ── issue-CRUD group ─────────────────────────────────────────────────────

func (s *Server) SearchIssues(ctx context.Context, req *issuetrackingv1.SearchIssuesRequest) (*issuetrackingv1.SearchIssuesResponse, error) {
	issues, err := s.searchIssues.Execute(ctx, usecase.SearchIssuesInput{
		Provider:    toDomainProvider(req.GetProvider()),
		Query:       req.GetQuery(),
		Limit:       req.GetLimit(),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.Issue, 0, len(issues))
	for _, issue := range issues {
		out = append(out, toProtoIssue(issue))
	}
	return &issuetrackingv1.SearchIssuesResponse{Issues: out}, nil
}

func (s *Server) GetIssue(ctx context.Context, req *issuetrackingv1.GetIssueRequest) (*issuetrackingv1.Issue, error) {
	issue, err := s.getIssue.Execute(ctx, usecase.GetIssueInput{
		Provider:    toDomainProvider(req.GetProvider()),
		IssueID:     req.GetIssueId(),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoIssue(issue), nil
}

func (s *Server) UpdateIssue(ctx context.Context, req *issuetrackingv1.UpdateIssueRequest) (*issuetrackingv1.Issue, error) {
	issue, err := s.updateIssue.Execute(ctx, usecase.UpdateIssueInput{
		Provider:         toDomainProvider(req.GetProvider()),
		IssueID:          req.GetIssueId(),
		Title:            req.GetTitle(),
		Description:      req.GetDescription(),
		AssigneeID:       req.GetAssigneeId(),
		PriorityID:       req.GetPriorityId(),
		LabelIDs:         req.GetLabelIds(),
		WorkflowStateID:  req.GetWorkflowStateId(),
		CustomFieldsJSON: req.GetCustomFieldsJson(),
		WorkspaceID:      req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoIssue(issue), nil
}

func (s *Server) AddIssueComment(ctx context.Context, req *issuetrackingv1.AddIssueCommentRequest) (*issuetrackingv1.IssueComment, error) {
	comment, err := s.addIssueComment.Execute(ctx, usecase.AddIssueCommentInput{
		Provider:     toDomainProvider(req.GetProvider()),
		IssueID:      req.GetIssueId(),
		BodyMarkdown: req.GetBodyMarkdown(),
		WorkspaceID:  req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoComment(comment), nil
}

func (s *Server) ListIssueComments(ctx context.Context, req *issuetrackingv1.ListIssueCommentsRequest) (*issuetrackingv1.ListIssueCommentsResponse, error) {
	comments, err := s.listIssueComments.Execute(ctx, usecase.ListIssueCommentsInput{
		Provider:    toDomainProvider(req.GetProvider()),
		IssueID:     req.GetIssueId(),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.IssueComment, 0, len(comments))
	for _, c := range comments {
		out = append(out, toProtoComment(c))
	}
	return &issuetrackingv1.ListIssueCommentsResponse{Comments: out}, nil
}

// ── metadata-lookup group ────────────────────────────────────────────────

func (s *Server) ListProjects(ctx context.Context, req *issuetrackingv1.ListProjectsRequest) (*issuetrackingv1.ListProjectsResponse, error) {
	projects, err := s.listProjects.Execute(ctx, usecase.ListProjectsInput{
		Provider:    toDomainProvider(req.GetProvider()),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.ListProjectsResponse{Projects: toProtoProjects(projects)}, nil
}

func (s *Server) ListIssueTypes(ctx context.Context, req *issuetrackingv1.ListIssueTypesRequest) (*issuetrackingv1.ListIssueTypesResponse, error) {
	types, err := s.listIssueTypes.Execute(ctx, usecase.ListIssueTypesInput{
		ProjectIDOrKey: req.GetProjectIdOrKey(),
		WorkspaceID:    req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.IssueType, 0, len(types))
	for _, t := range types {
		out = append(out, &issuetrackingv1.IssueType{Id: t.ID, Name: t.Name, Subtask: t.Subtask})
	}
	return &issuetrackingv1.ListIssueTypesResponse{IssueTypes: out}, nil
}

func (s *Server) ListCreateFields(ctx context.Context, req *issuetrackingv1.ListCreateFieldsRequest) (*issuetrackingv1.ListCreateFieldsResponse, error) {
	fields, err := s.listCreateFields.Execute(ctx, usecase.ListCreateFieldsInput{
		ProjectIDOrKey: req.GetProjectIdOrKey(),
		IssueTypeID:    req.GetIssueTypeId(),
		WorkspaceID:    req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.ListCreateFieldsResponse{Fields: toProtoCreateFields(fields)}, nil
}

func (s *Server) ListAssignableUsers(ctx context.Context, req *issuetrackingv1.ListAssignableUsersRequest) (*issuetrackingv1.ListAssignableUsersResponse, error) {
	users, err := s.listAssignableUsers.Execute(ctx, usecase.ListAssignableUsersInput{
		Provider:       toDomainProvider(req.GetProvider()),
		ProjectIDOrKey: req.GetProjectIdOrKey(),
		IssueID:        req.GetIssueId(),
		WorkspaceID:    req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.ListAssignableUsersResponse{Users: toProtoUsers(users)}, nil
}

func (s *Server) ListPriorities(ctx context.Context, req *issuetrackingv1.ListPrioritiesRequest) (*issuetrackingv1.ListPrioritiesResponse, error) {
	priorities, err := s.listPriorities.Execute(ctx, usecase.ListPrioritiesInput{WorkspaceID: req.GetWorkspaceId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.ListPrioritiesResponse{Priorities: toProtoPriorities(priorities)}, nil
}

func (s *Server) ListTransitions(ctx context.Context, req *issuetrackingv1.ListTransitionsRequest) (*issuetrackingv1.ListTransitionsResponse, error) {
	transitions, err := s.listTransitions.Execute(ctx, usecase.ListTransitionsInput{
		Provider:    domain.ProviderJira, // ListTransitionsRequest carries no provider field; Jira is the only wired caller today (see TASK-100)
		IssueID:     req.GetIssueId(),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.ListTransitionsResponse{Transitions: toProtoTransitions(transitions)}, nil
}

func (s *Server) GetProjectStatusOrder(ctx context.Context, req *issuetrackingv1.GetProjectStatusOrderRequest) (*issuetrackingv1.GetProjectStatusOrderResponse, error) {
	order, err := s.getProjectStatusOrder.Execute(ctx, usecase.GetProjectStatusOrderInput{
		ProjectIDOrKey: req.GetProjectIdOrKey(),
		WorkspaceID:    req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.GetProjectStatusOrderResponse{StatusIdsByColumn: toProtoStatusOrder(order)}, nil
}

// ── Linear project/team group ────────────────────────────────────────────

func (s *Server) CreateProject(ctx context.Context, req *issuetrackingv1.CreateProjectRequest) (*issuetrackingv1.Project, error) {
	// CreateProjectRequest has no provider field (Linear-only channel today,
	// see TASK-106) — always resolves Linear.
	project, err := s.createProject.Execute(ctx, usecase.CreateProjectInput{
		Provider:    domain.ProviderLinear,
		TeamID:      req.GetTeamId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoProject(project), nil
}

func (s *Server) GetProject(ctx context.Context, req *issuetrackingv1.GetProjectRequest) (*issuetrackingv1.Project, error) {
	project, err := s.getProject.Execute(ctx, usecase.GetProjectInput{
		Provider:    domain.ProviderLinear,
		ProjectID:   req.GetProjectId(),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return toProtoProject(project), nil
}

func (s *Server) ListTeams(ctx context.Context, req *issuetrackingv1.ListTeamsRequest) (*issuetrackingv1.ListTeamsResponse, error) {
	teams, err := s.listTeams.Execute(ctx, usecase.ListTeamsInput{WorkspaceID: req.GetWorkspaceId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.Team, 0, len(teams))
	for _, t := range teams {
		out = append(out, &issuetrackingv1.Team{Id: t.ID, WorkspaceId: t.WorkspaceID, Name: t.Name, Key: t.Key})
	}
	return &issuetrackingv1.ListTeamsResponse{Teams: out}, nil
}

func (s *Server) ListTeamLabels(ctx context.Context, req *issuetrackingv1.ListTeamLabelsRequest) (*issuetrackingv1.ListTeamLabelsResponse, error) {
	labels, err := s.listTeamLabels.Execute(ctx, usecase.ListTeamLabelsInput{TeamID: req.GetTeamId(), WorkspaceID: req.GetWorkspaceId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.Label, 0, len(labels))
	for _, l := range labels {
		out = append(out, &issuetrackingv1.Label{Id: l.ID, Name: l.Name, Color: l.Color})
	}
	return &issuetrackingv1.ListTeamLabelsResponse{Labels: out}, nil
}

func (s *Server) ListTeamMembers(ctx context.Context, req *issuetrackingv1.ListTeamMembersRequest) (*issuetrackingv1.ListTeamMembersResponse, error) {
	members, err := s.listTeamMembers.Execute(ctx, usecase.ListTeamMembersInput{TeamID: req.GetTeamId(), WorkspaceID: req.GetWorkspaceId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.Member, 0, len(members))
	for _, m := range members {
		out = append(out, &issuetrackingv1.Member{Id: m.ID, DisplayName: m.DisplayName, AvatarUrl: m.AvatarURL})
	}
	return &issuetrackingv1.ListTeamMembersResponse{Members: out}, nil
}

func (s *Server) GetCustomView(ctx context.Context, req *issuetrackingv1.GetCustomViewRequest) (*issuetrackingv1.CustomView, error) {
	view, err := s.getCustomView.Execute(ctx, usecase.GetCustomViewInput{
		ViewID:      req.GetViewId(),
		Model:       req.GetModel(),
		WorkspaceID: req.GetWorkspaceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.CustomView{
		Id: view.ID, WorkspaceId: view.WorkspaceID, Name: view.Name, Model: view.Model, TeamId: view.TeamID,
	}, nil
}

func (s *Server) ListWorkflowStates(ctx context.Context, req *issuetrackingv1.ListWorkflowStatesRequest) (*issuetrackingv1.ListWorkflowStatesResponse, error) {
	states, err := s.listWorkflowStates.Execute(ctx, usecase.ListWorkflowStatesInput{TeamID: req.GetTeamId(), WorkspaceID: req.GetWorkspaceId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*issuetrackingv1.WorkflowState, 0, len(states))
	for _, st := range states {
		out = append(out, &issuetrackingv1.WorkflowState{Id: st.ID, Name: st.Name, Category: st.Category})
	}
	return &issuetrackingv1.ListWorkflowStatesResponse{States: out}, nil
}

// ── credentials.* group (TASK-041) ──────────────────────────────────────

func (s *Server) SetIntegrationCredential(ctx context.Context, req *issuetrackingv1.SetIntegrationCredentialRequest) (*issuetrackingv1.SetIntegrationCredentialResponse, error) {
	if err := s.setIntegrationCredential.Execute(ctx, usecase.SetIntegrationCredentialInput{
		TenantID:   req.GetTenantId(),
		Provider:   toDomainProvider(req.GetProvider()),
		Token:      req.GetToken(),
		ConfigJSON: req.GetConfigJson(),
	}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.SetIntegrationCredentialResponse{}, nil
}

func (s *Server) GetIntegrationCredentialStatus(ctx context.Context, req *issuetrackingv1.GetIntegrationCredentialStatusRequest) (*issuetrackingv1.GetIntegrationCredentialStatusResponse, error) {
	result, err := s.getIntegrationCredentialStatus.Execute(ctx, usecase.GetIntegrationCredentialStatusInput{
		TenantID: req.GetTenantId(),
		Provider: toDomainProvider(req.GetProvider()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.GetIntegrationCredentialStatusResponse{
		Configured: result.Configured,
		ConfigJson: result.ConfigJSON,
	}, nil
}

func (s *Server) ListIntegrationCredentials(ctx context.Context, req *issuetrackingv1.ListIntegrationCredentialsRequest) (*issuetrackingv1.ListIntegrationCredentialsResponse, error) {
	providers, err := s.listIntegrationCredentials.Execute(ctx, req.GetTenantId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]issuetrackingv1.IssueProvider, 0, len(providers))
	for _, p := range providers {
		out = append(out, toProtoProvider(p))
	}
	return &issuetrackingv1.ListIntegrationCredentialsResponse{ConfiguredProviders: out}, nil
}

func (s *Server) RevokeAuth(ctx context.Context, req *issuetrackingv1.RevokeAuthRequest) (*issuetrackingv1.RevokeAuthResponse, error) {
	if err := s.revokeAuth.Execute(ctx, usecase.RevokeAuthInput{
		TenantID: req.GetTenantId(),
		Provider: toDomainProvider(req.GetProvider()),
	}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &issuetrackingv1.RevokeAuthResponse{}, nil
}

// ── translation helpers ──────────────────────────────────────────────────

func toDomainProvider(p issuetrackingv1.IssueProvider) domain.Provider {
	switch p {
	case issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA:
		return domain.ProviderJira
	case issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR:
		return domain.ProviderLinear
	default:
		return ""
	}
}

// toProtoProvider is toDomainProvider's inverse — ListIntegrationCredentials'
// response mapping.
func toProtoProvider(p domain.Provider) issuetrackingv1.IssueProvider {
	switch p {
	case domain.ProviderJira:
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA
	case domain.ProviderLinear:
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_LINEAR
	default:
		return issuetrackingv1.IssueProvider_ISSUE_PROVIDER_UNSPECIFIED
	}
}

func toProtoIssue(i domain.Issue) *issuetrackingv1.Issue {
	out := &issuetrackingv1.Issue{
		Id:                  i.ID,
		ProviderIssueId:     i.ProviderIssueID,
		Key:                 i.Key,
		Title:               i.Title,
		DescriptionMarkdown: i.DescriptionMarkdown,
		State:               i.State,
		Url:                 i.URL,
		Labels:              i.Labels,
		CustomFieldsJson:    i.CustomFieldsJSON,
	}
	if i.WorkflowState.ID != "" || i.WorkflowState.Name != "" {
		out.WorkflowState = &issuetrackingv1.WorkflowState{Id: i.WorkflowState.ID, Name: i.WorkflowState.Name, Category: i.WorkflowState.Category}
	}
	if i.Project.ID != "" || i.Project.Key != "" || i.Project.Name != "" {
		out.Project = &issuetrackingv1.Project{Id: i.Project.ID, Key: i.Project.Key, Name: i.Project.Name, WorkspaceId: i.Project.WorkspaceID}
	}
	if i.IssueType.ID != "" || i.IssueType.Name != "" {
		out.IssueType = &issuetrackingv1.IssueType{Id: i.IssueType.ID, Name: i.IssueType.Name, Subtask: i.IssueType.Subtask}
	}
	if i.Assignee.ID != "" {
		out.Assignee = &issuetrackingv1.UserRef{Id: i.Assignee.ID, DisplayName: i.Assignee.DisplayName, Email: i.Assignee.Email, AvatarUrl: i.Assignee.AvatarURL}
	}
	if i.Reporter.ID != "" {
		out.Reporter = &issuetrackingv1.UserRef{Id: i.Reporter.ID, DisplayName: i.Reporter.DisplayName, Email: i.Reporter.Email, AvatarUrl: i.Reporter.AvatarURL}
	}
	if i.Priority.ID != "" || i.Priority.Name != "" {
		out.Priority = &issuetrackingv1.Priority{Id: i.Priority.ID, Name: i.Priority.Name}
	}
	return out
}

func toProtoComment(c domain.IssueComment) *issuetrackingv1.IssueComment {
	out := &issuetrackingv1.IssueComment{Id: c.ID, BodyMarkdown: c.BodyMarkdown}
	if c.Author.ID != "" {
		out.Author = &issuetrackingv1.UserRef{Id: c.Author.ID, DisplayName: c.Author.DisplayName, Email: c.Author.Email, AvatarUrl: c.Author.AvatarURL}
	}
	return out
}

func toProtoConnectionStatus(s domain.ConnectionStatus) *issuetrackingv1.ConnectionStatus {
	workspaces := make([]*issuetrackingv1.Workspace, 0, len(s.Workspaces))
	for _, w := range s.Workspaces {
		workspaces = append(workspaces, &issuetrackingv1.Workspace{Id: w.ID, Name: w.Name, Url: w.URL})
	}
	return &issuetrackingv1.ConnectionStatus{
		Connected:           s.Connected,
		ViewerId:            s.Viewer.ID,
		ViewerDisplayName:   s.Viewer.DisplayName,
		ViewerEmail:         s.Viewer.Email,
		Workspaces:          workspaces,
		ActiveWorkspaceId:   s.ActiveWorkspaceID,
		SelectedWorkspaceId: s.SelectedWorkspaceID,
		CredentialError:     s.CredentialError,
	}
}

func toProtoProjects(projects []domain.ProjectRef) []*issuetrackingv1.Project {
	out := make([]*issuetrackingv1.Project, 0, len(projects))
	for _, p := range projects {
		out = append(out, toProtoProject(p))
	}
	return out
}

func toProtoProject(p domain.ProjectRef) *issuetrackingv1.Project {
	return &issuetrackingv1.Project{Id: p.ID, Key: p.Key, Name: p.Name, WorkspaceId: p.WorkspaceID}
}

func toProtoUsers(users []domain.UserRef) []*issuetrackingv1.UserRef {
	out := make([]*issuetrackingv1.UserRef, 0, len(users))
	for _, u := range users {
		out = append(out, &issuetrackingv1.UserRef{Id: u.ID, DisplayName: u.DisplayName, Email: u.Email, AvatarUrl: u.AvatarURL})
	}
	return out
}

func toProtoPriorities(priorities []domain.PriorityRef) []*issuetrackingv1.Priority {
	out := make([]*issuetrackingv1.Priority, 0, len(priorities))
	for _, p := range priorities {
		out = append(out, &issuetrackingv1.Priority{Id: p.ID, Name: p.Name})
	}
	return out
}

func toProtoTransitions(transitions []domain.Transition) []*issuetrackingv1.Transition {
	out := make([]*issuetrackingv1.Transition, 0, len(transitions))
	for _, t := range transitions {
		out = append(out, &issuetrackingv1.Transition{
			Id: t.ID, Name: t.Name,
			To: &issuetrackingv1.WorkflowState{Id: t.To.ID, Name: t.To.Name, Category: t.To.Category},
		})
	}
	return out
}

func toProtoStatusOrder(order domain.ProjectStatusOrder) []*issuetrackingv1.StatusIDList {
	out := make([]*issuetrackingv1.StatusIDList, 0, len(order.StatusIDsByColumn))
	for _, col := range order.StatusIDsByColumn {
		out = append(out, &issuetrackingv1.StatusIDList{StatusIds: col})
	}
	return out
}

func toProtoCreateFields(fields []domain.CreateField) []*issuetrackingv1.CreateField {
	out := make([]*issuetrackingv1.CreateField, 0, len(fields))
	for _, f := range fields {
		cf := &issuetrackingv1.CreateField{
			Key: f.Key, Name: f.Name, Required: f.Required,
			SchemaType: f.SchemaType, SchemaItems: f.SchemaItems, SchemaCustom: f.SchemaCustom,
		}
		if len(f.AllowedValues) > 0 {
			cf.AllowedValuesJson = marshalAllowedValues(f.AllowedValues)
		}
		out = append(out, cf)
	}
	return out
}

// marshalAllowedValues encodes a CreateField's allowed-value options as a
// JSON array — CreateField.allowed_values_json is deliberately a raw JSON
// string (heterogeneous per-field shape, not worth a dedicated message per
// TASK-096's proto comment). A marshal failure here is unreachable for
// this well-typed input, so it degrades to an empty array rather than
// propagating an error through every metadata-lookup call site.
func marshalAllowedValues(values []domain.CreateFieldOption) string {
	b, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(b)
}
