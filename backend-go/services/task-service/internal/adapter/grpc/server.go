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

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// Server implements taskv1.UnimplementedTaskServiceServer.
type Server struct {
	taskv1.UnimplementedTaskServiceServer

	createTask          *usecase.CreateTask
	getTask             *usecase.GetTask
	txRunner            usecase.TxRunner
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
	recalculateProgress *usecase.RecalculateProgress
	getSubtree          *usecase.GetSubtree
	generateAgentPrompt *usecase.GenerateAgentPrompt
	revokeGrant         *usecase.RevokeGrant
	listGrants          *usecase.ListGrants
	generateShareLink   *usecase.GenerateShareLink
	getTaskByShareToken *usecase.GetTaskByShareToken
}

func New(
	createTask *usecase.CreateTask,
	getTask *usecase.GetTask,
	txRunner usecase.TxRunner,
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
	recalculateProgress *usecase.RecalculateProgress,
	getSubtree *usecase.GetSubtree,
	generateAgentPrompt *usecase.GenerateAgentPrompt,
	revokeGrant *usecase.RevokeGrant,
	listGrants *usecase.ListGrants,
	generateShareLink *usecase.GenerateShareLink,
	getTaskByShareToken *usecase.GetTaskByShareToken,
) *Server {
	return &Server{
		createTask:          createTask,
		getTask:             getTask,
		txRunner:            txRunner,
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
		recalculateProgress: recalculateProgress,
		getSubtree:          getSubtree,
		generateAgentPrompt: generateAgentPrompt,
		revokeGrant:         revokeGrant,
		listGrants:          listGrants,
		generateShareLink:   generateShareLink,
		getTaskByShareToken: getTaskByShareToken,
	}
}

func (s *Server) CreateTask(ctx context.Context, req *taskv1.CreateTaskRequest) (*taskv1.CreateTaskResponse, error) {
	// No ID field on CreateTaskRequest — the usecase assigns one (uuid) when
	// Input.ID is left empty.
	task, err := s.createTask.Execute(ctx, usecase.CreateTaskInput{
		Title:     req.GetTitle(),
		ParentID:  req.GetParentId(),
		ProjectID: req.GetProjectId(),
		CreatorID: req.GetCreatorId(),
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

// AddEdge wraps the cycle-check + edge-write + auto-block sequence in one
// Postgres transaction via TxRunner — closing add_edge.go's previously
// admitted check-then-write race (see AddEdge usecase's doc comment) rather
// than constructing a single pool-scoped *usecase.AddEdge once at
// server-startup wiring time.
func (s *Server) AddEdge(ctx context.Context, req *taskv1.AddEdgeRequest) (*taskv1.AddEdgeResponse, error) {
	err := s.txRunner.RunInTx(ctx, func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error {
		_, err := usecase.NewAddEdge(tasks, edges).Execute(ctx, usecase.AddEdgeInput{
			FromTaskID: req.GetFromTaskId(),
			ToTaskID:   req.GetToTaskId(),
			Kind:       toDomainEdgeKind(req.GetType()),
		})
		return err
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
	err := s.grant.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GrantResponse{}, nil
}

func (s *Server) RevokeGrant(ctx context.Context, req *taskv1.RevokeGrantRequest) (*emptypb.Empty, error) {
	err := s.revokeGrant.Execute(ctx, usecase.RevokeGrantInput{
		TaskID:    req.GetTaskId(),
		SubjectID: req.GetSubjectId(),
		Level:     toDomainGrantLevel(req.GetLevel()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListGrants(ctx context.Context, req *taskv1.ListGrantsRequest) (*taskv1.ListGrantsResponse, error) {
	grants, err := s.listGrants.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.GrantView, 0, len(grants))
	for _, g := range grants {
		view := &taskv1.GrantView{SubjectId: g.SubjectID, Level: toProtoGrantLevel(g.Level), ApplyTree: g.ApplyTree}
		if g.ExpiresAt != nil {
			view.ExpiresAt = timestamppb.New(*g.ExpiresAt)
		}
		out = append(out, view)
	}
	return &taskv1.ListGrantsResponse{Grants: out}, nil
}

func (s *Server) ResolvePermission(ctx context.Context, req *taskv1.ResolvePermissionRequest) (*taskv1.ResolvePermissionResponse, error) {
	// action defaults to "read" when empty — either an older client built
	// against a pre-TASK-TG-003-06 proto that has no action field to send,
	// or a caller that legitimately wants the default. The usecase's own
	// input contract stays strict ("empty means empty"); this adapter layer
	// absorbs the rollout-compatibility shim, matching this codebase's
	// existing adapter-absorbs-wire-quirks convention.
	action := req.GetAction()
	if action == "" {
		action = "read"
	}
	level, err := s.resolvePermission.Execute(ctx, usecase.ResolvePermissionInput{
		TaskID: req.GetTaskId(),
		UserID: req.GetUserId(),
		Action: action,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.ResolvePermissionResponse{EffectiveLevel: toProtoGrantLevel(level)}, nil
}

func (s *Server) Execute(ctx context.Context, req *taskv1.TaskServiceExecuteRequest) (*taskv1.TaskServiceExecuteResponse, error) {
	ref, err := s.executeTask.Execute(ctx, usecase.ExecuteTaskInput{
		TaskID:    req.GetTaskId(),
		RequestID: req.GetRequestId(),
		Prompt:    req.GetPrompt(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.TaskServiceExecuteResponse{ExecutionRef: ref}, nil
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
		v := domain.Status(req.GetStatus().GetValue())
		in.Status = &v
	}
	if req.GetWorkflowTemplateId() != nil {
		v := req.GetWorkflowTemplateId().GetValue()
		in.WorkflowTemplateID = &v
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
	proposals, err := s.aiDecompose.Execute(ctx, usecase.AIDecomposeInput{TaskID: req.GetTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AIDecomposeResponse{Proposals: toProtoSubtaskProposals(proposals)}, nil
}

func (s *Server) AIApply(ctx context.Context, req *taskv1.AIApplyRequest) (*taskv1.AIApplyResponse, error) {
	created, err := s.aiApply.Execute(ctx, usecase.AIApplyInput{
		TaskID:    req.GetTaskId(),
		Proposals: toDomainSubtaskProposals(req.GetProposals()),
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

func (s *Server) RecalculateProgress(ctx context.Context, req *taskv1.RecalculateProgressRequest) (*taskv1.RecalculateProgressResponse, error) {
	if err := s.recalculateProgress.Execute(ctx, req.GetTaskId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.RecalculateProgressResponse{}, nil
}

func (s *Server) GetSubtree(ctx context.Context, req *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error) {
	tasks, err := s.getSubtree.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.GetSubtreeResponse{Tasks: out}, nil
}

func (s *Server) GenerateAgentPrompt(ctx context.Context, req *taskv1.GenerateAgentPromptRequest) (*taskv1.GenerateAgentPromptResponse, error) {
	result, err := s.generateAgentPrompt.Execute(ctx, usecase.GenerateAgentPromptInput{TaskID: req.GetTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GenerateAgentPromptResponse{PromptTemplate: result}, nil
}

// GenerateShareLink requires the caller to already hold admin-level
// permission (enforced inside the usecase, not here) — TASK-TG-003-05.
func (s *Server) GenerateShareLink(ctx context.Context, req *taskv1.GenerateShareLinkRequest) (*taskv1.GenerateShareLinkResponse, error) {
	token, err := s.generateShareLink.Execute(ctx, usecase.GenerateShareLinkInput{
		TaskID: req.GetTaskId(),
		UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GenerateShareLinkResponse{ShareToken: token}, nil
}

// GetTaskByShareToken is task-service's first unauthenticated RPC —
// TASK-TG-003-05, SECURITY REVIEW REQUIRED before merge. Deliberately does
// NOT call anything from common/tenant — see usecase.GetTaskByShareToken's
// doc comment. Confirmed reachable without any special allowlisting:
// common/grpcmw.ChainUnary's TenantExtractionInterceptor
// (common/grpcmw/grpcmw.go:50-66) only OPTIONALLY populates tenant/user
// context from incoming metadata when present — it never rejects a call
// for missing metadata, so there is no default-deny gate at the gRPC layer
// for ANY RPC in this service today; the actual "auth" enforcement is
// exclusively each usecase's own tenant.RequireTenantID call. Since this
// usecase deliberately never calls that, it's reachable with zero metadata
// exactly like every other RPC's transport-level reachability — the only
// thing making it meaningfully different is that it's SAFE to reach that
// way, unlike every other RPC. Whether task-service's gRPC port itself is
// reachable from outside the internal mesh (bypassing api-gateway's own
// auth) is a deployment-topology question the security reviewer should
// confirm explicitly, not something resolved by this handler.
func (s *Server) GetTaskByShareToken(ctx context.Context, req *taskv1.GetTaskByShareTokenRequest) (*taskv1.GetTaskByShareTokenResponse, error) {
	view, err := s.getTaskByShareToken.Execute(ctx, req.GetShareToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GetTaskByShareTokenResponse{Task: &taskv1.TaskShareView{
		Id:          view.ID,
		Title:       view.Title,
		Status:      view.Status,
		Description: view.Description,
	}}, nil
}

func toProtoSubtaskProposals(proposals []domain.SubtaskProposal) []*taskv1.SubtaskProposal {
	out := make([]*taskv1.SubtaskProposal, 0, len(proposals))
	for _, p := range proposals {
		dependsOn := make([]int32, len(p.DependsOnIndex))
		for i, idx := range p.DependsOnIndex {
			dependsOn[i] = int32(idx)
		}
		proto := &taskv1.SubtaskProposal{
			Title: p.Title, Description: p.Description, Type: p.Type,
			DependsOnIndex: dependsOn, PromptTemplate: p.PromptTemplate,
		}
		if p.EstimatedHours != nil {
			proto.EstimatedHours = *p.EstimatedHours
			proto.HasEstimatedHours = true
		}
		out = append(out, proto)
	}
	return out
}

func toDomainSubtaskProposals(proposals []*taskv1.SubtaskProposal) []domain.SubtaskProposal {
	out := make([]domain.SubtaskProposal, 0, len(proposals))
	for _, p := range proposals {
		dependsOn := make([]int, len(p.GetDependsOnIndex()))
		for i, idx := range p.GetDependsOnIndex() {
			dependsOn[i] = int(idx)
		}
		proposal := domain.SubtaskProposal{
			Title: p.GetTitle(), Description: p.GetDescription(), Type: p.GetType(),
			DependsOnIndex: dependsOn, PromptTemplate: p.GetPromptTemplate(),
		}
		if p.GetHasEstimatedHours() {
			v := p.GetEstimatedHours()
			proposal.EstimatedHours = &v
		}
		out = append(out, proposal)
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
		Id:                 t.ID,
		TenantId:           t.TenantID,
		Title:              t.Title,
		Status:             string(t.Status),
		ParentId:           t.ParentID,
		ProjectId:          t.ProjectID,
		WorkflowTemplateId: t.WorkflowTemplateID,
		Description:        t.Description,
		Type:               t.Type,
		Priority:           t.Priority,
		Labels:             t.Labels,
		AssigneeId:         stringPtrValue(t.AssigneeID),
		ReporterId:         stringPtrValue(t.ReporterID),
		OwnerId:            stringPtrValue(t.OwnerID),
		DueDate:            timePtrToProto(t.DueDate),
		EstimatedHours:     float64PtrValue(t.EstimatedHours),
		ActualHours:        float64PtrValue(t.ActualHours),
		PromptTemplate:     t.PromptTemplate,
		AiContext:          string(t.AIContext),
		AiPlanJson:         string(t.AIPlanJSON),
		Visibility:         t.Visibility,
		WorktreeId:         stringPtrValue(t.WorktreeID),
		AgentSessionId:     stringPtrValue(t.AgentSessionID),
		WorkflowExecId:     stringPtrValue(t.WorkflowExecID),
		DoneSubtasks:       int32(t.DoneSubtasks),
		TotalSubtasks:      int32(t.TotalSubtasks),
		ShareToken:         stringPtrValue(t.ShareToken),
	}
}

// stringPtrValue/float64PtrValue/timePtrToProto convert domain.Task's
// nullable pointer-backed fields to the plain-scalar wire representation
// task.proto's Task message uses (matching Status's own untyped-string wire
// convention) — a nil pointer maps to the type's zero value on the wire,
// same "unset == zero value" tradeoff BE-SOL-001's own sketch accepts for
// this message rather than introducing wrapper types for every optional
// field.
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func float64PtrValue(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func timePtrToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
