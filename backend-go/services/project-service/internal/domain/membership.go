package domain

import "errors"

// ProjectRole is a project member's role. Deliberately mirrors the proto
// enum's two values (member/owner) — the fuller owner/member/viewer model
// from project-service.md §4 is a documented follow-up once ListMembers/
// UpdateMemberRole/the "≥1 owner" invariant are ported.
type ProjectRole string

const (
	ProjectRoleMember ProjectRole = "member"
	ProjectRoleOwner  ProjectRole = "owner"
)

// Valid reports whether r is one of the known enum values.
func (r ProjectRole) Valid() bool {
	switch r {
	case ProjectRoleMember, ProjectRoleOwner:
		return true
	default:
		return false
	}
}

var (
	// ErrEmptyProjectID is returned by NewProjectMember when ProjectID is empty.
	ErrEmptyProjectID = errors.New("domain: project_id is required")
	// ErrEmptyUserID is returned by NewProjectMember when UserID is empty.
	ErrEmptyUserID = errors.New("domain: user_id is required")
	// ErrInvalidRole is returned by NewProjectMember when Role isn't a known enum value.
	ErrInvalidRole = errors.New("domain: invalid project role")
	// ErrMembershipNotFound is returned by ProjectRepository.GetMembership /
	// MembershipRepository.GetMembership when the acting user has no
	// membership row for the project in question — the OPA authorization
	// check's normal "not a member of this project" case, not an
	// infrastructure error.
	ErrMembershipNotFound = errors.New("domain: project membership not found")
)

// ProjectMember links a user into a project with a role.
type ProjectMember struct {
	ProjectID string
	UserID    string
	Role      ProjectRole
}

// NewProjectMember constructs a ProjectMember, enforcing its invariants.
func NewProjectMember(projectID, userID string, role ProjectRole) (ProjectMember, error) {
	if projectID == "" {
		return ProjectMember{}, ErrEmptyProjectID
	}
	if userID == "" {
		return ProjectMember{}, ErrEmptyUserID
	}
	if !role.Valid() {
		return ProjectMember{}, ErrInvalidRole
	}
	return ProjectMember{ProjectID: projectID, UserID: userID, Role: role}, nil
}
