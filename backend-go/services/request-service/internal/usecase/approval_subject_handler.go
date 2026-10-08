package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type requestedSubjectIDKey struct{}

// WithRequestedSubjectID carries the caller's subject id (a phase, a pre-deploy gate) to ValidateForRequest
// without changing the handler signature that several CRs implement.
func WithRequestedSubjectID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestedSubjectIDKey{}, id)
}

func RequestedSubjectID(ctx context.Context) string {
	id, _ := ctx.Value(requestedSubjectIDKey{}).(string)
	return id
}

// SubjectHandler is the contract every subject owner (CR-REQ-005/007/008/012/013/014) implements.
// ValidateForRequest must be pure for a given Request state; the On* hooks run inside the approval
// transaction, may only write this service's tables, and must not call other services.
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
	if r == nil {
		return nil
	}
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
