package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// Consent requests and grants. Every method runs in withTenantTx (RLS) and
// also binds tenant_id in SQL, so a missing set_config can't widen a query.
// State changes that publish an event write the outbox row in the same
// transaction.

const consentColumns = `id::text, tenant_id::text, user_id::text, client_id, client_name, COALESCE(client_uri, ''), redirect_uri,
	scopes, COALESCE(state, ''), code_challenge, resource, is_new_client, registered_via_dcr, created_at, expires_at,
	decided_at, COALESCE(decision, '')`

func scanConsent(row pgx.Row) (domain.ConsentRequest, error) {
	var c domain.ConsentRequest
	err := row.Scan(&c.ID, &c.TenantID, &c.UserID, &c.ClientID, &c.ClientName, &c.ClientURI, &c.RedirectURI,
		&c.Scopes, &c.State, &c.CodeChallenge, &c.Resource, &c.IsNewClient, &c.RegisteredViaDCR, &c.CreatedAt, &c.ExpiresAt,
		&c.DecidedAt, &c.Decision)
	return c, err
}

const grantColumns = `id::text, tenant_id::text, user_id::text, client_id, client_name, COALESCE(client_uri, ''), scopes, status,
	created_at, updated_at, last_used_at, revoked_at, COALESCE(revoked_by::text, ''), revocation_propagated_at`

func scanGrant(row pgx.Row) (domain.Grant, error) {
	var g domain.Grant
	err := row.Scan(&g.ID, &g.TenantID, &g.UserID, &g.ClientID, &g.ClientName, &g.ClientURI, &g.Scopes, &g.Status,
		&g.CreatedAt, &g.UpdatedAt, &g.LastUsedAt, &g.RevokedAt, &g.RevokedBy, &g.RevocationPropagatedAt)
	return g, err
}

// insertOutboxTx enqueues an event inside the caller's transaction.
func insertOutboxTx(ctx context.Context, tx pgx.Tx, tenantID string, rec domain.OutboxRecord) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO mcp.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, rec.ID, tenantID, rec.Subject, rec.OccurredAt, rec.Version, rec.PayloadJSON); err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}
	return nil
}

func (r *Repository) CreateConsentRequest(ctx context.Context, c domain.ConsentRequest) error {
	return r.withTenantTx(ctx, c.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO mcp.consent_requests (id, tenant_id, user_id, client_id, client_name, client_uri, redirect_uri, scopes, state,
				code_challenge, resource, is_new_client, registered_via_dcr, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, NULLIF($9, ''), $10, $11, $12, $13, $14, $15)
		`, c.ID, c.TenantID, c.UserID, c.ClientID, c.ClientName, c.ClientURI, c.RedirectURI, c.Scopes, c.State,
			c.CodeChallenge, c.Resource, c.IsNewClient, c.RegisteredViaDCR, c.CreatedAt, c.ExpiresAt)
		if err != nil {
			return fmt.Errorf("postgres: insert consent request: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetConsentRequest(ctx context.Context, tenantID, userID, requestID string) (domain.ConsentRequest, error) {
	var out domain.ConsentRequest
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		c, err := scanConsent(tx.QueryRow(ctx,
			`SELECT `+consentColumns+` FROM mcp.consent_requests WHERE id = $1 AND tenant_id = $2 AND user_id = $3`, requestID, tenantID, userID))
		if errors.Is(err, pgx.ErrNoRows) {
			return usecase.ErrConsentRequestNotFound
		}
		if err != nil {
			return fmt.Errorf("postgres: get consent request: %w", err)
		}
		out = c
		return nil
	})
	return out, err
}

// decideConsentTx is the one statement that decides a consent request: it only
// matches a pending, unexpired request owned by the caller, so concurrent
// decisions race on a single row and exactly one wins.
func decideConsentTx(ctx context.Context, tx pgx.Tx, tenantID, userID, requestID, decision string, now time.Time) (domain.ConsentRequest, error) {
	c, err := scanConsent(tx.QueryRow(ctx, `
		UPDATE mcp.consent_requests SET decided_at = $4, decision = $5
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND decided_at IS NULL AND expires_at > $4
		RETURNING `+consentColumns, requestID, tenantID, userID, now, decision))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ConsentRequest{}, usecase.ErrConsentNotDecidable
	}
	if err != nil {
		return domain.ConsentRequest{}, fmt.Errorf("postgres: decide consent request: %w", err)
	}
	return c, nil
}

func (r *Repository) DenyConsent(ctx context.Context, tenantID, userID, requestID string, now time.Time) (domain.ConsentRequest, error) {
	var out domain.ConsentRequest
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		c, err := decideConsentTx(ctx, tx, tenantID, userID, requestID, domain.DecisionDeny, now)
		out = c
		return err
	})
	return out, err
}

func (r *Repository) ApproveConsent(ctx context.Context, in usecase.ApproveConsentInput) (usecase.ApproveConsentResult, error) {
	var res usecase.ApproveConsentResult
	err := r.withTenantTx(ctx, in.TenantID, func(tx pgx.Tx) error {
		req, err := decideConsentTx(ctx, tx, in.TenantID, in.UserID, in.RequestID, domain.DecisionApprove, in.Now)
		if err != nil {
			return err
		}
		// The partial unique index uq_grants_active makes the upsert race-free;
		// xmax = 0 tells an insert from an update of the existing active grant.
		var created bool
		g, err := scanGrantWithInserted(tx.QueryRow(ctx, `
			INSERT INTO mcp.grants (id, tenant_id, user_id, client_id, client_name, client_uri, scopes, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, 'active', $8, $8)
			ON CONFLICT (tenant_id, user_id, client_id) WHERE status = 'active'
			DO UPDATE SET scopes = EXCLUDED.scopes, client_name = EXCLUDED.client_name, client_uri = EXCLUDED.client_uri,
				updated_at = EXCLUDED.updated_at
			RETURNING `+grantColumns+`, (xmax = 0)
		`, in.GrantID, in.TenantID, in.UserID, req.ClientID, req.ClientName, req.ClientURI, in.Scopes, in.Now), &created)
		if err != nil {
			return fmt.Errorf("postgres: upsert grant: %w", err)
		}
		subject := domain.SubjectGrantUpdated
		if created {
			subject = domain.SubjectGrantCreated
		}
		ev, err := domain.NewOutboxEvent(in.EventID, subject, in.TenantID, in.Now, map[string]any{
			"grant_id": g.ID, "user_id": g.UserID, "client_id": g.ClientID, "scopes": g.Scopes,
		})
		if err != nil {
			return fmt.Errorf("postgres: build grant event: %w", err)
		}
		if err := insertOutboxTx(ctx, tx, in.TenantID, ev); err != nil {
			return err
		}
		res = usecase.ApproveConsentResult{Request: req, Grant: g, Created: created}
		return nil
	})
	return res, err
}

func scanGrantWithInserted(row pgx.Row, inserted *bool) (domain.Grant, error) {
	var g domain.Grant
	err := row.Scan(&g.ID, &g.TenantID, &g.UserID, &g.ClientID, &g.ClientName, &g.ClientURI, &g.Scopes, &g.Status,
		&g.CreatedAt, &g.UpdatedAt, &g.LastUsedAt, &g.RevokedAt, &g.RevokedBy, &g.RevocationPropagatedAt, inserted)
	return g, err
}

func (r *Repository) GetActiveGrant(ctx context.Context, tenantID, userID, clientID string) (domain.Grant, error) {
	var out domain.Grant
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		g, err := scanGrant(tx.QueryRow(ctx, `
			SELECT `+grantColumns+` FROM mcp.grants
			WHERE tenant_id = $1 AND user_id = $2 AND client_id = $3 AND status = 'active'
		`, tenantID, userID, clientID))
		if errors.Is(err, pgx.ErrNoRows) {
			return usecase.ErrGrantNotFound
		}
		if err != nil {
			return fmt.Errorf("postgres: get active grant: %w", err)
		}
		out = g
		return nil
	})
	return out, err
}

func (r *Repository) CountGrantsForClient(ctx context.Context, tenantID, clientID string) (int, error) {
	var n int
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM mcp.grants WHERE tenant_id = $1 AND client_id = $2`, tenantID, clientID).Scan(&n)
	})
	return n, err
}

func (r *Repository) ListActiveGrants(ctx context.Context, tenantID, userID string) ([]domain.Grant, error) {
	var out []domain.Grant
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+grantColumns+` FROM mcp.grants
			WHERE tenant_id = $1 AND status = 'active' AND ($2 = '' OR user_id = NULLIF($2, '')::uuid)
			ORDER BY created_at DESC, id
		`, tenantID, userID)
		if err != nil {
			return fmt.Errorf("postgres: list grants: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			g, err := scanGrant(rows)
			if err != nil {
				return fmt.Errorf("postgres: scan grant: %w", err)
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) CountActiveGrantsByClient(ctx context.Context, tenantID string) (map[string]int, error) {
	out := map[string]int{}
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT client_id, count(*) FROM mcp.grants WHERE tenant_id = $1 AND status = 'active' GROUP BY client_id`, tenantID)
		if err != nil {
			return fmt.Errorf("postgres: count grants by client: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var n int
			if err := rows.Scan(&id, &n); err != nil {
				return err
			}
			out[id] = n
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) RevokeGrant(ctx context.Context, tenantID, grantID, ownerUserID, revokedBy string, now time.Time, eventID string) (domain.Grant, bool, error) {
	var out domain.Grant
	newly := false
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		g, err := scanGrant(tx.QueryRow(ctx, `
			SELECT `+grantColumns+` FROM mcp.grants
			WHERE id = $1 AND tenant_id = $2 AND ($3 = '' OR user_id = NULLIF($3, '')::uuid)
			FOR UPDATE
		`, grantID, tenantID, ownerUserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return usecase.ErrGrantNotFound
		}
		if err != nil {
			return fmt.Errorf("postgres: load grant for revoke: %w", err)
		}
		out = g
		if g.Status == domain.GrantRevoked {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE mcp.grants SET status = 'revoked', revoked_at = $2, revoked_by = NULLIF($3, '')::uuid, updated_at = $2
			WHERE id = $1
		`, grantID, now, revokedBy); err != nil {
			return fmt.Errorf("postgres: revoke grant: %w", err)
		}
		out.Status, out.RevokedAt, out.RevokedBy, out.UpdatedAt = domain.GrantRevoked, &now, revokedBy, now
		ev, err := domain.NewOutboxEvent(eventID, domain.SubjectGrantRevoked, tenantID, now, map[string]any{
			"grant_id": g.ID, "user_id": g.UserID, "client_id": g.ClientID, "by": revokedBy,
		})
		if err != nil {
			return fmt.Errorf("postgres: build revoke event: %w", err)
		}
		newly = true
		return insertOutboxTx(ctx, tx, tenantID, ev)
	})
	return out, newly, err
}

func (r *Repository) MarkRevocationPropagated(ctx context.Context, tenantID, grantID string, now time.Time) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE mcp.grants SET revocation_propagated_at = $3
			WHERE id = $1 AND tenant_id = $2 AND status = 'revoked' AND revocation_propagated_at IS NULL
		`, grantID, tenantID, now)
		if err != nil {
			return fmt.Errorf("postgres: mark revocation propagated: %w", err)
		}
		return nil
	})
}

// ListUnpropagatedRevocations is cross-tenant by design (reconcile job); it
// opts in via app.relay, whose policy only exposes still-pending revocations.
func (r *Repository) ListUnpropagatedRevocations(ctx context.Context, limit int) ([]domain.Grant, error) {
	var out []domain.Grant
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT `+grantColumns+` FROM mcp.grants
			WHERE status = 'revoked' AND revocation_propagated_at IS NULL
			ORDER BY revoked_at LIMIT $1
		`, limit)
		if err != nil {
			return fmt.Errorf("postgres: list unpropagated revocations: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			g, err := scanGrant(rows)
			if err != nil {
				return fmt.Errorf("postgres: scan grant: %w", err)
			}
			out = append(out, g)
		}
		return rows.Err()
	})
	return out, err
}

var _ usecase.AuthorizationRepository = (*Repository)(nil)
