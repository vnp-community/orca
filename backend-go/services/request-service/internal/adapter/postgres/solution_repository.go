package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type SolutionRecordRepository struct {
	*Repository
}

func NewSolutionRecordRepository(r *Repository) *SolutionRecordRepository {
	return &SolutionRecordRepository{Repository: r}
}

var _ usecase.SolutionStore = (*SolutionRecordRepository)(nil)

const solutionColumns = `id, tenant_id, request_id, kind, status, options, chosen_option, content_ref, generation_run_id, version, created_at, updated_at,
	seq, schema_version, provenance, input_request_revision, content_digest`

func scanSolution(row pgx.Row) (domain.Solution, error) {
	var s domain.Solution
	var chosen *int32
	var kind, status string
	var runID *string
	var seq *int32
	if err := row.Scan(&s.ID, &s.TenantID, &s.RequestID, &kind, &status, &s.OptionsJSON, &chosen, &s.ContentRef, &runID, &s.Version, &s.CreatedAt, &s.UpdatedAt,
		&seq, &s.SchemaVersion, &s.ProvenanceJSON, &s.InputRequestRevision, &s.ContentDigest); err != nil {
		return domain.Solution{}, err
	}
	if seq != nil {
		s.Seq = int(*seq)
	}
	if chosen != nil {
		v := int(*chosen)
		s.ChosenOption = &v
	}
	s.Kind, s.Status, s.GenerationRunID = domain.SolutionKind(kind), domain.SolutionStatus(status), derefString(runID)
	s.CreatedAt, s.UpdatedAt = s.CreatedAt.UTC(), s.UpdatedAt.UTC()
	return s, nil
}

func solutionOptions(s domain.Solution) string {
	if len(s.OptionsJSON) == 0 {
		return "[]"
	}
	return string(s.OptionsJSON)
}

// Rows written before the solution flow (and tests of the foundation) carry no kind/status; the column defaults apply.
func solutionKind(s domain.Solution) string {
	if s.Kind == "" {
		return string(domain.SolutionKindSolution)
	}
	return string(s.Kind)
}

func solutionStatus(s domain.Solution) string {
	if s.Status == "" {
		return string(domain.SolutionStatusDraft)
	}
	return string(s.Status)
}

func (r *SolutionRecordRepository) Insert(ctx context.Context, s domain.Solution) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if s.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		return insertSolutionRow(ctx, db, tenantID, s)
	})
}

func insertSolutionRow(ctx context.Context, db dbExecer, tenantID string, s domain.Solution) error {
	seq := s.Seq
	if seq == 0 {
		// Every write path gets a seq here; the request row lock serialises two inserts of one request.
		if _, err := db.Exec(ctx, `SELECT 1 FROM request.requests WHERE id = $1::uuid AND tenant_id = $2 FOR UPDATE`, s.RequestID, tenantID); err != nil {
			return fmt.Errorf("postgres: lock request for solution seq: %w", err)
		}
		if err := db.QueryRow(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM request.solutions WHERE tenant_id = $1 AND request_id = $2::uuid`, tenantID, s.RequestID).Scan(&seq); err != nil {
			return fmt.Errorf("postgres: next solution seq: %w", err)
		}
	}
	_, err := db.Exec(ctx, `INSERT INTO request.solutions (id, tenant_id, request_id, kind, status, options, chosen_option, content_ref, generation_run_id, version, created_at, updated_at,
			seq, schema_version, provenance, input_request_revision, content_digest)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12, $13, $14, $15::jsonb, $16, $17)`,
		s.ID, tenantID, s.RequestID, solutionKind(s), solutionStatus(s), solutionOptions(s), s.ChosenOption, s.ContentRef, nullIfEmpty(s.GenerationRunID), s.Version, s.CreatedAt, s.UpdatedAt,
		seq, positiveOr(s.SchemaVersion, 1), jsonOrEmptyObject(s.ProvenanceJSON), positiveOr(s.InputRequestRevision, 1), s.ContentDigest)
	if err != nil {
		return fmt.Errorf("postgres: insert solution: %w", err)
	}
	return nil
}

func positiveOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func jsonOrEmptyObject(b []byte) string {
	if len(b) == 0 {
		return "{}"
	}
	return string(b)
}

func (r *SolutionRecordRepository) Get(ctx context.Context, id string) (domain.Solution, error) {
	var out domain.Solution
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(id); perr != nil {
			return domain.ErrSolutionNotFound(id)
		}
		got, err := scanSolution(db.QueryRow(ctx, `SELECT `+solutionColumns+` FROM request.solutions WHERE id = $1 AND tenant_id = $2`, id, tenantID))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrSolutionNotFound(id)
		}
		if err != nil {
			return fmt.Errorf("postgres: get solution: %w", err)
		}
		out = got
		return nil
	})
	return out, err
}

func (r *SolutionRecordRepository) ListByRequestID(ctx context.Context, requestID string) ([]domain.Solution, error) {
	return r.ListByRequest(ctx, usecase.SolutionListFilter{RequestID: requestID})
}

func (r *SolutionRecordRepository) ListByRequest(ctx context.Context, f usecase.SolutionListFilter) ([]domain.Solution, error) {
	var out []domain.Solution
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, perr := uuid.Parse(f.RequestID); perr != nil {
			return nil
		}
		limit := f.Limit
		if limit <= 0 {
			limit = 200
		}
		rows, err := db.Query(ctx, `SELECT `+solutionColumns+` FROM request.solutions
			WHERE tenant_id = $1 AND request_id = $2::uuid
			  AND ($3 = '' OR kind = $3) AND ($4 = '' OR status = $4)
			ORDER BY created_at, id LIMIT $5`, tenantID, f.RequestID, string(f.Kind), string(f.Status), limit)
		if err != nil {
			return fmt.Errorf("postgres: list solutions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			s, err := scanSolution(rows)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}

func (r *SolutionRecordRepository) Update(ctx context.Context, s domain.Solution, expectedVersion int64) (domain.Solution, error) {
	var out domain.Solution
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		got, err := scanSolution(db.QueryRow(ctx, `UPDATE request.solutions SET options = $4::jsonb, chosen_option = $5,
			status = $6, content_ref = $7, version = version + 1, updated_at = now(),
			schema_version = CASE WHEN $8 > 0 THEN $8 ELSE schema_version END,
			provenance = COALESCE(NULLIF($9, '')::jsonb, provenance),
			input_request_revision = CASE WHEN $10 > 0 THEN $10 ELSE input_request_revision END,
			content_digest = CASE WHEN $11 <> '' THEN $11 ELSE content_digest END
			WHERE id = $1 AND tenant_id = $2 AND version = $3 RETURNING `+solutionColumns,
			s.ID, tenantID, expectedVersion, solutionOptions(s), s.ChosenOption, solutionStatus(s), s.ContentRef,
			s.SchemaVersion, string(s.ProvenanceJSON), s.InputRequestRevision, s.ContentDigest))
		if err == nil {
			out = got
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: update solution: %w", err)
		}
		return r.classifyMissing(ctx, db, "solution", tenantID, s.ID, domain.ErrSolutionNotFound(s.ID), domain.ErrSolutionVersionConflict(s.ID, expectedVersion))
	})
	return out, err
}

func (r *SolutionRecordRepository) Choose(ctx context.Context, id string, idx int, expectedVersion int64) (bool, error) {
	ok := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.solutions SET chosen_option = $3, version = version + 1, updated_at = now()
			WHERE id = $1 AND tenant_id = $2 AND status = 'proposed' AND version = $4`, id, tenantID, idx, expectedVersion)
		if err != nil {
			return fmt.Errorf("postgres: choose option: %w", err)
		}
		ok = tag.RowsAffected() == 1
		return nil
	})
	return ok, err
}

func (r *SolutionRecordRepository) SupersedeOpen(ctx context.Context, requestID string, kind domain.SolutionKind, exceptID string) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		tag, err := db.Exec(ctx, `UPDATE request.solutions SET status = 'superseded', version = version + 1, updated_at = now()
			WHERE tenant_id = $1 AND request_id = $2 AND kind = $3 AND status IN ('proposed','rejected')
			  AND ($4::uuid IS NULL OR id <> $4::uuid)`, tenantID, requestID, string(kind), nullIfEmpty(exceptID))
		if err != nil {
			return fmt.Errorf("postgres: supersede solutions: %w", err)
		}
		n = int(tag.RowsAffected())
		return nil
	})
	return n, err
}

func (r *SolutionRecordRepository) DeleteDraft(ctx context.Context, id string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.Exec(ctx, `DELETE FROM request.solutions WHERE id = $1 AND tenant_id = $2 AND status = 'draft'`, id, tenantID); err != nil {
			return fmt.Errorf("postgres: delete draft solution: %w", err)
		}
		return nil
	})
}
