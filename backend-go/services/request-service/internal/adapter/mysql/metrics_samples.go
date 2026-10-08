package mysql

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// MetricsSamples counts rows across tenants for the Prometheus gauges; it returns counts, never rows.
type MetricsSamples struct{ *Repository }

func NewMetricsSamples(r *Repository) *MetricsSamples { return &MetricsSamples{Repository: r} }

var _ usecase.MetricsSampleSource = (*MetricsSamples)(nil)

func (m *MetricsSamples) PendingApprovalsBySubject(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	rows, err := m.db.QueryContext(ctx, `SELECT subject_type, COUNT(*) FROM approvals WHERE status = 'pending' GROUP BY subject_type`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var subject string
		var n int
		if err := rows.Scan(&subject, &n); err != nil {
			return nil, err
		}
		out[subject] = n
	}
	return out, rows.Err()
}

func (m *MetricsSamples) CountStuck(ctx context.Context, status string, olderThan time.Time) (int, error) {
	var n int
	err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM requests WHERE status = ? AND updated_at < ?`, status, olderThan.UTC()).Scan(&n)
	return n, err
}

func (m *MetricsSamples) CountOutboxPending(ctx context.Context) (int, error) {
	var n int
	err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE published_at IS NULL`).Scan(&n)
	return n, err
}
