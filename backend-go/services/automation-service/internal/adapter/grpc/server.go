// Package grpc implements the generated automationv1.AutomationServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"

	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// Server implements automationv1.UnimplementedAutomationServiceServer.
type Server struct {
	automationv1.UnimplementedAutomationServiceServer

	createAutomation      *usecase.CreateAutomation
	runNow                *usecase.RunNow
	listRuns              *usecase.ListRuns
	handleExternalTrigger *usecase.HandleExternalTrigger
}

func New(create *usecase.CreateAutomation, runNow *usecase.RunNow, list *usecase.ListRuns, handleExternalTrigger *usecase.HandleExternalTrigger) *Server {
	return &Server{
		createAutomation:      create,
		runNow:                runNow,
		listRuns:              list,
		handleExternalTrigger: handleExternalTrigger,
	}
}

func (s *Server) CreateAutomation(ctx context.Context, req *automationv1.CreateAutomationRequest) (*automationv1.CreateAutomationResponse, error) {
	automation, err := s.createAutomation.Execute(ctx, usecase.CreateAutomationInput{
		Name:           req.GetName(),
		RRule:          req.GetRrule(),
		StepType:       fromProtoStepType(req.GetStepType()),
		StepConfigJSON: req.GetStepConfigJson(),
		DTStart:        req.GetDtstart(),
		Timezone:       req.GetTimezone(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &automationv1.CreateAutomationResponse{Automation: toProtoAutomation(automation)}, nil
}

func (s *Server) RunNow(ctx context.Context, req *automationv1.RunNowRequest) (*automationv1.RunNowResponse, error) {
	run, err := s.runNow.Execute(ctx, usecase.RunNowInput{
		AutomationID: req.GetAutomationId(),
		RequestID:    req.GetRequestId(),
		Trigger:      domain.RunTriggerManual,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &automationv1.RunNowResponse{Run: toProtoRun(run)}, nil
}

func (s *Server) ListRuns(ctx context.Context, req *automationv1.ListRunsRequest) (*automationv1.ListRunsResponse, error) {
	out, err := s.listRuns.Execute(ctx, usecase.ListRunsInput{
		AutomationID: req.GetAutomationId(),
		PageToken:    req.GetPageToken(),
		PageSize:     req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	runs := make([]*automationv1.AutomationRun, 0, len(out.Runs))
	for _, r := range out.Runs {
		runs = append(runs, toProtoRun(r))
	}
	return &automationv1.ListRunsResponse{Runs: runs, NextPageToken: out.NextPageToken}, nil
}

// HandleExternalTrigger maps an external trigger source's payload onto the
// same RunNow path every other trigger uses — see
// usecase.HandleExternalTrigger's doc comment. payload_json is accepted on
// the wire but not forwarded to the usecase input beyond being received;
// see this service's README "deviations" note.
func (s *Server) HandleExternalTrigger(ctx context.Context, req *automationv1.HandleExternalTriggerRequest) (*automationv1.HandleExternalTriggerResponse, error) {
	run, err := s.handleExternalTrigger.Execute(ctx, usecase.HandleExternalTriggerInput{
		AutomationID: req.GetAutomationId(),
		RequestID:    req.GetRequestId(),
		PayloadJSON:  req.GetPayloadJson(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &automationv1.HandleExternalTriggerResponse{Run: toProtoRun(run)}, nil
}

func toProtoAutomation(a domain.Automation) *automationv1.Automation {
	return &automationv1.Automation{
		Id:             a.ID,
		TenantId:       a.TenantID,
		Name:           a.Name,
		Rrule:          a.RRule,
		StepConfigJson: a.StepConfigJSON,
		StepType:       toProtoStepType(a.StepType),
		Enabled:        a.Enabled,
		Dtstart:        a.DTStart.Format(time.RFC3339),
		Timezone:       a.Timezone,
	}
}

func toProtoRun(r domain.AutomationRun) *automationv1.AutomationRun {
	return &automationv1.AutomationRun{
		Id:           r.ID,
		AutomationId: r.AutomationID,
		Status:       string(r.Status),
	}
}

// toProtoStepType/fromProtoStepType translate between domain.StepType and
// orca.workflow.v1.StepType — the same enum automation.proto's Automation
// message reuses (see automation.proto's comment on that field), so this
// wire boundary needs its own translation alongside
// internal/adapter/grpcclient's (which translates the same enum on the
// OUTBOUND call to workflow-service).
func toProtoStepType(s domain.StepType) workflowv1.StepType {
	switch s {
	case domain.StepTypeAgent:
		return workflowv1.StepType_STEP_TYPE_AGENT
	case domain.StepTypeShell:
		return workflowv1.StepType_STEP_TYPE_SHELL
	case domain.StepTypeNotification:
		return workflowv1.StepType_STEP_TYPE_NOTIFICATION
	case domain.StepTypeWebhook:
		return workflowv1.StepType_STEP_TYPE_WEBHOOK
	case domain.StepTypeCondition:
		return workflowv1.StepType_STEP_TYPE_CONDITION
	default:
		return workflowv1.StepType_STEP_TYPE_UNSPECIFIED
	}
}

func fromProtoStepType(s workflowv1.StepType) domain.StepType {
	switch s {
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
