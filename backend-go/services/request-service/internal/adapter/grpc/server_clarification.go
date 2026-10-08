package grpc

import (
	"context"
	"strconv"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// ClarificationUseCases are the CR-REQ-028 use cases behind the clarification, readiness and decision RPCs.
type ClarificationUseCases struct {
	Readiness    *usecase.GetRequestReadiness
	Request      *usecase.RequestClarification
	Queries      *usecase.ClarificationQueries
	Answer       *usecase.AnswerClarification
	Cancel       *usecase.CancelClarification
	Waive        *usecase.WaiveReadiness
	Decisions    *usecase.DecisionQueries
	DecisionConf *usecase.ConfirmDecision
}

func (s *Server) WithClarification(u ClarificationUseCases) *Server { s.clarification = u; return s }

func (s *Server) GetRequestReadiness(ctx context.Context, req *requestv1.GetRequestReadinessRequest) (*requestv1.GetRequestReadinessResponse, error) {
	if s.clarification.Readiness == nil {
		return nil, unwired("GetRequestReadiness")
	}
	v, err := s.clarification.Readiness.Execute(ctx, req.GetRequestId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.GetRequestReadinessResponse{Ready: v.Report.Ready, ContentRevision: int32(v.ContentRevision)}
	for _, m := range v.Report.Missing {
		out.Missing = append(out.Missing, &requestv1.ReadinessMissing{Path: m.Path, Rule: m.Rule, Blocking: m.Blocking})
	}
	return out, nil
}

// RequestClarification over the public RPC never creates the platform-only sources.
func (s *Server) RequestClarification(ctx context.Context, req *requestv1.RequestClarificationRequest) (*requestv1.RequestClarificationResponse, error) {
	if s.clarification.Request == nil || s.clarification.Queries == nil {
		return nil, unwired("RequestClarification")
	}
	source, err := domain.ParseClarificationSource(req.GetSource())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	if source.SystemOnly() {
		return nil, apperrors.ToGRPCStatus(domain.ErrClarificationStateNotAllowed("a " + req.GetSource() + " clarification is raised by the platform, not through this RPC"))
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	questions, err := questionInputsFromProto(req.GetQuestions())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	c, err := s.clarification.Request.Execute(ctx, usecase.RequestClarificationInput{
		RequestID: req.GetRequestId(), Source: source, SourceRef: req.GetSourceRef(), Questions: questions, ActorID: actor, ActorKind: domain.ActorKindUser,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	v, err := s.clarification.Queries.Get(ctx, c.ID)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.RequestClarificationResponse{Clarification: toProtoClarification(v)}, nil
}

func (s *Server) ListClarifications(ctx context.Context, req *requestv1.ListClarificationsRequest) (*requestv1.ListClarificationsResponse, error) {
	if s.clarification.Queries == nil {
		return nil, unwired("ListClarifications")
	}
	status, err := clarificationStatusFromProto(req.GetStatus())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	after, err := parseSeqToken(req.GetPageToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	page, err := s.clarification.Queries.List(ctx, req.GetRequestId(), status, after, int(req.GetPageSize()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListClarificationsResponse{}
	for _, v := range page.Items {
		out.Clarifications = append(out.Clarifications, toProtoClarification(v))
	}
	if page.NextAfter > 0 {
		out.NextPageToken = strconv.Itoa(page.NextAfter)
	}
	return out, nil
}

func parseSeqToken(tok string) (int, error) {
	if tok == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(tok)
	if err != nil || n < 0 {
		return 0, apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_PAGE_TOKEN", "invalid page token", nil)
	}
	return n, nil
}

func (s *Server) GetClarification(ctx context.Context, req *requestv1.GetClarificationRequest) (*requestv1.GetClarificationResponse, error) {
	if s.clarification.Queries == nil {
		return nil, unwired("GetClarification")
	}
	v, err := s.clarification.Queries.Get(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetClarificationResponse{Clarification: toProtoClarification(v)}, nil
}

func (s *Server) AnswerClarification(ctx context.Context, req *requestv1.AnswerClarificationRequest) (*requestv1.AnswerClarificationResponse, error) {
	if s.clarification.Answer == nil || s.clarification.Queries == nil {
		return nil, unwired("AnswerClarification")
	}
	items, err := answerItemsFromProto(req.GetAnswers())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	res, err := s.clarification.Answer.Execute(ctx, usecase.AnswerInput{
		ClarificationID: req.GetClarificationId(), Answers: items, Complete: req.GetComplete(), ExpectedVersion: req.GetExpectedVersion(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	v, err := s.clarification.Queries.Get(ctx, res.Clarification.ID)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.AnswerClarificationResponse{
		Clarification: toProtoClarification(v), RequestStatus: string(res.RequestStatus), RequestRevision: int32(res.RequestRevision), StillMissing: res.StillMissing,
	}, nil
}

func (s *Server) CancelClarification(ctx context.Context, req *requestv1.CancelClarificationRequest) (*requestv1.CancelClarificationResponse, error) {
	if s.clarification.Cancel == nil || s.clarification.Queries == nil {
		return nil, unwired("CancelClarification")
	}
	c, err := s.clarification.Cancel.Execute(ctx, usecase.CancelClarificationInput{ClarificationID: req.GetId(), Reason: req.GetReason(), ExpectedVersion: req.GetExpectedVersion()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	v, err := s.clarification.Queries.Get(ctx, c.ID)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.CancelClarificationResponse{Clarification: toProtoClarification(v)}, nil
}

func (s *Server) ListPendingClarificationsForUser(ctx context.Context, req *requestv1.ListPendingClarificationsForUserRequest) (*requestv1.ListPendingClarificationsForUserResponse, error) {
	if s.clarification.Queries == nil {
		return nil, unwired("ListPendingClarificationsForUser")
	}
	page, err := s.clarification.Queries.ListPending(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListPendingClarificationsForUserResponse{NextPageToken: page.NextPageToken}
	for _, v := range page.Items {
		out.Clarifications = append(out.Clarifications, toProtoClarification(v))
	}
	return out, nil
}

func (s *Server) WaiveReadiness(ctx context.Context, req *requestv1.WaiveReadinessRequest) (*requestv1.WaiveReadinessResponse, error) {
	if s.clarification.Waive == nil {
		return nil, unwired("WaiveReadiness")
	}
	res, err := s.clarification.Waive.Execute(ctx, usecase.WaiveInput{RequestID: req.GetRequestId(), Reason: req.GetReason(), ExpectedVersion: req.GetExpectedVersion()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.WaiveReadinessResponse{RequestStatus: string(res.Request.Status), RequestRevision: int32(res.RequestRevision)}, nil
}

func (s *Server) ListDecisions(ctx context.Context, req *requestv1.ListDecisionsRequest) (*requestv1.ListDecisionsResponse, error) {
	if s.clarification.Decisions == nil {
		return nil, unwired("ListDecisions")
	}
	status, err := decisionStatusFromProto(req.GetStatus())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	after, err := parseSeqToken(req.GetPageToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	page, err := s.clarification.Decisions.List(ctx, req.GetRequestId(), status, after, int(req.GetPageSize()))
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListDecisionsResponse{}
	for _, d := range page.Items {
		out.Decisions = append(out.Decisions, toProtoDecision(d, page.RequestNumber))
	}
	if page.NextAfter > 0 {
		out.NextPageToken = strconv.Itoa(page.NextAfter)
	}
	return out, nil
}

func (s *Server) GetDecision(ctx context.Context, req *requestv1.GetDecisionRequest) (*requestv1.GetDecisionResponse, error) {
	if s.clarification.Decisions == nil {
		return nil, unwired("GetDecision")
	}
	d, number, err := s.clarification.Decisions.Get(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetDecisionResponse{Decision: toProtoDecision(d, number)}, nil
}

func (s *Server) ConfirmDecision(ctx context.Context, req *requestv1.ConfirmDecisionRequest) (*requestv1.ConfirmDecisionResponse, error) {
	if s.clarification.DecisionConf == nil || s.clarification.Decisions == nil {
		return nil, unwired("ConfirmDecision")
	}
	d, err := s.clarification.DecisionConf.Execute(ctx, usecase.ConfirmDecisionInput{
		DecisionID: req.GetDecisionId(), ConfirmationText: req.GetConfirmationText(), ExpectedVersion: req.GetExpectedVersion(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	_, number, err := s.clarification.Decisions.Get(ctx, d.ID)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.ConfirmDecisionResponse{Decision: toProtoDecision(d, number)}, nil
}
