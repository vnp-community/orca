package metrics

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// DefaultSampleInterval is how often the database gauges are refreshed.
const DefaultSampleInterval = 30 * time.Second

// StuckThresholds maps a Request status to how long it may stay there before it counts as stuck.
type StuckThresholds map[string]time.Duration

// SampleOnce refreshes approvals_pending, stuck and outbox_pending. A failing source leaves the previous
// value in place and is logged: a stale gauge is better than a gauge that drops to zero during an outage.
func (s *Set) SampleOnce(ctx context.Context, src usecase.MetricsSampleSource, th StuckThresholds, now time.Time, log *slog.Logger) {
	if pending, err := src.PendingApprovalsBySubject(ctx); err != nil {
		log.WarnContext(ctx, "metrics: sampling pending approvals failed", slog.Any("error", err))
	} else {
		for subject := range s.pendingSubjects {
			if _, still := pending[subject]; !still {
				s.approvalsPend.WithLabelValues(subject).Set(0)
			}
		}
		for subject, n := range pending {
			s.pendingSubjects[subject] = true
			s.approvalsPend.WithLabelValues(label(subject)).Set(float64(n))
		}
	}
	statuses := make([]string, 0, len(th))
	for status := range th {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		n, err := src.CountStuck(ctx, status, now.Add(-th[status]))
		if err != nil {
			log.WarnContext(ctx, "metrics: sampling stuck requests failed", slog.String("status", status), slog.Any("error", err))
			continue
		}
		s.stuck.WithLabelValues(status).Set(float64(n))
	}
	if n, err := src.CountOutboxPending(ctx); err != nil {
		log.WarnContext(ctx, "metrics: sampling outbox backlog failed", slog.Any("error", err))
	} else {
		s.outboxPending.Set(float64(n))
	}
}

// RunSampler samples immediately and then every interval until ctx ends, then returns.
func (s *Set) RunSampler(ctx context.Context, src usecase.MetricsSampleSource, th StuckThresholds, interval time.Duration, log *slog.Logger) {
	if interval <= 0 {
		interval = DefaultSampleInterval
	}
	if log == nil {
		log = slog.Default()
	}
	s.SampleOnce(ctx, src, th, time.Now(), log)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.SampleOnce(ctx, src, th, now, log)
		}
	}
}
