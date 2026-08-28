package domain

import "time"

// GrantLevel is task-service's permission tier. Unlike the design doc's
// sketch schema (grantee_type ∈ {user,team,company} as a field separate
// from a 3-value level), the generated proto's GrantLevel enum
// (orca.task.v1.GrantLevel) folds grantee-kind into the level itself:
// Owner/Admin/User are direct per-user grants, Team and Company are
// scope-wide grants whose SubjectID names the team/company being granted
// to. This scaffold follows the generated proto, the authoritative wire
// contract — see this service's README.
type GrantLevel int

const (
	GrantLevelUnspecified GrantLevel = iota
	GrantLevelOwner
	GrantLevelAdmin
	GrantLevelUser
	GrantLevelTeam
	GrantLevelCompany
)

func (l GrantLevel) Valid() bool {
	switch l {
	case GrantLevelOwner, GrantLevelAdmin, GrantLevelUser, GrantLevelTeam, GrantLevelCompany:
		return true
	default:
		return false
	}
}

// priority ranks GrantLevel for resolution: lower number wins. Matches
// task-service.md §4.1 step 4: "owner > admin > user > team > company —
// priority wins over proximity, matching TS semantics."
func (l GrantLevel) priority() int {
	switch l {
	case GrantLevelOwner:
		return 0
	case GrantLevelAdmin:
		return 1
	case GrantLevelUser:
		return 2
	case GrantLevelTeam:
		return 3
	case GrantLevelCompany:
		return 4
	default:
		return 1 << 30 // unspecified/unknown never outranks a real grant
	}
}

// Grant is one task_grants row — a permission assignment at a specific
// task. ApplyTree=true means the grant is inherited by the task's
// descendant subtree during ancestor-walk resolution (§4.1).
type Grant struct {
	ID        string // new — needed by RevokeGrant
	TaskID    string
	SubjectID string
	Level     GrantLevel
	ApplyTree bool
	ExpiresAt *time.Time // new — nil = never expires
}

// CallerIdentity is the resolved-identity input to grant resolution: the
// caller's own user ID plus the team IDs tenant-service reports them as a
// member of (§4.1 step 1 — "team memberships resolved via tenant-service"),
// plus the company/tenant they belong to. Built by the ResolvePermission
// usecase, never by domain code itself (no gRPC calls from domain/).
type CallerIdentity struct {
	UserID    string
	TeamIDs   []string
	CompanyID string
}

func (c CallerIdentity) hasTeam(teamID string) bool {
	for _, t := range c.TeamIDs {
		if t == teamID {
			return true
		}
	}
	return false
}

// Matches reports whether this grant applies to caller, per the subject
// semantics GrantLevel encodes: Owner/Admin/User grants match by user ID,
// Team grants match by team membership, Company grants match by
// company/tenant ID.
func (g Grant) Matches(caller CallerIdentity) bool {
	switch g.Level {
	case GrantLevelOwner, GrantLevelAdmin, GrantLevelUser:
		return g.SubjectID != "" && g.SubjectID == caller.UserID
	case GrantLevelTeam:
		return g.SubjectID != "" && caller.hasTeam(g.SubjectID)
	case GrantLevelCompany:
		return g.SubjectID != "" && g.SubjectID == caller.CompanyID
	default:
		return false
	}
}
