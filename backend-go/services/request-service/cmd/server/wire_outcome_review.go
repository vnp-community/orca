package main

import (
	"log/slog"
	"os"

	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// loadOutcomeReviewConfig reads the deployment default and the kill switch once, for the review and for the readiness check.
func loadOutcomeReviewConfig(log *slog.Logger) domain.OutcomeReviewConfig {
	cfg, warnings := domain.OutcomeReviewConfigFromEnv(os.Getenv)
	for _, w := range warnings {
		log.Warn(w)
	}
	return cfg
}

// wireOutcomeReview builds the Outcome Review (CR-OR-001). It needs nothing the service does not already have: without
// git-gateway a delivered review is built without its diff and says so in its warnings, and the feature stays off unless the
// deployment or the project turns it on.
func wireOutcomeReview(log *slog.Logger, cfg domain.OutcomeReviewConfig, stores *requestStores, tasks *executionTasks, solution *solutionWiring, settings usecase.ReviewSettingsReader, audit usecase.ReviewAuditor) requestgrpc.OutcomeReviewUseCases {
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
	uc := requestgrpc.OutcomeReviewUseCases{Service: svc}
	if solution.conns != nil && solution.agent != nil && solution.caps != nil && solution.infra != nil {
		runner := &usecase.ReviewContextRunner{
			Service: svc, Runs: stores.reviewRuns, Contexts: stores.reviewContexts, Conns: solution.conns, Agent: solution.agent, Caps: solution.caps,
			Config: cfg, Settings: solution.settings, Projects: solution.projects, Observer: solution.observer, Log: log,
		}
		if solution.git != nil {
			runner.Probe = grpcclient.NewRepoStateProbe(solution.git)
			if solution.proj != nil {
				runner.Files = grpcclient.NewProposalWorkspace(solution.git, solution.proj)
			}
		}
		uc.Context = runner
	} else {
		log.Warn("outcome review: INFRA_FLEET_SERVICE_ADDR and PROJECT_SERVICE_ADDR unset, the context run is off; reviews are built from the facts alone")
	}
	log.Info("outcome review wired", slog.String("default_mode", string(cfg.Default)), slog.Bool("kill_switch", cfg.KillSwitch), slog.Bool("context_run", uc.Context != nil))
	return uc
}
