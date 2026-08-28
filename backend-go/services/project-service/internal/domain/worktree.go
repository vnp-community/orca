package domain

import (
	"errors"
	"time"
)

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
	CreatedAt time.Time

	// Lineage — explicit-capture only (nil unless this worktree was created
	// with a captured parent context). See
	// proto/orca/project/v1/project.proto's WorktreeLineageEntry doc
	// comment for what each field means; CaptureConfidence is always
	// "explicit" here — project-service never infers it.
	ParentWorktreeID        *string
	Origin                  *string
	CaptureSource           *string
	CaptureConfidence       *string
	TaskID                  *string
	OrchestrationRunID      *string
	CoordinatorHandle       *string
	CreatedByTerminalHandle *string
}

// WorktreeLineageCapture is the optional lineage context a caller may
// supply to NewWorktree — kept as its own type (rather than more NewWorktree
// positional params) since every field is optional and most callers pass
// none of them.
type WorktreeLineageCapture struct {
	ParentWorktreeID        string
	Origin                  string
	CaptureSource           string
	TaskID                  string
	OrchestrationRunID      string
	CoordinatorHandle       string
	CreatedByTerminalHandle string
}

// NewWorktree constructs a Worktree, enforcing the invariants a metadata
// record must satisfy to be meaningful. A freshly recorded worktree starts
// Active — RecordWorktreeCreated is only ever called after the real `git
// worktree add` already succeeded, so there is no "created but inactive"
// state to represent at construction time.
func NewWorktree(id, projectID, repoID, path, branch string, lineage WorktreeLineageCapture) (Worktree, error) {
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
	wt := Worktree{
		ID: id, ProjectID: projectID, RepoID: repoID, Path: path, Branch: branch, Active: true,
		ParentWorktreeID:        nonEmptyPtr(lineage.ParentWorktreeID),
		Origin:                  nonEmptyPtr(lineage.Origin),
		CaptureSource:           nonEmptyPtr(lineage.CaptureSource),
		TaskID:                  nonEmptyPtr(lineage.TaskID),
		OrchestrationRunID:      nonEmptyPtr(lineage.OrchestrationRunID),
		CoordinatorHandle:       nonEmptyPtr(lineage.CoordinatorHandle),
		CreatedByTerminalHandle: nonEmptyPtr(lineage.CreatedByTerminalHandle),
	}
	// Any captured lineage field means this worktree's lineage was captured
	// explicitly by its creator — project-service never infers lineage
	// itself (see WorktreeLineageEntry's doc comment), so this is the only
	// value CaptureConfidence ever takes today.
	if wt.ParentWorktreeID != nil || wt.Origin != nil || wt.TaskID != nil || wt.OrchestrationRunID != nil {
		explicit := "explicit"
		wt.CaptureConfidence = &explicit
	}
	return wt, nil
}

// nonEmptyPtr returns nil for an empty string, else a pointer to it — the
// idiom every optional string lineage field uses to distinguish "not
// supplied" from a genuinely empty value.
func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
