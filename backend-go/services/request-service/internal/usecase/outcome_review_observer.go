package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewObserver receives the numbers the Outcome Review is judged by. Labels are small fixed sets; nothing a review says goes in.
type ReviewObserver interface {
	// ReviewContextLookup is one search of the checked-result cache; the hit ratio is hits over all lookups.
	ReviewContextLookup(hit bool)
	// ReviewRunEnded is one run that ended and how long it ran.
	ReviewRunEnded(kind, result string, d time.Duration)
	// ReviewTokens is what the agent reported for one run; not called when it reported none.
	ReviewTokens(kind string, input, output int64)
	// ReviewCostEstimateError is |actual - estimate| / estimate for a run whose cost could be predicted from the one before it.
	ReviewCostEstimateError(kind string, ratio float64)
	// ReviewDrawn is the outcome of the diagrams of one draw run; code names why the failed ones failed.
	ReviewDrawn(drawn, failed int, code string)
	// ReviewRegenerated is one regeneration a person asked for, by reason.
	ReviewRegenerated(reason string)
	// ReviewFirstFacts is the time from asking until the facts Orca measures were stored.
	ReviewFirstFacts(d time.Duration)
	// ReviewItems counts the items of a review that came from the agent and how many of them could not be checked.
	ReviewItems(total, unverified int)
	// ReviewDesignReviewed is the share of the graph elements read that a person approved (AM-3).
	ReviewDesignReviewed(ratio float64)
}

type noopReviewObserver struct{}

func (noopReviewObserver) ReviewContextLookup(bool)                     {}
func (noopReviewObserver) ReviewRunEnded(string, string, time.Duration) {}
func (noopReviewObserver) ReviewTokens(string, int64, int64)            {}
func (noopReviewObserver) ReviewCostEstimateError(string, float64)      {}
func (noopReviewObserver) ReviewDrawn(int, int, string)                 {}
func (noopReviewObserver) ReviewRegenerated(string)                     {}
func (noopReviewObserver) ReviewFirstFacts(time.Duration)               {}
func (noopReviewObserver) ReviewItems(int, int)                         {}
func (noopReviewObserver) ReviewDesignReviewed(float64)                 {}

// reviewObserverOf finds the review half of an observer; an observer that has none, or no observer, reads as a no-op.
func reviewObserverOf(o EngineObserver) ReviewObserver {
	if ro, ok := o.(ReviewObserver); ok {
		return ro
	}
	return noopReviewObserver{}
}

// logReviewAudit writes the line a log reader filters on (audit=true). It names the inputs, never the content.
func logReviewAudit(ctx context.Context, log *slog.Logger, action, requestID, reviewID, inputsDigest string) {
	if log == nil {
		return
	}
	log.InfoContext(ctx, "outcome review", slog.Bool("audit", true), slog.String("action", action), slog.String("request_id", requestID),
		slog.String("review_id", reviewID), slog.String("inputs_digest", inputsDigest))
}

// runDuration is how long a run took; zero when the repository did not stamp its start.
func runDuration(run ReviewRun, now time.Time) time.Duration {
	if run.StartedAt.IsZero() || now.Before(run.StartedAt) {
		return 0
	}
	return now.Sub(run.StartedAt)
}

// observeRunEnd reports a run that ended: its duration, its tokens when the agent gave them, and how far off the prediction was.
func observeRunEnd(o ReviewObserver, run ReviewRun, status ReviewRunStatus, now time.Time, usage *domain.RunUsage, cost string) {
	o.ReviewRunEnded(string(run.Kind), string(status), runDuration(run, now))
	if usage != nil && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		o.ReviewTokens(string(run.Kind), usage.InputTokens, usage.OutputTokens)
	}
	est, eok := parseCostUSD(run.CostEstimate)
	act, aok := parseCostUSD(cost)
	if status == ReviewRunSucceeded && eok && aok && est > 0 {
		diff := act - est
		if diff < 0 {
			diff = -diff
		}
		o.ReviewCostEstimateError(string(run.Kind), diff/est)
	}
}

// predictedCost is the cost of the last run of this kind that succeeded and said what it cost: the same basis the estimate a
// person is shown uses. Empty when there is none.
func predictedCost(ctx context.Context, runs OutcomeReviewRunRepository, reviewID string, kind ReviewRunKind) string {
	list, err := runs.ListByReview(ctx, reviewID)
	if err != nil {
		return ""
	}
	for _, run := range list { // newest first
		if run.Kind == kind && run.Status == ReviewRunSucceeded {
			if _, ok := parseCostUSD(run.CostActual); ok {
				return run.CostActual
			}
		}
	}
	return ""
}
