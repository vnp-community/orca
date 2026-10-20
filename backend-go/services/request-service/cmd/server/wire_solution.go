package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stablyai/orca-go/prompts/output_language"
	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type solutionWiring struct {
	runner   *usecase.AnalysisRunner
	generate *usecase.GenerateSolution
	list     *usecase.ListSolutions
	choose   *usecase.ChooseSolutionOption
	// handlers are the SubjectHandlers for solution, findings and answer; hand them to buildApprovalRegistry.
	handlers map[domain.SubjectType]usecase.SubjectHandler

	opener  *lateApprovalOpener
	writer  *usecase.AnalysisResultWriter
	stores  *requestStores
	closers []func()

	// Kept for wirePlan, which runs Plan generation on the same runner, connections and AI relay.
	conns     usecase.AnalysisConnectionResolver
	completer usecase.ProjectAICompleter
	projects  usecase.ProjectContextReader
	settings  usecase.AnalysisSettings
	owner     string
	// Clients the openspec engine needs on top of the above; nil when the service addresses are not configured.
	agent usecase.AgentPromptRunner
	caps  usecase.DevServerCapabilityReader
	infra infrafleetv1.InfraFleetServiceClient
	// gate is the readiness check shared by the openspec engine and the settings RPC; nil when the dev server stack is not configured.
	gate *usecase.EngineReadinessGate
	// readonly is the agent worker of the read-only kinds and of the Solution; observer receives the engine metrics.
	readonly *usecase.RunAgentReadonlyAnalysis
	observer usecase.EngineObserver
	// executor runs short commands on the dev server; tasksSync keeps tasks.md and the archive of OpenSpec changes up to date.
	executor  usecase.DevServerExecutor
	tasksSync *usecase.TasksMdSyncer
	// workspace and janitor exist only when the openspec engine is configured: the janitor removes the proposal worktrees.
	workspace usecase.ProposalWorkspace
	janitor   *usecase.ProposalJanitor
	proj      projectv1.ProjectServiceClient
	git       gitgatewayv1.GitGatewayServiceClient
}

// wireSolution builds solution generation and read-only analysis (CR-REQ-007/008) and starts the lease-recovery sweeper.
// Approvals are bound later, once the registry exists (see bindApprovals), because the registry needs these handlers first.
func wireSolution(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, lang output_language.Language, log *slog.Logger, stores *requestStores, lifecycle *requestLifecycle, dirs *approvalDirectories) (*solutionWiring, error) {
	w := &solutionWiring{stores: stores, opener: &lateApprovalOpener{log: log}}
	settings := analysisSettings(cfg, lang)
	host, _ := os.Hostname()
	owner := fmt.Sprintf("%s-%d", host, os.Getpid())
	transition := lifecycle.Transition

	var (
		conns     usecase.AnalysisConnectionResolver = unavailableConnections{}
		completer usecase.ProjectAICompleter         = unavailableCompleter{}
		agent     usecase.AgentPromptRunner          = unavailableAgent{}
		projects  usecase.ProjectContextReader
		caps      usecase.DevServerCapabilityReader
		probe     usecase.RepoStateProbe
		infraC    infrafleetv1.InfraFleetServiceClient
		projectC  projectv1.ProjectServiceClient
		gitC      gitgatewayv1.GitGatewayServiceClient
	)
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
		projectClient := projectv1.NewProjectServiceClient(projectConn)
		var providers aiproviderv1.AiProviderServiceClient
		if cfg.AIProviderServiceAddr != "" {
			aiConn, err := grpcclient.Dial(cfg.AIProviderServiceAddr)
			if err != nil {
				return nil, err
			}
			w.closers = append(w.closers, func() { _ = aiConn.Close() })
			providers = aiproviderv1.NewAiProviderServiceClient(aiConn)
		}
		resolver := grpcclient.NewAIConnectionResolver(infra, projectClient)
		conns = grpcclient.NewAnalysisConnections(resolver)
		completer = grpcclient.NewAICompletionRelay(infra, resolver, providers, settings.AICompleteTimeout).
			WithCapabilities(grpcclient.NewInfraCapabilityClient(infra, 10*time.Second))
		agent = grpcclient.NewAgentPromptRelay(infra)
		projects = grpcclient.NewProjectContextResolver(projectClient)
		caps = grpcclient.NewInfraCapabilityClient(infra, 10*time.Second)
		infraC, projectC = infra, projectClient
	} else {
		log.Warn("INFRA_FLEET_SERVICE_ADDR or PROJECT_SERVICE_ADDR unset: GenerateSolution will fail with a no-connection error")
	}
	if cfg.GitGatewayServiceAddr != "" {
		gitConn, err := grpcclient.Dial(cfg.GitGatewayServiceAddr)
		if err != nil {
			return nil, err
		}
		w.closers = append(w.closers, func() { _ = gitConn.Close() })
		gitC = gitgatewayv1.NewGitGatewayServiceClient(gitConn)
		probe = grpcclient.NewRepoStateProbe(gitC)
	} else {
		log.Warn("GIT_GATEWAY_SERVICE_ADDR unset: read-only analyses record repo_check=skipped")
	}

	// CR-REQ-027/028: every finished Solution gets provenance, display ids and a coverage check; every pick becomes a Decision.
	artifacts := usecase.NewSolutionArtifactRecorder(usecase.NewMintArtifactIDs(stores.artifact.index), stores.artifact.relations).WithDecisions(stores.artifact.decisions)
	recordDecision := usecase.NewRecordDecision(stores.artifact.decisions, stores.outboxWriter).
		WithHighRiskServices(envInt("REQUEST_DECISION_HIGH_RISK_SERVICES", domain.DefaultHighRiskServices))
	gates := usecase.NewApprovalGates(stores.artifact.decisions, stores.artifact.clarifications)

	writer := usecase.NewAnalysisResultWriter(stores.requests, stores.solutions, stores.analysisRuns, w.opener, transition, stores.tx, stores.outboxWriter, owner).WithArtifacts(artifacts)
	generation := usecase.NewRunSolutionGeneration(stores.requests, stores.solutions, completer, projects, writer, settings).WithCoverage(artifacts).
		WithClarifications(stores.artifact.clarifications)
	readonly := usecase.NewRunAgentReadonlyAnalysis(usecase.AgentReadonlyDeps{
		Requests: stores.requests, Solutions: stores.solutions, Conns: conns, Agent: agent, Probe: probe, Caps: caps,
		Projects: projects, Writer: writer, Settings: settings, Clarifications: stores.artifact.clarifications,
	})
	w.writer = writer
	w.readonly = readonly
	w.runner = usecase.NewAnalysisRunner(stores.analysisRuns, stores.solutions, stores.tx, generation, readonly, settings, owner).WithRequests(stores.requests)
	w.conns, w.completer, w.projects, w.settings, w.owner = conns, completer, projects, settings, owner
	w.agent, w.caps, w.infra, w.proj, w.git = agent, caps, infraC, projectC, gitC

	// Reporter and admins steer analysis; whoever the pending approval's snapshot lets decide may also choose the option
	// and ask for a revision (never start the first analysis).
	auth, err := usecase.NewApproverAwareAuthorizer(stores.approvals, &usecase.AuthorizeApprovalDecision{ApproverRepo: stores.approvers, Teams: dirs.teams})
	if err != nil {
		return nil, err
	}
	w.generate = usecase.NewGenerateSolution(usecase.GenerateSolutionDeps{
		Requests: stores.requests, Solutions: stores.solutions, Runs: stores.analysisRuns, Conns: conns, Auth: auth, Approvals: lifecycle.Canceller,
		Transition: transition, Tx: stores.tx, Spawner: w.runner, Settings: settings, LeaseOwner: owner,
	}).WithDecisions(artifacts).WithEnginePin(usecase.NewPinSolutionEngine(stores.requests, stores.engineSettings)).
		WithSolutionAgent(stores.engineSettings, caps)
	w.list = usecase.NewListSolutions(stores.requests, stores.solutions, stores.analysisRuns)
	w.choose = usecase.NewChooseSolutionOption(stores.requests, stores.solutions, stores.approvals, auth, stores.tx).
		WithDecisions(recordDecision, selfChoicePolicy(stores))

	deps := usecase.AnalysisApprovalHandlerDeps{Requests: stores.requests, Solutions: stores.solutions, Returner: lifecycle.Return, Transition: transition, Outbox: stores.outboxWriter, Gates: gates, Coverage: artifacts, Decisions: artifacts}
	w.handlers = map[domain.SubjectType]usecase.SubjectHandler{
		domain.SubjectSolution: usecase.NewSolutionApprovalHandler(deps),
		domain.SubjectFindings: usecase.NewFindingsApprovalHandler(deps),
		domain.SubjectAnswer:   usecase.NewAnswerApprovalHandler(deps),
	}

	recovery := usecase.NewRecoverInterruptedAnalysisRuns(stores.analysisRuns, stores.solutions, stores.tx, settings, owner)
	wg.Add(1)
	go func() { defer wg.Done(); recovery.RunRecoveryLoop(ctx, settings.RecoveryInterval) }()
	return w, nil
}

// selfChoicePolicy follows the pending solution Approval: when it forbids self approval, the reporter may not pick either.
func selfChoicePolicy(stores *requestStores) usecase.SelfChoicePolicy {
	return func(ctx context.Context, req domain.Request) (bool, error) {
		pending, _, err := stores.approvals.List(ctx, req.TenantID, usecase.ApprovalListFilter{
			RequestID: req.ID, SubjectType: domain.SubjectSolution, Status: domain.ApprovalStatusPending, PageSize: 1,
		})
		if err != nil || len(pending) == 0 {
			return true, err
		}
		return pending[0].SelfApprovalAllowed, nil
	}
}

func analysisSettings(cfg config.Config, lang output_language.Language) usecase.AnalysisSettings {
	s := usecase.DefaultAnalysisSettings()
	s.OutputLanguage = lang
	if cfg.AICompleteTimeout > 0 {
		s.AICompleteTimeout = cfg.AICompleteTimeout
	}
	if cfg.AnalysisLeaseTTL > 0 {
		s.LeaseTTL = cfg.AnalysisLeaseTTL
	}
	if cfg.AnalysisHeartbeat > 0 {
		s.Heartbeat = cfg.AnalysisHeartbeat
	}
	if cfg.AnalysisRecoveryInterval > 0 {
		s.RecoveryInterval = cfg.AnalysisRecoveryInterval
	}
	if cfg.AgentReadonlyTimeoutMS > 0 {
		s.AgentTimeoutMS = cfg.AgentReadonlyTimeoutMS
	}
	if cfg.SpecTimeoutMS > 0 {
		s.SpecTimeoutMS = cfg.SpecTimeoutMS
	}
	if cfg.SpecTotalTimeoutMS > 0 {
		s.SpecTotalTimeoutMS = cfg.SpecTotalTimeoutMS
	}
	if cfg.AgentReadonlyMaxPerProject > 0 {
		s.MaxAgentRunsPerProject = cfg.AgentReadonlyMaxPerProject
	}
	s.UseAgentReadonlyFlag = cfg.AgentReadonlyUseAgentFlag
	s.RequireEnforcedReadonly = cfg.RequireEnforcedReadonly
	return s
}

// bindApprovals connects the approval opener once wireApproval has built it. With approval off the opener is nil
// and new proposals only log a warning, because there is no engine to open an Approval in.
func (w *solutionWiring) bindApprovals(opener usecase.SolutionApprovalOpener) {
	if opener != nil {
		w.opener.set(opener)
	}
}

func (w *solutionWiring) useCases() requestgrpc.SolutionUseCases {
	return requestgrpc.SolutionUseCases{Generate: w.generate, List: w.list, Choose: w.choose}
}

func (w *solutionWiring) close() {
	w.runner.Close()
	for _, c := range w.closers {
		c()
	}
}

// lateApprovalOpener warns instead of silently dropping: without an opener a proposal has no Approval to approve.
type lateApprovalOpener struct {
	inner atomic.Pointer[usecase.SolutionApprovalOpener]
	log   *slog.Logger
}

func (o *lateApprovalOpener) set(a usecase.SolutionApprovalOpener) { o.inner.Store(&a) }

func (o *lateApprovalOpener) Open(ctx context.Context, in usecase.OpenSolutionApprovalInput) error {
	if p := o.inner.Load(); p != nil {
		return (*p).Open(ctx, in)
	}
	o.log.WarnContext(ctx, "no approval opener wired: proposal stored without an Approval",
		slog.String("request_id", in.Request.ID), slog.String("subject", string(in.SubjectType)), slog.String("subject_id", in.SubjectID))
	return nil
}

type unavailableConnections struct{}

func (unavailableConnections) ResolveForProject(context.Context, string) (usecase.AnalysisConnection, error) {
	return usecase.AnalysisConnection{}, usecase.ErrNoDevServer
}

type unavailableCompleter struct{}

func (unavailableCompleter) Complete(context.Context, string, string) (string, error) {
	return "", usecase.ErrNoDevServer
}

type unavailableAgent struct{}

func (unavailableAgent) ExecPrompt(context.Context, usecase.AnalysisConnection, usecase.AgentPromptInput) (usecase.AgentPromptResult, error) {
	return usecase.AgentPromptResult{}, usecase.ErrNoDevServer
}

// observe sends the engine metrics (runs, knowledge tool calls, preflight failures, spec violations) to o. Call it before
// wirePlan, which hands the observer to the engine and the readiness check.
func (w *solutionWiring) observe(o usecase.EngineObserver) {
	w.observer = o
	w.runner.WithObserver(o)
	w.readonly.WithObserver(o)
}
