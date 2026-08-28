// Package domain holds annotation-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework.
package domain

import (
	"errors"
	"time"
)

var (
	// ErrEmptyRepoID is returned by NewAnchor when RepoID is empty — an
	// annotation anchored to no repository is never a valid domain state.
	ErrEmptyRepoID = errors.New("domain: repo_id is required")
	// ErrEmptyFilePath is returned by NewAnchor when FilePath is empty.
	ErrEmptyFilePath = errors.New("domain: file_path is required")
	// ErrNegativeLine guards against a caller passing a negative line
	// number, which would never point at a real diff/file location.
	ErrNegativeLine = errors.New("domain: line must not be negative")
	// ErrEmptyTenant is returned when TenantID/AuthorID are empty — an
	// annotation with no owning tenant/author is never a valid domain state.
	ErrEmptyTenant = errors.New("domain: tenant_id and author_id are required")
	// ErrEmptyContent guards against a caller persisting a blank comment.
	ErrEmptyContent = errors.New("domain: content is required")
	// ErrEmptyRequestID guards against a create with no idempotency key —
	// see usage-service/automation-service's identical (tenant_id,
	// request_id) convention this mirrors.
	ErrEmptyRequestID = errors.New("domain: request_id is required")
	// ErrAnnotationNotFound is returned by internal/adapter/postgres when a
	// lookup/update/delete targets an annotation that doesn't exist (or
	// isn't visible to the calling tenant) — usecase/ maps this to
	// apperrors.KindNotFound.
	ErrAnnotationNotFound = errors.New("domain: annotation not found")

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

// Anchor is the value object describing where a comment points: a specific
// file+line in a repository, resolved against a commit/ref so a later diff
// rebase doesn't silently misattach it. See annotation-service.md §4.
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

// Annotation is one inline code-review comment anchored to a file+line —
// see annotation-service.md §4. It is a logical reference, not a copy of
// reviewed content: this service never owns or caches diff/file content.
type Annotation struct {
	ID           string
	TenantID     string
	AuthorID     string
	Anchor       Anchor
	Content      string
	OriginalCode string // NEW — BL-CR-02 DiffComment.originalCode
	Resolved     bool
	SentToAgent  bool       // NEW — distinct from Resolved (BR-CR-08)
	SentAt       *time.Time // NEW
	RequestID    string     // idempotency key, see standards/api-design-guidelines.md
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

// NewAnnotation constructs an Annotation, enforcing the invariants a
// comment must satisfy to be meaningful — this is where "annotation-service
// owns this data's correctness" actually lives, not scattered validation in
// the gRPC handler.
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
