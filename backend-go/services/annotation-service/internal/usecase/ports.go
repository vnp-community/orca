// Package usecase holds annotation-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/annotation-service/internal/domain"
)

// Repository is the persistence port for annotations. Implemented by
// internal/adapter/postgres against annotation-service's own database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule. Every method takes tenantID explicitly (pulled
// from context by the usecase layer, never from the request) so no query
// can accidentally run unscoped.
type Repository interface {
	CreateAnnotation(ctx context.Context, annotation domain.Annotation) (domain.Annotation, error)
	ListAnnotations(ctx context.Context, tenantID, repoID, filePath, pageToken string, pageSize int32) ([]domain.Annotation, string, error)
	// GetAnnotation returns domain.ErrAnnotationNotFound if no annotation
	// with id exists for tenantID. UpdateAnnotation/DeleteAnnotation call
	// this first to read author_id for the OPA author-only check, before
	// mutating.
	GetAnnotation(ctx context.Context, tenantID, id string) (domain.Annotation, error)
	// UpdateAnnotation returns domain.ErrAnnotationNotFound if no annotation
	// with id exists for tenantID.
	UpdateAnnotation(ctx context.Context, tenantID, id, content string, resolved bool) (domain.Annotation, error)
	// DeleteAnnotation returns domain.ErrAnnotationNotFound if no annotation
	// with id exists for tenantID.
	DeleteAnnotation(ctx context.Context, tenantID, id string) error
	// FindByRequestID looks up an existing annotation for (tenantID,
	// requestID), for CreateAnnotation's idempotency check — mirrors
	// automation-service's AutomationRunRepository.FindByRequestID pattern.
	FindByRequestID(ctx context.Context, tenantID, requestID string) (domain.Annotation, bool, error)
	// MarkSent transitions every annotation in ids (scoped to tenantID) to
	// SentToAgent=true/SentAt=sentAt in one statement. Any id not found for
	// the tenant is silently skipped, not a hard failure — SOL-CR-03 calls
	// this after PTY injection already succeeded, so a partial id mismatch
	// (e.g. a concurrently-deleted annotation) must not turn a successful
	// send into an error response.
	MarkSent(ctx context.Context, tenantID string, ids []string, sentAt time.Time) ([]domain.Annotation, error)
}

// OPAClient is the authorization port UpdateAnnotation/DeleteAnnotation use
// to enforce author-only edit/delete (admin override) — implemented by
// internal/adapter/opaclient against the shared embedded OPA evaluator
// (common/policy), consuming backend-go/policy/orca-authz/annotation.rego.
type OPAClient interface {
	// Decision reports whether actorID may act on an annotation authored by
	// authorID, given actorRole ("" when the role isn't known — see this
	// service's README "Known gaps").
	Decision(ctx context.Context, actorID, authorID, actorRole string) (bool, error)
}
