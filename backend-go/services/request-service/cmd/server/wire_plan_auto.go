package main

import (
	"context"
	"log/slog"
	"os"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	eventbusadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/eventbus"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// planAutoBatch is how many due jobs one pass of the worker claims.
const planAutoBatch = 10

// planAutoUnavailable names why automatic Plan generation cannot run, or "" when it can. Every condition is needed:
// without the bus nothing queues a job, and without CommitPlan a finished run would have nobody to approve its Plan.
func planAutoUnavailable(envOn, flowEnabled, hasBus, commitWired bool) string {
	switch {
	case !envOn:
		return "REQUEST_PLAN_AUTO_GENERATE is false"
	case !flowEnabled:
		return "the request flow is disabled"
	case !hasBus:
		return "the event bus is unavailable"
	case !commitWired:
		return "CommitPlan is unavailable because approval is off"
	}
	return ""
}

// planAutoParts is what the feature adds; building it starts nothing, so it can be checked without a bus.
type planAutoParts struct {
	plan     requestgrpc.PlanUseCases
	consumer *eventbusadapter.PlanAutoConsumer
	worker   *usecase.PlanAutoWorker
	policy   domain.PlanAutoPolicy
}

// buildPlanAuto returns the Plan use cases with the retry RPC and the auto-plan block attached. When automatic
// generation cannot run it returns them with the block switched off (so a screen falls back to the manual flow), no
// consumer and no worker, and the reason.
func buildPlanAuto(cfg config.Config, stores *requestStores, plan requestgrpc.PlanUseCases, solution *solutionWiring,
	dirs *approvalDirectories, bus *commoneventbus.Consumer, policy domain.PlanAutoPolicy, envOn bool) (planAutoParts, string) {
	reason := planAutoUnavailable(envOn, cfg.RequestFlowEnabled, bus != nil, plan.Commit != nil)
	if reason == "" && plan.Generate == nil {
		reason = "GeneratePlan is unavailable"
	}
	if reason != "" {
		plan.Get = plan.Get.WithAutoPlan(stores.planAutoJobs, usecase.PlanAutoSwitch{})
		return planAutoParts{plan: plan, policy: policy}, reason
	}
	sw := usecase.PlanAutoSwitch{EnvOn: true, CommitWired: true, Settings: stores.engineSettings}
	enqueue := usecase.NewEnqueuePlanAuto(stores.planAutoJobs, stores.processed, stores.requests, stores.solutions, sw, policy, stores.tx)
	worker := usecase.NewPlanAutoWorker(usecase.PlanAutoWorkerDeps{
		Jobs: stores.planAutoJobs, Requests: stores.requests, Runs: stores.analysisRuns, Start: plan.Generate, Commit: plan.Commit,
		Switch: sw, Policy: policy, Owner: solution.owner + "-plan-auto",
	})
	// Only now does someone exist who can commit a revised Plan for a requester who may not (CR-REQ-045).
	plan.Generate.WithRevisionWorker(stores.planAutoJobs, planAuthorizer(stores, dirs, true))
	plan.Retry = usecase.NewRetryPlanGeneration(stores.requests, stores.analysisRuns, stores.planAutoJobs, planAuthorizer(stores, dirs, false), sw)
	plan.Get = plan.Get.WithAutoPlan(stores.planAutoJobs, sw)
	return planAutoParts{plan: plan, consumer: eventbusadapter.NewPlanAutoConsumer(bus, enqueue).WithMaxEventAge(policy.StartWindow), worker: worker, policy: policy}, ""
}

// wirePlanAuto starts automatic Plan generation (CR-REQ-044): the consumer that queues a job for a Request entering
// planning, and the worker that drives the jobs. It logs one line with the reason when the feature cannot run.
func wirePlanAuto(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores,
	plan requestgrpc.PlanUseCases, solution *solutionWiring, dirs *approvalDirectories, bus *commoneventbus.Consumer) requestgrpc.PlanUseCases {
	policy, warnings := domain.PlanAutoPolicyFromEnv(os.Getenv)
	for _, msg := range warnings {
		log.Warn("plan auto-generation setting ignored", slog.String("detail", msg))
	}
	parts, reason := buildPlanAuto(cfg, stores, plan, solution, dirs, bus, policy, envBool("REQUEST_PLAN_AUTO_GENERATE", true))
	if reason != "" {
		log.Info("automatic Plan generation is off", slog.String("reason", reason))
		return parts.plan
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := parts.consumer.Run(ctx); err != nil {
			log.Warn("plan auto-generation consumer stopped", slog.Any("error", err))
		}
	}()
	go func() { defer wg.Done(); parts.worker.RunLoop(ctx, policy.PollInterval, planAutoBatch) }()
	log.Info("automatic Plan generation is on", slog.Int("max_extra_attempts", policy.MaxExtraAttempts), slog.Duration("poll_interval", policy.PollInterval))
	return parts.plan
}

// planAuthorizer decides who may ask for a Plan; Retry only needs the reporter-or-admin rule of AuthorizePlan.
func planAuthorizer(stores *requestStores, dirs *approvalDirectories, approversMayRevise bool) usecase.PlanActorAuthorizer {
	return usecase.NewPlanActorAuthorizer(usecase.ApproverAwareAuthorizer{
		Approvals: stores.approvals, Decider: &usecase.AuthorizeApprovalDecision{ApproverRepo: stores.approvers, Teams: dirs.teams},
	}, stores.analysisRuns, approversMayRevise)
}
