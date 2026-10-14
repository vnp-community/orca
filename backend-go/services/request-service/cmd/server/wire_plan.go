package main

import (
	"fmt"
	"log/slog"
	"os"

	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
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
	})

	worker := usecase.NewRunPlanGeneration(usecase.RunPlanGenerationDeps{
		Requests: stores.requests, Solutions: stores.solutions, Completer: solution.completer, Projects: solution.projects,
		Policies: execution.Policies, Runs: stores.analysisRuns, Tx: stores.tx, Limits: limits, Settings: solution.settings, Owner: solution.owner,
	})
	solution.runner.WithPlanWorker(worker)

	bindHotfixFixTask(log, solution, approval, execution, tasks, limits)

	out := requestgrpc.PlanUseCases{
		Generate: usecase.NewGeneratePlan(usecase.GeneratePlanDeps{
			Requests: stores.requests, Solutions: stores.solutions, Runs: stores.analysisRuns, Conns: solution.conns, Auth: auth,
			Approvals: lifecycle.Canceller, Transition: lifecycle.Transition, Tx: stores.tx, Spawner: solution.runner,
			Settings: solution.settings, LeaseOwner: solution.owner,
			Pin: usecase.NewPinSolutionEngine(stores.requests, stores.engineSettings),
		}),
		Get: usecase.NewGetPlanProposal(stores.requests, stores.analysisRuns),
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
