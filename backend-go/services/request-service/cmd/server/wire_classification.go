package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	eventbusadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type classificationWiring struct {
	runner  *usecase.ClassificationRunner
	confirm *usecase.ConfirmRequestType
	change  *usecase.ChangeRequestType
	history *usecase.ListRequestTypeHistory
	closers []func()
}

// wireClassification builds classification (CR-REQ-005) and starts its background parts: the
// status_changed consumer (needs NATS), the lease-recovery sweeper and the processed_events prune.
// The Confirm/Change/History/Classify RPC handlers attach to these after the merged proto lands.
func wireClassification(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores,
	transition usecase.RequestTransitioner, approvals usecase.ApprovalRecorder, bus *commoneventbus.Consumer) (*classificationWiring, error) {
	w := &classificationWiring{}

	var classifier usecase.RequestClassifier = grpcclient.UnavailableClassifier{}
	if cfg.InfraFleetServiceAddr != "" && cfg.ProjectServiceAddr != "" {
		infraConn, err := grpcclient.Dial(cfg.InfraFleetServiceAddr)
		if err != nil {
			return nil, err
		}
		projectConn, err := grpcclient.Dial(cfg.ProjectServiceAddr)
		if err != nil {
			_ = infraConn.Close()
			return nil, err
		}
		w.closers = append(w.closers, func() { _ = infraConn.Close(); _ = projectConn.Close() })
		infra := infrafleetv1.NewInfraFleetServiceClient(infraConn)
		var providers aiproviderv1.AiProviderServiceClient
		if cfg.AIProviderServiceAddr != "" {
			aiConn, err := grpcclient.Dial(cfg.AIProviderServiceAddr)
			if err != nil {
				return nil, err
			}
			w.closers = append(w.closers, func() { _ = aiConn.Close() })
			providers = aiproviderv1.NewAiProviderServiceClient(aiConn)
		}
		resolver := grpcclient.NewAIConnectionResolver(infra, projectv1.NewProjectServiceClient(projectConn))
		classifier = grpcclient.NewRelayClassifier(infra, resolver, providers)
	} else {
		log.Warn("INFRA_FLEET_SERVICE_ADDR or PROJECT_SERVICE_ADDR unset: AI classification will always fall back to manual type confirmation")
	}

	propose := usecase.NewProposeRequestClassification(stores.requests, stores.history, stores.processed, classifier, transition,
		approvals, stores.classRuns, stores.tx, stores.outboxWriter)
	if stores.observer != nil {
		propose.WithObserver(stores.observer)
	}
	host, _ := os.Hostname()
	owner := fmt.Sprintf("%s-%d", host, os.Getpid())
	w.runner = usecase.NewClassificationRunner(stores.requests, stores.classRuns, propose, stores.tx, owner, 0)
	w.confirm = usecase.NewConfirmRequestType(stores.requests, stores.history, transition, approvals, stores.tx, stores.outboxWriter)
	w.change = usecase.NewChangeRequestType(stores.requests, stores.history, transition, usecase.NoopApprovalCanceller{}, usecase.NoopExecutionGuard{}, stores.tx, stores.outboxWriter)
	w.history = usecase.NewListRequestTypeHistory(stores.requests, stores.history)

	wg.Add(1)
	go func() { defer wg.Done(); w.runner.RecoverLoop(ctx, 30*time.Second) }()
	wg.Add(1)
	go func() { defer wg.Done(); eventbusadapter.RunPruneLoop(ctx, stores.processed, 0, 0, nil) }()
	if bus != nil {
		consumer := eventbusadapter.NewClassificationConsumer(bus, w.runner)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := consumer.Run(ctx); err != nil {
				log.Warn("classification consumer stopped", slog.Any("error", err))
			}
		}()
	} else {
		log.Warn("eventbus unavailable: classification starts only through ClassifyRequest")
	}
	return w, nil
}

func (w *classificationWiring) close() {
	w.runner.Close()
	for _, c := range w.closers {
		c()
	}
}
