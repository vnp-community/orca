# TASK-WT-01-02: Add `ValidateWorktreeName`/`SuggestAlternateName` domain functions

**From Solution:** SOL-WT-01
**Priority:** P0 — the usecase task depends on these
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/domain/worktree_name.go` (new)
**Depends on:** none
**Status:** `[x]` DONE — Created worktree_name.go with ValidateWorktreeName/SuggestAlternateName; go build clean, unit tests added and passing.

---

## Context

BR-WT-01 (charset validation) and [A1]'s alternate-name suggestion are pure input-validation/derivation logic with no I/O — per `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md`'s invariant-in-domain rule, and per [SOL-WT-01](../solutions/SOL-WT-01-tao-worktree.md)'s design, these are free functions (not `Worktree` entity methods — this service's domain is value-object-only, it never constructs a `Worktree` entity; that entity lives in `project-service`).

## Changes to make

Create `backend-go/services/git-gateway-service/internal/domain/worktree_name.go`:

```go
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
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

A follow-on test task ([TASK-WT-01-07](./TASK-WT-01-07-tests.md)) adds `worktree_name_test.go`; this task only needs to compile cleanly on its own.
