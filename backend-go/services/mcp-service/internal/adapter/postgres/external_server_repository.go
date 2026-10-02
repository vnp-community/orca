package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// External server registry storage (BE-MCP-SOL-014). Nothing here ever reads
// or writes a secret value: refs carry only the broker owner pointer.

// maxStoredDescription caps tool descriptions kept for the UI diff; the digest
// is always computed over the full text before storage.
const maxStoredDescription = 4 << 10

const externalServerColumns = `id::text, tenant_id::text, scope, scope_id, name, transport, COALESCE(url, ''), COALESCE(command, ''), args,
	status, spec_digest, COALESCE(last_probe_digest, ''), last_probe_at, COALESCE(approved_digest, ''), approved_tools,
	health_ok, health_checked_at, COALESCE(health_error, ''), created_by, COALESCE(reviewed_by, ''), reviewed_at, version, created_at, updated_at`

func scanExternalServer(row pgx.Row) (domain.ExternalServer, error) {
	var s domain.ExternalServer
	var args, approved []byte
	var healthOK *bool
	var healthAt *time.Time
	var healthErr string
	err := row.Scan(&s.ID, &s.TenantID, &s.Scope, &s.ScopeID, &s.Name, &s.Transport, &s.URL, &s.Command, &args,
		&s.Status, &s.SpecDigest, &s.LastProbeDigest, &s.LastProbeAt, &s.ApprovedDigest, &approved,
		&healthOK, &healthAt, &healthErr, &s.CreatedBy, &s.ReviewedBy, &s.ReviewedAt, &s.Version, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(args, &s.Args); err != nil {
		return s, fmt.Errorf("postgres: decode args: %w", err)
	}
	if len(approved) > 0 {
		if err := json.Unmarshal(approved, &s.ApprovedTools); err != nil {
			return s, fmt.Errorf("postgres: decode approved tools: %w", err)
		}
	}
	if healthOK != nil && healthAt != nil {
		s.Health = &domain.Health{OK: *healthOK, CheckedAt: *healthAt, Error: healthErr}
	}
	return s, nil
}

func (r *Repository) loadRefsTx(ctx context.Context, tx pgx.Tx, tenantID string, servers []domain.ExternalServer) error {
	if len(servers) == 0 {
		return nil
	}
	ids := make([]string, 0, len(servers))
	idx := map[string]int{}
	for i, s := range servers {
		ids = append(ids, s.ID)
		idx[s.ID] = i
	}
	rows, err := tx.Query(ctx, `
		SELECT server_id::text, kind, name, COALESCE(broker_owner_id, ''), COALESCE(set_by, ''), set_at
		FROM mcp.external_server_secret_refs WHERE tenant_id = $1 AND server_id = ANY($2::uuid[]) ORDER BY name`, tenantID, ids)
	if err != nil {
		return fmt.Errorf("postgres: list secret refs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sid string
		var ref domain.SecretRef
		if err := rows.Scan(&sid, &ref.Kind, &ref.Name, &ref.BrokerOwnerID, &ref.SetBy, &ref.SetAt); err != nil {
			return fmt.Errorf("postgres: scan secret ref: %w", err)
		}
		i := idx[sid]
		if ref.Kind == domain.SecretKindHeader {
			servers[i].HeaderRefs = append(servers[i].HeaderRefs, ref)
		} else {
			servers[i].EnvRefs = append(servers[i].EnvRefs, ref)
		}
	}
	return rows.Err()
}

func (r *Repository) GetExternalServer(ctx context.Context, tenantID, id string) (domain.ExternalServer, error) {
	var out domain.ExternalServer
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		s, err := scanExternalServer(tx.QueryRow(ctx, `SELECT `+externalServerColumns+` FROM mcp.external_servers WHERE tenant_id = $1 AND id = $2`, tenantID, id))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound()
		}
		if err != nil {
			return fmt.Errorf("postgres: get external server: %w", err)
		}
		list := []domain.ExternalServer{s}
		if err := r.loadRefsTx(ctx, tx, tenantID, list); err != nil {
			return err
		}
		out = list[0]
		return nil
	})
	return out, err
}

func (r *Repository) queryServers(ctx context.Context, tenantID, where string, args ...any) ([]domain.ExternalServer, error) {
	out := []domain.ExternalServer{}
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+externalServerColumns+` FROM mcp.external_servers WHERE tenant_id = $1 `+where+` ORDER BY scope, name, id`,
			append([]any{tenantID}, args...)...)
		if err != nil {
			return fmt.Errorf("postgres: list external servers: %w", err)
		}
		for rows.Next() {
			s, err := scanExternalServer(rows)
			if err != nil {
				rows.Close()
				return fmt.Errorf("postgres: scan external server: %w", err)
			}
			out = append(out, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		return r.loadRefsTx(ctx, tx, tenantID, out)
	})
	return out, err
}

func (r *Repository) ListExternalServers(ctx context.Context, tenantID string, f usecase.ExternalServerFilter) ([]domain.ExternalServer, error) {
	switch {
	case f.OwnerUserID != "":
		if f.Scope != "" && f.Scope != domain.ScopeUser {
			return []domain.ExternalServer{}, nil
		}
		return r.queryServers(ctx, tenantID, `AND scope = 'user' AND scope_id = $2`, f.OwnerUserID)
	case f.Scope != "":
		return r.queryServers(ctx, tenantID, `AND scope = $2`, f.Scope)
	}
	return r.queryServers(ctx, tenantID, ``)
}

func (r *Repository) ListServersByName(ctx context.Context, tenantID string, names []string) ([]domain.ExternalServer, error) {
	return r.queryServers(ctx, tenantID, `AND name = ANY($2::text[])`, names)
}

func argsJSON(a []string) ([]byte, error) {
	if a == nil {
		a = []string{}
	}
	return json.Marshal(a)
}

func (r *Repository) CreateExternalServer(ctx context.Context, s domain.ExternalServer, events []domain.OutboxRecord) error {
	args, err := argsJSON(s.Args)
	if err != nil {
		return err
	}
	return r.withTenantTx(ctx, s.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO mcp.external_servers (id, tenant_id, scope, scope_id, name, transport, url, command, args, status, spec_digest,
				created_by, version, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12, 1, $13, $13)`,
			s.ID, s.TenantID, s.Scope, s.ScopeID, s.Name, s.Transport, s.URL, s.Command, args, s.Status, s.SpecDigest, s.CreatedBy, s.CreatedAt)
		if isUniqueViolation(err) {
			return domain.ErrNameConflict()
		}
		if err != nil {
			return fmt.Errorf("postgres: insert external server: %w", err)
		}
		if err := upsertRefsTx(ctx, tx, s); err != nil {
			return err
		}
		return insertEventsTx(ctx, tx, s.TenantID, events)
	})
}

func upsertRefsTx(ctx context.Context, tx pgx.Tx, s domain.ExternalServer) error {
	for _, ref := range append(append([]domain.SecretRef{}, s.EnvRefs...), s.HeaderRefs...) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO mcp.external_server_secret_refs (server_id, tenant_id, kind, name, broker_owner_id, set_by, set_at)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7)
			ON CONFLICT (server_id, kind, name) DO NOTHING`,
			s.ID, s.TenantID, ref.Kind, ref.Name, ref.BrokerOwnerID, ref.SetBy, ref.SetAt); err != nil {
			return fmt.Errorf("postgres: upsert secret ref: %w", err)
		}
	}
	return nil
}

func (r *Repository) UpdateExternalServer(ctx context.Context, s domain.ExternalServer, events []domain.OutboxRecord) error {
	args, err := argsJSON(s.Args)
	if err != nil {
		return err
	}
	return r.withTenantTx(ctx, s.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE mcp.external_servers SET name = $3, transport = $4, url = NULLIF($5, ''), command = NULLIF($6, ''), args = $7,
				status = $8, spec_digest = $9,
				last_probe_digest = NULLIF($10, ''),
				last_probe_at = CASE WHEN $10 = '' THEN NULL ELSE last_probe_at END,
				last_probe_tools = CASE WHEN $10 = '' THEN NULL ELSE last_probe_tools END,
				health_ok = CASE WHEN $10 = '' THEN NULL ELSE health_ok END,
				health_checked_at = CASE WHEN $10 = '' THEN NULL ELSE health_checked_at END,
				health_error = CASE WHEN $10 = '' THEN NULL ELSE health_error END,
				version = version + 1, updated_at = now()
			WHERE tenant_id = $1 AND id = $2 AND version = $11`,
			s.TenantID, s.ID, s.Name, s.Transport, s.URL, s.Command, args, s.Status, s.SpecDigest, s.LastProbeDigest, s.Version)
		if isUniqueViolation(err) {
			return domain.ErrNameConflict()
		}
		if err != nil {
			return fmt.Errorf("postgres: update external server: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrServerInvalid("server was modified concurrently; reload and retry")
		}
		keepKinds, keepNames := []string{}, []string{}
		for _, ref := range append(append([]domain.SecretRef{}, s.EnvRefs...), s.HeaderRefs...) {
			keepKinds, keepNames = append(keepKinds, ref.Kind), append(keepNames, ref.Name)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM mcp.external_server_secret_refs WHERE tenant_id = $1 AND server_id = $2
			AND (kind, name) NOT IN (SELECT * FROM unnest($3::text[], $4::text[]))`, s.TenantID, s.ID, keepKinds, keepNames); err != nil {
			return fmt.Errorf("postgres: prune secret refs: %w", err)
		}
		if err := upsertRefsTx(ctx, tx, s); err != nil {
			return err
		}
		return insertEventsTx(ctx, tx, s.TenantID, events)
	})
}

func (r *Repository) SetSecretRef(ctx context.Context, tenantID, serverID string, ref domain.SecretRef, events []domain.OutboxRecord) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE mcp.external_server_secret_refs SET broker_owner_id = $5, set_by = $6, set_at = $7
			WHERE tenant_id = $1 AND server_id = $2 AND kind = $3 AND name = $4`,
			tenantID, serverID, ref.Kind, ref.Name, ref.BrokerOwnerID, ref.SetBy, ref.SetAt)
		if err != nil {
			return fmt.Errorf("postgres: set secret ref: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound()
		}
		return insertEventsTx(ctx, tx, tenantID, events)
	})
}

func storableTools(tools []domain.ToolInfo) []domain.ToolInfo {
	out := make([]domain.ToolInfo, 0, len(tools))
	for _, t := range tools {
		if len(t.Description) > maxStoredDescription {
			t.Description = t.Description[:maxStoredDescription]
		}
		out = append(out, domain.ToolInfo{Name: t.Name, Description: t.Description})
	}
	return out
}

func (r *Repository) RecordProbe(ctx context.Context, tenantID, serverID string, p usecase.ProbeRecord, events []domain.OutboxRecord) error {
	tools, err := json.Marshal(storableTools(p.Tools))
	if err != nil {
		return err
	}
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE mcp.external_servers SET last_probe_digest = $3, last_probe_at = $4, last_probe_tools = $5, updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, tenantID, serverID, p.Digest, p.At, tools)
		if err != nil {
			return fmt.Errorf("postgres: record probe: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO mcp.external_server_tools_history (id, tenant_id, server_id, digest, tools, observed_at, source)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, uuid.NewString(), tenantID, serverID, p.Digest, tools, p.At, p.Source); err != nil {
			return fmt.Errorf("postgres: insert tools history: %w", err)
		}
		return insertEventsTx(ctx, tx, tenantID, events)
	})
}

func (r *Repository) ApplyReview(ctx context.Context, rv usecase.ReviewRecord, events []domain.OutboxRecord) (domain.ExternalServer, error) {
	var out domain.ExternalServer
	err := r.withTenantTx(ctx, rv.TenantID, func(tx pgx.Tx) error {
		var tag pgconn.CommandTag
		var err error
		if rv.Approve {
			// The digest check and the write are one statement: a probe that
			// lands in between cannot be approved by accident.
			tag, err = tx.Exec(ctx, `
				UPDATE mcp.external_servers SET status = 'approved', approved_digest = last_probe_digest,
					approved_tools = COALESCE(last_probe_tools, '[]'::jsonb), reviewed_by = $3, reviewed_at = $4, version = version + 1, updated_at = now()
				WHERE tenant_id = $1 AND id = $2 AND last_probe_digest IS NOT NULL AND last_probe_digest = $5`,
				rv.TenantID, rv.ServerID, rv.ReviewerID, rv.At, rv.ExpectedDigest)
		} else {
			tag, err = tx.Exec(ctx, `
				UPDATE mcp.external_servers SET status = 'disabled', reviewed_by = $3, reviewed_at = $4, version = version + 1, updated_at = now()
				WHERE tenant_id = $1 AND id = $2`, rv.TenantID, rv.ServerID, rv.ReviewerID, rv.At)
		}
		if err != nil {
			return fmt.Errorf("postgres: apply review: %w", err)
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM mcp.external_servers WHERE tenant_id = $1 AND id = $2)`, rv.TenantID, rv.ServerID).Scan(&exists); err != nil {
				return fmt.Errorf("postgres: check server: %w", err)
			}
			if !exists {
				return domain.ErrNotFound()
			}
			return domain.ErrDigestMismatch()
		}
		decision := "rejected"
		digestCond := `digest = (SELECT last_probe_digest FROM mcp.external_servers WHERE id = $2)`
		if rv.Approve {
			decision = "approved"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE mcp.external_server_tools_history SET decision = $3, decided_by = $4
			WHERE id = (SELECT id FROM mcp.external_server_tools_history WHERE tenant_id = $1 AND server_id = $2 AND `+digestCond+`
				ORDER BY observed_at DESC LIMIT 1)`, rv.TenantID, rv.ServerID, decision, rv.ReviewerID); err != nil {
			return fmt.Errorf("postgres: mark history decision: %w", err)
		}
		s, err := scanExternalServer(tx.QueryRow(ctx, `SELECT `+externalServerColumns+` FROM mcp.external_servers WHERE tenant_id = $1 AND id = $2`, rv.TenantID, rv.ServerID))
		if err != nil {
			return fmt.Errorf("postgres: reload external server: %w", err)
		}
		list := []domain.ExternalServer{s}
		if err := r.loadRefsTx(ctx, tx, rv.TenantID, list); err != nil {
			return err
		}
		out = list[0]
		return insertEventsTx(ctx, tx, rv.TenantID, events)
	})
	return out, err
}

func (r *Repository) RecordHealth(ctx context.Context, tenantID, serverID string, h domain.Health, events []domain.OutboxRecord) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE mcp.external_servers SET health_ok = $3, health_checked_at = $4, health_error = NULLIF($5, ''), updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, tenantID, serverID, h.OK, h.CheckedAt, h.Error); err != nil {
			return fmt.Errorf("postgres: record health: %w", err)
		}
		return insertEventsTx(ctx, tx, tenantID, events)
	})
}

func (r *Repository) DeleteExternalServer(ctx context.Context, tenantID, id string, events []domain.OutboxRecord) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM mcp.external_servers WHERE tenant_id = $1 AND id = $2`, tenantID, id)
		if err != nil {
			return fmt.Errorf("postgres: delete external server: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound()
		}
		return insertEventsTx(ctx, tx, tenantID, events)
	})
}

func (r *Repository) ClaimHealthChecks(ctx context.Context, olderThan time.Time, limit int) ([]usecase.ServerKey, error) {
	var out []usecase.ServerKey
	err := r.withRelayTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE mcp.external_servers SET health_claimed_at = now()
			WHERE id IN (
				SELECT id FROM mcp.external_servers
				WHERE status = 'approved' AND transport = 'http' AND (health_claimed_at IS NULL OR health_claimed_at < $1)
				ORDER BY health_claimed_at NULLS FIRST LIMIT $2 FOR UPDATE SKIP LOCKED)
			RETURNING tenant_id::text, id::text`, olderThan, limit)
		if err != nil {
			return fmt.Errorf("postgres: claim health checks: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var k usecase.ServerKey
			if err := rows.Scan(&k.TenantID, &k.ServerID); err != nil {
				return err
			}
			out = append(out, k)
		}
		return rows.Err()
	})
	return out, err
}
