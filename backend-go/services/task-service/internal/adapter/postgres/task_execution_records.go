package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const executionRecordColumns = `id::text, tenant_id::text, task_id::text, COALESCE(execution_link_id::text, ''), attempt,
	spec_digest, packet_digest, template_version, parse_status, COALESCE(failure_class, ''),
	result::text, changes::text, stdout_tail, created_at`

func scanExecutionRecord(row pgx.Row) (domain.ExecutionRecord, error) {
	var rec domain.ExecutionRecord
	var parse, class string
	var result, changes *string
	if err := row.Scan(&rec.ID, &rec.TenantID, &rec.TaskID, &rec.ExecutionLinkID, &rec.Attempt,
		&rec.SpecDigest, &rec.PacketDigest, &rec.TemplateVersion, &parse, &class, &result, &changes, &rec.StdoutTail, &rec.CreatedAt); err != nil {
		return domain.ExecutionRecord{}, err
	}
	rec.ParseStatus, rec.FailureClass = domain.ParseStatus(parse), domain.FailureClass(class)
	if result != nil {
		rec.Result = []byte(*result)
	}
	if changes != nil {
		rec.Changes = []byte(*changes)
	}
	return rec, nil
}

func (r *Repository) InsertExecutionRecord(ctx context.Context, rec domain.ExecutionRecord) (domain.ExecutionRecord, error) {
	if rec.ID == "" {
		rec.ID = uuid.NewString()
	}
	var out domain.ExecutionRecord
	err := r.inTenantTx(ctx, rec.TenantID, func(db dbtx) error {
		var link, class *string
		if rec.ExecutionLinkID != "" {
			link = &rec.ExecutionLinkID
		}
		if rec.FailureClass != "" {
			c := string(rec.FailureClass)
			class = &c
		}
		got, err := scanExecutionRecord(db.QueryRow(ctx, `
			INSERT INTO task.task_execution_records
				(id, tenant_id, task_id, execution_link_id, attempt, spec_digest, packet_digest, template_version,
				 parse_status, failure_class, result, changes, stdout_tail)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12::jsonb,$13)
			RETURNING `+executionRecordColumns,
			rec.ID, rec.TenantID, rec.TaskID, link, rec.Attempt, rec.SpecDigest, rec.PacketDigest, rec.TemplateVersion,
			string(rec.ParseStatus), class, nullableJSON(rec.Result), nullableJSON(rec.Changes), rec.StdoutTail))
		if err != nil {
			return fmt.Errorf("postgres: insert execution record: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func nullableJSON(b []byte) *string {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	return &s
}

func (r *Repository) ListExecutionRecords(ctx context.Context, tenantID string, taskIDs []string, latestOnly bool, limit int) ([]domain.ExecutionRecord, error) {
	if len(taskIDs) == 0 {
		return nil, usecase.ErrInvalidArgument
	}
	if limit <= 0 {
		limit = usecase.DefaultExecutionRecordLimit
	}
	limit = min(limit, usecase.MaxExecutionRecordLimit)
	query := `SELECT ` + executionRecordColumns + ` FROM task.task_execution_records
		WHERE tenant_id = $1 AND task_id = ANY($2::uuid[]) ORDER BY created_at DESC, id LIMIT $3`
	if latestOnly {
		query = `SELECT ` + executionRecordColumns + ` FROM (
			SELECT DISTINCT ON (task_id) * FROM task.task_execution_records
			WHERE tenant_id = $1 AND task_id = ANY($2::uuid[]) ORDER BY task_id, created_at DESC, id) latest
			ORDER BY created_at DESC, id LIMIT $3`
	}
	var out []domain.ExecutionRecord
	err := r.inTenantTx(ctx, tenantID, func(db dbtx) error {
		rows, err := db.Query(ctx, query, tenantID, taskIDs, limit)
		if err != nil {
			return fmt.Errorf("postgres: list execution records: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			rec, err := scanExecutionRecord(rows)
			if err != nil {
				return fmt.Errorf("postgres: scan execution record: %w", err)
			}
			out = append(out, rec)
		}
		return rows.Err()
	})
	return out, err
}

var _ usecase.TaskExecutionRecordRepository = (*Repository)(nil)
