package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type SubjectHandler interface {
	ValidateForRequest(ctx context.Context, tx context.Context, req domain.Request, st domain.SubjectType) (subjectID, digest string, err error)
	OnApproved(ctx context.Context, tx context.Context, a domain.Approval) error
	OnRejected(ctx context.Context, tx context.Context, a domain.Approval) error
	OnClosedWithoutDecision(ctx context.Context, tx context.Context, a domain.Approval, why string) error
}

type SubjectHandlerRegistry struct {
	handlers map[domain.SubjectType]SubjectHandler
}

func NewSubjectHandlerRegistry() *SubjectHandlerRegistry {
	return &SubjectHandlerRegistry{
		handlers: make(map[domain.SubjectType]SubjectHandler),
	}
}

func (r *SubjectHandlerRegistry) Register(st domain.SubjectType, h SubjectHandler) {
	r.handlers[st] = h
}

func (r *SubjectHandlerRegistry) Get(st domain.SubjectType) SubjectHandler {
	return r.handlers[st]
}

func (r *SubjectHandlerRegistry) MustCoverAll() error {
	for _, st := range domain.AllSubjectTypes {
		if _, ok := r.handlers[st]; !ok {
			return fmt.Errorf("missing handler for subject type: %s", st)
		}
	}
	return nil
}

type NoopSubjectHandler struct {
	Reason string
	Logger *slog.Logger
}

func (h *NoopSubjectHandler) ValidateForRequest(ctx context.Context, tx context.Context, req domain.Request, st domain.SubjectType) (string, string, error) {
	if h.Logger != nil {
		h.Logger.Warn("NoopSubjectHandler ValidateForRequest called", "reason", h.Reason, "subject_type", st)
	}
	return "noop_id", "noop_digest", nil
}

func (h *NoopSubjectHandler) OnApproved(ctx context.Context, tx context.Context, a domain.Approval) error {
	if h.Logger != nil {
		h.Logger.Warn("NoopSubjectHandler OnApproved called", "reason", h.Reason, "approval_id", a.ID)
	}
	return nil
}

func (h *NoopSubjectHandler) OnRejected(ctx context.Context, tx context.Context, a domain.Approval) error {
	if h.Logger != nil {
		h.Logger.Warn("NoopSubjectHandler OnRejected called", "reason", h.Reason, "approval_id", a.ID)
	}
	return nil
}

func (h *NoopSubjectHandler) OnClosedWithoutDecision(ctx context.Context, tx context.Context, a domain.Approval, why string) error {
	if h.Logger != nil {
		h.Logger.Warn("NoopSubjectHandler OnClosedWithoutDecision called", "reason", h.Reason, "approval_id", a.ID)
	}
	return nil
}
