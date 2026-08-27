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

	"google.golang.org/protobuf/types/known/emptypb"
)

// Server implements automationv1.UnimplementedAutomationServiceServer.
type Server struct {
	automationv1.UnimplementedAutomationServiceServer

	createAutomation      *usecase.CreateAutomation
	runNow                *usecase.RunNow
	listRuns              *usecase.ListRuns
	handleExternalTrigger *usecase.HandleExternalTrigger
	listAutomations       *usecase.ListAutomations
	updateAutomation      *usecase.UpdateAutomation
	deleteAutomation      *usecase.DeleteAutomation
	writeCleanupReport    *usecase.WriteCleanupReport
}

func New(
	create *usecase.CreateAutomation,
	runNow *usecase.RunNow,
	list *usecase.ListRuns,
	handleExternalTrigger *usecase.HandleExternalTrigger,
	listAutomations *usecase.ListAutomations,
	updateAutomation *usecase.UpdateAutomation,
	deleteAutomation *usecase.DeleteAutomation,
	writeCleanupReport *usecase.WriteCleanupReport,
) *Server {
	return &Server{
		createAutomation:      create,
		runNow:                runNow,
		listRuns:              list,
		handleExternalTrigger: handleExternalTrigger,
		listAutomations:       listAutomations,
		updateAutomation:      updateAutomation,
		deleteAutomation:      deleteAutomation,
		writeCleanupReport:    writeCleanupReport,
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

func (s *Server) ListAutomations(ctx context.Context, req *automationv1.ListAutomationsRequest) (*automationv1.ListAutomationsResponse, error) {
	result, err := s.listAutomations.Execute(ctx, usecase.ListAutomationsInput{
		TenantID:  req.GetTenantId(),
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*automationv1.Automation, 0, len(result.Automations))
	for _, a := range result.Automations {
		out = append(out, toProtoAutomation(a))
	}
	return &automationv1.ListAutomationsResponse{Automations: out, NextPageToken: result.NextPageToken}, nil
}

func (s *Server) UpdateAutomation(ctx context.Context, req *automationv1.UpdateAutomationRequest) (*automationv1.UpdateAutomationResponse, error) {
	in := usecase.UpdateAutomationInput{TenantID: req.GetTenantId(), ID: req.GetId()}
	if req.GetName() != nil {
		v := req.GetName().GetValue()
		in.Name = &v
	}
	if req.GetRrule() != nil {
		v := req.GetRrule().GetValue()
		in.RRule = &v
	}
	if req.GetStepConfigJson() != nil {
		v := req.GetStepConfigJson().GetValue()
		in.StepConfigJSON = &v
	}
	if req.GetStepType() != workflowv1.StepType_STEP_TYPE_UNSPECIFIED {
		v := fromProtoStepType(req.GetStepType())
		in.StepType = &v
	}
	if req.GetEnabled() != nil {
		v := req.GetEnabled().GetValue()
		in.Enabled = &v
	}
	if req.GetDtstart() != nil {
		parsed, err := time.Parse(time.RFC3339, req.GetDtstart().GetValue())
		if err != nil {
			return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID_DTSTART", "dtstart must be RFC3339", err))
		}
		in.Dtstart = &parsed
	}
	if req.GetTimezone() != nil {
		v := req.GetTimezone().GetValue()
		in.Timezone = &v
	}
	automation, err := s.updateAutomation.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &automationv1.UpdateAutomationResponse{Automation: toProtoAutomation(automation)}, nil
}

func (s *Server) DeleteAutomation(ctx context.Context, req *automationv1.DeleteAutomationRequest) (*emptypb.Empty, error) {
	if err := s.deleteAutomation.Execute(ctx, usecase.DeleteAutomationInput{TenantID: req.GetTenantId(), ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// WriteCleanupReport is a reverse-direction call — workflow-service ->
// automation-service — from CleanupWorktreesStepExecutor after a bulk
// worktree-delete run, recording BR-AT-14's per-worktree audit trail.
func (s *Server) WriteCleanupReport(ctx context.Context, req *automationv1.WriteCleanupReportRequest) (*emptypb.Empty, error) {
	entries := make([]domain.CleanupLogEntry, 0, len(req.GetEntries()))
	for _, e := range req.GetEntries() {
		entries = append(entries, domain.CleanupLogEntry{WorktreeID: e.GetWorktreeId(), Action: e.GetAction(), Reason: e.GetReason()})
	}
	if err := s.writeCleanupReport.Execute(ctx, usecase.WriteCleanupReportInput{RunID: req.GetRunId(), Entries: entries}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
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
	case domain.StepTypeCleanupWorktrees:
		return workflowv1.StepType_STEP_TYPE_CLEANUP_WORKTREES
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
	case workflowv1.StepType_STEP_TYPE_CLEANUP_WORKTREES:
		return domain.StepTypeCleanupWorktrees
	default:
		return domain.StepTypeUnspecified
	}
}
