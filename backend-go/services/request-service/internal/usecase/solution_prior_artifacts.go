package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// priorArtifacts are the superseded and rejected attempts shown to the model, oldest first; the draft being filled is
// excluded. Each carries the reviewer feedback its generation run was asked with, so a later round sees every earlier
// request for changes (CR-REQ-041).
func priorArtifacts(ctx context.Context, store SolutionStore, requestID string, kind domain.SolutionKind, exceptID string) []PriorArtifact {
	all, err := store.ListByRequest(ctx, SolutionListFilter{RequestID: requestID})
	if err != nil {
		return nil // best effort: a missing history only costs quality
	}
	var picked []domain.Solution
	var runIDs []string
	for _, s := range all {
		if s.ID == exceptID || (s.Status != domain.SolutionStatusSuperseded && s.Status != domain.SolutionStatusRejected) {
			continue
		}
		picked = append(picked, s)
		runIDs = append(runIDs, s.GenerationRunID)
	}
	if len(picked) == 0 {
		return nil
	}
	feedback, err := store.RunFeedbackByIDs(ctx, runIDs)
	if err != nil {
		// Log the failure, never the feedback: the history still helps without it and the current feedback is passed separately.
		slog.WarnContext(ctx, "earlier reviewer feedback unreadable; prompting without it", slog.String("request_id", requestID), slog.Any("error", err))
		feedback = nil
	}
	out := make([]PriorArtifact, 0, len(picked))
	for _, s := range picked {
		out = append(out, PriorArtifact{Kind: string(s.Kind), Status: string(s.Status), Content: string(s.OptionsJSON), Feedback: feedback[s.GenerationRunID]})
	}
	return out
}
