package postgres

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// Add persists a single task_edges row. The cycle check runs in the
// usecase layer (internal/usecase.AddEdge), not here — see that package's
// doc comment. NOTE: on AddEdge's own standalone RPC path, this is a plain
// INSERT against r.db (the pool), not wrapped in the same transaction as
// the cycle-check SELECT that precedes it in the usecase — task-service.md
// §8 calls for check-and-write atomicity that path doesn't provide yet; see
// this service's README. This is a DIFFERENT, narrower gap than TASK-224
// Gap 2 (AIApply's create-subtask+add-edge loop): when Add runs inside
// Repository.RunInTx's fn (r.db is a pgx.Tx there), it participates in that
// transaction correctly — see ai_apply.go's doc comment.
func (r *Repository) Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO task.task_edges (tenant_id, from_task_id, to_task_id, edge_type)
		VALUES ($1, $2, $3, $4)
	`, tenantID, edge.FromTaskID, edge.ToTaskID, string(edge.Kind))
	if err != nil {
		return fmt.Errorf("postgres: insert task edge: %w", err)
	}
	return nil
}

// ListByKind returns every edge of the given kind for the tenant — the
// graph AddEdge's cycle check walks (domain.DetectCycle). Fetching the
// whole kind-scoped edge set rather than only the reachable subgraph keeps
// this query simple; see this service's README for the scale follow-up
// task-service.md §8 flags for large hierarchies.
func (r *Repository) ListByKind(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task.task_edges
		WHERE tenant_id = $1 AND edge_type = $2
	`, tenantID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("postgres: query edges by kind: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// ListFrom returns the edges of the given kind originating at fromTaskID —
// used by the Execute usecase's complexity branch (§3.1).
func (r *Repository) ListFrom(ctx context.Context, tenantID, fromTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task.task_edges
		WHERE tenant_id = $1 AND from_task_id = $2 AND edge_type = $3
	`, tenantID, fromTaskID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("postgres: query edges from task: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// ListByKindForUpdate is ListByKind's transaction-scoped, row-locked
// variant — SELECT ... FOR UPDATE over the kind-scoped edge set, closing
// the check-then-write race AddEdge's prior two-call shape allowed. Only
// meaningful when called through TxRunner.RunInTx's fn (r.db is a pgx.Tx
// there); called outside a transaction it behaves like ListByKind.
func (r *Repository) ListByKindForUpdate(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task.task_edges
		WHERE tenant_id = $1 AND edge_type = $2
		FOR UPDATE
	`, tenantID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("postgres: query edges by kind for update: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// ListTo returns the edges of the given kind terminating AT toTaskID — the
// symmetric counterpart to ListFrom, used by UpdateTask's un-block step to
// find a task's dependents.
func (r *Repository) ListTo(ctx context.Context, tenantID, toTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error) {
	rows, err := r.db.Query(ctx, `
		SELECT from_task_id, to_task_id, edge_type
		FROM task.task_edges
		WHERE tenant_id = $1 AND to_task_id = $2 AND edge_type = $3
	`, tenantID, toTaskID, string(kind))
	if err != nil {
		return nil, fmt.Errorf("postgres: query edges to task: %w", err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// edgeRows is the minimal interface both pgx.Rows results here satisfy —
// lets scanEdges be shared by ListByKind and ListFrom without depending on
// pgx.Rows's full surface.
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
			return nil, fmt.Errorf("postgres: scan edge row: %w", err)
		}
		e.Kind = domain.EdgeKind(kind)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate edge rows: %w", err)
	}
	return out, nil
}
