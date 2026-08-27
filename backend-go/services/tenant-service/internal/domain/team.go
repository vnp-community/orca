package domain

import "errors"

// ErrEmptyUserID is returned when a required user_id field is empty.
var ErrEmptyUserID = errors.New("domain: user_id is required")

// Team belongs to a Company. Deliberately no department_id — teams are not
// scoped to one department by design, carried forward from
// docs/guides/user-profile-team-department-rbac.md §5.2 (tenant-service.md
// §4). A user may belong to zero, one, or many teams.
type Team struct {
	ID        string
	CompanyID string
	Name      string
	// Settings is the team-layer profile override. tenant.proto's
	// CreateTeamRequest doesn't yet expose a way to set it (no UpdateTeam
	// RPC in this reduced surface) — see README "Known gaps" — but the
	// field exists here because ResolveProfile's team layer needs it
	// regardless of which RPCs currently populate it.
	Settings Settings
}

// NewTeam constructs a Team, enforcing the invariants every row in
// tenant.teams must satisfy.
func NewTeam(id, companyID, name string, settings Settings) (Team, error) {
	if id == "" {
		return Team{}, ErrEmptyID
	}
	if companyID == "" {
		return Team{}, ErrEmptyID
	}
	if name == "" {
		return Team{}, ErrEmptyName
	}
	return Team{ID: id, CompanyID: companyID, Name: name, Settings: emptySettings(settings)}, nil
}

// TeamMember joins a Team and a user, with the Priority tiebreaker used by
// ResolveProfile when a user belongs to several teams that define the same
// setting. Named TeamMember (not the design doc's prose "TeamMembership")
// to match tenant.proto's TeamMember message and AddTeamMemberRequest 1:1.
// Priority carries forward unchanged from the TS system's migration 0016
// (tenant-service.md §4).
type TeamMember struct {
	TeamID   string
	UserID   string
	Priority int32
}

// NewTeamMember constructs a TeamMember, enforcing the invariants every row
// in tenant.team_members must satisfy.
func NewTeamMember(teamID, userID string, priority int32) (TeamMember, error) {
	if teamID == "" {
		return TeamMember{}, ErrEmptyID
	}
	if userID == "" {
		return TeamMember{}, ErrEmptyUserID
	}
	return TeamMember{TeamID: teamID, UserID: userID, Priority: priority}, nil
}
