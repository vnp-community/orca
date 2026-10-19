package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const maxPromptAnswers = 20

// PromptAnswer is one answered question the model may see when it analyzes the Request again.
type PromptAnswer struct {
	Key      string
	Question string
	Answer   string
}

// answeredForPrompt returns what people already answered about open questions of earlier proposals. Answering one
// retires the proposal it was about and starts a new analysis; without this the model would ask the same thing again
// and the blocking question could never be closed. Readiness answers are already part of the Request content.
func answeredForPrompt(ctx context.Context, repo ClarificationRepository, requestID string) []PromptAnswer {
	if repo == nil {
		return nil
	}
	list, err := repo.List(ctx, ClarificationListFilter{RequestID: requestID, Status: domain.ClarificationStatusAnswered, Limit: 200})
	if err != nil {
		slog.WarnContext(ctx, "answered clarifications unreadable; analyzing without them", slog.String("request_id", requestID), slog.Any("error", err))
		return nil // best effort like prior artifacts: the gate still blocks an approval that ignores them
	}
	var out []PromptAnswer
	for _, c := range list {
		if c.Source != domain.ClarificationSourceSolutionOpenQuestion && c.Source != domain.ClarificationSourceManual {
			continue
		}
		for _, q := range c.Questions {
			if q.HasAnswer() {
				out = append(out, PromptAnswer{Key: q.QuestionKey, Question: q.Prompt, Answer: string(q.Answer)})
			}
		}
	}
	if len(out) > maxPromptAnswers {
		out = out[len(out)-maxPromptAnswers:] // the newest answers win
	}
	return out
}
