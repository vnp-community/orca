package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/httpwebhook"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type intakeWiring struct {
	createRequest *usecase.CreateRequest
	lookup        *usecase.LookupRequestBySource
	closers       []func()
}

// wireRequestIntake builds the intake use cases (CR-REQ-004). The gRPC handlers for CreateRequest
// and LookupRequestBySource attach to these once the merged proto defines them.
func wireRequestIntake(cfg config.Config, log *slog.Logger, stores *requestStores, transition usecase.RequestTransitioner) (*intakeWiring, error) {
	w := &intakeWiring{}
	var fetcher usecase.IssueFetcher = grpcclient.UnavailableIssueFetcher{}
	if cfg.IssueTrackingServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.IssueTrackingServiceAddr)
		if err != nil {
			return nil, err
		}
		w.closers = append(w.closers, func() { _ = conn.Close() })
		fetcher = grpcclient.NewIssueTrackingClient(issuetrackingv1.NewIssueTrackingServiceClient(conn))
	} else {
		log.Warn("ISSUE_TRACKING_SERVICE_ADDR unset: Jira/Linear intake without a title will be rejected")
	}
	w.createRequest = usecase.NewCreateRequest(stores.requests, stores.idempotency, stores.tx, stores.outboxWriter, transition, fetcher).
		WithSecurityFlags(stores.secFlags)
	w.lookup = usecase.NewLookupRequestBySource(stores.requests, stores.idempotency)
	return w, nil
}

// webhookHandler wraps next with the request-webhook route; without REQUEST_WEBHOOK_SOURCES the route is not mounted.
func (w *intakeWiring) webhookHandler(cfg config.Config, log *slog.Logger, next http.Handler, replay httpwebhook.ReplayGuard) (http.Handler, error) {
	sources, err := httpwebhook.ParseStaticSources(cfg.WebhookSourcesJSON, os.Getenv, os.ReadFile)
	if err != nil {
		return nil, fmt.Errorf("webhook sources: %w", err)
	}
	if sources.Len() == 0 {
		return next, nil
	}
	mux := http.NewServeMux()
	httpwebhook.NewHandler(sources, w.createRequest).WithReplayProtection(replay, cfg.WebhookRequireTimestamp).Mount(mux)
	mux.Handle("/", next)
	log.Info("request webhook route enabled", slog.Int("sources", sources.Len()))
	return mux, nil
}

func (w *intakeWiring) close() {
	for _, c := range w.closers {
		c()
	}
}

// attachIntake swaps the lifecycle defaults for the intake-backed ones: children go through
// CreateWithinTx (so they get classified), and reopen resets the AI attempt counter.
func (w *intakeWiring) attachIntake(l *requestLifecycle, stores *requestStores) {
	l.ChildCreator = usecase.NewIntakeChildCreator(w.createRequest)
	l.SpawnChild = usecase.NewSpawnChildRequest(stores.requests, stores.links, l.ChildCreator, stores.tx)
	l.Reopen = usecase.NewReopenRequest(stores.requests, l.Transition, stores.returns, usecase.NewClassificationAttemptsReset(stores.requests), stores.tx)
}
