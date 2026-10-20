package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stablyai/orca-go/common/dbcapability"
	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/common/outbox"
	mysqladapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/mysql"
	postgresadapter "github.com/stablyai/orca-go/services/request-service/internal/adapter/postgres"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"

	_ "github.com/go-sql-driver/mysql"
)

// requestStores groups the dialect-specific adapters behind the usecase ports.
// History, solutions, links and idempotency have no use case yet (request-lifecycle wave);
// they are built here so that wave only consumes them.
type requestStores struct {
	outbox       outbox.Store
	outboxWriter usecase.OutboxWriter
	tx           usecase.TxRunner
	txScope      usecase.TxScope
	approvals    usecase.ApprovalRepository
	approvers    usecase.ApprovalApproverRepository
	policies     usecase.ApprovalPolicyRepository
	locker       usecase.RequestLocker
	returns      usecase.ReturnHistoryRepository
	requests     usecase.RequestRepository
	// executionModes pins the execution mode of a Request when it enters executing (CR-REQ-038); the same repository as requests.
	executionModes usecase.ExecutionModePinner
	// baseRefs stores the base ref (CR-REQ-039); worktreeRecords tells which Requests already have a worktree request-service knows of.
	baseRefs        usecase.RequestBaseRefWriter
	worktreeRecords usecase.WorktreePresence
	// engineDefaults are the deployment defaults (engine, agent-written Solution, execution mode); set by main after the stores open.
	engineDefaults domain.EngineDefaults
	history        usecase.RequestTypeHistoryRepository
	solutions      usecase.SolutionStore
	analysisRuns   usecase.AnalysisRunStore
	// engineSettings holds the per-project engine choice; the Request pins its engine from it at the first generation.
	engineSettings usecase.EngineSettingsRepository
	// openspecChanges is the authored OpenSpec change of a Request: its files, branch, commit and what it was built from.
	openspecChanges usecase.OpenSpecChangeRepository
	// openspecRevisions keeps the versions of that change, one per Plan run (CR-REQ-045).
	openspecRevisions usecase.OpenSpecChangeRevisionRepository
	links             usecase.RequestLinkRepository
	idempotency       usecase.RequestIdempotencyRepository
	processed         usecase.ProcessedEventRepository
	planAutoJobs      usecase.PlanAutoJobRepository // automatic Plan generation of each Request (CR-REQ-044)
	classRuns         usecase.ClassificationRunRepository
	artifact          artifactStores
	// Rollout (CR-REQ-024/025): flag row, active-source lookup, cross-tenant gauge counts.
	flowSettings usecase.FlowSettingsRepository
	sourceFinder usecase.ActiveSourceFinder
	samples      usecase.MetricsSampleSource
	observer     usecase.RequestObserver // set by wireRollout; nil means no metrics
	auditOutbox  usecase.AuditOutboxStore
	nonces       usecase.WebhookNonceStore
	secFlags     usecase.SecurityFlagStore
	concurrency  usecase.ConcurrencyCounter
	retention    usecase.RetentionStore
	execution    executionStores
	// outcomeReviews and reviewRuns store the Outcome Review (CR-OR-001) and the agent runs that made it.
	outcomeReviews usecase.OutcomeReviewRepository
	reviewRuns     usecase.OutcomeReviewRunRepository
	close          func()
}

func openStores(ctx context.Context, dsn string, dialect dbcapability.Dialect, healthSrv *health.Server) (*requestStores, error) {
	switch dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return nil, fmt.Errorf("postgres connect: %w", err)
		}
		base := postgresadapter.New(pool)
		healthSrv.Register("postgres", func() error { return probeWithTimeout(pool.Ping) })
		return &requestStores{
			outbox:            base,
			outboxWriter:      base,
			tx:                base,
			txScope:           base,
			approvals:         postgresadapter.NewApprovalRepository(base),
			approvers:         postgresadapter.NewApproverRepository(base),
			policies:          postgresadapter.NewPolicyRepository(base),
			locker:            postgresadapter.NewRequestLocker(base),
			returns:           postgresadapter.NewReturnHistoryRepository(base),
			requests:          postgresadapter.NewRequestRepository(base),
			executionModes:    postgresadapter.NewRequestRepository(base),
			baseRefs:          postgresadapter.NewRequestRepository(base),
			worktreeRecords:   postgresadapter.NewWorktreePresence(base),
			history:           postgresadapter.NewRequestTypeHistoryRepository(base),
			solutions:         postgresadapter.NewSolutionRecordRepository(base),
			analysisRuns:      postgresadapter.NewAnalysisRunRepository(base),
			engineSettings:    postgresadapter.NewProjectEngineSettingsRepository(base),
			openspecChanges:   postgresadapter.NewOpenSpecChangeRepository(base),
			openspecRevisions: postgresadapter.NewOpenSpecChangeRevisionRepository(base),
			links:             postgresadapter.NewRequestLinkRepository(base),
			idempotency:       postgresadapter.NewRequestIdempotencyRepository(base),
			processed:         postgresadapter.NewProcessedEventRepository(base),
			planAutoJobs:      postgresadapter.NewPlanAutoJobRepository(base),
			classRuns:         postgresadapter.NewClassificationRunRepository(base),
			artifact:          postgresArtifactStores(base),
			flowSettings:      postgresadapter.NewFlowSettingsRepository(base),
			sourceFinder:      postgresadapter.NewSourceLookup(base),
			samples:           postgresadapter.NewMetricsSamples(base),
			auditOutbox:       postgresadapter.NewAuditOutboxRepository(base),
			nonces:            postgresadapter.NewWebhookNonceRepository(base),
			secFlags:          postgresadapter.NewSecurityFlagRepository(base),
			concurrency:       postgresadapter.NewConcurrencyCountRepository(base),
			retention:         postgresadapter.NewRetentionRepository(base),
			execution:         postgresExecutionStores(base),
			outcomeReviews:    postgresadapter.NewOutcomeReviewRepository(base),
			reviewRuns:        postgresadapter.NewOutcomeReviewRunRepository(base),
			close:             pool.Close,
		}, nil
	case dbcapability.DialectMySQL:
		mysqlDSN, err := toMySQLDriverDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("format mysql dsn: %w", err)
		}
		db, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			return nil, fmt.Errorf("mysql open: %w", err)
		}
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(25)
		db.SetConnMaxLifetime(0)
		base := mysqladapter.New(db)
		healthSrv.Register("mysql", func() error { return probeWithTimeout(db.PingContext) })
		return &requestStores{
			outbox:            base,
			outboxWriter:      base,
			tx:                base,
			txScope:           base,
			approvals:         mysqladapter.NewApprovalRepository(base),
			approvers:         mysqladapter.NewApproverRepository(base),
			policies:          mysqladapter.NewPolicyRepository(base),
			locker:            mysqladapter.NewRequestLocker(base),
			returns:           mysqladapter.NewReturnHistoryRepository(base),
			requests:          mysqladapter.NewRequestRepository(base),
			executionModes:    mysqladapter.NewRequestRepository(base),
			baseRefs:          mysqladapter.NewRequestRepository(base),
			worktreeRecords:   mysqladapter.NewWorktreePresence(base),
			history:           mysqladapter.NewRequestTypeHistoryRepository(base),
			solutions:         mysqladapter.NewSolutionRecordRepository(base),
			analysisRuns:      mysqladapter.NewAnalysisRunRepository(base),
			engineSettings:    mysqladapter.NewProjectEngineSettingsRepository(base),
			openspecChanges:   mysqladapter.NewOpenSpecChangeRepository(base),
			openspecRevisions: mysqladapter.NewOpenSpecChangeRevisionRepository(base),
			links:             mysqladapter.NewRequestLinkRepository(base),
			idempotency:       mysqladapter.NewRequestIdempotencyRepository(base),
			processed:         mysqladapter.NewProcessedEventRepository(base),
			planAutoJobs:      mysqladapter.NewPlanAutoJobRepository(base),
			classRuns:         mysqladapter.NewClassificationRunRepository(base),
			artifact:          mysqlArtifactStores(base),
			flowSettings:      mysqladapter.NewFlowSettingsRepository(base),
			sourceFinder:      mysqladapter.NewSourceLookup(base),
			samples:           mysqladapter.NewMetricsSamples(base),
			auditOutbox:       mysqladapter.NewAuditOutboxRepository(base),
			nonces:            mysqladapter.NewWebhookNonceRepository(base),
			secFlags:          mysqladapter.NewSecurityFlagRepository(base),
			concurrency:       mysqladapter.NewConcurrencyCountRepository(base),
			retention:         mysqladapter.NewRetentionRepository(base),
			execution:         mysqlExecutionStores(base),
			outcomeReviews:    mysqladapter.NewOutcomeReviewRepository(base),
			reviewRuns:        mysqladapter.NewOutcomeReviewRunRepository(base),
			close:             func() { _ = db.Close() },
		}, nil
	default:
		return nil, fmt.Errorf("unsupported database dialect: %s", dialect)
	}
}
