package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RequestRepository is a separate type because Repository already owns Insert/List/Get-style
// methods for approvals and policies with different signatures.
type RequestRepository struct {
	*Repository
}

func NewRequestRepository(r *Repository) *RequestRepository {
	return &RequestRepository{Repository: r}
}

var _ usecase.RequestRepository = (*RequestRepository)(nil)

func (r *RequestRepository) Create(ctx context.Context, req domain.Request) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if req.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		if req.Number <= 0 {
			return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_NUMBER_REQUIRED", "request number must come from NextNumber", nil)
		}
		args := append([]any{req.ID, req.TenantID, req.Number}, requestArgs(req)...)
		args = append(args, req.CreatedAt, req.UpdatedAt, req.Version)
		args = append(args, contentArgs(req)...)
		_, err := db.ExecContext(ctx, `
			INSERT INTO requests (id, tenant_id, number, project_id, title, body, source_provider, source_ref, source_url,
				source_site, type, type_source, size, urgency, confidence, classification_reason, status, returned_from_stage,
				return_reason, plan_task_id, reporter_id, solution_engine, returned_category, source_hints, classification_attempts, created_at, updated_at, version,
				content_schema_version, acceptance_criteria, type_fields, content_revision, content_digest)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, args...)
		if err != nil {
			return fmt.Errorf("mysql: create request: %w", err)
		}
		return nil
	})
}

func (r *RequestRepository) Get(ctx context.Context, id string) (domain.Request, error) {
	return r.getBy(ctx, "id = ?", id, id)
}

func (r *RequestRepository) GetByNumber(ctx context.Context, number int64) (domain.Request, error) {
	return r.getBy(ctx, "number = ?", number, fmt.Sprint(number))
}

func (r *RequestRepository) getBy(ctx context.Context, cond string, arg any, label string) (domain.Request, error) {
	var out domain.Request
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if s, ok := arg.(string); ok {
			if _, perr := uuid.Parse(s); perr != nil {
				return domain.ErrRequestNotFound(label)
			}
		}
		got, err := scanRequest(db.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM requests WHERE tenant_id = ? AND `+cond, tenantID, arg))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrRequestNotFound(label)
		}
		if err != nil {
			return fmt.Errorf("mysql: get request: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *RequestRepository) Update(ctx context.Context, req domain.Request, expectedVersion int64) (domain.Request, error) {
	var out domain.Request
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		args := append(requestArgs(req), req.ID, tenantID, expectedVersion)
		res, err := db.ExecContext(ctx, `
			UPDATE requests SET project_id=?, title=?, body=?, source_provider=?, source_ref=?, source_url=?,
				source_site=?, type=?, type_source=?, size=?, urgency=?, confidence=?, classification_reason=?,
				status=?, returned_from_stage=?, return_reason=?, plan_task_id=?, reporter_id=?, solution_engine=?, returned_category=?,
				source_hints=?, classification_attempts=?,
				version = version + 1, updated_at = CURRENT_TIMESTAMP(6)
			WHERE id = ? AND tenant_id = ? AND version = ?`, args...)
		if err != nil {
			return fmt.Errorf("mysql: update request: %w", err)
		}
		// version always changes, so affected rows is 1 on success even without clientFoundRows.
		if n, _ := res.RowsAffected(); n == 1 {
			got, err := scanRequest(db.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM requests WHERE id = ? AND tenant_id = ?`, req.ID, tenantID))
			if err != nil {
				return fmt.Errorf("mysql: reread request: %w", err)
			}
			out = got
			return nil
		}
		return r.classifyMissing(ctx, db, "request", tenantID, req.ID, domain.ErrRequestNotFound(req.ID), domain.ErrRequestVersionConflict(req.ID, expectedVersion))
	})
	return out, err
}

// classifyMissing tells a CAS miss on a missing row apart from a stale version.
func (r *Repository) classifyMissing(ctx context.Context, db dbExecer, table, tenantID, id string, notFound, conflict error) error {
	var one int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM `+table+`s WHERE id = ? AND tenant_id = ?`, id, tenantID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound
	}
	if err != nil {
		return fmt.Errorf("mysql: probe %s: %w", table, err)
	}
	return conflict
}

func (r *RequestRepository) UpdateSolutionEngine(ctx context.Context, id string, name domain.EngineName, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE requests SET solution_engine = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP(6)
			WHERE id = ? AND tenant_id = ? AND version = ?`, string(name), id, tenantID, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: update solution engine: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return nil
		}
		return r.classifyMissing(ctx, db, "request", tenantID, id, domain.ErrRequestNotFound(id), domain.ErrRequestVersionConflict(id, expectedVersion))
	})
}

func (r *RequestRepository) List(ctx context.Context, f usecase.ListFilter) (usecase.ListResult, error) {
	f, err := f.Normalize()
	if err != nil {
		return usecase.ListResult{}, err
	}
	cursorAt, cursorID, err := usecase.DecodePageToken(f.PageToken)
	if err != nil {
		return usecase.ListResult{}, err
	}
	var res usecase.ListResult
	err = r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		args := []any{tenantID}
		var where []string
		add := func(cond string, v any) {
			args = append(args, v)
			where = append(where, cond)
		}
		addIn := func(col string, vals []string) {
			where = append(where, col+" IN ("+strings.TrimSuffix(strings.Repeat("?,", len(vals)), ",")+")")
			for _, v := range vals {
				args = append(args, v)
			}
		}
		if f.ProjectID != "" {
			add("project_id = ?", f.ProjectID)
		}
		if len(f.ProjectIDs) > 0 {
			addIn("project_id", f.ProjectIDs)
		}
		if f.ReporterID != "" {
			add("reporter_id = ?", f.ReporterID)
		}
		if f.SourceProvider != "" {
			add("source_provider = ?", f.SourceProvider)
		}
		if f.SourceSite != "" {
			add("source_site = ?", f.SourceSite)
		}
		if f.SourceRef != "" {
			add("source_ref = ?", f.SourceRef)
		}
		if len(f.Statuses) > 0 {
			ss := make([]string, len(f.Statuses))
			for i, s := range f.Statuses {
				ss[i] = string(s)
			}
			addIn("status", ss)
		}
		if len(f.Types) > 0 {
			ts := make([]string, len(f.Types))
			for i, t := range f.Types {
				ts[i] = string(t)
			}
			addIn("type", ts)
		}
		if f.PageToken != "" {
			args = append(args, cursorAt, cursorID)
			where = append(where, "(created_at, id) < (?, ?)")
		}
		q := `SELECT ` + requestColumns + ` FROM requests WHERE tenant_id = ?`
		if len(where) > 0 {
			q += " AND " + strings.Join(where, " AND ")
		}
		args = append(args, f.PageSize+1)
		q += " ORDER BY created_at DESC, id DESC LIMIT ?"

		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("mysql: list requests: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			got, err := scanRequest(rows)
			if err != nil {
				return fmt.Errorf("mysql: scan request: %w", err)
			}
			res.Requests = append(res.Requests, got)
		}
		return rows.Err()
	})
	if err != nil {
		return usecase.ListResult{}, err
	}
	if len(res.Requests) > f.PageSize {
		res.Requests = res.Requests[:f.PageSize]
		last := res.Requests[len(res.Requests)-1]
		res.NextPageToken = usecase.EncodePageToken(last.CreatedAt, last.ID)
	}
	return res, nil
}

// NextNumber is only valid inside InTx: a number handed out outside a
// transaction would survive a rolled-back Create and leave a gap.
func (r *RequestRepository) NextNumber(ctx context.Context) (int64, error) {
	if !r.inTx(ctx) {
		return 0, apperrors.New(apperrors.KindInternal, "REQUEST_NEXT_NUMBER_NEEDS_TX", "usecase: NextNumber requires InTx", nil)
	}
	var n int64
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.ExecContext(ctx, `INSERT INTO request_counters (tenant_id, next_number) VALUES (?, 1)
			ON DUPLICATE KEY UPDATE next_number = next_number + 1`, tenantID); err != nil {
			return err
		}
		// Same transaction still holds the counter row lock, so this read is the number we just took.
		return db.QueryRowContext(ctx, `SELECT next_number FROM request_counters WHERE tenant_id = ?`, tenantID).Scan(&n)
	})
	if err != nil {
		return 0, fmt.Errorf("mysql: next number: %w", err)
	}
	return n, nil
}
