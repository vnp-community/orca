package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewEvents publishes review events through the outbox. A nil value publishes nothing, so a service without a bus still works.
type ReviewEvents struct {
	Outbox OutboxWriter
	Log    *slog.Logger
}

// ReviewEventPayload is what leaves the service: ids and enums, never review text (a subscriber may not be allowed to read it).
type ReviewEventPayload struct {
	RequestID  string `json:"request_id"`
	ReviewID   string `json:"review_id"`
	Phase      string `json:"phase,omitempty"`
	RunID      string `json:"run_id,omitempty"`
	RunKind    string `json:"run_kind,omitempty"`
	RunStatus  string `json:"run_status,omitempty"`
	Decision   string `json:"decision,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
	Approved   bool   `json:"approved,omitempty"`
	InputsHash string `json:"inputs_digest,omitempty"`
}

// Publish is best effort: the review is already saved when it runs, and a lost event only delays the screen until its next read.
// Consumers see an event at least once, so they must treat a repeat as the same fact.
func (e *ReviewEvents) Publish(ctx context.Context, subject string, p ReviewEventPayload) {
	if e == nil || e.Outbox == nil {
		return
	}
	ev, err := NewOutboxEvent(ctx, subject, p)
	if err == nil {
		err = e.Outbox.InsertOutboxEvent(ctx, ev)
	}
	if err != nil && e.Log != nil {
		e.Log.WarnContext(ctx, "outcome review: could not publish an event", slog.String("subject", subject), slog.String("review_id", p.ReviewID), slog.Any("error", err))
	}
}

func (e *ReviewEvents) runProgress(ctx context.Context, run ReviewRun, status ReviewRunStatus, code string) {
	e.Publish(ctx, domain.SubjectReviewProgress, ReviewEventPayload{
		RequestID: run.RequestID, ReviewID: run.ReviewID, RunID: run.ID, RunKind: string(run.Kind), RunStatus: string(status), ErrorCode: code,
	})
}

// runFinished tells that a run ended and, when it produced something, that the review changed.
func (e *ReviewEvents) runFinished(ctx context.Context, run ReviewRun, status ReviewRunStatus, code string) {
	if e == nil {
		return
	}
	e.runProgress(ctx, run, status, code)
	if status != ReviewRunSucceeded {
		return
	}
	subject := domain.SubjectReviewGenerated
	if run.Kind == ReviewRunDraw {
		subject = domain.SubjectReviewArtifactRegenerated
	}
	e.Publish(ctx, subject, ReviewEventPayload{RequestID: run.RequestID, ReviewID: run.ReviewID, RunID: run.ID, RunKind: string(run.Kind)})
}
