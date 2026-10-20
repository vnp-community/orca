package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WithEngineSettings attaches Get/SetProjectEngineSettings; without it they stay Unimplemented.
func (s *Server) WithEngineSettings(u *usecase.ManageProjectEngineSettings) *Server {
	s.engineSettings = u
	return s
}

func (s *Server) GetProjectEngineSettings(ctx context.Context, req *requestv1.GetProjectEngineSettingsRequest) (*requestv1.GetProjectEngineSettingsResponse, error) {
	if s.engineSettings == nil {
		return nil, status.Error(codes.Unimplemented, "GetProjectEngineSettings is not wired")
	}
	view, err := s.engineSettings.GetProjectEngineSettings(ctx, req.GetProjectId(), req.GetCheck())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetProjectEngineSettingsResponse{Settings: toProtoEngineSettings(view.Settings), Preflight: toProtoPreflight(view)}, nil
}

func (s *Server) SetProjectEngineSettings(ctx context.Context, req *requestv1.SetProjectEngineSettingsRequest) (*requestv1.SetProjectEngineSettingsResponse, error) {
	if s.engineSettings == nil {
		return nil, status.Error(codes.Unimplemented, "SetProjectEngineSettings is not wired")
	}
	view, err := s.engineSettings.SetProjectEngineSettings(ctx, usecase.SetProjectEngineSettingsInput{
		ProjectID: req.GetProjectId(), Engine: req.GetSolutionEngine(), MinVersion: req.GetOpenspecMinVersion(),
		SolutionAgent: req.GetSolutionAgent(), ExecutionMode: req.GetExecutionMode(), PlanAutoGenerate: req.PlanAutoGenerate, OutcomeReview: req.GetOutcomeReview(), ExpectedVersion: req.GetExpectedVersion(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.SetProjectEngineSettingsResponse{Settings: toProtoEngineSettings(view.Settings), Preflight: toProtoPreflight(view)}, nil
}

func toProtoEngineSettings(s domain.ProjectEngineSettings) *requestv1.ProjectEngineSettings {
	return &requestv1.ProjectEngineSettings{
		ProjectId: s.ProjectID, SolutionEngine: string(s.Engine), OpenspecMinVersion: s.MinVersion, Version: s.Version, SolutionAgent: s.SolutionAgent, ExecutionMode: string(s.ExecutionMode.OrDefault()), PlanAutoGenerate: s.PlanAutoGenerate,
		OutcomeReview: string(s.OutcomeReview),
	}
}

// toProtoPreflight is nil when no check was asked for; a check that could not run says so in its warnings.
func toProtoPreflight(v usecase.EngineSettingsView) *requestv1.EnginePreflight {
	if v.Preflight == nil && len(v.Warnings) == 0 {
		return nil
	}
	out := &requestv1.EnginePreflight{Warnings: append([]string(nil), v.Warnings...)}
	if r := v.Preflight; r != nil {
		out.Ran, out.Ok, out.OpenspecVersion = true, r.OK, r.OpenSpecVersion
		out.CheckedAt = r.CheckedAt.Format("2006-01-02T15:04:05Z07:00")
		out.Warnings = append(out.Warnings, r.Warnings...)
		for _, f := range r.Failures {
			out.Blockers = append(out.Blockers, &requestv1.EnginePreflightNote{Code: f.Code, Message: f.Message, FixHint: f.FixHint})
		}
		for _, f := range r.Gaps {
			out.Gaps = append(out.Gaps, &requestv1.EnginePreflightNote{Code: f.Code, Message: f.Message, FixHint: f.FixHint})
		}
		for _, f := range r.ReviewGaps {
			out.ReviewGaps = append(out.ReviewGaps, &requestv1.EnginePreflightNote{Code: f.Code, Message: f.Message, FixHint: f.FixHint})
		}
	}
	return out
}
