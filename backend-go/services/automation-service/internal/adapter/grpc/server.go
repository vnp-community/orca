// Package grpc implements the generated automationv1.AutomationServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"

	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
)

// Server implements automationv1.UnimplementedAutomationServiceServer.
type Server struct {
	automationv1.UnimplementedAutomationServiceServer

	createAutomation *usecase.CreateAutomation
	runNow           *usecase.RunNow
	listRuns         *usecase.ListRuns
}

func New(create *usecase.CreateAutomation, runNow *usecase.RunNow, list *usecase.ListRuns) *Server {
	return &Server{createAutomation: create, runNow: runNow, listRuns: list}
}

func (s *Server) CreateAutomation(ctx context.Context, req *automationv1.CreateAutomationRequest) (*automationv1.CreateAutomationResponse, error) {
	automation, err := s.createAutomation.Execute(ctx, usecase.CreateAutomationInput{
		Name:           req.GetName(),
		RRule:          req.GetRrule(),
		StepConfigJSON: req.GetStepConfigJson(),
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

func toProtoAutomation(a domain.Automation) *automationv1.Automation {
	return &automationv1.Automation{
		Id:             a.ID,
		TenantId:       a.TenantID,
		Name:           a.Name,
		Rrule:          a.RRule,
		StepConfigJson: a.StepConfigJSON,
	}
}

func toProtoRun(r domain.AutomationRun) *automationv1.AutomationRun {
	return &automationv1.AutomationRun{
		Id:           r.ID,
		AutomationId: r.AutomationID,
		Status:       string(r.Status),
	}
}
