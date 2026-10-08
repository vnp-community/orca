package postgres

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// MetricsSamples counts rows across tenants for the Prometheus gauges. It uses the relay role of the
// RLS policies (read only) and returns counts, never rows.
type MetricsSamples struct{ *Repository }

func NewMetricsSamples(r *Repository) *MetricsSamples { return &MetricsSamples{Repository: r} }

var _ usecase.MetricsSampleSource = (*MetricsSamples)(nil)

func (m *MetricsSamples) PendingApprovalsBySubject(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	err := m.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		rows, err := db.Query(ctx, `SELECT subject_type, count(*) FROM request.approvals WHERE status = 'pending' GROUP BY subject_type`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var subject string
			var n int
			if err := rows.Scan(&subject, &n); err != nil {
				return err
			}
			out[subject] = n
		}
		return rows.Err()
	})
	return out, err
}

func (m *MetricsSamples) CountStuck(ctx context.Context, status string, olderThan time.Time) (int, error) {
	var n int
	err := m.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		return db.QueryRow(ctx, `SELECT count(*) FROM request.requests WHERE status = $1 AND updated_at < $2`, status, olderThan).Scan(&n)
	})
	return n, err
}

func (m *MetricsSamples) CountOutboxPending(ctx context.Context) (int, error) {
	var n int
	err := m.withRelayTx(ctx, func(ctx context.Context, db dbExecer) error {
		return db.QueryRow(ctx, `SELECT count(*) FROM request.outbox_events WHERE published_at IS NULL`).Scan(&n)
	})
	return n, err
}
