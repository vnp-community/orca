package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// UserWorkspaceSessionRepository implements usecase.WorkspaceSessionRepository
// against user_workspace_sessions — CR-STORAGE-004a's per-(user_id, host_id)
// session state, mirrors internal/adapter/postgres.UserWorkspaceSessionRepository
// 1:1.
type UserWorkspaceSessionRepository struct {
	db *sql.DB
}

func NewUserWorkspaceSessionRepository(db *sql.DB) *UserWorkspaceSessionRepository {
	return &UserWorkspaceSessionRepository{db: db}
}

// Get reads userID's session for hostID, scoped by companyID — a row from a
// different company resolves as not-found, same isolation rule as
// UserProfileRepository (tenant-service.md §9). found=false means "no
// session ever saved for this (user, host) pair", never an error.
func (r *UserWorkspaceSessionRepository) Get(ctx context.Context, companyID, userID, hostID string) (string, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT session_json FROM user_workspace_sessions
		WHERE user_id = ? AND host_id = ? AND company_id = ?
	`, userID, hostID, companyID)

	var sessionJSON string
	if err := row.Scan(&sessionJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("mysql: query workspace session: %w", err)
	}
	return sessionJSON, true, nil
}

// Set fully replaces (or creates) userID's session row for hostID.
// ON DUPLICATE KEY UPDATE is MySQL's equivalent of Postgres's
// ON CONFLICT (user_id, host_id) DO UPDATE (the table's composite PRIMARY
// KEY is exactly that pair).
func (r *UserWorkspaceSessionRepository) Set(ctx context.Context, companyID, userID, hostID, sessionJSON string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_workspace_sessions (user_id, company_id, host_id, session_json, updated_at)
		VALUES (?, ?, ?, ?, NOW(6))
		ON DUPLICATE KEY UPDATE
			company_id   = VALUES(company_id),
			session_json = VALUES(session_json),
			updated_at   = NOW(6)
	`, userID, companyID, hostID, sessionJSON)
	if err != nil {
		return fmt.Errorf("mysql: set workspace session: %w", err)
	}
	return nil
}

// Patch shallow-merges patchJSON's top-level fields into the existing
// session row (creating one if none exists), inside a single transaction
// using SELECT ... FOR UPDATE to lock the row for the read-modify-write —
// InnoDB supports the same FOR UPDATE row-locking syntax as Postgres within
// a transaction, so this translates 1:1 from
// internal/adapter/postgres.UserWorkspaceSessionRepository.Patch. See
// BE-SOL-STORAGE-001 §4.
func (r *UserWorkspaceSessionRepository) Patch(ctx context.Context, companyID, userID, hostID, patchJSON string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin patch workspace session tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op once Commit succeeds

	row := tx.QueryRowContext(ctx, `
		SELECT session_json FROM user_workspace_sessions
		WHERE user_id = ? AND host_id = ? AND company_id = ?
		FOR UPDATE
	`, userID, hostID, companyID)

	var currentJSON string
	if err := row.Scan(&currentJSON); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("mysql: query workspace session for update: %w", err)
	}

	merged, err := mergeShallowJSON(currentJSON, patchJSON)
	if err != nil {
		return fmt.Errorf("mysql: merge workspace session patch: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_workspace_sessions (user_id, company_id, host_id, session_json, updated_at)
		VALUES (?, ?, ?, ?, NOW(6))
		ON DUPLICATE KEY UPDATE
			company_id   = VALUES(company_id),
			session_json = VALUES(session_json),
			updated_at   = NOW(6)
	`, userID, companyID, hostID, merged); err != nil {
		return fmt.Errorf("mysql: upsert patched workspace session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit patch workspace session tx: %w", err)
	}
	return nil
}

// mergeShallowJSON mirrors internal/adapter/postgres's function of the same
// name exactly — patch fields taking precedence over currentJSON's, a
// shallow one-level merge only, per PatchWorkspaceSessionRequest's proto doc
// comment.
func mergeShallowJSON(currentJSON, patchJSON string) (string, error) {
	base := map[string]json.RawMessage{}
	if currentJSON != "" {
		if err := json.Unmarshal([]byte(currentJSON), &base); err != nil {
			return "", fmt.Errorf("unmarshal existing session_json: %w", err)
		}
	}

	var patch map[string]json.RawMessage
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		return "", fmt.Errorf("unmarshal patch_json: %w", err)
	}
	for k, v := range patch {
		base[k] = v
	}

	merged, err := json.Marshal(base)
	if err != nil {
		return "", fmt.Errorf("marshal merged session_json: %w", err)
	}
	return string(merged), nil
}
