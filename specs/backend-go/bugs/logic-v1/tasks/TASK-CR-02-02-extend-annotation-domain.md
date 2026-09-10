# TASK-CR-02-02: Extend `Anchor`/`Annotation` domain types with side, range, and sent-state

**From Solution:** SOL-CR-02
**Priority:** P0
**Service:** `annotation-service`
**File:** `backend-go/services/annotation-service/internal/domain/annotation.go`
**Depends on:** none
**Status:** `[x]` DONE — domain/annotation.go rewritten with Side type, extended Anchor/Annotation, MarkSent; annotation_test.go + domain_test.go updated and passing

---

## Context

Domain types (`Anchor`, `Annotation`) are pure Go with zero framework
imports (`03-clean-architecture-guidelines.md`) — they don't depend on the
generated proto stubs from TASK-CR-02-01, so this can be built independently.
This closes the domain-invariant half of BUG-CR-02's gaps: `EndLine >= Line`
(BR-CR-06) and a pure `MarkSent` transition.

## Changes to make

Add a new error and the `Side` type near the existing `var (...)` error
block:

```go
var (
	// ... existing errors unchanged ...

	// ErrEndLineBeforeLine is returned by NewAnchor when EndLine is set and
	// is less than Line — a range that ends before it starts is never a
	// valid domain state.
	ErrEndLineBeforeLine = errors.New("domain: end_line must not be before line")
)

// Side identifies which half of a diff an annotation is anchored to.
// SideUnspecified is a legitimate state for a non-diff comment (a plain
// file/line note outside diff-review context) — NewAnchor does not reject
// it.
type Side int32

const (
	SideUnspecified Side = iota
	SideOld
	SideNew
)
```

Replace the `Anchor` struct and `NewAnchor`:

```go
type Anchor struct {
	RepoID     string
	WorktreeID string // optional; empty = not worktree-scoped (existing callers)
	FilePath   string
	Line       int32
	EndLine    int32 // 0 treated as == Line; BR-CR-06
	Side       Side
	Ref        string
}

// NewAnchor constructs an Anchor, enforcing the invariants a location
// reference must satisfy to be meaningful: a non-empty repo and file, a
// non-negative line number, and (when set) an end_line that isn't before
// line.
func NewAnchor(repoID, worktreeID, filePath string, line, endLine int32, side Side, ref string) (Anchor, error) {
	if repoID == "" {
		return Anchor{}, ErrEmptyRepoID
	}
	if filePath == "" {
		return Anchor{}, ErrEmptyFilePath
	}
	if line < 0 {
		return Anchor{}, ErrNegativeLine
	}
	if endLine != 0 && endLine < line {
		return Anchor{}, ErrEndLineBeforeLine
	}
	return Anchor{
		RepoID: repoID, WorktreeID: worktreeID, FilePath: filePath,
		Line: line, EndLine: endLine, Side: side, Ref: ref,
	}, nil
}
```

Add fields to `Annotation` and a `MarkSent` method:

```go
type Annotation struct {
	ID           string
	TenantID     string
	AuthorID     string
	Anchor       Anchor
	Content      string
	OriginalCode string     // NEW
	Resolved     bool
	SentToAgent  bool       // NEW
	SentAt       *time.Time // NEW
	RequestID    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// MarkSent returns a copy of a with SentToAgent=true and SentAt=&at — pure,
// like the rest of domain/, called from mark_annotations_sent.go
// (TASK-CR-02-06).
func (a Annotation) MarkSent(at time.Time) Annotation {
	a.SentToAgent = true
	a.SentAt = &at
	return a
}
```

Update `NewAnnotation`'s signature to accept `originalCode` and call the new
`NewAnchor` shape:

```go
func NewAnnotation(
	id, tenantID, authorID string,
	anchor Anchor,
	content, originalCode string,
	resolved bool,
	requestID string,
	createdAt, updatedAt time.Time,
) (Annotation, error) {
	if tenantID == "" || authorID == "" {
		return Annotation{}, ErrEmptyTenant
	}
	if content == "" {
		return Annotation{}, ErrEmptyContent
	}
	if requestID == "" {
		return Annotation{}, ErrEmptyRequestID
	}
	if _, err := NewAnchor(anchor.RepoID, anchor.WorktreeID, anchor.FilePath, anchor.Line, anchor.EndLine, anchor.Side, anchor.Ref); err != nil {
		return Annotation{}, err
	}
	return Annotation{
		ID: id, TenantID: tenantID, AuthorID: authorID,
		Anchor: anchor, Content: content, OriginalCode: originalCode,
		Resolved: resolved, RequestID: requestID,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}
```

`originalCode` is passed through unvalidated — empty is legitimate (a
comment on a deleted/binary line has no meaningful original-code snapshot).

Callers of `NewAnchor`/`NewAnnotation` (TASK-CR-02-04, TASK-CR-02-06, and
`internal/adapter/postgres`) must be updated to the new signatures in their
own tasks — this task only changes `domain/annotation.go` and its test
file.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/annotation-service
go build ./internal/domain/...
go test ./internal/domain/... -run TestNewAnchor -v
go test ./internal/domain/... -run TestAnnotation_MarkSent -v
```

Add/extend `internal/domain/annotation_test.go` with cases for:
`NewAnchor` rejects `EndLine < Line`; `SideUnspecified` accepted;
`Annotation.MarkSent` sets both fields and leaves the rest of the struct
unchanged (copy semantics — the receiver `a` must not be mutated).
