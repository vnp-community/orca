package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/secretscan"
	eventbusadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/eventbus"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// secretRedactor masks secrets in everything sent to the dev server (the same masking every prompt gets); it never masks nothing.
type secretRedactor struct{}

func (secretRedactor) Apply(_ context.Context, text string) string {
	out, _ := secretscan.Redact(text)
	return out
}

// wireTestSpecs builds the test-spec lane (docs/integrate/TDD-REQ-TESTSPEC-001): next to the code-execution lane, sharing only the
// event that starts it and the dev server. Its handlers are always returned (the screen asks whether the lane is on); the
// consumer and the reconcile loop start only when REQUEST_TESTSPEC_ENABLED is true and the dev server clients exist.
func wireTestSpecs(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores, solution *solutionWiring,
	execution *executionWiring, bus *commoneventbus.Consumer) requestgrpc.TestSpecUseCases {
	enabled := cfg.TestSpecEnabled
	if enabled && solution.infra == nil {
		log.Warn("REQUEST_TESTSPEC_ENABLED is true but INFRA_FLEET_SERVICE_ADDR or PROJECT_SERVICE_ADDR is unset: test specs stay off")
		enabled = false
	}
	settings := usecase.TestSpecSettingsFunc(func(context.Context, string) bool { return enabled })
	var agent usecase.TestSpecAgent = unavailableTestSpecAgent{}
	if solution.infra != nil {
		agent = grpcclient.NewTddAgentMCP(solution.infra)
	}
	builder := &usecase.TestSpecContextBuilder{Solutions: stores.solutions, Changes: stores.openspecChanges, Redactor: secretRedactor{}}
	start := &usecase.StartTestSpecs{
		Requests: stores.requests, Runs: stores.testSpecRuns, Agent: agent, Resolver: solution.conns, Context: builder,
		Actors: execution.Actors, Settings: settings, Log: log,
		// The join may be holding the Request's first dispatch; try it again now that the lane has finished.
		OnFinished: func(ctx context.Context, requestID string) {
			if _, err := execution.StartExecution.Execute(ctx, requestID); err != nil {
				log.Warn("test specs: could not start execution after the lane finished", slog.String("request_id", requestID), slog.Any("error", err))
			}
		},
	}
	host, _ := os.Hostname()
	reconcile := &usecase.ReconcileTestSpecs{
		Runs: stores.testSpecRuns, Requests: stores.requests, Start: start, Agent: agent, Resolver: solution.conns,
		Timing: usecase.TestSpecTiming{Quiet: cfg.TestSpecQuiet, Deadline: cfg.TestSpecDeadline}, Owner: fmt.Sprintf("%s-%d", host, os.Getpid()), Log: log,
	}
	if enabled {
		wg.Add(1)
		go func() { defer wg.Done(); reconcile.RunReconcileLoop(ctx, cfg.TestSpecReconcileInterval) }()
		if bus != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := eventbusadapter.NewTestSpecStatusConsumer(bus, start).Run(ctx); err != nil {
					log.Warn("test specs consumer stopped", slog.Any("error", err))
				}
			}()
		} else {
			log.Warn("eventbus unavailable: test specs start only from the reconcile loop")
		}
		log.Info("test specs enabled: generated beside Plan generation; execution waits for them (REQUEST_TESTSPEC_GATE)", slog.String("gate", cfg.TestSpecGate))
	}
	return requestgrpc.TestSpecUseCases{
		Get:      &usecase.GetTestSpecs{Requests: stores.requests, Runs: stores.testSpecRuns, Settings: settings, Join: execution.Join},
		Start:    start,
		Document: &usecase.GetTestSpecDocument{Requests: stores.requests, Runs: stores.testSpecRuns, Agent: agent, Resolver: solution.conns},
	}
}

// unavailableTestSpecAgent stands in when the dev server clients are not configured: every call says so.
type unavailableTestSpecAgent struct{}

var errTestSpecAgentOff = errors.New("test specs need INFRA_FLEET_SERVICE_ADDR and PROJECT_SERVICE_ADDR")

func (unavailableTestSpecAgent) Config(context.Context, usecase.AnalysisConnection) (usecase.TestSpecConfig, error) {
	return usecase.TestSpecConfig{}, errTestSpecAgentOff
}
func (unavailableTestSpecAgent) Start(context.Context, usecase.AnalysisConnection, usecase.TestSpecStartInput) (usecase.TestSpecStarted, error) {
	return usecase.TestSpecStarted{}, errTestSpecAgentOff
}
func (unavailableTestSpecAgent) Dispatch(context.Context, usecase.AnalysisConnection, string) error {
	return errTestSpecAgentOff
}
func (unavailableTestSpecAgent) Run(context.Context, usecase.AnalysisConnection, string) (usecase.TestSpecRunState, error) {
	return usecase.TestSpecRunState{}, errTestSpecAgentOff
}
func (unavailableTestSpecAgent) Features(context.Context, usecase.AnalysisConnection) ([]string, error) {
	return nil, errTestSpecAgentOff
}
func (unavailableTestSpecAgent) Summary(context.Context, usecase.AnalysisConnection, string) (json.RawMessage, error) {
	return nil, errTestSpecAgentOff
}
func (unavailableTestSpecAgent) Document(context.Context, usecase.AnalysisConnection, string, string, int) (usecase.TestSpecText, error) {
	return usecase.TestSpecText{}, errTestSpecAgentOff
}
func (unavailableTestSpecAgent) Cancel(context.Context, usecase.AnalysisConnection, string, string) error {
	return errTestSpecAgentOff
}
