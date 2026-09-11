package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// StarNagStateRepository implements usecase.StarNagStateRepository against
// star_nag_state — 1:1 with a user, same isolation rule as
// UserProfileRepository (a row from another company resolves as not-found,
// tenant-service.md §9).
type StarNagStateRepository struct {
	db *sql.DB
}

func NewStarNagStateRepository(db *sql.DB) *StarNagStateRepository {
	return &StarNagStateRepository{db: db}
}

func (r *StarNagStateRepository) GetOrCreate(ctx context.Context, companyID, userID string) (domain.StarNagState, error) {
	state, found, err := r.get(ctx, companyID, userID)
	if err != nil {
		return domain.StarNagState{}, err
	}
	if found {
		return state, nil
	}
	def := domain.NewDefaultStarNagState(userID, companyID)
	if err := r.Save(ctx, def); err != nil {
		return domain.StarNagState{}, err
	}
	return def, nil
}

func (r *StarNagStateRepository) get(ctx context.Context, companyID, userID string) (domain.StarNagState, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, company_id, baseline_agents, app_version, next_threshold,
		       completed, deferred_until, agent_value_moment_app_version,
		       active_prompt, updated_at
		FROM star_nag_state
		WHERE user_id = ? AND company_id = ?
	`, userID, companyID)

	var s domain.StarNagState
	var activePromptJSON []byte
	if err := row.Scan(&s.UserID, &s.CompanyID, &s.BaselineAgents, &s.AppVersion, &s.NextThreshold,
		&s.Completed, &s.DeferredUntil, &s.AgentValueMomentAppVersion, &activePromptJSON, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.StarNagState{}, false, nil
		}
		return domain.StarNagState{}, false, fmt.Errorf("mysql: query star nag state: %w", err)
	}
	if len(activePromptJSON) > 0 {
		var p domain.ActiveStarNagPrompt
		if err := json.Unmarshal(activePromptJSON, &p); err != nil {
			return domain.StarNagState{}, false, fmt.Errorf("mysql: unmarshal active_prompt: %w", err)
		}
		s.ActivePrompt = &p
	}
	return s, true, nil
}

// Save upserts every mutable column, keyed on user_id (the PRIMARY KEY) —
// ON DUPLICATE KEY UPDATE is MySQL's equivalent of Postgres's
// ON CONFLICT (user_id) DO UPDATE.
func (r *StarNagStateRepository) Save(ctx context.Context, s domain.StarNagState) error {
	var activePromptJSON []byte
	if s.ActivePrompt != nil {
		b, err := json.Marshal(s.ActivePrompt)
		if err != nil {
			return fmt.Errorf("mysql: marshal active_prompt: %w", err)
		}
		activePromptJSON = b
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO star_nag_state
			(user_id, company_id, baseline_agents, app_version, next_threshold,
			 completed, deferred_until, agent_value_moment_app_version, active_prompt, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(6))
		ON DUPLICATE KEY UPDATE
			company_id                     = VALUES(company_id),
			baseline_agents                = VALUES(baseline_agents),
			app_version                    = VALUES(app_version),
			next_threshold                 = VALUES(next_threshold),
			completed                      = VALUES(completed),
			deferred_until                 = VALUES(deferred_until),
			agent_value_moment_app_version = VALUES(agent_value_moment_app_version),
			active_prompt                  = VALUES(active_prompt),
			updated_at                     = NOW(6)
	`, s.UserID, s.CompanyID, s.BaselineAgents, s.AppVersion, s.NextThreshold,
		s.Completed, s.DeferredUntil, s.AgentValueMomentAppVersion, activePromptJSON)
	if err != nil {
		return fmt.Errorf("mysql: upsert star nag state: %w", err)
	}
	return nil
}
