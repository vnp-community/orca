package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ConfirmDecisionInput struct {
	DecisionID       string
	ConfirmationText string
	ExpectedVersion  int64
}

// ConfirmDecision is the second step for a high-risk choice: the chooser (or an admin) types the option's
// title. A machine identity cannot do it, and there is deliberately no MCP tool for this use case.
type ConfirmDecision struct {
	repo      RequestRepository
	decisions DecisionRepository
	tx        TxRunner
	outbox    OutboxWriter
	clock     func() time.Time
}

func NewConfirmDecision(repo RequestRepository, decisions DecisionRepository, tx TxRunner, outbox OutboxWriter) *ConfirmDecision {
	return &ConfirmDecision{repo: repo, decisions: decisions, tx: tx, outbox: outbox, clock: func() time.Time { return time.Now().UTC() }}
}

func (uc *ConfirmDecision) Execute(ctx context.Context, in ConfirmDecisionInput) (domain.Decision, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Decision{}, domain.ErrRequestTenantRequired()
	}
	if callerIsMachine(ctx) {
		return domain.Decision{}, domain.ErrDecisionAgentForbidden()
	}
	var out domain.Decision
	err := uc.tx.InTx(ctx, func(txCtx context.Context) error {
		d, err := uc.decisions.Get(txCtx, in.DecisionID)
		if err != nil {
			return err
		}
		r, err := uc.repo.Get(txCtx, d.RequestID)
		if err != nil {
			return domain.ErrDecisionNotFound(in.DecisionID)
		}
		me := callerID(txCtx)
		if !callerIsAdmin(txCtx) && (me == "" || me != d.ChooserID) {
			return errRequestForbidden("only the person who chose, or an admin, may confirm this decision")
		}
		if d.Status == domain.DecisionStatusEffective {
			out = d // already in force (confirmed earlier, or never needed a confirmation): a repeat is a no-op
			return nil
		}
		if in.ExpectedVersion != 0 && in.ExpectedVersion != d.Version {
			return domain.ErrDecisionVersionConflict(d.ID, in.ExpectedVersion)
		}
		now := uc.clock()
		if err := d.Confirm(me, in.ConfirmationText, now); err != nil {
			return err
		}
		saved, err := uc.decisions.Update(txCtx, d, d.Version)
		if err != nil {
			return err
		}
		if err := uc.decisions.AppendHistory(txCtx, domain.DecisionHistory{
			ID: uuid.NewString(), DecisionID: saved.ID, Action: domain.DecisionActionConfirmed, OptionID: saved.ChosenOptionID, ActorID: me, At: now,
		}); err != nil {
			return err
		}
		ev, err := NewOutboxEvent(txCtx, domain.SubjectDecisionConfirmed, DecisionPayload{
			DecisionID: saved.ID, RequestID: saved.RequestID, DisplayID: saved.DisplayID(r.Number), SubjectID: saved.SubjectID,
			RiskLevel: string(saved.RiskLevel), Status: string(saved.Status),
		})
		if err != nil {
			return err
		}
		ev.OccurredAt = now
		out = saved
		return uc.outbox.InsertOutboxEvent(txCtx, ev)
	})
	return out, err
}

// DecisionQueries serves ListDecisions and GetDecision.
type DecisionQueries struct {
	repo      RequestRepository
	decisions DecisionRepository
}

func NewDecisionQueries(repo RequestRepository, decisions DecisionRepository) *DecisionQueries {
	return &DecisionQueries{repo: repo, decisions: decisions}
}

func (q *DecisionQueries) Get(ctx context.Context, id string) (domain.Decision, int64, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Decision{}, 0, domain.ErrRequestTenantRequired()
	}
	d, err := q.decisions.Get(ctx, id)
	if err != nil {
		return domain.Decision{}, 0, err
	}
	r, err := q.repo.Get(ctx, d.RequestID)
	if err != nil {
		return domain.Decision{}, 0, domain.ErrDecisionNotFound(id)
	}
	return d, r.Number, nil
}

type DecisionPage struct {
	Items         []domain.Decision
	RequestNumber int64
	NextAfter     int
}

func (q *DecisionQueries) List(ctx context.Context, requestID string, status domain.DecisionStatus, afterSeq, pageSize int) (DecisionPage, error) {
	r, err := loadReadableRequest(ctx, q.repo, requestID)
	if err != nil {
		return DecisionPage{}, err
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	got, err := q.decisions.List(ctx, DecisionListFilter{RequestID: requestID, Status: status, AfterSeq: afterSeq, Limit: pageSize + 1})
	if err != nil {
		return DecisionPage{}, err
	}
	page := DecisionPage{RequestNumber: r.Number}
	if len(got) > pageSize {
		got = got[:pageSize]
		page.NextAfter = got[pageSize-1].Seq
	}
	page.Items = got
	return page, nil
}
