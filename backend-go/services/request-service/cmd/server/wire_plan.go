package main

import (
	"fmt"
	"log/slog"
	"os"

	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// wirePlan builds Plan generation (CR-REQ-012): the PROPOSE worker rides on the Solution analysis runner, and
// CommitPlan writes through task-service and opens the plan approval. It needs the approval stack, the task
// client and the type policies, so it runs after wireApproval and wireExecution.
func wirePlan(log *slog.Logger, stores *requestStores, lifecycle *requestLifecycle, solution *solutionWiring, approval *approvalWiring,
	execution *executionWiring, tasks *executionTasks, dirs *approvalDirectories) (requestgrpc.PlanUseCases, error) {
	limits, warnings := domain.PlanLimitsFromEnv(os.Getenv)
	for _, msg := range warnings {
		log.Warn("plan limit override ignored", slog.String("detail", msg))
	}
	writer, ok := tasks.client.(usecase.TaskPlanWriter)
	if !ok {
		return requestgrpc.PlanUseCases{}, fmt.Errorf("task client %T cannot write a Plan", tasks.client)
	}
	auth := usecase.NewPlanActorAuthorizer(usecase.ApproverAwareAuthorizer{
		Approvals: stores.approvals, Decider: &usecase.AuthorizeApprovalDecision{ApproverRepo: stores.approvers, Teams: dirs.teams},
	}, stores.analysisRuns, false) // wirePlanAuto lets the approvers revise once the worker that commits for them runs

	worker := usecase.NewRunPlanGeneration(usecase.RunPlanGenerationDeps{
		Requests: stores.requests, Solutions: stores.solutions, Completer: solution.completer, Projects: solution.projects,
		Policies: execution.Policies, Runs: stores.analysisRuns, Tx: stores.tx, Limits: limits, Settings: solution.settings, Owner: solution.owner,
	})
	solution.runner.WithPlanWorker(worker)
	registry, err := wireOpenSpecEngine(log, stores, solution, execution.Policies, limits)
	if err != nil {
		return requestgrpc.PlanUseCases{}, err
	}

	bindHotfixFixTask(log, solution, approval, execution, tasks, limits)

	out := requestgrpc.PlanUseCases{
		Generate: usecase.NewGeneratePlan(usecase.GeneratePlanDeps{
			Requests: stores.requests, Solutions: stores.solutions, Runs: stores.analysisRuns, Conns: solution.conns, Auth: auth,
			Approvals: lifecycle.Canceller, Transition: lifecycle.Transition, Tx: stores.tx, Spawner: solution.runner,
			Settings: solution.settings, LeaseOwner: solution.owner,
			Pin: usecase.NewPinSolutionEngine(stores.requests, stores.engineSettings), Registry: registry,
		}),
		Get:       usecase.NewGetPlanProposal(stores.requests, stores.analysisRuns).WithChanges(stores.openspecChanges).WithRevisions(stores.openspecRevisions),
		Bundle:    usecase.NewGetPlanBundleFile(stores.requests, stores.openspecChanges).WithRevisions(stores.analysisRuns, stores.openspecRevisions),
		Revisions: usecase.NewListPlanRevisions(stores.requests, stores.analysisRuns, stores.openspecRevisions, stores.openspecChanges),
	}
	if solution.workspace != nil {
		retention, warning := domain.ProposalRetentionFromEnv(os.Getenv)
		if warning != "" {
			log.Warn(warning)
		}
		solution.janitor = &usecase.ProposalJanitor{
			Changes: stores.openspecChanges, Requests: stores.requests, Runs: stores.analysisRuns, Workspace: solution.workspace,
			Tenants: stores.retention, Retention: retention, Log: log,
		}
		out.Cleanup = usecase.NewCleanupProposal(stores.requests, solution.janitor)
		if solution.executor != nil {
			sync := &usecase.TasksMdSyncer{
				Changes: stores.openspecChanges, Requests: stores.requests, Tasks: tasks.client, Outcomes: stores.execution.outcomes,
				Runs: stores.analysisRuns, Workspace: solution.workspace, Exec: solution.executor, Conns: solution.conns, Tenants: stores.retention, Log: log,
			}
			if obs, ok := solution.observer.(usecase.SyncObserver); ok {
				sync.Observer = obs
			}
			solution.tasksSync = sync
			execution.taskSync.set(sync)
		}
	}
	if !approval.Enabled || approval.Open == nil {
		log.Warn("approval is off: CommitPlan is unavailable because a committed Plan has nobody to approve it")
		return out, nil
	}
	out.Commit = usecase.NewCommitPlan(usecase.CommitPlanDeps{
		Requests: stores.requests, Auth: auth, Writer: writer, Policies: execution.Policies, Approvals: approval.Open,
		Transition: lifecycle.Transition, Tx: stores.tx, Outbox: stores.outboxWriter, Limits: limits,
	})
	return out, nil
}

// bindHotfixFixTask lets a finished hotfix diagnosis create its fix task and open the pre_deploy approval
// (CR-REQ-012 section 2.6). Like CommitPlan it needs approval on: a fix task nobody can approve would only wait.
func bindHotfixFixTask(log *slog.Logger, solution *solutionWiring, approval *approvalWiring, execution *executionWiring, tasks *executionTasks, limits domain.PlanLimits) {
	writer, ok := tasks.client.(usecase.FixTaskWriter)
	if !ok || !approval.Enabled || approval.Open == nil {
		log.Warn("approval or the task client is unavailable: a finished hotfix diagnosis will not create its fix task")
		return
	}
	solution.writer.WithHotfixFixTasks(usecase.NewEnsureHotfixFixTask(tasks.client, writer, execution.Policies, limits))
}

// wireOpenSpecEngine registers the openspec engine and its plan worker when the services it drives are configured: the
// dev server relay (agent and exec), project-service and git-gateway. Without them the registry stays empty, and
// GeneratePlan refuses a Request pinned to openspec instead of starting a run nothing can execute.
func wireOpenSpecEngine(log *slog.Logger, stores *requestStores, solution *solutionWiring, policies usecase.PlanPolicies, limits domain.PlanLimits) (usecase.EngineRegistry, error) {
	registry := usecase.NewDefaultEngineRegistry()
	if solution.infra == nil || solution.proj == nil || solution.git == nil || solution.caps == nil {
		log.Warn("openspec engine is off: INFRA_FLEET_SERVICE_ADDR, PROJECT_SERVICE_ADDR and GIT_GATEWAY_SERVICE_ADDR must all be set")
		return registry, nil
	}
	raw := grpcclient.NewProposalWorkspace(solution.git, solution.proj)
	// Every Ensure reserves the worktree under the Request lock first, so a base ref change cannot slip in while git works.
	workspace, err := usecase.NewReservingProposalWorkspace(raw, stores.locker, stores.openspecChanges, stores.tx)
	if err != nil {
		return nil, fmt.Errorf("openspec engine wiring: %w", err)
	}
	solution.workspace = workspace
	executor := grpcclient.NewDevServerExecutor(solution.infra)
	solution.executor = executor
	gate := usecase.NewEngineReadinessGate(solution.conns, executor, os.Getenv("REQUEST_OPENSPEC_MIN_VERSION")).
		WithCapabilities(solution.caps).WithRepoFiles(raw).WithRepoState(grpcclient.NewRepoStateProbe(solution.git)).
		WithSkillsRepository(os.Getenv("REQUEST_SKILLS_REPOSITORY_URL")).WithObserver(solution.observer)
	solution.gate = gate
	engine := usecase.NewOpenSpecEngine(usecase.OpenSpecEngineDeps{
		Gate: gate, Workspace: workspace, Agent: solution.agent, Conns: solution.conns, Caps: solution.caps, Settings: solution.settings, Observer: solution.observer,
	})
	registry.Register(engine)
	// A proposal run that cannot record its version would leave the change without history, so refuse to start.
	if stores.openspecRevisions == nil {
		return nil, fmt.Errorf("openspec engine wiring: the change revision repository is missing")
	}
	solution.runner.WithProposalWorker(usecase.NewRunProposalGeneration(usecase.RunProposalGenerationDeps{
		Requests: stores.requests, Solutions: stores.solutions, Registry: registry, Projects: solution.projects, Policies: policies,
		EngineSettings: stores.engineSettings, Settings: solution.settings, Workspace: workspace, Changes: stores.openspecChanges, Revisions: stores.openspecRevisions, Checker: usecase.NewOpenSpecBundleChecker(executor, solution.conns, limits), Observer: solution.observer, Runs: stores.analysisRuns, Tx: stores.tx, Limits: limits, Owner: solution.owner,
	}))
	return registry, nil
}
