package domain

import (
	"errors"
	"regexp"
	"strconv"
)

var worktreeNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// ErrInvalidWorktreeName is returned by ValidateWorktreeName when name fails
// BR-WT-01's charset rule.
var ErrInvalidWorktreeName = errors.New("domain: worktree name must match [a-z0-9_-]+")

// ValidateWorktreeName enforces BR-WT-01: a worktree name is non-empty and
// matches [a-z0-9_-]+. Free function, not a Worktree entity method — this
// package's domain is value-object-only (git-gateway-service owns no
// Worktree entity; project-service does).
func ValidateWorktreeName(name string) error {
	if !worktreeNamePattern.MatchString(name) {
		return ErrInvalidWorktreeName
	}
	return nil
}

// SuggestAlternateName appends "-2", "-3", ... to base until a name not
// present in taken is found — [A1]'s duplicate-path recovery suggestion.
// base itself is returned unchanged if it isn't taken.
func SuggestAlternateName(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		if !taken[candidate] {
			return candidate
		}
	}
}
