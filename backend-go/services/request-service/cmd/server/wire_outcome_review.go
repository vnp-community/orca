package main

import (
	"log/slog"
	"os"

	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// wireOutcomeReview builds the Outcome Review (CR-OR-001). It needs nothing the service does not already have: without
// git-gateway a delivered review is built without its diff and says so in its warnings, and the feature stays off unless the
// deployment or the project turns it on.
func wireOutcomeReview(log *slog.Logger, stores *requestStores, tasks *executionTasks, solution *solutionWiring, settings usecase.ReviewSettingsReader, audit usecase.ReviewAuditor) requestgrpc.OutcomeReviewUseCases {
	cfg, warnings := domain.OutcomeReviewConfigFromEnv(os.Getenv)
	for _, w := range warnings {
		log.Warn(w)
	}
	collector := &usecase.ReviewFactsCollector{
		Tasks: tasks.client, Outcomes: stores.execution.outcomes, Coverage: stores.artifact.coverage, Log: log,
	}
	if solution.git != nil {
		collector.Diff = grpcclient.NewGitReviewDiffReader(solution.git)
	} else {
		log.Warn("GIT_GATEWAY_SERVICE_ADDR unset: a delivered review is built without the diff")
	}
	svc := &usecase.OutcomeReviewService{
		Requests: stores.requests, Reviews: stores.outcomeReviews, Collector: collector, Settings: settings, Config: cfg,
		Redact: usecase.RedactRaw, Audit: audit, Log: log,
	}
	log.Info("outcome review wired", slog.String("default_mode", string(cfg.Default)), slog.Bool("kill_switch", cfg.KillSwitch))
	return requestgrpc.OutcomeReviewUseCases{Service: svc}
}
