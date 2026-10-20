package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewArtifactRepository stores the drawings of reviews. Every call is scoped to the tenant of the context.
type ReviewArtifactRepository interface {
	// Save stores the blob (once per Request and digest) and the row, takes the next revision of the key, and when the row is
	// active makes it the only active drawing of its slot. blob is nil for a failed attempt, which has no content.
	Save(ctx context.Context, a domain.ReviewArtifact, blob *domain.ReviewBlob) (domain.ReviewArtifact, error)
	// FindByKey returns the newest revision stored under the key, in any state.
	FindByKey(ctx context.Context, requestID string, key domain.ArtifactKey) (domain.ReviewArtifact, bool, error)
	// Active returns the drawing in force for the slot.
	Active(ctx context.Context, requestID string, section domain.ReviewSection, variant, name string) (domain.ReviewArtifact, bool, error)
	// Activate makes a superseded drawing the active one of its slot again, which costs nothing because the content is kept.
	Activate(ctx context.Context, requestID, id string) (domain.ReviewArtifact, error)
	Get(ctx context.Context, requestID, id string) (domain.ReviewArtifact, error)
	// ListRevisions returns every drawing of the slot, newest first.
	ListRevisions(ctx context.Context, requestID string, section domain.ReviewSection, variant, name string) ([]domain.ReviewArtifact, error)
	// ListActive returns the active drawings of the Request, without content.
	ListActive(ctx context.Context, requestID string) ([]domain.ReviewArtifact, error)
	// Blob returns the content; ok is false when there is none under the digest.
	Blob(ctx context.Context, requestID, digest string) (domain.ReviewBlob, bool, error)
	// StoredBytes is what the tenant keeps, for the quota.
	StoredBytes(ctx context.Context) (int64, error)
	// DeleteSupersededBefore deletes up to limit drawings that stopped being active before the time, and failed attempts older than it.
	DeleteSupersededBefore(ctx context.Context, before time.Time, limit int) (int, error)
	// DeleteOrphanBlobs deletes up to limit blobs no drawing points at that are older than the time.
	DeleteOrphanBlobs(ctx context.Context, olderThan time.Time, limit int) (int, error)
}
