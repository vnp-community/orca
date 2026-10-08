package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AppendInput describes one content revision. ExpectedVersion 0 skips the version check (the caller holds the row lock).
type AppendInput struct {
	RequestID       string
	Content         domain.RequestContent
	Cause           domain.RevisionCause
	ActorID         string
	ActorKind       domain.ActorKind
	ClarificationID string
	ExpectedVersion int64
	// Meta is recorded in the revision snapshot only (e.g. a readiness waiver); it is not part of the digest.
	Meta map[string]any
}

type AppendResult struct {
	Request domain.Request
	// Applied is false when the content was already current (nothing written, nothing emitted).
	Applied bool
}

// RevisedPayload is the wire shape of orca.request.request.revised; it carries no Request content.
type RevisedPayload struct {
	RequestID string `json:"request_id"`
	Revision  int    `json:"revision"`
	Cause     string `json:"cause"`
	Digest    string `json:"digest"`
}

// ArtifactValidator checks a document against its JSON Schema; *domain.SchemaRegistry satisfies it.
type ArtifactValidator interface {
	ValidateDocument(kind domain.ArtifactKind, raw []byte) []domain.Violation
}

// AppendRequestRevision is the only code allowed to change Request content after creation
// (request_content_write_guard_test.go enforces it).
type AppendRequestRevision struct {
	repo      RequestRepository
	content   RequestContentWriter
	revisions RequestRevisionRepository
	tx        TxRunner
	outbox    OutboxWriter
	schemas   ArtifactValidator
	clock     func() time.Time
}

func NewAppendRequestRevision(repo RequestRepository, content RequestContentWriter, revisions RequestRevisionRepository, tx TxRunner, outbox OutboxWriter) *AppendRequestRevision {
	return &AppendRequestRevision{repo: repo, content: content, revisions: revisions, tx: tx, outbox: outbox, clock: func() time.Time { return time.Now().UTC() }}
}

// WithSchemas makes every snapshot pass the request JSON Schema before it is stored.
func (uc *AppendRequestRevision) WithSchemas(v ArtifactValidator) *AppendRequestRevision {
	uc.schemas = v
	return uc
}

func (uc *AppendRequestRevision) Execute(ctx context.Context, in AppendInput) (AppendResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return AppendResult{}, domain.ErrRequestTenantRequired()
	}
	var out AppendResult
	err := uc.tx.InTx(ctx, func(txCtx context.Context) error {
		res, err := uc.AppendWithinTx(txCtx, in)
		out = res
		return err
	})
	return out, err
}

// AppendWithinTx joins the caller's transaction (it never opens a nested one); CreateRequest and AnswerClarification use it.
func (uc *AppendRequestRevision) AppendWithinTx(ctx context.Context, in AppendInput) (AppendResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return AppendResult{}, domain.ErrRequestTenantRequired()
	}
	if _, err := domain.ParseRevisionCause(string(in.Cause)); err != nil {
		return AppendResult{}, err
	}
	r, err := uc.repo.Get(ctx, in.RequestID)
	if err != nil {
		return AppendResult{}, err
	}
	if in.ExpectedVersion != 0 && in.ExpectedVersion != r.Version {
		return AppendResult{}, domain.ErrRequestVersionConflict(r.ID, in.ExpectedVersion)
	}
	if vs := domain.ValidateRequestContent(r.Type, in.Content, domain.ValidationLevelDraft); len(vs) > 0 {
		return AppendResult{}, domain.ErrArtifactSchemaInvalid(vs)
	}
	digest, err := in.Content.Digest()
	if err != nil {
		return AppendResult{}, err
	}
	if digest == r.ContentDigest && len(in.Meta) == 0 && in.Cause != domain.RevisionCauseClarificationAnswered {
		return AppendResult{Request: r}, nil // idempotent: the same content is already the current revision
	}
	snapshot, err := in.Content.Snapshot(in.Meta)
	if err != nil {
		return AppendResult{}, err
	}
	if uc.schemas != nil {
		if vs := uc.schemas.ValidateDocument(domain.ArtifactKindRequest, snapshot); len(vs) > 0 {
			return AppendResult{}, domain.ErrArtifactSchemaInvalid(vs)
		}
	}

	next, err := r.WithContent(in.Content)
	if err != nil {
		return AppendResult{}, err
	}
	next.ContentRevision = r.ContentRevision + 1
	next.ContentSchemaVersion = domain.LatestSchemaVersion(domain.ArtifactKindRequest)
	updated, err := uc.content.UpdateContent(ctx, next, r.Version)
	if err != nil {
		return AppendResult{}, err
	}
	at := uc.clock()
	if err := uc.revisions.Append(ctx, domain.RequestRevision{
		ID: uuid.NewString(), RequestID: r.ID, Revision: updated.ContentRevision, Cause: in.Cause, Snapshot: snapshot, Digest: digest,
		ActorID: in.ActorID, ActorKind: domain.RevisionActorFrom(in.ActorKind), ClarificationID: in.ClarificationID, CreatedAt: at,
	}); err != nil {
		return AppendResult{}, err
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectRequestRevised, RevisedPayload{
		RequestID: r.ID, Revision: updated.ContentRevision, Cause: string(in.Cause), Digest: digest,
	})
	if err != nil {
		return AppendResult{}, err
	}
	ev.OccurredAt = at
	if err := uc.outbox.InsertOutboxEvent(ctx, ev); err != nil {
		return AppendResult{}, err
	}
	return AppendResult{Request: updated, Applied: true}, nil
}
