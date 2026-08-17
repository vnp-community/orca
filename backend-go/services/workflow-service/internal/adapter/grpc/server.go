// Package grpc implements the generated workflowv1.WorkflowServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// Server implements workflowv1.UnimplementedWorkflowServiceServer.
type Server struct {
	workflowv1.UnimplementedWorkflowServiceServer

	createTemplate   *usecase.CreateTemplate
	execute          *usecase.Execute
	getExecution     *usecase.GetExecution
	pauseExecution   *usecase.PauseExecution
	resumeExecution  *usecase.ResumeExecution
	executeAdHocStep *usecase.ExecuteAdHocStep
}

func New(
	createTemplate *usecase.CreateTemplate,
	execute *usecase.Execute,
	getExecution *usecase.GetExecution,
	pauseExecution *usecase.PauseExecution,
	resumeExecution *usecase.ResumeExecution,
	executeAdHocStep *usecase.ExecuteAdHocStep,
) *Server {
	return &Server{
		createTemplate:   createTemplate,
		execute:          execute,
		getExecution:     getExecution,
		pauseExecution:   pauseExecution,
		resumeExecution:  resumeExecution,
		executeAdHocStep: executeAdHocStep,
	}
}

func (s *Server) CreateTemplate(ctx context.Context, req *workflowv1.CreateTemplateRequest) (*workflowv1.CreateTemplateResponse, error) {
	tmpl, err := s.createTemplate.Execute(ctx, usecase.CreateTemplateInput{
		Name:    req.GetName(),
		DAGJSON: req.GetDagJson(),
		Scope:   req.GetScope(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.CreateTemplateResponse{Template: toProtoTemplate(tmpl)}, nil
}

func (s *Server) Execute(ctx context.Context, req *workflowv1.ExecuteRequest) (*workflowv1.ExecuteResponse, error) {
	exec, err := s.execute.Execute(ctx, usecase.ExecuteInput{
		TemplateID:  req.GetTemplateId(),
		ProjectID:   req.GetProjectId(),
		RootTraceID: req.GetRootTraceId(),
		RequestID:   req.GetRequestId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.ExecuteResponse{Execution: toProtoExecution(exec)}, nil
}

func (s *Server) GetExecution(ctx context.Context, req *workflowv1.GetExecutionRequest) (*workflowv1.GetExecutionResponse, error) {
	exec, err := s.getExecution.Execute(ctx, usecase.GetExecutionInput{ID: req.GetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.GetExecutionResponse{Execution: toProtoExecution(exec)}, nil
}

func (s *Server) PauseExecution(ctx context.Context, req *workflowv1.PauseExecutionRequest) (*workflowv1.PauseExecutionResponse, error) {
	exec, err := s.pauseExecution.Execute(ctx, usecase.PauseExecutionInput{ID: req.GetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.PauseExecutionResponse{Execution: toProtoExecution(exec)}, nil
}

func (s *Server) ResumeExecution(ctx context.Context, req *workflowv1.ResumeExecutionRequest) (*workflowv1.ResumeExecutionResponse, error) {
	exec, err := s.resumeExecution.Execute(ctx, usecase.ResumeExecutionInput{ID: req.GetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.ResumeExecutionResponse{Execution: toProtoExecution(exec)}, nil
}

func (s *Server) ExecuteAdHocStep(ctx context.Context, req *workflowv1.ExecuteAdHocStepRequest) (*workflowv1.ExecuteAdHocStepResponse, error) {
	result, err := s.executeAdHocStep.Execute(ctx, usecase.ExecuteAdHocStepInput{
		StepType:       toDomainStepType(req.GetStepType()),
		StepConfigJSON: req.GetStepConfigJson(),
		RequestID:      req.GetRequestId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.ExecuteAdHocStepResponse{Result: toProtoStepResult(result)}, nil
}

func toDomainStepType(t workflowv1.StepType) domain.StepType {
	switch t {
	case workflowv1.StepType_STEP_TYPE_AGENT:
		return domain.StepTypeAgent
	case workflowv1.StepType_STEP_TYPE_SHELL:
		return domain.StepTypeShell
	case workflowv1.StepType_STEP_TYPE_NOTIFICATION:
		return domain.StepTypeNotification
	case workflowv1.StepType_STEP_TYPE_WEBHOOK:
		return domain.StepTypeWebhook
	case workflowv1.StepType_STEP_TYPE_CONDITION:
		return domain.StepTypeCondition
	default:
		return domain.StepTypeUnspecified
	}
}

func toProtoTemplate(t domain.WorkflowTemplate) *workflowv1.WorkflowTemplate {
	return &workflowv1.WorkflowTemplate{
		Id:       t.ID,
		TenantId: t.TenantID,
		Name:     t.Name,
		DagJson:  t.DAGJSON,
		Scope:    string(t.Scope),
	}
}

func toProtoExecution(e domain.WorkflowExecution) *workflowv1.WorkflowExecution {
	return &workflowv1.WorkflowExecution{
		Id:          e.ID,
		TemplateId:  e.TemplateID,
		Status:      string(e.Status),
		RootTraceId: e.RootTraceID,
	}
}

func toProtoStepResult(r domain.StepResult) *workflowv1.StepResult {
	return &workflowv1.StepResult{
		Status:     string(r.Status),
		OutputJson: r.OutputJSON,
	}
}
