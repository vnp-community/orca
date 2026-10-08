package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	eventbusadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/eventbus"
	requestgrpc "github.com/stablyai/orca-go/services/request-service/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"github.com/stablyai/orca-go/services/request-service/schemas"
)

// artifactWiring holds what CR-REQ-027 (artifact model) and CR-REQ-028 (clarification, decision) add.
// ChooseSolutionOption and the solution approval handler get their own RecordDecision and ApprovalGates in
// wireSolution; the plan approval will take them from here once its handler exists.
type artifactWiring struct {
	artifactRPC      requestgrpc.ArtifactUseCases
	clarificationRPC requestgrpc.ClarificationUseCases
	RecordDecision   *usecase.RecordDecision
	Gates            *usecase.ApprovalGates
	Mint             *usecase.MintArtifactIDs
	ReplaceCoverage  *usecase.ReplaceRequestCoverage
	closers          []func()
}

func (w *artifactWiring) close() {
	for _, c := range w.closers {
		c()
	}
}

// attach puts the artifact and clarification RPCs on the server.
func (w *artifactWiring) attach(s *requestgrpc.Server) *requestgrpc.Server {
	return s.WithArtifact(w.artifactRPC).WithClarification(w.clarificationRPC)
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func envBool(name string, def bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(name)); err == nil {
		return v
	}
	return def
}

// wireArtifact builds both features over the shared stores and hooks them into the existing use cases:
// request creation records revision 1, type confirmation gates on readiness, and the leave-awaiting-information
// paths (type change, cancel, return) close the open clarification. It starts the resume consumer and the
// expiry and reminder sweep.
func wireArtifact(ctx context.Context, wg *sync.WaitGroup, cfg config.Config, log *slog.Logger, stores *requestStores,
	lifecycle *requestLifecycle, intake *intakeWiring, classification *classificationWiring,
	bus *commoneventbus.Consumer, dirs *approvalDirectories, solution *solutionWiring) (*artifactWiring, error) {
	w := &artifactWiring{}
	a := stores.artifact

	reg, err := schemas.Default()
	if err != nil {
		return nil, fmt.Errorf("artifact schemas: %w", err)
	}

	appendRevision := usecase.NewAppendRequestRevision(stores.requests, a.content, a.revisions, stores.tx, stores.outboxWriter).WithSchemas(reg)
	// Revision 1 joins the creation transaction; REQ-<n> is indexed there too.
	mint := usecase.NewMintArtifactIDs(a.index)
	recorder := usecase.NewRequestRevisionRecorder(a.revisions, stores.outboxWriter).WithSchemas(reg).WithIndex(mint)
	intake.createRequest.WithCreationRecorder(recorder)

	reader := usecase.NewArtifactReader(stores.requests, a.revisions, a.coverage, a.relations, stores.links, a.index)
	if cfg.TaskServiceAddr != "" {
		conn, err := grpcclient.Dial(cfg.TaskServiceAddr)
		if err != nil {
			return nil, err
		}
		w.closers = append(w.closers, func() { _ = conn.Close() })
		reader.WithPlanTree(grpcclient.NewPlanTreeClient(taskv1.NewTaskServiceClient(conn)))
	} else {
		log.Warn("TASK_SERVICE_ADDR unset: GetArtifactGraph will report partial results for requests that have a plan")
	}
	w.Mint = mint
	w.ReplaceCoverage = usecase.NewReplaceRequestCoverage(a.coverage)
	w.artifactRPC = requestgrpc.ArtifactUseCases{
		Edit:   usecase.NewEditRequestContent(stores.requests, appendRevision, stores.tx),
		Reader: reader,
		Export: usecase.NewExportArtifactProjection(stores.requests, a.revisions, stores.solutions, a.index, reg),
	}

	// Clarification and readiness.
	canceller := lifecycle.Canceller // the same closer the lifecycle and solution regeneration use, with its lock and transaction wiring
	maxRounds := envInt("REQUEST_CLARIFICATION_MAX_ROUNDS", 3)
	// Teams and the admin role expand through the same directories the approval engine uses; they fail closed, and
	// PrincipalRecipients skips an unavailable directory instead of dropping the clarification.
	recipients := usecase.PrincipalRecipients{Teams: dirs.teams, Admins: dirs.admins}
	clarify := usecase.NewRequestClarification(stores.requests, a.clarifications, lifecycle.Transition, canceller, stores.tx, stores.outboxWriter).WithRecipients(recipients)
	gate := usecase.NewReadinessGate(stores.requests, clarify, a.clarifications, a.revisions, lifecycle.Transition, maxRounds)
	classification.confirm.WithReadiness(gate)

	closer := usecase.NewOpenClarificationCanceller(a.clarifications, stores.requests, stores.outboxWriter)
	lifecycle.Return.WithClarifications(closer)
	lifecycle.Cancel.WithClarifications(closer)
	classification.change.WithClarifications(closer)

	answer := usecase.NewAnswerClarification(stores.requests, a.clarifications, appendRevision, lifecycle.Transition, lifecycle.Return, clarify,
		a.revisions, a.decisions, stores.tx, stores.outboxWriter, maxRounds).
		WithTeams(dirs.teams).WithSolutions(usecase.NewProposedSolutionSuperseder(stores.solutions))
	queries := usecase.NewClarificationQueries(stores.requests, a.clarifications).WithTeams(dirs.teams)
	w.RecordDecision = usecase.NewRecordDecision(a.decisions, stores.outboxWriter).WithHighRiskServices(envInt("REQUEST_DECISION_HIGH_RISK_SERVICES", domain.DefaultHighRiskServices))
	w.Gates = usecase.NewApprovalGates(a.decisions, a.clarifications)
	w.clarificationRPC = requestgrpc.ClarificationUseCases{
		Readiness:    usecase.NewGetRequestReadiness(stores.requests),
		Request:      clarify,
		Queries:      queries,
		Answer:       answer,
		Cancel:       usecase.NewCancelClarification(stores.requests, a.clarifications, lifecycle.Return, stores.tx, stores.outboxWriter),
		Waive:        usecase.NewWaiveReadiness(stores.requests, a.clarifications, appendRevision, lifecycle.Transition, stores.tx, stores.outboxWriter),
		Decisions:    usecase.NewDecisionQueries(stores.requests, a.decisions),
		DecisionConf: usecase.NewConfirmDecision(stores.requests, a.decisions, stores.tx, stores.outboxWriter),
	}

	if envBool("REQUEST_CLARIFICATION_WORKERS_ENABLED", true) && cfg.RequestFlowEnabled {
		sweeper := usecase.NewClarificationSweeper(stores.requests, a.clarifications, lifecycle.Return, recipients, stores.tx, stores.outboxWriter)
		wg.Add(1)
		go func() { defer wg.Done(); sweeper.RunLoop(ctx, 60*time.Second, 50) }()
		if bus != nil {
			resume := usecase.NewResumeAfterClarification(a.clarifications, stores.processed, stores.tx).
				WithAnalysis(usecase.NewSolutionRegenerator(solution.generate)).
				WithAutoRegenerate(envBool("REQUEST_CLARIFICATION_AUTO_REGENERATE", true))
			consumer := eventbusadapter.NewClarificationResumeConsumer(bus, resume)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := consumer.Run(ctx); err != nil {
					log.Warn("clarification resume consumer stopped", slog.Any("error", err))
				}
			}()
		} else {
			log.Warn("eventbus unavailable: answered clarifications will not restart analysis")
		}
	}
	return w, nil
}
