package domain

import "errors"

var (
	// ErrEmptyRepoID is returned by NewWorktree when RepoID is empty — a
	// worktree with no owning repo is never a valid domain state.
	ErrEmptyRepoID = errors.New("domain: repo_id is required")
	// ErrEmptyWorktreePath is returned by NewWorktree when Path is empty.
	ErrEmptyWorktreePath = errors.New("domain: path is required")
	// ErrEmptyWorktreeBranch is returned by NewWorktree when Branch is empty.
	ErrEmptyWorktreeBranch = errors.New("domain: branch is required")
	// ErrWorktreeNotFound is the sentinel adapter/postgres returns (wrapped)
	// when a lookup/mutation targets a worktree that doesn't exist —
	// usecase/ maps this to apperrors.KindNotFound.
	ErrWorktreeNotFound = errors.New("domain: worktree not found")
)

// Worktree is metadata about a git worktree — explicitly NOT authoritative
// for whether it still exists on disk. git-gateway-service performs the
// real `git worktree add/remove` on the Dev Server Agent and writes this
// record back after the fact (RecordWorktreeCreated/RecordWorktreeRemoved);
// project-service never triggers or verifies a filesystem operation itself.
// See project-service.md §4's Worktree note.
type Worktree struct {
	ID        string
	ProjectID string
	RepoID    string
	Path      string
	Branch    string
	Active    bool
	BaseRef   *string // NEW (SOL-WT-04) — the branch/tag/sha this worktree was created from; nil for worktrees created before this backfill
}

// NewWorktree constructs a Worktree, enforcing the invariants a metadata
// record must satisfy to be meaningful. A freshly recorded worktree starts
// Active — RecordWorktreeCreated is only ever called after the real `git
// worktree add` already succeeded, so there is no "created but inactive"
// state to represent at construction time.
func NewWorktree(id, projectID, repoID, path, branch, baseRef string) (Worktree, error) {
	if projectID == "" {
		return Worktree{}, ErrEmptyProjectID
	}
	if repoID == "" {
		return Worktree{}, ErrEmptyRepoID
	}
	if path == "" {
		return Worktree{}, ErrEmptyWorktreePath
	}
	if branch == "" {
		return Worktree{}, ErrEmptyWorktreeBranch
	}
	wt := Worktree{ID: id, ProjectID: projectID, RepoID: repoID, Path: path, Branch: branch, Active: true}
	if baseRef != "" {
		wt.BaseRef = &baseRef
	}
	return wt, nil
}
