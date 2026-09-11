package mysql

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// Add persists a single task_edges row — see
// internal/adapter/postgres/edges.go's identical doc comment for the
// check-then-write atomicity gap this shares (closed only when called
// through RunInTx's fn, where r.db is a *sql.Tx).
func (r *Repository) Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO task_edges (tenant_id, from_task_id, to_task_id, edge_type)
		VALUES (?, ?, ?, ?)
	`, tenantID, edge.FromTaskID, edge.ToTaskID, string(edge.Kind))
	if err != nil {
		return fmt.Errorf("mysql: insert task edge: %w", err)
	}
	return nil
}

func (r *Repository) ListByKind(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task_edges
		WHERE tenant_id = ? AND edge_type = ?
	`, tenantID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("mysql: query edges by kind: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

func (r *Repository) ListFrom(ctx context.Context, tenantID, fromTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task_edges
		WHERE tenant_id = ? AND from_task_id = ? AND edge_type = ?
	`, tenantID, fromTaskID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("mysql: query edges from task: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// ListByKindForUpdate is ListByKind's row-locked variant — SELECT ... FOR
// UPDATE, meaningful only inside RunInTx's fn (r.db is a *sql.Tx there);
// InnoDB supports FOR UPDATE identically to Postgres for this purpose.
func (r *Repository) ListByKindForUpdate(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task_edges
		WHERE tenant_id = ? AND edge_type = ?
		FOR UPDATE
	`, tenantID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("mysql: query edges by kind for update: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

func (r *Repository) ListTo(ctx context.Context, tenantID, toTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task_edges
		WHERE tenant_id = ? AND to_task_id = ? AND edge_type = ?
	`, tenantID, toTaskID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("mysql: query edges to task: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// edgeRows is the minimal interface *sql.Rows satisfies here — mirrors
// internal/adapter/postgres's identical abstraction.
type edgeRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanEdges(rows edgeRows) ([]domain.TaskEdge, error) {
	var out []domain.TaskEdge
	for rows.Next() {
		var e domain.TaskEdge
		var kind string
		if err := rows.Scan(&e.FromTaskID, &e.ToTaskID, &kind); err != nil {
			return nil, fmt.Errorf("mysql: scan edge row: %w", err)
		}
		e.Kind = domain.EdgeKind(kind)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate edge rows: %w", err)
	}
	return out, nil
}
