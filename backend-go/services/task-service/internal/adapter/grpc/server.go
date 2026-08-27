// Package grpc implements the generated taskv1.TaskServiceServer interface
// by translating wire messages to/from usecase calls — no business logic
// here, per specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// Server implements taskv1.UnimplementedTaskServiceServer.
type Server struct {
	taskv1.UnimplementedTaskServiceServer

	createTask          *usecase.CreateTask
	getTask             *usecase.GetTask
	addEdge             *usecase.AddEdge
	grant               *usecase.Grant
	resolvePermission   *usecase.ResolvePermission
	executeTask         *usecase.ExecuteTask
	hasActiveExecutions *usecase.HasActiveExecutions
	listTasks           *usecase.ListTasks
	updateTask          *usecase.UpdateTask
	deleteTask          *usecase.DeleteTask
	getDependencies     *usecase.GetDependencies
	aiDecompose         *usecase.AIDecompose
	aiApply             *usecase.AIApply
	generateAgentPrompt *usecase.GenerateAgentPrompt
	revokeGrant         *usecase.RevokeGrant
	listGrants          *usecase.ListGrants
	createPublicLink    *usecase.CreatePublicLink
	revokePublicLink    *usecase.RevokePublicLink
	resolvePublicLink   *usecase.ResolvePublicLink
	getSubtree          *usecase.GetSubtree
	recalculateProgress *usecase.RecalculateProgress
	addComment          *usecase.AddComment
	listComments        *usecase.ListComments
}

func New(
	createTask *usecase.CreateTask,
	getTask *usecase.GetTask,
	addEdge *usecase.AddEdge,
	grant *usecase.Grant,
	resolvePermission *usecase.ResolvePermission,
	executeTask *usecase.ExecuteTask,
	hasActiveExecutions *usecase.HasActiveExecutions,
	listTasks *usecase.ListTasks,
	updateTask *usecase.UpdateTask,
	deleteTask *usecase.DeleteTask,
	getDependencies *usecase.GetDependencies,
	aiDecompose *usecase.AIDecompose,
	aiApply *usecase.AIApply,
	generateAgentPrompt *usecase.GenerateAgentPrompt,
	revokeGrant *usecase.RevokeGrant,
	listGrants *usecase.ListGrants,
	createPublicLink *usecase.CreatePublicLink,
	revokePublicLink *usecase.RevokePublicLink,
	resolvePublicLink *usecase.ResolvePublicLink,
	getSubtree *usecase.GetSubtree,
	recalculateProgress *usecase.RecalculateProgress,
	addComment *usecase.AddComment,
	listComments *usecase.ListComments,
) *Server {
	return &Server{
		createTask:          createTask,
		getTask:             getTask,
		addEdge:             addEdge,
		grant:               grant,
		resolvePermission:   resolvePermission,
		executeTask:         executeTask,
		hasActiveExecutions: hasActiveExecutions,
		listTasks:           listTasks,
		updateTask:          updateTask,
		deleteTask:          deleteTask,
		getDependencies:     getDependencies,
		aiDecompose:         aiDecompose,
		aiApply:             aiApply,
		generateAgentPrompt: generateAgentPrompt,
		revokeGrant:         revokeGrant,
		listGrants:          listGrants,
		createPublicLink:    createPublicLink,
		revokePublicLink:    revokePublicLink,
		resolvePublicLink:   resolvePublicLink,
		getSubtree:          getSubtree,
		recalculateProgress: recalculateProgress,
		addComment:          addComment,
		listComments:        listComments,
	}
}

func (s *Server) GenerateAgentPrompt(ctx context.Context, req *taskv1.GenerateAgentPromptRequest) (*taskv1.GenerateAgentPromptResponse, error) {
	prompt, err := s.generateAgentPrompt.Execute(ctx, usecase.GenerateAgentPromptInput{TaskID: req.GetTaskId(), Save: req.GetSave()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GenerateAgentPromptResponse{Prompt: prompt}, nil
}

func (s *Server) CreateTask(ctx context.Context, req *taskv1.CreateTaskRequest) (*taskv1.CreateTaskResponse, error) {
	// No ID field on CreateTaskRequest — the usecase assigns one (uuid) when
	// Input.ID is left empty.
	task, err := s.createTask.Execute(ctx, usecase.CreateTaskInput{
		Title:     req.GetTitle(),
		ParentID:  req.GetParentId(),
		ProjectID: req.GetProjectId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.CreateTaskResponse{Task: toProtoTask(task)}, nil
}

func (s *Server) GetTask(ctx context.Context, req *taskv1.GetTaskRequest) (*taskv1.GetTaskResponse, error) {
	task, err := s.getTask.Execute(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GetTaskResponse{Task: toProtoTask(task)}, nil
}

func (s *Server) AddEdge(ctx context.Context, req *taskv1.AddEdgeRequest) (*taskv1.AddEdgeResponse, error) {
	_, err := s.addEdge.Execute(ctx, usecase.AddEdgeInput{
		FromTaskID: req.GetFromTaskId(),
		ToTaskID:   req.GetToTaskId(),
		Kind:       toDomainEdgeKind(req.GetType()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AddEdgeResponse{}, nil
}

func (s *Server) Grant(ctx context.Context, req *taskv1.GrantRequest) (*taskv1.GrantResponse, error) {
	in := usecase.GrantInput{
		TaskID:    req.GetTaskId(),
		SubjectID: req.GetSubjectId(),
		Level:     toDomainGrantLevel(req.GetLevel()),
		ApplyTree: req.GetApplyTree(),
	}
	if req.GetExpiresAt() != nil {
		t := req.GetExpiresAt().AsTime()
		in.ExpiresAt = &t
	}
	id, err := s.grant.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GrantResponse{Id: id}, nil
}

func (s *Server) RevokeGrant(ctx context.Context, req *taskv1.RevokeGrantRequest) (*emptypb.Empty, error) {
	if err := s.revokeGrant.Execute(ctx, usecase.RevokeGrantInput{TaskID: req.GetTaskId(), GrantID: req.GetGrantId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListGrants(ctx context.Context, req *taskv1.ListGrantsRequest) (*taskv1.ListGrantsResponse, error) {
	grants, err := s.listGrants.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Grant, 0, len(grants))
	for _, g := range grants {
		pg := &taskv1.Grant{Id: g.ID, TaskId: g.TaskID, SubjectId: g.SubjectID, Level: toProtoGrantLevel(g.Level), ApplyTree: g.ApplyTree}
		if g.ExpiresAt != nil {
			pg.ExpiresAt = timestamppb.New(*g.ExpiresAt)
		}
		out = append(out, pg)
	}
	return &taskv1.ListGrantsResponse{Grants: out}, nil
}

func (s *Server) CreatePublicLink(ctx context.Context, req *taskv1.CreatePublicLinkRequest) (*taskv1.CreatePublicLinkResponse, error) {
	id, token, err := s.createPublicLink.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.CreatePublicLinkResponse{Id: id, Token: token}, nil
}

func (s *Server) RevokePublicLink(ctx context.Context, req *taskv1.RevokePublicLinkRequest) (*emptypb.Empty, error) {
	if err := s.revokePublicLink.Execute(ctx, req.GetId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// ResolvePublicLink is the one RPC in this service meaningfully callable
// without a JWT (spec: "anonymous read-only access via a random token") —
// see TASK-TG-03-08's Context section for why api-gateway is NOT yet wired
// to expose this to a browser (a new unauthenticated-route trust boundary,
// out of scope here) and why tenantID below still comes from
// tenant.RequireTenantID(ctx) rather than the wire request (which has no
// tenant_id field) in the meantime.
func (s *Server) ResolvePublicLink(ctx context.Context, req *taskv1.ResolvePublicLinkRequest) (*taskv1.ResolvePublicLinkResponse, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err))
	}
	taskID, err := s.resolvePublicLink.Execute(ctx, tenantID, req.GetToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.ResolvePublicLinkResponse{TaskId: taskID}, nil
}

func (s *Server) ResolvePermission(ctx context.Context, req *taskv1.ResolvePermissionRequest) (*taskv1.ResolvePermissionResponse, error) {
	level, err := s.resolvePermission.Execute(ctx, usecase.ResolvePermissionInput{
		TaskID: req.GetTaskId(),
		UserID: req.GetUserId(),
		Action: req.GetAction(), // real field now (TASK-TG-03-04/03-06) — closes README.md's "not reachable through the RPC surface yet" gap
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.ResolvePermissionResponse{EffectiveLevel: toProtoGrantLevel(level)}, nil
}

func (s *Server) Execute(ctx context.Context, req *taskv1.TaskServiceExecuteRequest) (*taskv1.TaskServiceExecuteResponse, error) {
	result, err := s.executeTask.Execute(ctx, usecase.ExecuteTaskInput{
		TaskID:    req.GetTaskId(),
		RequestID: req.GetRequestId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.TaskServiceExecuteResponse{ExecutionRef: result.ExecutionRef, Async: result.Async}, nil
}

func (s *Server) HasActiveExecutions(ctx context.Context, req *taskv1.HasActiveExecutionsRequest) (*taskv1.HasActiveExecutionsResponse, error) {
	hasActive, err := s.hasActiveExecutions.Execute(ctx, usecase.HasActiveExecutionsInput{ProjectID: req.GetProjectId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.HasActiveExecutionsResponse{HasActive: hasActive}, nil
}

func (s *Server) ListTasks(ctx context.Context, req *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
	result, err := s.listTasks.Execute(ctx, usecase.ListTasksInput{
		ProjectID: req.GetProjectId(),
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.ListTasksResponse{Tasks: out, NextPageToken: result.NextPageToken}, nil
}

func (s *Server) UpdateTask(ctx context.Context, req *taskv1.UpdateTaskRequest) (*taskv1.UpdateTaskResponse, error) {
	in := usecase.UpdateTaskInput{ID: req.GetId()}
	if req.GetTitle() != nil {
		v := req.GetTitle().GetValue()
		in.Title = &v
	}
	if req.GetStatus() != nil {
		v := req.GetStatus().GetValue()
		in.Status = &v
	}
	task, err := s.updateTask.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.UpdateTaskResponse{Task: toProtoTask(task)}, nil
}

func (s *Server) DeleteTask(ctx context.Context, req *taskv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := s.deleteTask.Execute(ctx, usecase.DeleteTaskInput{ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetDependencies(ctx context.Context, req *taskv1.GetDependenciesRequest) (*taskv1.GetDependenciesResponse, error) {
	deps, err := s.getDependencies.Execute(ctx, usecase.GetDependenciesInput{TaskID: req.GetTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(deps))
	for _, t := range deps {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.GetDependenciesResponse{Dependencies: out}, nil
}

func (s *Server) AIDecompose(ctx context.Context, req *taskv1.AIDecomposeRequest) (*taskv1.AIDecomposeResponse, error) {
	result, err := s.aiDecompose.Execute(ctx, usecase.AIDecomposeInput{TaskID: req.GetTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AIDecomposeResponse{
		Proposals:   toProtoSubtaskProposals(result.Proposals),
		RawResponse: result.RawResponse,
	}, nil
}

func (s *Server) AIApply(ctx context.Context, req *taskv1.AIApplyRequest) (*taskv1.AIApplyResponse, error) {
	created, err := s.aiApply.Execute(ctx, usecase.AIApplyInput{
		TaskID:        req.GetTaskId(),
		Proposals:     toDomainSubtaskProposals(req.GetProposals()),
		RawAIResponse: req.GetRawAiResponse(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(created))
	for _, t := range created {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.AIApplyResponse{CreatedSubtasks: out}, nil
}

func (s *Server) GetSubtree(ctx context.Context, req *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error) {
	result, err := s.getSubtree.Execute(ctx, usecase.GetSubtreeInput{RootID: req.GetRootId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		out = append(out, toProtoTask(t))
	}
	edges := make([]*taskv1.AddEdgeRequest, 0, len(result.DependsOnEdges))
	for _, e := range result.DependsOnEdges {
		edges = append(edges, &taskv1.AddEdgeRequest{FromTaskId: e.FromTaskID, ToTaskId: e.ToTaskID, Type: taskv1.EdgeType_EDGE_TYPE_DEPENDS_ON})
	}
	return &taskv1.GetSubtreeResponse{Tasks: out, DependsOnEdges: edges}, nil
}

func (s *Server) RecalculateProgress(ctx context.Context, req *taskv1.RecalculateProgressRequest) (*taskv1.RecalculateProgressResponse, error) {
	p, err := s.recalculateProgress.Execute(ctx, req.GetRootId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.RecalculateProgressResponse{ProgressPercent: int32(p)}, nil
}

func (s *Server) AddComment(ctx context.Context, req *taskv1.AddCommentRequest) (*taskv1.AddCommentResponse, error) {
	c, err := s.addComment.Execute(ctx, req.GetTaskId(), req.GetContent())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AddCommentResponse{Id: c.ID, AuthorId: c.AuthorID, Content: c.Content, CreatedAt: c.CreatedAt.Format(time.RFC3339)}, nil
}

func (s *Server) ListComments(ctx context.Context, req *taskv1.ListCommentsRequest) (*taskv1.ListCommentsResponse, error) {
	comments, next, err := s.listComments.Execute(ctx, req.GetTaskId(), req.GetPageToken(), req.GetPageSize())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.AddCommentResponse, 0, len(comments))
	for _, c := range comments {
		out = append(out, &taskv1.AddCommentResponse{Id: c.ID, AuthorId: c.AuthorID, Content: c.Content, CreatedAt: c.CreatedAt.Format(time.RFC3339)})
	}
	return &taskv1.ListCommentsResponse{Comments: out, NextPageToken: next}, nil
}

func toProtoSubtaskProposals(proposals []domain.SubtaskProposal) []*taskv1.SubtaskProposal {
	out := make([]*taskv1.SubtaskProposal, 0, len(proposals))
	for _, p := range proposals {
		wire := &taskv1.SubtaskProposal{
			Title: p.Title, Description: p.Description, Type: p.Type,
			DependsOnIndices: toInt32Slice(p.DependsOnIndices), PromptTemplate: p.PromptTemplate,
		}
		if p.EstimatedHours != nil {
			wire.EstimatedHours = wrapperspb.Double(*p.EstimatedHours)
		}
		out = append(out, wire)
	}
	return out
}

func toDomainSubtaskProposals(proposals []*taskv1.SubtaskProposal) []domain.SubtaskProposal {
	out := make([]domain.SubtaskProposal, 0, len(proposals))
	for _, p := range proposals {
		proposal := domain.SubtaskProposal{
			Title: p.GetTitle(), Description: p.GetDescription(), Type: p.GetType(),
			DependsOnIndices: toIntSlice(p.GetDependsOnIndices()), PromptTemplate: p.GetPromptTemplate(),
		}
		if p.GetEstimatedHours() != nil {
			v := p.GetEstimatedHours().GetValue()
			proposal.EstimatedHours = &v
		}
		out = append(out, proposal)
	}
	return out
}

func toInt32Slice(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}

func toIntSlice(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

func toDomainEdgeKind(t taskv1.EdgeType) domain.EdgeKind {
	switch t {
	case taskv1.EdgeType_EDGE_TYPE_PARENT_CHILD:
		return domain.EdgeKindParentChild
	case taskv1.EdgeType_EDGE_TYPE_DEPENDS_ON:
		return domain.EdgeKindDependsOn
	default:
		return ""
	}
}

func toDomainGrantLevel(l taskv1.GrantLevel) domain.GrantLevel {
	switch l {
	case taskv1.GrantLevel_GRANT_LEVEL_OWNER:
		return domain.GrantLevelOwner
	case taskv1.GrantLevel_GRANT_LEVEL_ADMIN:
		return domain.GrantLevelAdmin
	case taskv1.GrantLevel_GRANT_LEVEL_USER:
		return domain.GrantLevelUser
	case taskv1.GrantLevel_GRANT_LEVEL_TEAM:
		return domain.GrantLevelTeam
	case taskv1.GrantLevel_GRANT_LEVEL_COMPANY:
		return domain.GrantLevelCompany
	default:
		return domain.GrantLevelUnspecified
	}
}

func toProtoGrantLevel(l domain.GrantLevel) taskv1.GrantLevel {
	switch l {
	case domain.GrantLevelOwner:
		return taskv1.GrantLevel_GRANT_LEVEL_OWNER
	case domain.GrantLevelAdmin:
		return taskv1.GrantLevel_GRANT_LEVEL_ADMIN
	case domain.GrantLevelUser:
		return taskv1.GrantLevel_GRANT_LEVEL_USER
	case domain.GrantLevelTeam:
		return taskv1.GrantLevel_GRANT_LEVEL_TEAM
	case domain.GrantLevelCompany:
		return taskv1.GrantLevel_GRANT_LEVEL_COMPANY
	default:
		return taskv1.GrantLevel_GRANT_LEVEL_UNSPECIFIED
	}
}

func toProtoTask(t domain.Task) *taskv1.Task {
	return &taskv1.Task{
		Id:        t.ID,
		TenantId:  t.TenantID,
		Title:     t.Title,
		Status:    t.Status,
		ParentId:  t.ParentID,
		ProjectId: t.ProjectID,
	}
}
