package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	eventbusadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/eventbus"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// testScriptWiring is the test-script lane: its handlers and the starter the test-spec lane calls when it finishes.
type testScriptWiring struct {
	handlers requestgrpc.TestScriptUseCases
	// afterTestSpecs starts the lane for an executing Request whose test specs just finished; nil when the lane is off.
	afterTestSpecs func(ctx context.Context, requestID string)
}

// wireTestScripts builds the test-script lane (docs/integrate/TDD-REQ-TESTSCRIPT-001): beside the code lane from the moment execution
// may start, sharing only the start events and the dev server. The handlers are always returned (the screen asks whether the lane is
// on); the consumer, the hook and the reconcile loop run only when REQUEST_TESTSCRIPT_ENABLED is true and the dev server clients exist.
func wireTestScripts(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores, solution *solutionWiring,
	execution *executionWiring, bus *commoneventbus.Consumer) testScriptWiring {
	enabled := cfg.TestScriptEnabled
	if enabled && solution.infra == nil {
		log.Warn("REQUEST_TESTSCRIPT_ENABLED is true but INFRA_FLEET_SERVICE_ADDR or PROJECT_SERVICE_ADDR is unset: test scripts stay off")
		enabled = false
	}
	if enabled && !cfg.TestSpecEnabled {
		log.Warn("REQUEST_TESTSCRIPT_ENABLED is true but REQUEST_TESTSPEC_ENABLED is not: every Request is skipped (no test specs to read)")
	}
	settings := usecase.TestSpecSettingsFunc(func(context.Context, string) bool { return enabled })
	var agent usecase.TestScriptAgent = unavailableTestScriptAgent{}
	if solution.infra != nil {
		agent = grpcclient.NewTestScriptAgentMCP(solution.infra)
	}
	start := &usecase.StartTestScripts{
		Requests: stores.requests, Runs: stores.testScriptRuns, SpecRuns: stores.testSpecRuns, Agent: agent, Resolver: solution.conns,
		Actors: execution.Actors, Settings: settings, Log: log,
		Options: usecase.TestScriptOptions{TestsPaths: cfg.TestScriptTestsPaths, TestRepo: cfg.TestScriptTestRepo, Generator: cfg.TestScriptGenerator,
			Model: cfg.TestScriptModel},
	}
	host, _ := os.Hostname()
	reconcile := &usecase.ReconcileTestScripts{
		Runs: stores.testScriptRuns, Requests: stores.requests, Start: start, Agent: agent, Resolver: solution.conns,
		Timing: usecase.TestSpecTiming{Quiet: cfg.TestScriptQuiet, Deadline: cfg.TestScriptDeadline}, Owner: fmt.Sprintf("%s-%d", host, os.Getpid()), Log: log,
	}
	out := testScriptWiring{handlers: requestgrpc.TestScriptUseCases{
		Get:    &usecase.GetTestScripts{Requests: stores.requests, Runs: stores.testScriptRuns, Settings: settings},
		Start:  start,
		Report: &usecase.GetTestScriptReport{Requests: stores.requests, Runs: stores.testScriptRuns, Agent: agent, Resolver: solution.conns},
		File:   &usecase.GetTestScriptFile{Requests: stores.requests, Runs: stores.testScriptRuns, Agent: agent, Resolver: solution.conns},
	}}
	if !enabled {
		return out
	}
	out.afterTestSpecs = func(ctx context.Context, requestID string) {
		if _, err := start.Execute(ctx, requestID); err != nil {
			log.Warn("test scripts: could not start after the test specs finished", slog.String("request_id", requestID), slog.Any("error", err))
		}
	}
	wg.Add(1)
	go func() { defer wg.Done(); reconcile.RunReconcileLoop(ctx, cfg.TestScriptReconcileInterval) }()
	if bus != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := eventbusadapter.NewTestScriptStatusConsumer(bus, start).Run(ctx); err != nil {
				log.Warn("test scripts consumer stopped", slog.Any("error", err))
			}
		}()
	} else {
		log.Warn("eventbus unavailable: test scripts start only from the test-spec hook and the reconcile loop")
	}
	log.Info("test scripts enabled: generated from the TCIR beside the code once execution may start")
	return out
}

// unavailableTestScriptAgent stands in when the dev server clients are not configured: every call says so.
type unavailableTestScriptAgent struct{}

var errTestScriptAgentOff = errors.New("test scripts need INFRA_FLEET_SERVICE_ADDR and PROJECT_SERVICE_ADDR")

func (unavailableTestScriptAgent) Start(context.Context, usecase.AnalysisConnection, usecase.TestScriptStartInput) (usecase.TestScriptStarted, error) {
	return usecase.TestScriptStarted{}, errTestScriptAgentOff
}
func (unavailableTestScriptAgent) Job(context.Context, usecase.AnalysisConnection, string) (usecase.TestScriptJob, error) {
	return usecase.TestScriptJob{}, errTestScriptAgentOff
}
func (unavailableTestScriptAgent) Report(context.Context, usecase.AnalysisConnection, string, int) (usecase.TestScriptText, error) {
	return usecase.TestScriptText{}, errTestScriptAgentOff
}
func (unavailableTestScriptAgent) File(context.Context, usecase.AnalysisConnection, string, string, int) (usecase.TestScriptText, error) {
	return usecase.TestScriptText{}, errTestScriptAgentOff
}
func (unavailableTestScriptAgent) Cancel(context.Context, usecase.AnalysisConnection, string, string) error {
	return errTestScriptAgentOff
}
