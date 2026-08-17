// Package usecase holds annotation-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"

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
	// UpdateAnnotation returns domain.ErrAnnotationNotFound if no annotation
	// with id exists for tenantID.
	UpdateAnnotation(ctx context.Context, tenantID, id, content string, resolved bool) (domain.Annotation, error)
	// DeleteAnnotation returns domain.ErrAnnotationNotFound if no annotation
	// with id exists for tenantID.
	DeleteAnnotation(ctx context.Context, tenantID, id string) error
}
