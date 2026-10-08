package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type PolicyRepository struct {
	*Repository
}

func NewPolicyRepository(r *Repository) *PolicyRepository {
	return &PolicyRepository{Repository: r}
}

var _ usecase.ApprovalPolicyRepository = (*PolicyRepository)(nil)

const policyColumns = `id, tenant_id, project_id, subject_type, request_type, size, urgency, approvers, allow_requester_approve, due_after_seconds, priority, enabled, version, created_by, created_at, updated_at`

func scanPolicy(row rowScanner) (domain.ApprovalPolicy, error) {
	var (
		p                              domain.ApprovalPolicy
		approversJSON                  []byte
		dueSecs                        sql.NullInt32
		subject                        string
		project, reqType, size, urgent sql.NullString
	)
	if err := row.Scan(&p.ID, &p.TenantID, &project, &subject, &reqType, &size, &urgent, &approversJSON,
		&p.AllowRequesterApprove, &dueSecs, &p.Priority, &p.Enabled, &p.Version, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.ApprovalPolicy{}, err
	}
	p.SubjectType = domain.SubjectType(subject)
	p.ProjectID, p.RequestType, p.Size, p.Urgency = nullStringPtr(project), nullStringPtr(reqType), nullStringPtr(size), nullStringPtr(urgent)
	if dueSecs.Valid {
		d := time.Duration(dueSecs.Int32) * time.Second
		p.DueAfter = &d
	}
	var strs []string
	if err := json.Unmarshal(approversJSON, &strs); err != nil {
		return domain.ApprovalPolicy{}, fmt.Errorf("mysql: parse policy approvers: %w", err)
	}
	approvers, err := domain.ParseApprovers(strs)
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}
	p.Approvers = approvers
	return p, nil
}

func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

func policyDueSeconds(p domain.ApprovalPolicy) any {
	if p.DueAfter == nil {
		return nil
	}
	return int32(p.DueAfter.Seconds())
}

func (r *PolicyRepository) ListEnabledCandidates(ctx context.Context, tenantID string, subjectType domain.SubjectType, projectID, requestType, size, urgency string) ([]domain.ApprovalPolicy, error) {
	var list []domain.ApprovalPolicy
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		rows, err := db.QueryContext(ctx, `SELECT `+policyColumns+` FROM approval_policies
			WHERE tenant_id = ? AND subject_type = ? AND enabled = 1
			  AND (project_id IS NULL OR project_id = ?)
			  AND (request_type IS NULL OR request_type = ?)
			  AND (size IS NULL OR size = ?)
			  AND (urgency IS NULL OR urgency = ?)`,
			tenantID, string(subjectType), nullIfEmpty(projectID), nullIfEmpty(requestType), nullIfEmpty(size), nullIfEmpty(urgency))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			p, err := scanPolicy(rows)
			if err != nil {
				return err
			}
			list = append(list, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("mysql: list policy candidates: %w", err)
	}
	return list, nil
}

func (r *PolicyRepository) Get(ctx context.Context, tenantID, id string) (domain.ApprovalPolicy, error) {
	var out domain.ApprovalPolicy
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return domain.ErrApprovalPolicyNotFound
		}
		got, err := scanPolicy(db.QueryRowContext(ctx, `SELECT `+policyColumns+` FROM approval_policies WHERE tenant_id = ? AND id = ?`, tenantID, id))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrApprovalPolicyNotFound
		}
		out = got
		return err
	})
	return out, err
}

func (r *PolicyRepository) List(ctx context.Context, tenantID string, f usecase.PolicyListFilter) ([]domain.ApprovalPolicy, string, error) {
	where := []string{"tenant_id = ?"}
	args := []any{tenantID}
	if f.ProjectID != "" {
		where, args = append(where, "project_id = ?"), append(args, f.ProjectID)
	}
	if f.SubjectType != "" {
		where, args = append(where, "subject_type = ?"), append(args, string(f.SubjectType))
	}
	if f.PageToken != "" {
		at, id, err := usecase.DecodeApprovalCursor(f.PageToken)
		if err != nil {
			return nil, "", err
		}
		where, args = append(where, "(created_at < ? OR (created_at = ? AND id < ?))"), append(args, at, at, id)
	}
	args = append(args, f.PageSize+1)
	var list []domain.ApprovalPolicy
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		rows, err := db.QueryContext(ctx, `SELECT `+policyColumns+` FROM approval_policies WHERE `+strings.Join(where, " AND ")+
			` ORDER BY created_at DESC, id DESC LIMIT ?`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			p, err := scanPolicy(rows)
			if err != nil {
				return err
			}
			list = append(list, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", fmt.Errorf("mysql: list policies: %w", err)
	}
	next := ""
	if len(list) > f.PageSize {
		list = list[:f.PageSize]
		last := list[len(list)-1]
		next = usecase.EncodeApprovalCursor(last.CreatedAt, last.ID)
	}
	return list, next, nil
}

func (r *PolicyRepository) Upsert(ctx context.Context, p domain.ApprovalPolicy, expectedVersion int64) (domain.ApprovalPolicy, error) {
	approvers, err := json.Marshal(domain.ApproverStrings(p.Approvers))
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}
	var out domain.ApprovalPolicy
	err = r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		if p.TenantID != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		if expectedVersion == 0 {
			_, err := db.ExecContext(ctx, `INSERT INTO approval_policies (id, tenant_id, project_id, subject_type, request_type, size, urgency,
					approvers, allow_requester_approve, due_after_seconds, priority, enabled, version, created_by)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,?,1,?)`,
				p.ID, tenantID, p.ProjectID, string(p.SubjectType), p.RequestType, p.Size, p.Urgency, string(approvers),
				p.AllowRequesterApprove, policyDueSeconds(p), p.Priority, p.Enabled, p.CreatedBy)
			var myErr *mysql.MySQLError
			if errors.As(err, &myErr) && myErr.Number == 1062 {
				return domain.ErrApprovalVersionConflict // id already exists: creating needs a fresh id
			}
			if err != nil {
				return err
			}
		} else {
			res, err := db.ExecContext(ctx, `UPDATE approval_policies SET project_id = ?, subject_type = ?, request_type = ?, size = ?,
					urgency = ?, approvers = ?, allow_requester_approve = ?, due_after_seconds = ?, priority = ?, enabled = ?,
					version = version + 1
				WHERE tenant_id = ? AND id = ? AND version = ?`,
				p.ProjectID, string(p.SubjectType), p.RequestType, p.Size, p.Urgency, string(approvers), p.AllowRequesterApprove,
				policyDueSeconds(p), p.Priority, p.Enabled, tenantID, p.ID, expectedVersion)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return r.missingOrConflict(ctx, db, tenantID, p.ID)
			}
		}
		got, err := scanPolicy(db.QueryRowContext(ctx, `SELECT `+policyColumns+` FROM approval_policies WHERE tenant_id = ? AND id = ?`, tenantID, p.ID))
		out = got
		return err
	})
	if err != nil {
		return domain.ApprovalPolicy{}, wrapPolicyErr("upsert policy", err)
	}
	return out, nil
}

func (r *PolicyRepository) missingOrConflict(ctx context.Context, db dbExecer, tenantID, id string) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM approval_policies WHERE tenant_id = ? AND id = ?`, tenantID, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrApprovalPolicyNotFound
	}
	return domain.ErrApprovalVersionConflict
}

func (r *PolicyRepository) Delete(ctx context.Context, tenantID, id string, expectedVersion int64) error {
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return domain.ErrApprovalPolicyNotFound
		}
		res, err := db.ExecContext(ctx, `DELETE FROM approval_policies WHERE tenant_id = ? AND id = ? AND (? = 0 OR version = ?)`,
			tenantID, id, expectedVersion, expectedVersion)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return r.missingOrConflict(ctx, db, tenantID, id)
		}
		return nil
	})
	return wrapPolicyErr("delete policy", err)
}

func wrapPolicyErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrApprovalPolicyNotFound) || errors.Is(err, domain.ErrApprovalVersionConflict) {
		return err
	}
	return fmt.Errorf("mysql: %s: %w", op, err)
}

func (r *PolicyRepository) NowDB(ctx context.Context) (time.Time, error) {
	var now time.Time
	err := r.exec(ctx).QueryRowContext(ctx, "SELECT CURRENT_TIMESTAMP(6)").Scan(&now)
	return now.UTC(), err
}

type ApproverRepository struct {
	*Repository
}

func NewApproverRepository(r *Repository) *ApproverRepository {
	return &ApproverRepository{Repository: r}
}

var _ usecase.ApprovalApproverRepository = (*ApproverRepository)(nil)

func (r *ApproverRepository) InsertSnapshot(ctx context.Context, approvalID, tenantID string, approvers []domain.Principal) error {
	return r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return domain.ErrRequestTenantRequired()
		}
		for _, p := range approvers {
			if _, err := db.ExecContext(ctx, `INSERT INTO approval_approvers (approval_id, tenant_id, principal_kind, principal_id)
				VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE principal_id = principal_id`, approvalID, tenantID, string(p.Kind), p.ID); err != nil {
				return fmt.Errorf("mysql: insert approver snapshot: %w", err)
			}
		}
		return nil
	})
}

func (r *ApproverRepository) ListForApproval(ctx context.Context, tenantID, approvalID string) ([]domain.Principal, error) {
	var list []domain.Principal
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		rows, err := db.QueryContext(ctx, `SELECT principal_kind, principal_id FROM approval_approvers
			WHERE tenant_id = ? AND approval_id = ? ORDER BY principal_kind, principal_id`, tenantID, approvalID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p domain.Principal
			var kind string
			if err := rows.Scan(&kind, &p.ID); err != nil {
				return err
			}
			p.Kind = domain.PrincipalKind(kind)
			list = append(list, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("mysql: list approvers: %w", err)
	}
	return list, nil
}
