package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type WaiveInput struct {
	RequestID       string
	Reason          string
	ExpectedVersion int64
}

type WaiveResult struct {
	Request         domain.Request
	RequestRevision int
}

// WaiveReadiness lets an admin skip the Definition of Ready for one request. It is refused for hotfix,
// security and ops_request, where missing data does real harm. The waiver is recorded as a revision whose
// meta carries it, so ConfirmRequestType and the audit trail both find it.
type WaiveReadiness struct {
	repo           RequestRepository
	clarifications ClarificationRepository
	appendRevision *AppendRequestRevision
	transition     RequestTransitioner
	tx             TxRunner
	outbox         OutboxWriter
	clock          func() time.Time
}

func NewWaiveReadiness(repo RequestRepository, clarifications ClarificationRepository, appendRevision *AppendRequestRevision,
	transition RequestTransitioner, tx TxRunner, outbox OutboxWriter) *WaiveReadiness {
	return &WaiveReadiness{repo: repo, clarifications: clarifications, appendRevision: appendRevision, transition: transition,
		tx: tx, outbox: outbox, clock: func() time.Time { return time.Now().UTC() }}
}

func waiveForbiddenType(t domain.RequestType) bool {
	return t == domain.RequestTypeHotfix || t == domain.RequestTypeSecurity || t == domain.RequestTypeOpsRequest
}

func (uc *WaiveReadiness) Execute(ctx context.Context, in WaiveInput) (WaiveResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return WaiveResult{}, domain.ErrRequestTenantRequired()
	}
	if !callerIsAdmin(ctx) || callerIsMachine(ctx) {
		return WaiveResult{}, domain.ErrReadinessWaiveForbidden("only an admin may waive the Definition of Ready")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return WaiveResult{}, domain.ErrReasonRequired()
	}
	var out WaiveResult
	err := uc.tx.InTx(ctx, func(txCtx context.Context) error {
		r, err := uc.repo.Get(txCtx, in.RequestID)
		if err != nil {
			return err
		}
		if waiveForbiddenType(r.Type) {
			return domain.ErrReadinessWaiveForbidden("readiness cannot be waived for " + string(r.Type))
		}
		if in.ExpectedVersion != 0 && in.ExpectedVersion != r.Version {
			return domain.ErrRequestVersionConflict(r.ID, in.ExpectedVersion)
		}
		var open *domain.Clarification
		switch r.Status {
		case domain.RequestStatusAwaitingTypeConfirmation:
		case domain.RequestStatusAwaitingInformation:
			if open, err = uc.clarifications.GetOpenByRequest(txCtx, r.ID); err != nil {
				return err
			}
			if open == nil || open.Source != domain.ClarificationSourceReadiness {
				return domain.ErrClarificationStateNotAllowed("only a readiness clarification can be waived")
			}
		default:
			return domain.ErrClarificationStateNotAllowed("readiness can be waived only before analysis starts")
		}
		content, err := domain.ContentFromRequest(r)
		if err != nil {
			return err
		}
		now := uc.clock()
		res, err := uc.appendRevision.AppendWithinTx(txCtx, AppendInput{
			RequestID: r.ID, Content: content, Cause: domain.RevisionCauseEdited, ActorID: callerID(txCtx), ActorKind: domain.ActorKindUser,
			ExpectedVersion: r.Version,
			Meta:            map[string]any{"waiver": map[string]any{"kind": "readiness", "reason": reason, "by": callerID(txCtx), "at": now.Format(time.RFC3339)}},
		})
		if err != nil {
			return err
		}
		out = WaiveResult{Request: res.Request, RequestRevision: res.Request.ContentRevision}
		if open == nil {
			return nil // still awaiting type confirmation: ConfirmRequestType will see the waiver and move on
		}
		if err := uc.clarifications.MarkCancelled(txCtx, open.ID, "waived", now, 0); err != nil {
			return err
		}
		ev, err := NewOutboxEvent(txCtx, domain.SubjectClarificationCancelled, ClarificationEventPayload{
			ClarificationID: open.ID, RequestID: r.ID, DisplayID: open.DisplayID(r.Number), Reason: "waived",
		})
		if err != nil {
			return err
		}
		if err := uc.outbox.InsertOutboxEvent(txCtx, ev); err != nil {
			return err
		}
		from := domain.RequestStatusAwaitingInformation
		tr, err := uc.transition.Execute(txCtx, TransitionInput{
			RequestID: r.ID, Trigger: domain.TriggerInformationProvided, ExpectedFrom: &from, ResumeStatus: open.ResumeStatus,
			ActorID: callerID(txCtx), ActorKind: domain.ActorKindUser, Reason: "readiness waived",
		})
		if err != nil {
			return err
		}
		out.Request = tr.Request
		return nil
	})
	return out, err
}
