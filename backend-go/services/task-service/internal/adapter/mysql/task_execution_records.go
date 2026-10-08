package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const executionRecordColumns = `id, tenant_id, task_id, execution_link_id, attempt, spec_digest, packet_digest, template_version,
	parse_status, failure_class, CAST(result AS CHAR), CAST(changes AS CHAR), stdout_tail, created_at`

func scanExecutionRecord(row rowScanner) (domain.ExecutionRecord, error) {
	var rec domain.ExecutionRecord
	var link, class, result, changes sql.NullString
	var parse string
	if err := row.Scan(&rec.ID, &rec.TenantID, &rec.TaskID, &link, &rec.Attempt, &rec.SpecDigest, &rec.PacketDigest,
		&rec.TemplateVersion, &parse, &class, &result, &changes, &rec.StdoutTail, &rec.CreatedAt); err != nil {
		return domain.ExecutionRecord{}, err
	}
	rec.ParseStatus, rec.ExecutionLinkID, rec.FailureClass = domain.ParseStatus(parse), link.String, domain.FailureClass(class.String)
	if result.Valid {
		rec.Result = []byte(result.String)
	}
	if changes.Valid {
		rec.Changes = []byte(changes.String)
	}
	return rec, nil
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

func (r *Repository) InsertExecutionRecord(ctx context.Context, rec domain.ExecutionRecord) (domain.ExecutionRecord, error) {
	if rec.ID == "" {
		rec.ID = uuid.NewString() // DEFAULT (UUID()) cannot be read back after the insert
	}
	var link, class any
	if rec.ExecutionLinkID != "" {
		link = rec.ExecutionLinkID
	}
	if rec.FailureClass != "" {
		class = string(rec.FailureClass)
	}
	var out domain.ExecutionRecord
	err := r.inTx(ctx, func(db dbtx) error {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO task_execution_records
				(id, tenant_id, task_id, execution_link_id, attempt, spec_digest, packet_digest, template_version,
				 parse_status, failure_class, result, changes, stdout_tail)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			rec.ID, rec.TenantID, rec.TaskID, link, rec.Attempt, rec.SpecDigest, rec.PacketDigest, rec.TemplateVersion,
			string(rec.ParseStatus), class, nullableJSON(rec.Result), nullableJSON(rec.Changes), rec.StdoutTail); err != nil {
			return fmt.Errorf("mysql: insert execution record: %w", err)
		}
		got, err := scanExecutionRecord(db.QueryRowContext(ctx, `SELECT `+executionRecordColumns+` FROM task_execution_records WHERE tenant_id = ? AND id = ?`, rec.TenantID, rec.ID))
		if err != nil {
			return fmt.Errorf("mysql: reread execution record: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *Repository) ListExecutionRecords(ctx context.Context, tenantID string, taskIDs []string, latestOnly bool, limit int) ([]domain.ExecutionRecord, error) {
	if len(taskIDs) == 0 {
		return nil, usecase.ErrInvalidArgument
	}
	if limit <= 0 {
		limit = usecase.DefaultExecutionRecordLimit
	}
	limit = min(limit, usecase.MaxExecutionRecordLimit)
	in := inClause(len(taskIDs))
	query := `SELECT ` + executionRecordColumns + ` FROM task_execution_records
		WHERE tenant_id = ? AND task_id IN (` + in + `) ORDER BY created_at DESC, id LIMIT ?`
	if latestOnly {
		query = `SELECT ` + executionRecordColumns + ` FROM (
			SELECT r.*, ROW_NUMBER() OVER (PARTITION BY task_id ORDER BY created_at DESC, id) AS rn
			FROM task_execution_records r WHERE tenant_id = ? AND task_id IN (` + in + `)) latest
			WHERE rn = 1 ORDER BY created_at DESC, id LIMIT ?`
	}
	args := append(append([]any{tenantID}, toAny(taskIDs)...), limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: list execution records: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.ExecutionRecord
	for rows.Next() {
		rec, err := scanExecutionRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan execution record: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

var _ usecase.TaskExecutionRecordRepository = (*Repository)(nil)
