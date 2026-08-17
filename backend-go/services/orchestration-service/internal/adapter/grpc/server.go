// Package grpc implements the generated
// orchestrationv1.OrchestrationServiceServer interface by translating wire
// messages to/from usecase calls — no business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
)

// Server implements orchestrationv1.UnimplementedOrchestrationServiceServer.
type Server struct {
	orchestrationv1.UnimplementedOrchestrationServiceServer

	createDispatchContext      *usecase.CreateDispatchContext
	createGate                 *usecase.CreateGate
	resolveGate                *usecase.ResolveGate
	updateTaskStatusAndPromote *usecase.UpdateTaskStatusAndPromote
}

func New(
	createDispatchContext *usecase.CreateDispatchContext,
	createGate *usecase.CreateGate,
	resolveGate *usecase.ResolveGate,
	updateTaskStatusAndPromote *usecase.UpdateTaskStatusAndPromote,
) *Server {
	return &Server{
		createDispatchContext:      createDispatchContext,
		createGate:                 createGate,
		resolveGate:                resolveGate,
		updateTaskStatusAndPromote: updateTaskStatusAndPromote,
	}
}

func (s *Server) CreateDispatchContext(ctx context.Context, req *orchestrationv1.CreateDispatchContextRequest) (*orchestrationv1.CreateDispatchContextResponse, error) {
	dc, err := s.createDispatchContext.Execute(ctx, usecase.CreateDispatchContextInput{
		Handle:           req.GetHandle(),
		CoordinatorRunID: req.GetCoordinatorRunId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &orchestrationv1.CreateDispatchContextResponse{
		Context: &orchestrationv1.DispatchContext{
			Id:               dc.ID,
			Handle:           dc.Handle,
			CoordinatorRunId: dc.CoordinatorRunID,
		},
	}, nil
}

func (s *Server) CreateGate(ctx context.Context, req *orchestrationv1.CreateGateRequest) (*orchestrationv1.CreateGateResponse, error) {
	gate, err := s.createGate.Execute(ctx, usecase.CreateGateInput{
		DispatchContextID: req.GetDispatchContextId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &orchestrationv1.CreateGateResponse{
		Gate: &orchestrationv1.DecisionGate{
			Id:                gate.ID,
			DispatchContextId: gate.DispatchContextID,
			Status:            string(gate.Status),
		},
	}, nil
}

func (s *Server) ResolveGate(ctx context.Context, req *orchestrationv1.ResolveGateRequest) (*orchestrationv1.ResolveGateResponse, error) {
	out, err := s.resolveGate.Execute(ctx, usecase.ResolveGateInput{
		GateID:      req.GetGateId(),
		OutcomeJSON: req.GetOutcomeJson(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &orchestrationv1.ResolveGateResponse{
		Gate: &orchestrationv1.DecisionGate{
			Id:                out.Gate.ID,
			DispatchContextId: out.Gate.DispatchContextID,
			Status:            string(out.Gate.Status),
		},
	}, nil
}

func (s *Server) UpdateTaskStatusAndPromote(ctx context.Context, req *orchestrationv1.UpdateTaskStatusAndPromoteRequest) (*orchestrationv1.UpdateTaskStatusAndPromoteResponse, error) {
	out, err := s.updateTaskStatusAndPromote.Execute(ctx, usecase.UpdateTaskStatusAndPromoteInput{
		OrchestrationTaskID: req.GetOrchestrationTaskId(),
		NewStatus:           req.GetNewStatus(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &orchestrationv1.UpdateTaskStatusAndPromoteResponse{
		PromotedTaskIds: out.PromotedTaskIDs,
	}, nil
}
