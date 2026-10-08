package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// maxTransitionAttempts bounds CAS retries of the outermost call.
const maxTransitionAttempts = 3

type TransitionInput struct {
	RequestID string
	Trigger   domain.Trigger
	// ExpectedFrom makes redelivery safe: nil skips the check.
	ExpectedFrom *domain.RequestStatus
	ActorID      string
	ActorKind    domain.ActorKind
	// Stage, Category and Reason apply when the trigger lands in request_backlog.
	Stage    domain.ReturnStage
	Category domain.ReturnCategory
	Reason   string
	// ResumeStatus is where information_provided lands (analyzing, planning or executing); other triggers ignore it.
	ResumeStatus domain.RequestStatus
}

type TransitionResult struct {
	Request domain.Request
	Applied bool
}

// TransitionRequest is the only code allowed to assign Request.Status (see status_write_guard_test.go).
type TransitionRequest struct {
	repo   RequestRepository
	tx     TxScope
	outbox OutboxWriter
	clock  func() time.Time
	tracer trace.Tracer // nil: the global provider's tracer
}

func NewTransitionRequest(repo RequestRepository, tx TxScope, outbox OutboxWriter) *TransitionRequest {
	return &TransitionRequest{repo: repo, tx: tx, outbox: outbox, clock: func() time.Time { return time.Now().UTC() }}
}

// Execute joins the caller's transaction when there is one. Otherwise it owns the transaction
// and retries a lost CAS in a new one: a REPEATABLE READ snapshot would make an in-place retry see stale data.
func (uc *TransitionRequest) Execute(ctx context.Context, in TransitionInput) (TransitionResult, error) {
	if uc.tx.InTransaction(ctx) {
		return uc.once(ctx, in)
	}
	var res TransitionResult
	var err error
	for attempt := 0; attempt < maxTransitionAttempts; attempt++ {
		err = uc.tx.InTx(ctx, func(txCtx context.Context) error {
			var innerErr error
			res, innerErr = uc.once(txCtx, in)
			return innerErr
		})
		if !isVersionConflict(err) {
			break
		}
	}
	if err != nil {
		return TransitionResult{}, err
	}
	return res, nil
}

func errorHasCode(err error, code string) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == code
}

func isVersionConflict(err error) bool { return errorHasCode(err, "REQUEST_VERSION_CONFLICT") }

func isNotFound(err error) bool { return errorHasCode(err, "REQUEST_NOT_FOUND") }

// once runs one transition under its own span, so the traceparent in the emitted events points at it.
func (uc *TransitionRequest) once(ctx context.Context, in TransitionInput) (TransitionResult, error) {
	tracer := uc.tracer
	if tracer == nil {
		tracer = otel.Tracer("request-service")
	}
	ctx, span := tracer.Start(ctx, "request.Transition",
		trace.WithAttributes(attribute.String("request.id", in.RequestID), attribute.String("request.trigger", string(in.Trigger))))
	defer span.End()
	res, err := uc.apply(ctx, in)
	if err != nil {
		span.RecordError(err)
		return res, err
	}
	span.SetAttributes(attribute.String("request.type", string(res.Request.Type)), attribute.String("request.status_to", string(res.Request.Status)))
	return res, nil
}

func (uc *TransitionRequest) apply(ctx context.Context, in TransitionInput) (TransitionResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return TransitionResult{}, domain.ErrRequestTenantRequired()
	}
	r, err := uc.repo.Get(ctx, in.RequestID)
	if err != nil {
		return TransitionResult{}, err
	}

	// An untyped request has no flow; triggers that need one fail inside NextStatus with REQUEST_TYPE_NOT_SET.
	var flow domain.FlowDefinition
	if r.Type != "" {
		if flow, err = domain.FlowFor(r.Type); err != nil {
			return TransitionResult{}, err
		}
	}

	if in.ExpectedFrom != nil && r.Status != *in.ExpectedFrom {
		if dest, derr := domain.NextStatusWithResume(flow, r.Size, *in.ExpectedFrom, in.Trigger, in.ResumeStatus); derr == nil && dest == r.Status {
			return TransitionResult{Request: r, Applied: false}, nil
		}
		return TransitionResult{}, domain.ErrStateStale(*in.ExpectedFrom, r.Status)
	}

	to, err := domain.NextStatusWithResume(flow, r.Size, r.Status, in.Trigger, in.ResumeStatus)
	if err != nil {
		return TransitionResult{}, err
	}
	if domain.RequiresReason(in.Trigger) && strings.TrimSpace(in.Reason) == "" {
		return TransitionResult{}, domain.ErrReasonRequired()
	}

	from := r.Status
	stage, err := uc.backlogStage(r, flow, in)
	if err != nil {
		return TransitionResult{}, err
	}
	if domain.TargetsBacklog(in.Trigger) {
		if _, err := domain.ParseReturnCategory(string(in.Category)); err != nil {
			return TransitionResult{}, err
		}
	}

	r.Status = to
	if to == domain.RequestStatusRequestBacklog {
		r.ReturnedFromStage = stage
		r.ReturnedCategory = in.Category
		r.ReturnReason = in.Reason
	} else {
		r.ReturnedFromStage, r.ReturnedCategory, r.ReturnReason = "", "", ""
	}

	updated, err := uc.repo.Update(ctx, r, r.Version)
	if err != nil {
		return TransitionResult{}, err
	}
	if err := uc.emit(ctx, updated, from, in, stage); err != nil {
		return TransitionResult{}, err
	}
	return TransitionResult{Request: updated, Applied: true}, nil
}

// backlogStage resolves the stage recorded in returned_from_stage; empty when the trigger does not land in the backlog.
func (uc *TransitionRequest) backlogStage(r domain.Request, flow domain.FlowDefinition, in TransitionInput) (domain.ReturnStage, error) {
	switch in.Trigger {
	case domain.TriggerAnalysisRejected, domain.TriggerPlanRejected:
		forced := domain.ReturnStageAnalysis
		if in.Trigger == domain.TriggerPlanRejected {
			forced = domain.ReturnStagePlan
		}
		if in.Stage != "" && in.Stage != forced {
			return "", domain.ErrReturnStageInvalid(string(in.Stage))
		}
		return forced, nil
	case domain.TriggerReturnToBacklog:
		valid, err := domain.StageForStatus(r.Status, flow, r.Size)
		if err != nil {
			return "", err
		}
		if !domain.ContainsStage(valid, in.Stage) {
			return "", domain.ErrReturnStageInvalid(string(in.Stage))
		}
		return in.Stage, nil
	}
	return "", nil
}

func (uc *TransitionRequest) emit(ctx context.Context, r domain.Request, from domain.RequestStatus, in TransitionInput, stage domain.ReturnStage) error {
	at := uc.clock()
	events := []struct {
		subject string
		payload any
	}{{domain.SubjectRequestStatusChanged, statusChangedPayload(ctx, r, from, in, stage, at)}}
	if r.Status == domain.RequestStatusCompleted {
		events = append(events, struct {
			subject string
			payload any
		}{domain.SubjectRequestCompleted, completedPayload(ctx, r, in, at)})
	}
	for _, e := range events {
		ev, err := NewOutboxEvent(ctx, e.subject, e.payload)
		if err != nil {
			return err
		}
		ev.OccurredAt = at
		if err := uc.outbox.InsertOutboxEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}
