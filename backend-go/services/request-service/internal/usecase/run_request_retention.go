package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	retentionBatch      = 200
	retentionMaxBatches = 5
)

// RetentionSummary counts what one run changed; it is also the audit metadata (numbers only).
type RetentionSummary struct {
	Tenants    int
	Anonymized int
}

// RunRequestRetention anonymizes finished Requests older than each tenant's retention window.
type RunRequestRetention struct {
	store RetentionStore
	audit *AuditRecorder
	key   []byte
	clock Clock
	log   *slog.Logger
}

func NewRunRequestRetention(store RetentionStore, audit *AuditRecorder, hmacKey []byte, clock Clock, log *slog.Logger) *RunRequestRetention {
	if clock == nil {
		clock = systemClock{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &RunRequestRetention{store: store, audit: audit, key: hmacKey, clock: clock, log: log}
}

// Execute fails closed without the erase key: anonymizing with a guessable pseudonym would defeat it.
func (uc *RunRequestRetention) Execute(ctx context.Context) (RetentionSummary, error) {
	if len(uc.key) == 0 {
		return RetentionSummary{}, domain.ErrEraseKeyMissing()
	}
	tenants, err := uc.store.ListTenants(ctx)
	if err != nil {
		return RetentionSummary{}, err
	}
	var sum RetentionSummary
	for _, tenantID := range tenants {
		if ctx.Err() != nil {
			return sum, ctx.Err()
		}
		n, err := uc.runTenant(ctx, tenantID)
		if err != nil {
			uc.log.WarnContext(ctx, "retention run failed for a tenant", slog.String("tenant_id", tenantID), slog.Any("error", err))
			continue // one broken tenant must not stop the others.
		}
		sum.Tenants++
		sum.Anonymized += n
	}
	return sum, nil
}

func (uc *RunRequestRetention) runTenant(ctx context.Context, tenantID string) (int, error) {
	tctx := tenant.WithActorType(tenant.WithTenantID(ctx, tenantID), tenant.ActorSystem)
	settings, err := uc.store.Settings(tctx)
	if err != nil {
		return 0, err
	}
	now := uc.clock.Now()
	plan := domain.PlanFor(tenantID, settings, now)
	if plan.RequestCutoff.IsZero() {
		return 0, nil // 0 days: this tenant keeps its Requests.
	}
	pseudonym := func(reporter string) string { return domain.PseudonymizeReporter(uc.key, tenantID, reporter) }
	total := 0
	for i := 0; i < retentionMaxBatches; i++ {
		n, err := uc.store.AnonymizeExpired(tctx, plan.RequestCutoff, retentionBatch, pseudonym, now)
		if err != nil {
			return total, err
		}
		total += n
		if n < retentionBatch {
			break
		}
	}
	if total > 0 {
		err = uc.audit.RecordDurable(tctx, AuditEvent{
			Action: domain.AuditRetentionRun, TargetType: "tenant", TargetID: tenantID, Outcome: "allowed",
			Metadata: map[string]any{"processed": total, "anonymized": total, "cutoff": plan.RequestCutoff.Format(time.RFC3339)},
		})
	}
	return total, err
}
