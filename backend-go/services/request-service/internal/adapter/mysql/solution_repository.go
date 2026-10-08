package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
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

func scanSolution(row rowScanner) (domain.Solution, error) {
	var s domain.Solution
	var chosen sql.NullInt32
	var runID sql.NullString
	var seq sql.NullInt32
	var kind, status string
	if err := row.Scan(&s.ID, &s.TenantID, &s.RequestID, &kind, &status, &s.OptionsJSON, &chosen, &s.ContentRef, &runID, &s.Version, &s.CreatedAt, &s.UpdatedAt,
		&seq, &s.SchemaVersion, &s.ProvenanceJSON, &s.InputRequestRevision, &s.ContentDigest); err != nil {
		return domain.Solution{}, err
	}
	if seq.Valid {
		s.Seq = int(seq.Int32)
	}
	if chosen.Valid {
		v := int(chosen.Int32)
		s.ChosenOption = &v
	}
	s.Kind, s.Status, s.GenerationRunID = domain.SolutionKind(kind), domain.SolutionStatus(status), runID.String
	s.CreatedAt, s.UpdatedAt = s.CreatedAt.UTC(), s.UpdatedAt.UTC()
	return s, nil
}

func solutionOptions(s domain.Solution) string {
	if len(s.OptionsJSON) == 0 {
		return "[]"
	}
	return string(s.OptionsJSON)
}

func chosenArg(s domain.Solution) any {
	if s.ChosenOption == nil {
		return nil
	}
	return *s.ChosenOption
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
		var one int
		if err := db.QueryRowContext(ctx, `SELECT 1 FROM requests WHERE id = ? AND tenant_id = ? FOR UPDATE`, s.RequestID, tenantID).Scan(&one); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("mysql: lock request for solution seq: %w", err)
		}
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM solutions WHERE tenant_id = ? AND request_id = ?`, tenantID, s.RequestID).Scan(&seq); err != nil {
			return fmt.Errorf("mysql: next solution seq: %w", err)
		}
	}
	_, err := db.ExecContext(ctx, `INSERT INTO solutions (id, tenant_id, request_id, kind, status, options, chosen_option, content_ref, generation_run_id, version, created_at, updated_at,
			seq, schema_version, provenance, input_request_revision, content_digest)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, tenantID, s.RequestID, solutionKind(s), solutionStatus(s), solutionOptions(s), chosenArg(s), s.ContentRef, nullIfEmpty(s.GenerationRunID), s.Version, s.CreatedAt.UTC(), s.UpdatedAt.UTC(),
		seq, positiveOr(s.SchemaVersion, 1), jsonOrEmptyObject(s.ProvenanceJSON), positiveOr(s.InputRequestRevision, 1), s.ContentDigest)
	if err != nil {
		return fmt.Errorf("mysql: insert solution: %w", err)
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
		got, err := scanSolution(db.QueryRowContext(ctx, `SELECT `+solutionColumns+` FROM solutions WHERE id = ? AND tenant_id = ?`, id, tenantID))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrSolutionNotFound(id)
		}
		if err != nil {
			return fmt.Errorf("mysql: get solution: %w", err)
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
		limit := f.Limit
		if limit <= 0 {
			limit = 200
		}
		rows, err := db.QueryContext(ctx, `SELECT `+solutionColumns+` FROM solutions
			WHERE tenant_id = ? AND request_id = ?
			  AND (? = '' OR kind = ?) AND (? = '' OR status = ?)
			ORDER BY created_at, id LIMIT ?`, tenantID, f.RequestID, string(f.Kind), string(f.Kind), string(f.Status), string(f.Status), limit)
		if err != nil {
			return fmt.Errorf("mysql: list solutions: %w", err)
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
		res, err := db.ExecContext(ctx, `UPDATE solutions SET options = ?, chosen_option = ?, status = ?, content_ref = ?,
			version = version + 1, updated_at = CURRENT_TIMESTAMP(6),
			schema_version = CASE WHEN ? > 0 THEN ? ELSE schema_version END,
			provenance = COALESCE(CAST(NULLIF(?, '') AS JSON), provenance),
			input_request_revision = CASE WHEN ? > 0 THEN ? ELSE input_request_revision END,
			content_digest = CASE WHEN ? <> '' THEN ? ELSE content_digest END
			WHERE id = ? AND tenant_id = ? AND version = ?`,
			solutionOptions(s), chosenArg(s), solutionStatus(s), s.ContentRef,
			s.SchemaVersion, s.SchemaVersion, string(s.ProvenanceJSON), s.InputRequestRevision, s.InputRequestRevision, s.ContentDigest, s.ContentDigest,
			s.ID, tenantID, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: update solution: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			got, err := scanSolution(db.QueryRowContext(ctx, `SELECT `+solutionColumns+` FROM solutions WHERE id = ? AND tenant_id = ?`, s.ID, tenantID))
			if err != nil {
				return fmt.Errorf("mysql: reread solution: %w", err)
			}
			out = got
			return nil
		}
		return r.classifyMissing(ctx, db, "solution", tenantID, s.ID, domain.ErrSolutionNotFound(s.ID), domain.ErrSolutionVersionConflict(s.ID, expectedVersion))
	})
	return out, err
}

func (r *SolutionRecordRepository) Choose(ctx context.Context, id string, idx int, expectedVersion int64) (bool, error) {
	ok := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE solutions SET chosen_option = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP(6)
			WHERE id = ? AND tenant_id = ? AND status = 'proposed' AND version = ?`, idx, id, tenantID, expectedVersion)
		if err != nil {
			return fmt.Errorf("mysql: choose option: %w", err)
		}
		n, _ := res.RowsAffected()
		ok = n == 1
		return nil
	})
	return ok, err
}

func (r *SolutionRecordRepository) SupersedeOpen(ctx context.Context, requestID string, kind domain.SolutionKind, exceptID string) (int, error) {
	n := 0
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		res, err := db.ExecContext(ctx, `UPDATE solutions SET status = 'superseded', version = version + 1, updated_at = CURRENT_TIMESTAMP(6)
			WHERE tenant_id = ? AND request_id = ? AND kind = ? AND status IN ('proposed','rejected')
			  AND (? IS NULL OR id <> ?)`, tenantID, requestID, string(kind), nullIfEmpty(exceptID), exceptID)
		if err != nil {
			return fmt.Errorf("mysql: supersede solutions: %w", err)
		}
		c, _ := res.RowsAffected()
		n = int(c)
		return nil
	})
	return n, err
}

func (r *SolutionRecordRepository) DeleteDraft(ctx context.Context, id string) error {
	return r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if _, err := db.ExecContext(ctx, `DELETE FROM solutions WHERE id = ? AND tenant_id = ? AND status = 'draft'`, id, tenantID); err != nil {
			return fmt.Errorf("mysql: delete draft solution: %w", err)
		}
		return nil
	})
}
