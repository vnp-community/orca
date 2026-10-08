package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ErrResumeSkipped marks a resume that will not succeed by retrying (for example no dev server to run the
// analysis); the person triggers it again from the UI.
var ErrResumeSkipped = errors.New("resume skipped")

// AnalysisStarter regenerates the Solution after a clarification, with the clarification as feedback.
// ExecutionAdvancer asks the executor to carry on. Both default to "log and skip" until the owning features wire them.
type AnalysisStarter interface {
	StartSolution(ctx context.Context, requestID, feedback, idempotencyKey string) error
}

// DeferredAnalysisStarter is an AnalysisStarter whose background work must wait for the transaction to commit:
// the worker reads rows the transaction just wrote. The returned func runs after a successful commit.
type DeferredAnalysisStarter interface {
	StartSolutionDeferred(ctx context.Context, requestID, feedback, idempotencyKey string) (after func(), err error)
}

type ExecutionAdvancer interface {
	Advance(ctx context.Context, requestID string) error
}

type unwiredResume struct{ what string }

func (u unwiredResume) StartSolution(ctx context.Context, requestID, _, _ string) error {
	slog.WarnContext(ctx, "resume: solution generation is not wired; the person must start it", slog.String("request_id", requestID))
	return nil
}

func (u unwiredResume) Advance(ctx context.Context, requestID string) error {
	slog.WarnContext(ctx, "resume: execution advance is not wired", slog.String("request_id", requestID))
	return nil
}

// ResumeEvent is the part of orca.request.request.status_changed the resume reads.
type ResumeEvent struct {
	ID        string
	TenantID  string
	RequestID string
	To        string
	Trigger   string
}

// ResumeAfterClarification restarts the stage that was waiting. Redelivery is safe: the processed_events
// marker commits with the work, so a failed attempt is retried and a finished one never repeats.
type ResumeAfterClarification struct {
	clarifications ClarificationRepository
	processed      ProcessedEventRepository
	analysis       AnalysisStarter
	execution      ExecutionAdvancer
	tx             TxRunner
	autoRegenerate bool
}

func NewResumeAfterClarification(clarifications ClarificationRepository, processed ProcessedEventRepository, tx TxRunner) *ResumeAfterClarification {
	u := unwiredResume{}
	return &ResumeAfterClarification{clarifications: clarifications, processed: processed, analysis: u, execution: u, tx: tx, autoRegenerate: true}
}

func (uc *ResumeAfterClarification) WithAnalysis(a AnalysisStarter) *ResumeAfterClarification {
	uc.analysis = a
	return uc
}

func (uc *ResumeAfterClarification) WithExecution(e ExecutionAdvancer) *ResumeAfterClarification {
	uc.execution = e
	return uc
}

// WithAutoRegenerate(false) leaves analysis to the person (REQUEST_CLARIFICATION_AUTO_REGENERATE).
func (uc *ResumeAfterClarification) WithAutoRegenerate(on bool) *ResumeAfterClarification {
	uc.autoRegenerate = on
	return uc
}

func (uc *ResumeAfterClarification) Handle(ctx context.Context, ev ResumeEvent) error {
	if ev.Trigger != string(domain.TriggerInformationProvided) || ev.RequestID == "" {
		return nil
	}
	var after func()
	err := uc.tx.InTx(ctx, func(txCtx context.Context) error {
		after = nil
		already, err := uc.processed.MarkProcessed(txCtx, ev.ID, domain.SubjectRequestStatusChanged)
		if err != nil || already {
			return err
		}
		var act error
		switch domain.RequestStatus(ev.To) {
		case domain.RequestStatusAnalyzing:
			if !uc.autoRegenerate {
				return nil
			}
			id, err := uc.latestAnswered(txCtx, ev.RequestID)
			if err != nil {
				return err
			}
			if id == "" {
				id = ev.ID // information came without a clarification (a waiver): the event itself keys the run
			}
			if d, ok := uc.analysis.(DeferredAnalysisStarter); ok {
				after, act = d.StartSolutionDeferred(txCtx, ev.RequestID, "clarification:"+id, "clr-"+id)
			} else {
				act = uc.analysis.StartSolution(txCtx, ev.RequestID, "clarification:"+id, "clr-"+id)
			}
		case domain.RequestStatusExecuting:
			act = uc.execution.Advance(txCtx, ev.RequestID)
		default:
			return nil // planning is never started on the person's behalf
		}
		if errors.Is(act, ErrResumeSkipped) {
			slog.WarnContext(txCtx, "resume skipped", slog.String("request_id", ev.RequestID), slog.Any("reason", act))
			return nil
		}
		return act
	})
	if err == nil && after != nil {
		after()
	}
	return err
}

func (uc *ResumeAfterClarification) latestAnswered(ctx context.Context, requestID string) (string, error) {
	list, err := uc.clarifications.List(ctx, ClarificationListFilter{RequestID: requestID, Status: domain.ClarificationStatusAnswered, Limit: 200})
	if err != nil || len(list) == 0 {
		return "", err
	}
	return list[len(list)-1].ID, nil
}
