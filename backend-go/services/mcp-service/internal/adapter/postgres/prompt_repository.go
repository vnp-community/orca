package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// Custom prompts (BE-MCP-SOL-011). Reads and writes run under withTenantTx so
// RLS applies; the explicit tenant_id predicates are belt and braces.

const promptColumns = `id::text, name, description, arguments, template, version, created_by::text, updated_by::text, updated_at`

func scanPrompt(row pgx.Row) (domain.CustomPrompt, error) {
	var p domain.CustomPrompt
	var args []byte
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &args, &p.Template, &p.Version, &p.CreatedBy, &p.UpdatedBy, &p.UpdatedAt); err != nil {
		return p, err
	}
	if err := json.Unmarshal(args, &p.Arguments); err != nil {
		return p, fmt.Errorf("postgres: decode prompt arguments: %w", err)
	}
	if p.Arguments == nil {
		p.Arguments = []domain.PromptArgument{}
	}
	return p, nil
}

func (r *Repository) ListPrompts(ctx context.Context, tenantID string) ([]domain.CustomPrompt, error) {
	out := []domain.CustomPrompt{}
	err := r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+promptColumns+` FROM mcp.custom_prompts
			WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY name`, tenantID)
		if err != nil {
			return fmt.Errorf("postgres: list prompts: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			p, err := scanPrompt(rows)
			if err != nil {
				return fmt.Errorf("postgres: scan prompt: %w", err)
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) CreatePrompt(ctx context.Context, tenantID string, p domain.CustomPrompt, events []domain.OutboxRecord) (domain.CustomPrompt, error) {
	args, err := json.Marshal(p.Arguments)
	if err != nil {
		return domain.CustomPrompt{}, err
	}
	var out domain.CustomPrompt
	err = r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM mcp.custom_prompts WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID).Scan(&n); err != nil {
			return fmt.Errorf("postgres: count prompts: %w", err)
		}
		if n >= domain.MaxPromptsPerTenant {
			return domain.ErrPromptInvalid("prompts", fmt.Sprintf("at most %d custom prompts per tenant", domain.MaxPromptsPerTenant))
		}
		got, err := scanPrompt(tx.QueryRow(ctx, `
			INSERT INTO mcp.custom_prompts (id, tenant_id, name, description, arguments, template, version, created_by, updated_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $7, $8, $8)
			RETURNING `+promptColumns, p.ID, tenantID, p.Name, p.Description, args, p.Template, p.UpdatedBy, p.UpdatedAt))
		if isUniqueViolation(err) {
			return domain.ErrPromptNameConflict(p.Name)
		}
		if err != nil {
			return fmt.Errorf("postgres: insert prompt: %w", err)
		}
		out = got
		return insertEventsTx(ctx, tx, tenantID, events)
	})
	return out, err
}

func (r *Repository) UpdatePrompt(ctx context.Context, tenantID string, p domain.CustomPrompt, events []domain.OutboxRecord) (domain.CustomPrompt, error) {
	args, err := json.Marshal(p.Arguments)
	if err != nil {
		return domain.CustomPrompt{}, err
	}
	var out domain.CustomPrompt
	err = r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		got, err := scanPrompt(tx.QueryRow(ctx, `
			UPDATE mcp.custom_prompts SET version = version + 1, name = $4, description = $5, arguments = $6, template = $7,
				updated_by = $8, updated_at = $9
			WHERE id = $1 AND tenant_id = $2 AND version = $3 AND deleted_at IS NULL
			RETURNING `+promptColumns, p.ID, tenantID, p.Version, p.Name, p.Description, args, p.Template, p.UpdatedBy, p.UpdatedAt))
		if errors.Is(err, pgx.ErrNoRows) {
			var current int
			qerr := tx.QueryRow(ctx, `SELECT version FROM mcp.custom_prompts WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, p.ID, tenantID).Scan(&current)
			if errors.Is(qerr, pgx.ErrNoRows) {
				return domain.ErrNotFound()
			}
			if qerr != nil {
				return fmt.Errorf("postgres: read prompt version: %w", qerr)
			}
			return domain.ErrPromptVersionConflict(current)
		}
		if isUniqueViolation(err) {
			return domain.ErrPromptNameConflict(p.Name)
		}
		if err != nil {
			return fmt.Errorf("postgres: update prompt: %w", err)
		}
		out = got
		return insertEventsTx(ctx, tx, tenantID, events)
	})
	return out, err
}

func (r *Repository) DeletePrompt(ctx context.Context, tenantID, id string, events []domain.OutboxRecord) error {
	return r.withTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE mcp.custom_prompts SET deleted_at = now(), version = version + 1
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID)
		if err != nil {
			return fmt.Errorf("postgres: delete prompt: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound()
		}
		return insertEventsTx(ctx, tx, tenantID, events)
	})
}
