package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

const policyColumns = `id, tenant_id, project_id, subject_type, request_type, size, urgency, approvers, allow_requester_approve,
	due_after_seconds, priority, enabled, version, created_by, created_at, updated_at`

func scanPolicy(row pgx.Row) (domain.ApprovalPolicy, error) {
	var (
		p             domain.ApprovalPolicy
		approversJSON []byte
		dueSecs       *int32
		subject       string
	)
	if err := row.Scan(&p.ID, &p.TenantID, &p.ProjectID, &subject, &p.RequestType, &p.Size, &p.Urgency, &approversJSON,
		&p.AllowRequesterApprove, &dueSecs, &p.Priority, &p.Enabled, &p.Version, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.ApprovalPolicy{}, err
	}
	p.SubjectType = domain.SubjectType(subject)
	if dueSecs != nil {
		d := time.Duration(*dueSecs) * time.Second
		p.DueAfter = &d
	}
	var strs []string
	if err := json.Unmarshal(approversJSON, &strs); err != nil {
		return domain.ApprovalPolicy{}, fmt.Errorf("postgres: parse policy approvers: %w", err)
	}
	approvers, err := domain.ParseApprovers(strs)
	if err != nil {
		return domain.ApprovalPolicy{}, err
	}
	p.Approvers = approvers
	return p, nil
}

func policyDueSeconds(p domain.ApprovalPolicy) any {
	if p.DueAfter == nil {
		return nil
	}
	return int32(p.DueAfter.Seconds())
}

// ListEnabledCandidates returns every enabled policy that could match; SelectPolicy ranks them in the usecase.
// Empty context values bind as NULL so a policy that pins project/size never matches an unknown one.
func (r *PolicyRepository) ListEnabledCandidates(ctx context.Context, tenantID string, subjectType domain.SubjectType, projectID, requestType, size, urgency string) ([]domain.ApprovalPolicy, error) {
	var list []domain.ApprovalPolicy
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		rows, err := db.Query(ctx, `SELECT `+policyColumns+` FROM request.approval_policies
			WHERE tenant_id = $1 AND subject_type = $2 AND enabled = true
			  AND (project_id IS NULL OR project_id::text = $3)
			  AND (request_type IS NULL OR request_type = $4)
			  AND (size IS NULL OR size = $5)
			  AND (urgency IS NULL OR urgency = $6)`,
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
		return nil, fmt.Errorf("postgres: list policy candidates: %w", err)
	}
	return list, nil
}

func (r *PolicyRepository) Get(ctx context.Context, tenantID, id string) (domain.ApprovalPolicy, error) {
	var out domain.ApprovalPolicy
	if _, err := uuid.Parse(id); err != nil {
		return out, domain.ErrApprovalPolicyNotFound
	}
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return domain.ErrApprovalPolicyNotFound
		}
		got, err := scanPolicy(db.QueryRow(ctx, `SELECT `+policyColumns+` FROM request.approval_policies WHERE tenant_id = $1 AND id = $2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrApprovalPolicyNotFound
		}
		out = got
		return err
	})
	return out, err
}

func (r *PolicyRepository) List(ctx context.Context, tenantID string, f usecase.PolicyListFilter) ([]domain.ApprovalPolicy, string, error) {
	where := []string{"tenant_id = $1"}
	args := []any{tenantID}
	if f.ProjectID != "" {
		args = append(args, f.ProjectID)
		where = append(where, fmt.Sprintf("project_id::text = $%d", len(args)))
	}
	if f.SubjectType != "" {
		args = append(args, string(f.SubjectType))
		where = append(where, fmt.Sprintf("subject_type = $%d", len(args)))
	}
	if f.PageToken != "" {
		at, id, err := usecase.DecodeApprovalCursor(f.PageToken)
		if err != nil {
			return nil, "", err
		}
		args = append(args, at, id)
		where = append(where, fmt.Sprintf("(created_at, id) < ($%d, $%d::uuid)", len(args)-1, len(args)))
	}
	args = append(args, f.PageSize+1)
	var list []domain.ApprovalPolicy
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return nil
		}
		rows, err := db.Query(ctx, fmt.Sprintf(`SELECT `+policyColumns+` FROM request.approval_policies WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d`,
			strings.Join(where, " AND "), len(args)), args...)
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
		return nil, "", fmt.Errorf("postgres: list policies: %w", err)
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
			tag, err := db.Exec(ctx, `INSERT INTO request.approval_policies (id, tenant_id, project_id, subject_type, request_type, size, urgency,
					approvers, allow_requester_approve, due_after_seconds, priority, enabled, version, created_by)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12,1,$13) ON CONFLICT (id) DO NOTHING`,
				p.ID, tenantID, p.ProjectID, string(p.SubjectType), p.RequestType, p.Size, p.Urgency, string(approvers),
				p.AllowRequesterApprove, policyDueSeconds(p), p.Priority, p.Enabled, p.CreatedBy)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return domain.ErrApprovalVersionConflict
			}
		} else {
			tag, err := db.Exec(ctx, `UPDATE request.approval_policies SET project_id = $1, subject_type = $2, request_type = $3, size = $4,
					urgency = $5, approvers = $6::jsonb, allow_requester_approve = $7, due_after_seconds = $8, priority = $9, enabled = $10,
					version = version + 1, updated_at = now()
				WHERE tenant_id = $11 AND id = $12 AND version = $13`,
				p.ProjectID, string(p.SubjectType), p.RequestType, p.Size, p.Urgency, string(approvers), p.AllowRequesterApprove,
				policyDueSeconds(p), p.Priority, p.Enabled, tenantID, p.ID, expectedVersion)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				var exists bool
				if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM request.approval_policies WHERE tenant_id = $1 AND id = $2)`, tenantID, p.ID).Scan(&exists); err != nil {
					return err
				}
				if !exists {
					return domain.ErrApprovalPolicyNotFound
				}
				return domain.ErrApprovalVersionConflict
			}
		}
		got, err := scanPolicy(db.QueryRow(ctx, `SELECT `+policyColumns+` FROM request.approval_policies WHERE tenant_id = $1 AND id = $2`, tenantID, p.ID))
		out = got
		return err
	})
	if err != nil {
		return domain.ApprovalPolicy{}, wrapPolicyErr("upsert policy", err)
	}
	return out, nil
}

func (r *PolicyRepository) Delete(ctx context.Context, tenantID, id string, expectedVersion int64) error {
	if _, err := uuid.Parse(id); err != nil {
		return domain.ErrApprovalPolicyNotFound
	}
	err := r.scoped(ctx, func(ctx context.Context, ctxTenant string, db dbExecer) error {
		if ctxTenant != tenantID {
			return domain.ErrApprovalPolicyNotFound
		}
		tag, err := db.Exec(ctx, `DELETE FROM request.approval_policies WHERE tenant_id = $1 AND id = $2 AND ($3::bigint = 0 OR version = $3)`, tenantID, id, expectedVersion)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM request.approval_policies WHERE tenant_id = $1 AND id = $2)`, tenantID, id).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return domain.ErrApprovalPolicyNotFound
			}
			return domain.ErrApprovalVersionConflict
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
	return fmt.Errorf("postgres: %s: %w", op, err)
}

func (r *PolicyRepository) NowDB(ctx context.Context) (time.Time, error) {
	var now time.Time
	err := r.exec(ctx).QueryRow(ctx, "SELECT now()").Scan(&now)
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
			if _, err := db.Exec(ctx, `INSERT INTO request.approval_approvers (approval_id, tenant_id, principal_kind, principal_id)
				VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, approvalID, tenantID, string(p.Kind), p.ID); err != nil {
				return fmt.Errorf("postgres: insert approver snapshot: %w", err)
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
		rows, err := db.Query(ctx, `SELECT principal_kind, principal_id FROM request.approval_approvers
			WHERE tenant_id = $1 AND approval_id = $2 ORDER BY principal_kind, principal_id`, tenantID, approvalID)
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
		return nil, fmt.Errorf("postgres: list approvers: %w", err)
	}
	return list, nil
}
