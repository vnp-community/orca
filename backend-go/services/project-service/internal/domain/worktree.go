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

// WorktreeStatus is a worktree's lifecycle status — orthogonal to Active
// (activation_state): a worktree can be "completed" yet still Active, or
// "active" yet deactivated. BL-AT-04's cleanup_worktrees step filters on
// this, not on Active.
type WorktreeStatus string

const (
	WorktreeStatusActive    WorktreeStatus = "active"
	WorktreeStatusCompleted WorktreeStatus = "completed"
	WorktreeStatusError     WorktreeStatus = "error"
	WorktreeStatusStopped   WorktreeStatus = "stopped"
)

func (s WorktreeStatus) Valid() bool {
	switch s {
	case WorktreeStatusActive, WorktreeStatusCompleted, WorktreeStatusError, WorktreeStatusStopped:
		return true
	default:
		return false
	}
}

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

	IdempotencyKey *string // BR-CLI-01: caller-supplied dedupe key, nil when not set

	// LinkedIssueProvider/LinkedIssueRef carry BR-PI-06's linked-issue
	// reference through to the worktree.created/worktree.deleted outbox
	// events (SOL-PI-03) — empty means "no linked issue".
	LinkedIssueProvider string
	LinkedIssueRef      string

	// Status is orthogonal to Active — see WorktreeStatus's doc comment.
	Status WorktreeStatus

	BaseRef *string // NEW (SOL-WT-04) — the branch/tag/sha this worktree was created from; nil for worktrees created before this backfill
}

// NewWorktree constructs a Worktree, enforcing the invariants a metadata
// record must satisfy to be meaningful. A freshly recorded worktree starts
// Active and WorktreeStatusActive — RecordWorktreeCreated is only ever
// called after the real `git worktree add` already succeeded, so there is
// no "created but inactive"/"created but not yet active-status" state to
// represent at construction time.
func NewWorktree(id, projectID, repoID, path, branch, idempotencyKey, baseRef string) (Worktree, error) {
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
	return Worktree{
		ID: id, ProjectID: projectID, RepoID: repoID, Path: path, Branch: branch, Active: true,
		IdempotencyKey: nonEmptyPtr(idempotencyKey),
		Status:         WorktreeStatusActive,
		BaseRef:        nonEmptyPtr(baseRef),
	}, nil
}

// nonEmptyPtr returns nil for an empty string, otherwise a pointer to s —
// the idiom used for every optional string field this package models as
// "unset" rather than "empty".
func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
