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
	// ErrAnnotationNotFound is returned by internal/adapter/postgres when a
	// lookup/update/delete targets an annotation that doesn't exist (or
	// isn't visible to the calling tenant) — usecase/ maps this to
	// apperrors.KindNotFound.
	ErrAnnotationNotFound = errors.New("domain: annotation not found")
)

// Anchor is the value object describing where a comment points: a specific
// file+line in a repository, resolved against a commit/ref so a later diff
// rebase doesn't silently misattach it. See annotation-service.md §4.
type Anchor struct {
	RepoID   string
	FilePath string
	Line     int32
	Ref      string
}

// NewAnchor constructs an Anchor, enforcing the invariants a location
// reference must satisfy to be meaningful: a non-empty repo and file, and a
// non-negative line number.
func NewAnchor(repoID, filePath string, line int32, ref string) (Anchor, error) {
	if repoID == "" {
		return Anchor{}, ErrEmptyRepoID
	}
	if filePath == "" {
		return Anchor{}, ErrEmptyFilePath
	}
	if line < 0 {
		return Anchor{}, ErrNegativeLine
	}
	return Anchor{RepoID: repoID, FilePath: filePath, Line: line, Ref: ref}, nil
}

// Annotation is one inline code-review comment anchored to a file+line —
// see annotation-service.md §4. It is a logical reference, not a copy of
// reviewed content: this service never owns or caches diff/file content.
type Annotation struct {
	ID        string
	TenantID  string
	AuthorID  string
	Anchor    Anchor
	Content   string
	Resolved  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewAnnotation constructs an Annotation, enforcing the invariants a
// comment must satisfy to be meaningful — this is where "annotation-service
// owns this data's correctness" actually lives, not scattered validation in
// the gRPC handler.
func NewAnnotation(
	id, tenantID, authorID string,
	anchor Anchor,
	content string,
	resolved bool,
	createdAt, updatedAt time.Time,
) (Annotation, error) {
	if tenantID == "" || authorID == "" {
		return Annotation{}, ErrEmptyTenant
	}
	if content == "" {
		return Annotation{}, ErrEmptyContent
	}
	if _, err := NewAnchor(anchor.RepoID, anchor.FilePath, anchor.Line, anchor.Ref); err != nil {
		return Annotation{}, err
	}
	return Annotation{
		ID:        id,
		TenantID:  tenantID,
		AuthorID:  authorID,
		Anchor:    anchor,
		Content:   content,
		Resolved:  resolved,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}
