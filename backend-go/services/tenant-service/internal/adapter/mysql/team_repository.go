package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// TeamRepository implements usecase.TeamRepository against teams and
// team_members, always scoped by company_id — same not-found-not-
// wrong-company rule as DepartmentRepository (tenant-service.md §9).
type TeamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) Create(ctx context.Context, t domain.Team) (domain.Team, error) {
	settingsJSON, err := marshalSettings(t.Settings)
	if err != nil {
		return domain.Team{}, fmt.Errorf("mysql: marshal team settings: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO teams (id, company_id, name, settings_json) VALUES (?, ?, ?, ?)
	`, t.ID, t.CompanyID, t.Name, settingsJSON)
	if err != nil {
		return domain.Team{}, fmt.Errorf("mysql: insert team: %w", err)
	}
	return t, nil
}

func (r *TeamRepository) Get(ctx context.Context, companyID, id string) (domain.Team, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, name, settings_json
		FROM teams
		WHERE company_id = ? AND id = ?
	`, companyID, id)

	var team domain.Team
	var settingsJSON string
	if err := row.Scan(&team.ID, &team.CompanyID, &team.Name, &settingsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Team{}, false, nil
		}
		return domain.Team{}, false, fmt.Errorf("mysql: query team: %w", err)
	}

	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Team{}, false, fmt.Errorf("mysql: unmarshal team settings: %w", err)
	}
	team.Settings = settings
	return team, true, nil
}

// ListByCompany backs usecase.ListTeams — every teams row scoped to
// companyID.
func (r *TeamRepository) ListByCompany(ctx context.Context, companyID string) ([]domain.Team, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, name, settings_json FROM teams WHERE company_id = ?
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query teams: %w", err)
	}
	defer rows.Close()

	var out []domain.Team
	for rows.Next() {
		var t domain.Team
		var settingsJSON string
		if err := rows.Scan(&t.ID, &t.CompanyID, &t.Name, &settingsJSON); err != nil {
			return nil, fmt.Errorf("mysql: scan team row: %w", err)
		}
		settings, err := unmarshalSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("mysql: unmarshal team settings: %w", err)
		}
		t.Settings = settings
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate team rows: %w", err)
	}
	return out, nil
}

// AddMember upserts a team_members row — AddTeamMemberRequest is documented
// as an upsert (role + priority) in tenant-service.md §3. ON DUPLICATE KEY
// UPDATE is MySQL's equivalent of Postgres's ON CONFLICT (team_id, user_id)
// DO UPDATE (team_members' PRIMARY KEY is exactly that pair).
func (r *TeamRepository) AddMember(ctx context.Context, m domain.TeamMember) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, priority) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE priority = VALUES(priority)
	`, m.TeamID, m.UserID, m.Priority)
	if err != nil {
		return fmt.Errorf("mysql: upsert team member: %w", err)
	}
	return nil
}

// RemoveMember deletes one team_members row — backs usecase.RemoveTeamMember.
// Unlike the UPDATE-based not-found translation elsewhere in this package,
// DELETE's RowsAffected() counts WHERE-matched rows on every dialect (no
// "changed value" ambiguity — that ambiguity is specific to UPDATE), so
// reading it directly here is safe and matches the Postgres adapter 1:1 —
// see BE-DB-SOL-005/TASK-BE-DB-010's finding on this exact distinction.
func (r *TeamRepository) RemoveMember(ctx context.Context, teamID, userID string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM team_members WHERE team_id = ? AND user_id = ?
	`, teamID, userID)
	if err != nil {
		return false, fmt.Errorf("mysql: delete team member: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mysql: delete team member rows affected: %w", err)
	}
	return affected > 0, nil
}

func (r *TeamRepository) ListMembers(ctx context.Context, teamID string) ([]domain.TeamMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT team_id, user_id, priority FROM team_members WHERE team_id = ?
	`, teamID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query team members: %w", err)
	}
	defer rows.Close()

	var out []domain.TeamMember
	for rows.Next() {
		var m domain.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Priority); err != nil {
			return nil, fmt.Errorf("mysql: scan team member row: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate team member rows: %w", err)
	}
	return out, nil
}

// ListUserTeamLayers returns, for one user within one company, every team
// they belong to with that team's Settings and the membership's Priority —
// scoped by company_id via the join to teams, not filtered after the fact.
func (r *TeamRepository) ListUserTeamLayers(ctx context.Context, companyID, userID string) ([]domain.TeamSettingsLayer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.settings_json, tm.priority
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id
		WHERE tm.user_id = ? AND t.company_id = ?
	`, userID, companyID)
	if err != nil {
		return nil, fmt.Errorf("mysql: query user team layers: %w", err)
	}
	defer rows.Close()

	var out []domain.TeamSettingsLayer
	for rows.Next() {
		var teamID, settingsJSON string
		var priority int32
		if err := rows.Scan(&teamID, &settingsJSON, &priority); err != nil {
			return nil, fmt.Errorf("mysql: scan user team layer row: %w", err)
		}
		settings, err := unmarshalSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("mysql: unmarshal team settings: %w", err)
		}
		out = append(out, domain.TeamSettingsLayer{TeamID: teamID, Priority: priority, Settings: settings})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate user team layer rows: %w", err)
	}
	return out, nil
}
