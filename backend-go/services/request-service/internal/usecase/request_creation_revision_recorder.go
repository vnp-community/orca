package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestRevisionRecorder plugs into CreateRequest's RequestCreationRecorder hook: inside the creation
// transaction it records revision 1 (cause=created) of the content the repository just stored, so an
// aborted creation leaves no orphan revision and burns no number.
type RequestRevisionRecorder struct {
	revisions RequestRevisionRepository
	outbox    OutboxWriter
	schemas   ArtifactValidator
	mint      *MintArtifactIDs
}

func NewRequestRevisionRecorder(revisions RequestRevisionRepository, outbox OutboxWriter) *RequestRevisionRecorder {
	return &RequestRevisionRecorder{revisions: revisions, outbox: outbox}
}

func (rec *RequestRevisionRecorder) WithSchemas(v ArtifactValidator) *RequestRevisionRecorder {
	rec.schemas = v
	return rec
}

var _ RequestCreationRecorder = (*RequestRevisionRecorder)(nil)

func (rec *RequestRevisionRecorder) RecordCreated(ctx context.Context, r domain.Request) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ErrRequestTenantRequired()
	}
	c, err := domain.ContentFromRequest(r)
	if err != nil {
		return err
	}
	if vs := domain.ValidateRequestContent(r.Type, c, domain.ValidationLevelDraft); len(vs) > 0 {
		return domain.ErrArtifactSchemaInvalid(vs)
	}
	snapshot, err := c.Snapshot(nil)
	if err != nil {
		return err
	}
	if rec.schemas != nil {
		if vs := rec.schemas.ValidateDocument(domain.ArtifactKindRequest, snapshot); len(vs) > 0 {
			return domain.ErrArtifactSchemaInvalid(vs)
		}
	}
	digest := domain.DigestOfCanonical(snapshot)
	if digest != r.ContentDigest {
		return domain.ErrRequestContentInvalid("the stored content digest does not match the content")
	}
	at := time.Now().UTC()
	if err := rec.revisions.Append(ctx, domain.RequestRevision{
		ID: uuid.NewString(), RequestID: r.ID, Revision: 1, Cause: domain.RevisionCauseCreated, Snapshot: snapshot, Digest: digest,
		ActorID: r.ReporterID, ActorKind: domain.RevisionActorUser, CreatedAt: at,
	}); err != nil {
		return err
	}
	if rec.mint != nil {
		if err := rec.mint.MintRequestID(ctx, r); err != nil {
			return err
		}
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectRequestRevised, RevisedPayload{RequestID: r.ID, Revision: 1, Cause: string(domain.RevisionCauseCreated), Digest: digest})
	if err != nil {
		return err
	}
	ev.OccurredAt = at
	return rec.outbox.InsertOutboxEvent(ctx, ev)
}

// WithIndex also indexes REQ-<number> in artifact_index inside the creation transaction.
func (rec *RequestRevisionRecorder) WithIndex(m *MintArtifactIDs) *RequestRevisionRecorder {
	rec.mint = m
	return rec
}
