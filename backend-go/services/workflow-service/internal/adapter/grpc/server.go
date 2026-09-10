// Package grpc implements the generated workflowv1.WorkflowServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"encoding/json"
	"log/slog"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// Server implements workflowv1.UnimplementedWorkflowServiceServer.
type Server struct {
	workflowv1.UnimplementedWorkflowServiceServer

	createTemplate      *usecase.CreateTemplate
	execute             *usecase.Execute
	getExecution        *usecase.GetExecution
	pauseExecution      *usecase.PauseExecution
	resumeExecution     *usecase.ResumeExecution
	executeAdHocStep    *usecase.ExecuteAdHocStep
	hasActiveExecutions *usecase.HasActiveExecutions
	cancelExecution     *usecase.CancelExecution
	listTemplates       *usecase.ListTemplates
	resolveTemplate     *usecase.ResolveTemplate
	updateTemplate      *usecase.UpdateTemplate
	listExecutions      *usecase.ListExecutions
	cloneTemplate       *usecase.CloneTemplate
}

func New(
	createTemplate *usecase.CreateTemplate,
	execute *usecase.Execute,
	getExecution *usecase.GetExecution,
	pauseExecution *usecase.PauseExecution,
	resumeExecution *usecase.ResumeExecution,
	executeAdHocStep *usecase.ExecuteAdHocStep,
	hasActiveExecutions *usecase.HasActiveExecutions,
	cancelExecution *usecase.CancelExecution,
	listTemplates *usecase.ListTemplates,
	resolveTemplate *usecase.ResolveTemplate,
	updateTemplate *usecase.UpdateTemplate,
	listExecutions *usecase.ListExecutions,
	cloneTemplate *usecase.CloneTemplate,
) *Server {
	return &Server{
		createTemplate:      createTemplate,
		execute:             execute,
		getExecution:        getExecution,
		pauseExecution:      pauseExecution,
		resumeExecution:     resumeExecution,
		executeAdHocStep:    executeAdHocStep,
		hasActiveExecutions: hasActiveExecutions,
		cancelExecution:     cancelExecution,
		listTemplates:       listTemplates,
		resolveTemplate:     resolveTemplate,
		updateTemplate:      updateTemplate,
		listExecutions:      listExecutions,
		cloneTemplate:       cloneTemplate,
	}
}

func (s *Server) CreateTemplate(ctx context.Context, req *workflowv1.CreateTemplateRequest) (*workflowv1.CreateTemplateResponse, error) {
	tmpl, err := s.createTemplate.Execute(ctx, usecase.CreateTemplateInput{
		Name:             req.GetName(),
		DAGJSON:          req.GetDagJson(),
		Scope:            req.GetScope(),
		ParentTemplateID: req.GetParentTemplateId(),
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
		Inputs:      req.GetInputs().AsMap(),
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

func (s *Server) HasActiveExecutions(ctx context.Context, req *workflowv1.HasActiveExecutionsRequest) (*workflowv1.HasActiveExecutionsResponse, error) {
	hasActive, err := s.hasActiveExecutions.Execute(ctx, usecase.HasActiveExecutionsInput{ProjectID: req.GetProjectId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.HasActiveExecutionsResponse{HasActive: hasActive}, nil
}

func (s *Server) CancelExecution(ctx context.Context, req *workflowv1.CancelExecutionRequest) (*workflowv1.CancelExecutionResponse, error) {
	exec, err := s.cancelExecution.Execute(ctx, usecase.CancelExecutionInput{ID: req.GetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.CancelExecutionResponse{Execution: toProtoExecution(exec)}, nil
}

func (s *Server) ListTemplates(ctx context.Context, req *workflowv1.ListTemplatesRequest) (*workflowv1.ListTemplatesResponse, error) {
	out, err := s.listTemplates.Execute(ctx, usecase.ListTemplatesInput{
		Scope:     req.GetScope(),
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	templates := make([]*workflowv1.WorkflowTemplate, 0, len(out.Templates))
	for _, tmpl := range out.Templates {
		templates = append(templates, toProtoTemplate(tmpl))
	}
	return &workflowv1.ListTemplatesResponse{Templates: templates, NextPageToken: out.NextPageToken}, nil
}

func (s *Server) ResolveTemplate(ctx context.Context, req *workflowv1.ResolveTemplateRequest) (*workflowv1.ResolveTemplateResponse, error) {
	out, err := s.resolveTemplate.Execute(ctx, usecase.ResolveTemplateInput{TemplateID: req.GetTemplateId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	chain := make([]*workflowv1.WorkflowTemplate, 0, len(out.Chain))
	for _, tmpl := range out.Chain {
		chain = append(chain, toProtoTemplate(tmpl))
	}
	return &workflowv1.ResolveTemplateResponse{Template: toProtoTemplate(out.Template), Chain: chain}, nil
}

func (s *Server) UpdateTemplate(ctx context.Context, req *workflowv1.UpdateTemplateRequest) (*workflowv1.UpdateTemplateResponse, error) {
	updated, err := s.updateTemplate.Execute(ctx, usecase.UpdateTemplateInput{
		ID:               req.GetId(),
		Name:             req.GetName(),
		DAGJSON:          req.GetDagJson(),
		Scope:            domain.Scope(req.GetScope()),
		ParentTemplateID: req.GetParentTemplateId(),
		ExpectedVersion:  req.GetExpectedVersion(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.UpdateTemplateResponse{Template: toProtoTemplate(updated)}, nil
}

func (s *Server) ListExecutions(ctx context.Context, req *workflowv1.ListExecutionsRequest) (*workflowv1.ListExecutionsResponse, error) {
	out, err := s.listExecutions.Execute(ctx, usecase.ListExecutionsInput{
		ProjectID: req.GetProjectId(),
		Cursor:    req.GetCursor(),
		Limit:     req.GetLimit(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	execs := make([]*workflowv1.WorkflowExecution, 0, len(out.Executions))
	for _, e := range out.Executions {
		execs = append(execs, toProtoExecution(e))
	}
	return &workflowv1.ListExecutionsResponse{Executions: execs, NextCursor: out.NextCursor}, nil
}

func (s *Server) CloneTemplate(ctx context.Context, req *workflowv1.CloneTemplateRequest) (*workflowv1.CloneTemplateResponse, error) {
	tmpl, err := s.cloneTemplate.Execute(ctx, usecase.CloneTemplateInput{
		SourceTemplateID: req.GetSourceTemplateId(),
		NewName:          req.GetNewName(),
		Scope:            req.GetScope(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &workflowv1.CloneTemplateResponse{Template: toProtoTemplate(tmpl)}, nil
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
	case workflowv1.StepType_STEP_TYPE_ACTION:
		return domain.StepTypeAction
	case workflowv1.StepType_STEP_TYPE_PARALLEL:
		return domain.StepTypeParallel
	default:
		return domain.StepTypeUnspecified
	}
}

func toProtoTemplate(t domain.WorkflowTemplate) *workflowv1.WorkflowTemplate {
	tpl := &workflowv1.WorkflowTemplate{
		Id:               t.ID,
		TenantId:         t.TenantID,
		Name:             t.Name,
		DagJson:          t.DAGJSON,
		Scope:            string(t.Scope),
		ParentTemplateId: t.ParentTemplateID,
		Version:          t.Version,
	}
	if len(t.Overrides) > 0 {
		if s, err := structpb.NewStruct(t.Overrides); err == nil {
			tpl.Overrides = s
		} else {
			// Overrides holding a value structpb can't represent (a type
			// outside JSON's data model) is a template-authoring bug, not
			// something this read-path conversion should fail the whole
			// response over — drop it from the response and log, same
			// "best-effort, log don't crash" tolerance this service already
			// gives ResolveConnection's HiddenTargetID lookup.
			slog.Error("workflow: template overrides not representable as google.protobuf.Struct", slog.String("template_id", t.ID), slog.Any("error", err))
		}
	}
	if len(t.InjectSteps) > 0 {
		tpl.InjectSteps = make([]*workflowv1.StepInjection, 0, len(t.InjectSteps))
		for _, inj := range t.InjectSteps {
			stepJSON, err := json.Marshal(inj.Step)
			if err != nil {
				slog.Error("workflow: template injectSteps step not JSON-serializable", slog.String("template_id", t.ID), slog.Any("error", err))
				continue
			}
			tpl.InjectSteps = append(tpl.InjectSteps, &workflowv1.StepInjection{
				AnchorStepId: inj.AnchorStepID,
				Position:     inj.Position,
				StepJson:     string(stepJSON),
			})
		}
	}
	if len(t.RemoveSteps) > 0 {
		tpl.RemoveSteps = t.RemoveSteps
	}
	return tpl
}

func toProtoExecution(e domain.WorkflowExecution) *workflowv1.WorkflowExecution {
	return &workflowv1.WorkflowExecution{
		Id:          e.ID,
		TemplateId:  e.TemplateID,
		Status:      string(e.Status),
		RootTraceId: e.RootTraceID,
		ProjectId:   e.ProjectID,
	}
}

func toProtoStepResult(r domain.StepResult) *workflowv1.StepResult {
	return &workflowv1.StepResult{
		Status:     string(r.Status),
		OutputJson: r.OutputJSON,
	}
}
