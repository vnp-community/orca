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
	history      usecase.RequestTypeHistoryRepository
	solutions    usecase.SolutionStore
	analysisRuns usecase.AnalysisRunStore
	links        usecase.RequestLinkRepository
	idempotency  usecase.RequestIdempotencyRepository
	processed    usecase.ProcessedEventRepository
	classRuns    usecase.ClassificationRunRepository
	artifact     artifactStores
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
	close        func()
}

func openStores(ctx context.Context, dsn string, dialect dbcapability.Dialect, healthSrv *health.Server) (*requestStores, error) {
	switch dialect {
	case dbcapability.DialectPostgres:
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return nil, fmt.Errorf("postgres connect: %w", err)
		}
		base := postgresadapter.New(pool)
		healthSrv.Register("postgres", func() error { return pool.Ping(context.Background()) })
		return &requestStores{
			outbox:       base,
			outboxWriter: base,
			tx:           base,
			txScope:      base,
			approvals:    postgresadapter.NewApprovalRepository(base),
			approvers:    postgresadapter.NewApproverRepository(base),
			policies:     postgresadapter.NewPolicyRepository(base),
			locker:       postgresadapter.NewRequestLocker(base),
			returns:      postgresadapter.NewReturnHistoryRepository(base),
			requests:     postgresadapter.NewRequestRepository(base),
			history:      postgresadapter.NewRequestTypeHistoryRepository(base),
			solutions:    postgresadapter.NewSolutionRecordRepository(base),
			analysisRuns: postgresadapter.NewAnalysisRunRepository(base),
			links:        postgresadapter.NewRequestLinkRepository(base),
			idempotency:  postgresadapter.NewRequestIdempotencyRepository(base),
			processed:    postgresadapter.NewProcessedEventRepository(base),
			classRuns:    postgresadapter.NewClassificationRunRepository(base),
			artifact:     postgresArtifactStores(base),
			flowSettings: postgresadapter.NewFlowSettingsRepository(base),
			sourceFinder: postgresadapter.NewSourceLookup(base),
			samples:      postgresadapter.NewMetricsSamples(base),
			auditOutbox:  postgresadapter.NewAuditOutboxRepository(base),
			nonces:       postgresadapter.NewWebhookNonceRepository(base),
			secFlags:     postgresadapter.NewSecurityFlagRepository(base),
			concurrency:  postgresadapter.NewConcurrencyCountRepository(base),
			retention:    postgresadapter.NewRetentionRepository(base),
			execution:    postgresExecutionStores(base),
			close:        pool.Close,
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
		healthSrv.Register("mysql", func() error { return db.PingContext(context.Background()) })
		return &requestStores{
			outbox:       base,
			outboxWriter: base,
			tx:           base,
			txScope:      base,
			approvals:    mysqladapter.NewApprovalRepository(base),
			approvers:    mysqladapter.NewApproverRepository(base),
			policies:     mysqladapter.NewPolicyRepository(base),
			locker:       mysqladapter.NewRequestLocker(base),
			returns:      mysqladapter.NewReturnHistoryRepository(base),
			requests:     mysqladapter.NewRequestRepository(base),
			history:      mysqladapter.NewRequestTypeHistoryRepository(base),
			solutions:    mysqladapter.NewSolutionRecordRepository(base),
			analysisRuns: mysqladapter.NewAnalysisRunRepository(base),
			links:        mysqladapter.NewRequestLinkRepository(base),
			idempotency:  mysqladapter.NewRequestIdempotencyRepository(base),
			processed:    mysqladapter.NewProcessedEventRepository(base),
			classRuns:    mysqladapter.NewClassificationRunRepository(base),
			artifact:     mysqlArtifactStores(base),
			flowSettings: mysqladapter.NewFlowSettingsRepository(base),
			sourceFinder: mysqladapter.NewSourceLookup(base),
			samples:      mysqladapter.NewMetricsSamples(base),
			auditOutbox:  mysqladapter.NewAuditOutboxRepository(base),
			nonces:       mysqladapter.NewWebhookNonceRepository(base),
			secFlags:     mysqladapter.NewSecurityFlagRepository(base),
			concurrency:  mysqladapter.NewConcurrencyCountRepository(base),
			retention:    mysqladapter.NewRetentionRepository(base),
			execution:    mysqlExecutionStores(base),
			close:        func() { _ = db.Close() },
		}, nil
	default:
		return nil, fmt.Errorf("unsupported database dialect: %s", dialect)
	}
}
