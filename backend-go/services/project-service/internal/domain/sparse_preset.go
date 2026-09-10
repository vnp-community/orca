package domain

import (
	"errors"
	"time"
)

// SparsePreset is a saved directory set for sparse worktree creation,
// scoped to one specific repo — ports backend/src/shared/types.ts's
// SparsePreset (legacy TS reference) 1:1.
type SparsePreset struct {
	ID          string
	RepoID      string
	Name        string
	Directories []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ErrSparsePresetNotFound is returned by SparsePresetRepository when no
// preset matches the given (repoID, presetID) pair.
var ErrSparsePresetNotFound = errors.New("domain: sparse preset not found")
