package domain

import "errors"

var (
	// ErrEmptyRepoURL is returned by NewRepo when URL is empty — a repo
	// catalog entry pointing nowhere is never a valid domain state.
	ErrEmptyRepoURL = errors.New("domain: url is required")
	// ErrRepoNotFound is the sentinel adapter/postgres returns (wrapped) when
	// a lookup/remove targets a repo that doesn't exist — usecase/ maps this
	// to apperrors.KindNotFound.
	ErrRepoNotFound = errors.New("domain: repo not found")
	// ErrRepoProjectChanged is the sentinel adapter/postgres's ReassignProject
	// returns (wrapped) when the repo's project_id no longer matches the
	// value the caller was authorized against (a concurrent move raced this
	// one) — usecase.AssignRepoToProject maps this to
	// apperrors.KindFailedPrecondition, distinct from ErrRepoNotFound (the
	// repo does exist, just not where the authorization check assumed).
	ErrRepoProjectChanged = errors.New("domain: repo's project changed concurrently")
)

// Repo is a project's repository catalog entry — metadata only (url,
// display name, ordering), no working-tree state. See project-service.md
// §4's Repo entity; this is the slice of that fuller model
// (path/remote_url/icon_ref) the current proto surface (AddRepo/ListRepos/
// ReorderRepos/RemoveRepo) actually exercises.
type Repo struct {
	ID          string
	ProjectID   string
	URL         string
	DisplayName string
	// Position orders repos within a project — see usecase.ReorderRepos.
	// Ordering is by position value, not contiguity: removing a repo leaves
	// a gap deliberately rather than renumbering the rest.
	Position int32
	// DevServerID is THIS repo's own dev-server binding — Phase 10's fix:
	// previously only project.projects.dev_server_id existed, so a repo's
	// host was inferred from its project, never stated directly, and a
	// repo actually checked out on a different host than its project's
	// binding had no way to say so. Empty = local (no dev server). See
	// migrations/0017_repo_dev_server for the backfill from the old
	// project-level column.
	DevServerID string
}

// NewRepo constructs a Repo, enforcing the invariants a catalog entry must
// satisfy to be meaningful. Position isn't a constructor parameter — it's
// assigned by the repository (AddRepo appends at the next available slot),
// not chosen by the caller. devServerID may be empty (a local repo).
func NewRepo(id, projectID, url, displayName, devServerID string) (Repo, error) {
	if projectID == "" {
		return Repo{}, ErrEmptyProjectID
	}
	if url == "" {
		return Repo{}, ErrEmptyRepoURL
	}
	return Repo{ID: id, ProjectID: projectID, URL: url, DisplayName: displayName, DevServerID: devServerID}, nil
}
