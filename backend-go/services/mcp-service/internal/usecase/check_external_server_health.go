package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// CheckHealth probes approved http servers that were not checked within
// interval. Each replica may run it: ClaimHealthChecks stamps rows so a server
// is probed once per interval. It records health and the observed digest and
// emits events on a health flip or a tools change, but never re-approves.
func (uc *ExternalServerRegistry) CheckHealth(ctx context.Context, interval time.Duration, limit int) (int, error) {
	now := uc.clock.Now()
	keys, err := uc.repo.ClaimHealthChecks(ctx, now.Add(-interval), limit)
	if err != nil {
		return 0, wrapRepoErr(err, "failed to claim health checks")
	}
	done := 0
	for _, k := range keys {
		s, err := uc.repo.GetExternalServer(ctx, k.TenantID, k.ServerID)
		if err != nil {
			continue
		}
		at := uc.clock.Now()
		tools, digest, perr := uc.observe(ctx, s)
		h := domain.Health{OK: perr == nil, CheckedAt: at}
		var evs []domain.OutboxRecord
		if perr != nil {
			h.Error = "probe failed"
			if ae := asAppErr(perr); ae != nil {
				h.Error = ae.Message
			}
		}
		if s.Health == nil || s.Health.OK != h.OK {
			if ev, e := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectExternalServerHealth, k.TenantID, at,
				map[string]any{"server_id": s.ID, "name": s.Name, "ok": h.OK}); e == nil {
				evs = append(evs, ev)
			}
		}
		if perr == nil {
			if s.ApprovedDigest != "" && digest != s.ApprovedDigest && digest != s.LastProbeDigest {
				if ev, e := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectExternalServerToolsChanged, k.TenantID, at,
					map[string]any{"server_id": s.ID, "name": s.Name, "digest": digest, "approved_digest": s.ApprovedDigest}); e == nil {
					evs = append(evs, ev)
				}
			}
			if err := uc.repo.RecordProbe(ctx, k.TenantID, s.ID, ProbeRecord{Digest: digest, Tools: tools, At: at, Source: "health"}, nil); err != nil {
				continue
			}
		}
		if err := uc.repo.RecordHealth(ctx, k.TenantID, s.ID, h, evs); err == nil {
			done++
		}
	}
	return done, nil
}
