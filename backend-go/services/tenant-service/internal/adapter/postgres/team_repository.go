package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// TeamRepository implements usecase.TeamRepository against tenant.teams and
// tenant.team_members, always scoped by company_id — same not-found-not-
// wrong-company rule as DepartmentRepository (tenant-service.md §9).
type TeamRepository struct {
	pool *pgxpool.Pool
}

func NewTeamRepository(pool *pgxpool.Pool) *TeamRepository {
	return &TeamRepository{pool: pool}
}

func (r *TeamRepository) Create(ctx context.Context, t domain.Team) (domain.Team, error) {
	settingsJSON, err := marshalSettings(t.Settings)
	if err != nil {
		return domain.Team{}, fmt.Errorf("postgres: marshal team settings: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO tenant.teams (id, company_id, name, settings_json) VALUES ($1, $2, $3, $4)
	`, t.ID, t.CompanyID, t.Name, settingsJSON)
	if err != nil {
		return domain.Team{}, fmt.Errorf("postgres: insert team: %w", err)
	}
	return t, nil
}

func (r *TeamRepository) Get(ctx context.Context, companyID, id string) (domain.Team, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, company_id, name, settings_json
		FROM tenant.teams
		WHERE company_id = $1 AND id = $2
	`, companyID, id)

	var team domain.Team
	var settingsJSON string
	if err := row.Scan(&team.ID, &team.CompanyID, &team.Name, &settingsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Team{}, false, nil
		}
		return domain.Team{}, false, fmt.Errorf("postgres: query team: %w", err)
	}

	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Team{}, false, fmt.Errorf("postgres: unmarshal team settings: %w", err)
	}
	team.Settings = settings
	return team, true, nil
}

// AddMember upserts a tenant.team_members row — AddTeamMemberRequest is
// documented as an upsert (role + priority) in tenant-service.md §3.
func (r *TeamRepository) AddMember(ctx context.Context, m domain.TeamMember) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tenant.team_members (team_id, user_id, priority) VALUES ($1, $2, $3)
		ON CONFLICT (team_id, user_id) DO UPDATE SET priority = EXCLUDED.priority
	`, m.TeamID, m.UserID, m.Priority)
	if err != nil {
		return fmt.Errorf("postgres: upsert team member: %w", err)
	}
	return nil
}

func (r *TeamRepository) ListMembers(ctx context.Context, teamID string) ([]domain.TeamMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT team_id, user_id, priority FROM tenant.team_members WHERE team_id = $1
	`, teamID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query team members: %w", err)
	}
	defer rows.Close()

	var out []domain.TeamMember
	for rows.Next() {
		var m domain.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Priority); err != nil {
			return nil, fmt.Errorf("postgres: scan team member row: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate team member rows: %w", err)
	}
	return out, nil
}

// ListUserTeamLayers returns, for one user within one company, every team
// they belong to with that team's Settings and the membership's Priority —
// exactly the pre-fetched input domain.ResolveProfile's team layer needs
// (tenant-service.md §4/§6). Scoped by company_id via the join to
// tenant.teams, not filtered after the fact.
func (r *TeamRepository) ListUserTeamLayers(ctx context.Context, companyID, userID string) ([]domain.TeamSettingsLayer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.settings_json, tm.priority
		FROM tenant.team_members tm
		JOIN tenant.teams t ON t.id = tm.team_id
		WHERE tm.user_id = $1 AND t.company_id = $2
	`, userID, companyID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query user team layers: %w", err)
	}
	defer rows.Close()

	var out []domain.TeamSettingsLayer
	for rows.Next() {
		var teamID, settingsJSON string
		var priority int32
		if err := rows.Scan(&teamID, &settingsJSON, &priority); err != nil {
			return nil, fmt.Errorf("postgres: scan user team layer row: %w", err)
		}
		settings, err := unmarshalSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("postgres: unmarshal team settings: %w", err)
		}
		out = append(out, domain.TeamSettingsLayer{TeamID: teamID, Priority: priority, Settings: settings})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate user team layer rows: %w", err)
	}
	return out, nil
}
