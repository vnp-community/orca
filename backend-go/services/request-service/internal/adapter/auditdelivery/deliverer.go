// Package auditdelivery moves durable audit entries from request_audit_outbox to auth-service. The entry
// is queued in the same transaction as the change it describes, so an auth-service outage delays but never loses it.
package auditdelivery

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/stablyai/orca-go/common/auditclient"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const (
	baseBackoff     = 5 * time.Second
	maxBackoff      = 15 * time.Minute
	alertAttempts   = 20
	purgeAfter      = 7 * 24 * time.Hour
	defaultBatch    = 50
	defaultInterval = 5 * time.Second
	deliverTimeout  = 10 * time.Second
)

// StrictClient is auditclient.Client.AppendDetailedStrict: it reports the RPC error.
type StrictClient interface {
	AppendDetailedStrict(ctx context.Context, e auditclient.Entry) error
}

// Deliverer is safe to run in several replicas: the store locks rows with SKIP LOCKED.
type Deliverer struct {
	Store    usecase.AuditOutboxStore
	Client   StrictClient
	Log      *slog.Logger
	Now      func() time.Time
	Batch    int
	Interval time.Duration

	failures atomic.Int64
	pending  atomic.Int64
}

// Backoff is min(2^attempts * 5s, 15m).
func Backoff(attempts int) time.Duration {
	if attempts >= 10 {
		return maxBackoff
	}
	d := baseBackoff << attempts
	if d > maxBackoff || d <= 0 {
		return maxBackoff
	}
	return d
}

// DeliveryFailures backs request_audit_delivery_failures_total; Pending backs request_audit_outbox_pending.
func (d *Deliverer) DeliveryFailures() int64 { return d.failures.Load() }
func (d *Deliverer) Pending() int64          { return d.pending.Load() }

// Once delivers one batch and returns how many entries were delivered and failed.
func (d *Deliverer) Once(ctx context.Context) (delivered, failed int, err error) {
	now, batch := time.Now, d.Batch
	if d.Now != nil {
		now = d.Now
	}
	if batch <= 0 {
		batch = defaultBatch
	}
	delivered, failed, err = d.Store.ProcessDue(ctx, now(), batch, Backoff, d.deliver)
	d.failures.Add(int64(failed))
	return delivered, failed, err
}

func (d *Deliverer) deliver(ctx context.Context, r usecase.AuditOutboxRecord) error {
	ctx, cancel := context.WithTimeout(ctx, deliverTimeout)
	defer cancel()
	err := d.Client.AppendDetailedStrict(ctx, auditclient.Entry{
		TenantID: r.TenantID, ActorID: r.ActorID, ActorType: r.ActorType, Action: r.Action,
		Target: r.TargetType + ":" + r.TargetID, TargetType: r.TargetType, TargetID: r.TargetID,
		Outcome: r.Outcome, IPAddress: r.IPAddress, MetadataJSON: r.MetadataJSON,
	})
	if err != nil && r.Attempts+1 == alertAttempts && d.Log != nil {
		d.Log.Error("audit entry still undelivered", slog.String("audit_id", r.AuditID), slog.String("action", r.Action), slog.Int("attempts", r.Attempts+1))
	}
	return err
}

// Run delivers until ctx ends, then returns; also purges old delivered rows and refreshes the pending gauge.
func (d *Deliverer) Run(ctx context.Context) {
	interval := d.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	lastHousekeeping := time.Time{}
	for {
		if _, _, err := d.Once(ctx); err != nil && ctx.Err() == nil && d.Log != nil {
			d.Log.Warn("audit delivery pass failed", slog.Any("error", err))
		}
		if time.Since(lastHousekeeping) > 30*time.Second {
			lastHousekeeping = time.Now()
			d.housekeeping(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (d *Deliverer) housekeeping(ctx context.Context) {
	if n, err := d.Store.CountPending(ctx); err == nil {
		d.pending.Store(int64(n))
	}
	if _, err := d.Store.PurgeDelivered(ctx, time.Now().Add(-purgeAfter), 200); err != nil && d.Log != nil && ctx.Err() == nil {
		d.Log.Warn("audit outbox purge failed", slog.Any("error", err))
	}
}
