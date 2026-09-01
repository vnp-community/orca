package domain

import "errors"

// RepoRole is a user's functional role on one specific repo within a
// project — a second, separate authorization tier layered on top of
// ProjectRole. ProjectRole (owner/member) decides who's in a project at
// all; RepoRole decides what a project member can do on one particular
// repo (a developer might act on repo X but have no RepoRole grant on repo
// Y in the same project). A project owner always bypasses this tier
// entirely on their own project's repos — see requireRepoAccess.
type RepoRole string

const (
	RepoRoleDeveloper RepoRole = "developer"
	RepoRoleLead      RepoRole = "lead"
	RepoRoleAdmin     RepoRole = "admin"
)

// Valid reports whether r is one of the known enum values.
func (r RepoRole) Valid() bool {
	switch r {
	case RepoRoleDeveloper, RepoRoleLead, RepoRoleAdmin:
		return true
	default:
		return false
	}
}

var (
	// ErrInvalidRepoRole is returned by NewRepoMember when Role isn't a known enum value.
	ErrInvalidRepoRole = errors.New("domain: invalid repo role")
	// ErrRepoMembershipNotFound is returned by RepoMembershipRepository.
	// GetRepoMembership when the acting user has no repo_members row for
	// the repo in question — requireRepoAccess's normal "no functional-role
	// grant on this repo" case, not an infrastructure error.
	ErrRepoMembershipNotFound = errors.New("domain: repo membership not found")
)

// RepoMember links a user into one repo with a functional role. Requires
// the user to already be a project member (owner/member) — this is an
// additional grant on top of that, not a replacement for it.
type RepoMember struct {
	RepoID string
	UserID string
	Role   RepoRole
}

// NewRepoMember constructs a RepoMember, enforcing its invariants.
func NewRepoMember(repoID, userID string, role RepoRole) (RepoMember, error) {
	if repoID == "" {
		return RepoMember{}, ErrEmptyRepoID
	}
	if userID == "" {
		return RepoMember{}, ErrEmptyUserID
	}
	if !role.Valid() {
		return RepoMember{}, ErrInvalidRepoRole
	}
	return RepoMember{RepoID: repoID, UserID: userID, Role: role}, nil
}
