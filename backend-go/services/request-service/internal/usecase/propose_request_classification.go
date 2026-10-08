package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ProposeInput struct {
	RequestID string
	EventID   string // status_changed envelope id; empty for manual runs
	Trigger   string
	Manual    bool
	RunID     string // classification_runs row finished in the same transaction as the result
}

type ProposeRequestClassification struct {
	repo       RequestRepository
	history    RequestTypeHistoryRepository
	processed  ProcessedEventRepository
	classifier RequestClassifier
	transition RequestTransitioner
	approvals  ApprovalRecorder
	runs       ClassificationRunRepository
	tx         TxRunner
	outbox     OutboxWriter
	observer   RequestObserver
}

// WithObserver plugs the AI latency metric; the default observes nothing.
func (uc *ProposeRequestClassification) WithObserver(o RequestObserver) *ProposeRequestClassification {
	uc.observer = o
	return uc
}

func NewProposeRequestClassification(repo RequestRepository, history RequestTypeHistoryRepository, processed ProcessedEventRepository,
	classifier RequestClassifier, transition RequestTransitioner, approvals ApprovalRecorder, runs ClassificationRunRepository,
	tx TxRunner, outbox OutboxWriter) *ProposeRequestClassification {
	return &ProposeRequestClassification{repo: repo, history: history, processed: processed, classifier: classifier,
		transition: transition, approvals: approvals, runs: runs, tx: tx, outbox: outbox}
}

func (uc *ProposeRequestClassification) observeAI(err error, elapsed time.Duration) {
	if uc.observer == nil {
		return
	}
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	uc.observer.ObserveAIGeneration("classification", outcome, elapsed)
}

func classifiable(s domain.RequestStatus) bool {
	return s == domain.RequestStatusClassifying || s == domain.RequestStatusAwaitingTypeConfirmation
}

// Validate is the cheap pre-check shared by the manual RPC and the background run.
func (uc *ProposeRequestClassification) Validate(r domain.Request, manual bool) error {
	if !classifiable(r.Status) {
		return domain.ErrRequestNotClassifiable(r.Status)
	}
	if manual && r.ClassificationAttempts >= domain.MaxClassificationAttempts {
		return domain.ErrRequestClassificationLimit()
	}
	return nil
}

// Execute runs the AI outside any transaction, then writes the outcome, the processed-event
// marker and the run's end state atomically, so a redelivery can never record two proposals.
func (uc *ProposeRequestClassification) Execute(ctx context.Context, in ProposeInput) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ErrRequestTenantRequired()
	}
	r, err := uc.repo.Get(ctx, in.RequestID)
	if err != nil {
		return err
	}
	if err := uc.Validate(r, in.Manual); err != nil {
		if in.Manual {
			return err
		}
		if !classifiable(r.Status) {
			return uc.finishOnly(ctx, in.RunID, domain.ClassificationRunSucceeded, "")
		}
	}
	limitReached := r.ClassificationAttempts >= domain.MaxClassificationAttempts

	var proposal domain.ClassificationProposal
	var classifyErr error
	if limitReached {
		classifyErr = errClassificationLimit
	} else {
		aiCtx := tenant.WithUserID(ctx, r.ReporterID)
		started := time.Now()
		proposal, classifyErr = uc.classifier.Classify(aiCtx, ClassificationInput{
			RequestID: r.ID, ProjectID: r.ProjectID, Title: r.Title, Body: r.Body,
			IssueType: r.SourceHints.IssueType, Labels: r.SourceHints.Labels,
		})
		uc.observeAI(classifyErr, time.Since(started))
	}

	return uc.tx.InTx(ctx, func(txCtx context.Context) error {
		if in.EventID != "" {
			already, err := uc.processed.MarkProcessed(txCtx, in.EventID, domain.SubjectRequestStatusChanged)
			if err != nil {
				return err
			}
			if already {
				return uc.finishRun(txCtx, in.RunID, domain.ClassificationRunSucceeded, "")
			}
		}
		cur, err := uc.repo.Get(txCtx, in.RequestID)
		if err != nil {
			return err
		}
		if !classifiable(cur.Status) {
			// The request moved on while the AI ran (reopen, cancel, ...): the result is stale.
			return uc.finishRun(txCtx, in.RunID, domain.ClassificationRunSucceeded, "")
		}
		failed := classifyErr != nil
		hadProposal := cur.Type != ""
		next := cur
		if failed {
			if !hadProposal {
				next.ClassificationReason = failureReason(classifyErr)
			}
			if !errors.Is(classifyErr, errClassificationLimit) {
				next.ClassificationAttempts++
			}
		} else {
			conf := proposal.Confidence
			next.Type, next.Size, next.Urgency = proposal.Type, proposal.Size, proposal.Urgency
			next.Confidence, next.ClassificationReason = &conf, proposal.Reason
			next.TypeSource = domain.TypeSourceAI
			next.ClassificationAttempts++
		}
		if _, err := uc.repo.Update(txCtx, next, cur.Version); err != nil {
			return err
		}
		if !failed {
			if err := uc.history.Append(txCtx, domain.RequestTypeChange{
				RequestID: cur.ID, FromType: cur.Type, ToType: proposal.Type,
				ActorID: uuid.Nil.String(), ActorKind: domain.ActorKindAgent, Reason: proposal.Reason,
			}); err != nil {
				return err
			}
		}
		if cur.Status == domain.RequestStatusClassifying {
			from := domain.RequestStatusClassifying
			if _, err := uc.transition.Execute(txCtx, TransitionInput{
				RequestID: cur.ID, Trigger: domain.TriggerProposalReady, ExpectedFrom: &from,
				ActorID: uuid.Nil.String(), ActorKind: domain.ActorKindAgent,
			}); err != nil {
				return err
			}
		}
		if !failed {
			if err := uc.approvals.RequestTypeApproval(txCtx, cur.ID); err != nil {
				return err
			}
		}
		payload := map[string]any{"request_id": cur.ID, "failed": failed}
		if !failed {
			payload["type"], payload["size"], payload["urgency"], payload["confidence"] = proposal.Type, proposal.Size, proposal.Urgency, proposal.Confidence
		}
		ev, err := NewOutboxEvent(txCtx, domain.SubjectRequestClassified, payload)
		if err != nil {
			return err
		}
		if err := uc.outbox.InsertOutboxEvent(txCtx, ev); err != nil {
			return err
		}
		if failed {
			slog.WarnContext(txCtx, "request classification failed", slog.String("request_id", cur.ID), slog.String("reason", failureReason(classifyErr)))
			return uc.finishRun(txCtx, in.RunID, domain.ClassificationRunFailed, failureCode(classifyErr))
		}
		return uc.finishRun(txCtx, in.RunID, domain.ClassificationRunSucceeded, "")
	})
}

var errClassificationLimit = errors.New("classification limit")

func failureReason(err error) string {
	var s string
	switch {
	case errors.Is(err, errClassificationLimit):
		s = "classification limit"
	case errors.Is(err, ErrNoDevServer):
		s = "no dev server connected"
	case errors.Is(err, domain.ErrProposalInvalid):
		s = "invalid classifier output"
	case errors.Is(err, ErrClassifierTimeout), errors.Is(err, context.DeadlineExceeded):
		s = "classifier timeout"
	default:
		s = "classifier error"
	}
	if utf8.RuneCountInString(s) > 500 {
		s = string([]rune(s)[:500])
	}
	return s
}

func failureCode(err error) string {
	switch {
	case errors.Is(err, errClassificationLimit):
		return "CLASSIFICATION_LIMIT"
	case errors.Is(err, ErrNoDevServer):
		return "NO_DEV_SERVER"
	case errors.Is(err, domain.ErrProposalInvalid):
		return "PROPOSAL_INVALID"
	case errors.Is(err, ErrClassifierTimeout), errors.Is(err, context.DeadlineExceeded):
		return "CLASSIFIER_TIMEOUT"
	}
	return "CLASSIFIER_ERROR"
}

func (uc *ProposeRequestClassification) finishRun(ctx context.Context, runID string, st domain.ClassificationRunStatus, code string) error {
	if runID == "" || uc.runs == nil {
		return nil
	}
	return uc.runs.Finish(ctx, runID, st, code)
}

func (uc *ProposeRequestClassification) finishOnly(ctx context.Context, runID string, st domain.ClassificationRunStatus, code string) error {
	return uc.tx.InTx(ctx, func(txCtx context.Context) error { return uc.finishRun(txCtx, runID, st, code) })
}
