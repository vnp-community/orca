package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
)

const (
	artifactCleanupBatch    = 200
	artifactCleanupBatches  = 10
	artifactBlobGrace       = time.Hour
	artifactCleanupInterval = 6 * time.Hour
)

// CleanupSummary counts what one cleanup removed; numbers only, it is also what the log says.
type CleanupSummary struct {
	Tenants  int
	Drawings int
	Blobs    int
}

// ReviewArtifactCleanup removes drawings that stopped being in force a long time ago and the content nothing points at. A drawing in
// force is never removed. Blobs are touched whenever a drawing is saved, so one is removed only when it has gone unused for the grace
// period, which is what keeps the cleanup from racing a draw run that is saving the same content.
type ReviewArtifactCleanup struct {
	Repo ReviewArtifactRepository
	// Tenants lists the tenants to visit; RetentionStore.ListTenants is the cross-tenant read the service already has.
	Tenants   func(ctx context.Context) ([]string, error)
	Retention time.Duration
	Now       func() time.Time
	Log       *slog.Logger
}

func (c *ReviewArtifactCleanup) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

func (c *ReviewArtifactCleanup) logger() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}

// Run visits every tenant once. One tenant that fails does not stop the others.
func (c *ReviewArtifactCleanup) Run(ctx context.Context) (CleanupSummary, error) {
	tenants, err := c.Tenants(ctx)
	if err != nil {
		return CleanupSummary{}, err
	}
	var sum CleanupSummary
	for _, id := range tenants {
		if ctx.Err() != nil {
			return sum, ctx.Err()
		}
		tctx := tenant.WithActorType(tenant.WithTenantID(ctx, id), tenant.ActorSystem)
		d, b, terr := c.runTenant(tctx)
		sum.Drawings += d
		sum.Blobs += b
		if terr != nil {
			c.logger().WarnContext(ctx, "review drawing cleanup failed for a tenant", slog.String("tenant_id", id), slog.Any("error", terr))
			continue
		}
		sum.Tenants++
	}
	return sum, nil
}

func (c *ReviewArtifactCleanup) runTenant(ctx context.Context) (drawings, blobs int, err error) {
	now := c.now()
	for i := 0; i < artifactCleanupBatches; i++ {
		n, err := c.Repo.DeleteSupersededBefore(ctx, now.Add(-c.Retention), artifactCleanupBatch)
		drawings += n
		if err != nil {
			return drawings, blobs, err
		}
		if n < artifactCleanupBatch {
			break
		}
	}
	for i := 0; i < artifactCleanupBatches; i++ {
		n, err := c.Repo.DeleteOrphanBlobs(ctx, now.Add(-artifactBlobGrace), artifactCleanupBatch)
		blobs += n
		if err != nil {
			return drawings, blobs, err
		}
		if n < artifactCleanupBatch {
			break
		}
	}
	return drawings, blobs, nil
}

// RunLoop cleans at the interval until the context ends. A run that fails is logged and the loop goes on.
func (c *ReviewArtifactCleanup) RunLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = artifactCleanupInterval
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		if sum, err := c.Run(ctx); err != nil {
			c.logger().WarnContext(ctx, "review drawing cleanup failed", slog.Any("error", err))
		} else if sum.Drawings+sum.Blobs > 0 {
			c.logger().InfoContext(ctx, "review drawings cleaned", slog.Int("tenants", sum.Tenants), slog.Int("drawings", sum.Drawings), slog.Int("blobs", sum.Blobs))
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
