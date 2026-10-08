package grpc

import (
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// IntakeUseCases are the CR-REQ-004 use cases behind CreateRequest and LookupRequestBySource.
type IntakeUseCases struct {
	Create *usecase.CreateRequest
	Lookup *usecase.LookupRequestBySource
}

// ClassificationUseCases are the CR-REQ-005 use cases. Classify only enqueues a run (decision D3).
type ClassificationUseCases struct {
	Runner  *usecase.ClassificationRunner
	Confirm *usecase.ConfirmRequestType
	Change  *usecase.ChangeRequestType
	History *usecase.ListRequestTypeHistory
}

// LifecycleUseCases are the CR-REQ-003/006 use cases.
type LifecycleUseCases struct {
	Flow       *usecase.GetRequestFlow
	Return     *usecase.ReturnRequestToBacklog
	Reopen     *usecase.ReopenRequest
	Cancel     *usecase.CancelRequest
	SpawnChild *usecase.SpawnChildRequest
	Links      *usecase.ListRequestLinks
}

// WithIntake attaches the intake handlers; without it those RPCs stay Unimplemented.
func (s *Server) WithIntake(u IntakeUseCases) *Server { s.intake = u; return s }

func (s *Server) WithClassification(u ClassificationUseCases) *Server {
	s.classification = u
	return s
}

func (s *Server) WithLifecycle(u LifecycleUseCases) *Server { s.lifecycle = u; return s }
