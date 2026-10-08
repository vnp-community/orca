package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		_, err := db.Exec(ctx, `
			INSERT INTO request.requests (id, tenant_id, number, project_id, title, body, source_provider, source_ref, source_url,
				source_site, type, type_source, size, urgency, confidence, classification_reason, status, returned_from_stage,
				return_reason, plan_task_id, reporter_id, solution_engine, returned_category, source_hints, classification_attempts, created_at, updated_at, version,
				content_schema_version, acceptance_criteria, type_fields, content_revision, content_digest)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24::jsonb,$25,$26,$27,$28,$29,$30::jsonb,$31::jsonb,$32,$33)`, args...)
		if err != nil {
			return fmt.Errorf("postgres: create request: %w", err)
		}
		return nil
	})
}

func (r *RequestRepository) Get(ctx context.Context, id string) (domain.Request, error) {
	return r.getBy(ctx, "id = $2", id, id)
}

func (r *RequestRepository) GetByNumber(ctx context.Context, number int64) (domain.Request, error) {
	return r.getBy(ctx, "number = $2", number, fmt.Sprint(number))
}

func (r *RequestRepository) getBy(ctx context.Context, cond string, arg any, label string) (domain.Request, error) {
	var out domain.Request
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if s, ok := arg.(string); ok {
			if _, perr := uuid.Parse(s); perr != nil {
				return domain.ErrRequestNotFound(label)
			}
		}
		row := db.QueryRow(ctx, `SELECT `+requestColumns+` FROM request.requests WHERE tenant_id = $1 AND `+cond, tenantID, arg)
		got, err := scanRequest(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRequestNotFound(label)
		}
		if err != nil {
			return fmt.Errorf("postgres: get request: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *RequestRepository) Update(ctx context.Context, req domain.Request, expectedVersion int64) (domain.Request, error) {
	var out domain.Request
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		args := append([]any{req.ID, tenantID, expectedVersion}, requestArgs(req)...)
		row := db.QueryRow(ctx, `
			UPDATE request.requests SET project_id=$4, title=$5, body=$6, source_provider=$7, source_ref=$8, source_url=$9,
				source_site=$10, type=$11, type_source=$12, size=$13, urgency=$14, confidence=$15, classification_reason=$16,
				status=$17, returned_from_stage=$18, return_reason=$19, plan_task_id=$20, reporter_id=$21, solution_engine=$22, returned_category=$23,
				source_hints=$24::jsonb, classification_attempts=$25,
				version = version + 1, updated_at = now()
			WHERE id = $1 AND tenant_id = $2 AND version = $3
			RETURNING `+requestColumns, args...)
		got, err := scanRequest(row)
		if err == nil {
			out = got
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: update request: %w", err)
		}
		return r.classifyMissing(ctx, db, "request", tenantID, req.ID, domain.ErrRequestNotFound(req.ID), domain.ErrRequestVersionConflict(req.ID, expectedVersion))
	})
	return out, err
}

// classifyMissing tells a CAS miss on a missing row apart from a stale version.
func (r *Repository) classifyMissing(ctx context.Context, db dbExecer, table, tenantID, id string, notFound, conflict error) error {
	var one int
	err := db.QueryRow(ctx, `SELECT 1 FROM request.`+table+`s WHERE id = $1 AND tenant_id = $2`, id, tenantID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	if err != nil {
		return fmt.Errorf("postgres: probe %s: %w", table, err)
	}
	return conflict
}

func (r *RequestRepository) UpdateSolutionEngine(ctx context.Context, id string, name domain.EngineName, expectedVersion int64) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.requests SET solution_engine = $4, version = version + 1, updated_at = now()
			WHERE id = $1 AND tenant_id = $2 AND version = $3`, id, tenantID, expectedVersion, string(name))
		if err != nil {
			return fmt.Errorf("postgres: update solution engine: %w", err)
		}
		if tag.RowsAffected() == 1 {
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
			where = append(where, fmt.Sprintf(cond, len(args)))
		}
		if f.ProjectID != "" {
			add("project_id = $%d::uuid", f.ProjectID)
		}
		if len(f.ProjectIDs) > 0 {
			add("project_id = ANY($%d::uuid[])", f.ProjectIDs)
		}
		if f.ReporterID != "" {
			add("reporter_id = $%d::uuid", f.ReporterID)
		}
		if f.SourceProvider != "" {
			add("source_provider = $%d", f.SourceProvider)
		}
		if f.SourceSite != "" {
			add("source_site = $%d", f.SourceSite)
		}
		if f.SourceRef != "" {
			add("source_ref = $%d", f.SourceRef)
		}
		if len(f.Statuses) > 0 {
			ss := make([]string, len(f.Statuses))
			for i, s := range f.Statuses {
				ss[i] = string(s)
			}
			add("status = ANY($%d::text[])", ss)
		}
		if len(f.Types) > 0 {
			ts := make([]string, len(f.Types))
			for i, t := range f.Types {
				ts[i] = string(t)
			}
			add("type = ANY($%d::text[])", ts)
		}
		if f.PageToken != "" {
			args = append(args, cursorAt, cursorID)
			where = append(where, fmt.Sprintf("(created_at, id) < ($%d, $%d::uuid)", len(args)-1, len(args)))
		}
		q := `SELECT ` + requestColumns + ` FROM request.requests WHERE tenant_id = $1`
		if len(where) > 0 {
			q += " AND " + strings.Join(where, " AND ")
		}
		args = append(args, f.PageSize+1)
		q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args))

		rows, err := db.Query(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("postgres: list requests: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			got, err := scanRequest(rows)
			if err != nil {
				return fmt.Errorf("postgres: scan request: %w", err)
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
		return db.QueryRow(ctx, `
			INSERT INTO request.request_counters (tenant_id, next_number) VALUES ($1, 1)
			ON CONFLICT (tenant_id) DO UPDATE SET next_number = request.request_counters.next_number + 1
			RETURNING next_number`, tenantID).Scan(&n)
	})
	if err != nil {
		return 0, fmt.Errorf("postgres: next number: %w", err)
	}
	return n, nil
}
