package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewArtifactObserver counts what the drawing store does, for the metrics (hit ratio, bytes kept, content shared).
type ReviewArtifactObserver interface {
	// Lookup is hit, miss, failed or corrupt.
	ArtifactLookup(result string)
	ArtifactStored(bytes int, deduped bool)
}

type noopArtifactObserver struct{}

func (noopArtifactObserver) ArtifactLookup(string)    {}
func (noopArtifactObserver) ArtifactStored(int, bool) {}

func artifactObserverOrNoop(o ReviewArtifactObserver) ReviewArtifactObserver {
	if o == nil {
		return noopArtifactObserver{}
	}
	return o
}

// ReviewArtifactStore is the cache of drawings. A drawing is looked up by the digest of its diagram model and of the preset, so the
// same model is never drawn twice for the same Request, whichever review, commit or phase asks.
type ReviewArtifactStore struct {
	Repo     ReviewArtifactRepository
	Observer ReviewArtifactObserver
	// QuotaBytes is the most a tenant may keep; zero means the default.
	QuotaBytes int64
	Now        func() time.Time
}

func (s *ReviewArtifactStore) obs() ReviewArtifactObserver { return artifactObserverOrNoop(s.Observer) }

func (s *ReviewArtifactStore) quota() int64 {
	if s.QuotaBytes > 0 {
		return s.QuotaBytes
	}
	return domain.DefaultArtifactQuotaMB << 20
}

// ArtifactLookup is what the store knows about the drawing of one model.
type ArtifactLookup struct {
	Model    domain.DiagramModel
	Key      domain.ArtifactKey
	Result   domain.CacheStatus
	Artifact *domain.ReviewArtifact
}

// KeyFor builds the cache key of a model under the profile in force and the skill version.
func KeyFor(m domain.DiagramModel, profile domain.ResolvedProfile, skillVersion string) (domain.ArtifactKey, error) {
	preset, err := profile.PresetDigest(m.Section, m.Variant)
	if err != nil {
		return domain.ArtifactKey{}, err
	}
	return domain.ArtifactKey{Section: m.Section, Variant: m.Variant, Name: m.Name, InputDigest: m.InputDigest, PresetDigest: preset, SkillVersion: skillVersion}, nil
}

// Resolve looks up the drawing of each model the profile wants drawn. A hit is free; a drawing that was replaced is put back at no
// cost; a failed attempt is reported as failed and is not tried again until a person asks.
func (s *ReviewArtifactStore) Resolve(ctx context.Context, requestID string, models []domain.DiagramModel, profile domain.ResolvedProfile, skillVersion string) ([]ArtifactLookup, error) {
	var out []ArtifactLookup
	for _, m := range models {
		if !profile.Draws(m.Section, m.Variant) {
			continue
		}
		key, err := KeyFor(m, profile, skillVersion)
		if err != nil {
			return nil, err
		}
		l := ArtifactLookup{Model: m, Key: key, Result: domain.CacheMiss}
		found, ok, err := s.Repo.FindByKey(ctx, requestID, key)
		if err != nil {
			return nil, err
		}
		switch {
		case !ok:
			s.obs().ArtifactLookup("miss")
		case found.State == domain.ArtifactFailed:
			l.Result, l.Artifact = domain.CacheFailed, &found
			s.obs().ArtifactLookup("failed")
		case found.State == domain.ArtifactSuperseded:
			restored, aerr := s.Repo.Activate(ctx, requestID, found.ID)
			if aerr != nil {
				return nil, aerr
			}
			l.Result, l.Artifact = domain.CacheHit, &restored
			s.obs().ArtifactLookup("hit")
		default:
			l.Result, l.Artifact = domain.CacheHit, &found
			s.obs().ArtifactLookup("hit")
		}
		out = append(out, l)
	}
	return out, nil
}

// StoreInput is a checked drawing to keep.
type StoreInput struct {
	RequestID string
	Lookup    ArtifactLookup
	Content   []byte
	Info      domain.ArtifactInfo
	Origin    domain.ReviewArtifactOrigin
	Reason    string
	CreatedBy string
}

// Store keeps a drawing that passed the check and makes it the active one of its slot.
func (s *ReviewArtifactStore) Store(ctx context.Context, in StoreInput) (domain.ReviewArtifact, error) {
	used, err := s.Repo.StoredBytes(ctx)
	if err != nil {
		return domain.ReviewArtifact{}, err
	}
	if used+int64(len(in.Content)) > s.quota() {
		return domain.ReviewArtifact{}, domain.ErrReviewBudgetExceeded(fmt.Sprintf("this tenant already keeps %d bytes of drawings; the limit is %d", used, s.quota()))
	}
	model, err := compactModel(in.Lookup.Model)
	if err != nil {
		return domain.ReviewArtifact{}, err
	}
	blob := &domain.ReviewBlob{Digest: domain.BlobDigest(in.Content), MediaType: domain.ReviewArtifactMediaType, Content: in.Content}
	_, existing, _ := s.Repo.Blob(ctx, in.RequestID, blob.Digest)
	a := domain.ReviewArtifact{
		ID: uuid.NewString(), RequestID: in.RequestID, Key: in.Lookup.Key, Type: in.Lookup.Model.Type, State: domain.ArtifactActive, Origin: originOrAuto(in.Origin),
		Reason: in.Reason, BlobDigest: blob.Digest, SizeBytes: len(in.Content), ViewBox: in.Info.ViewBox, Title: in.Info.Title, Desc: in.Info.Desc, Model: model, CreatedBy: in.CreatedBy,
	}
	saved, err := s.Repo.Save(ctx, a, blob)
	if err != nil {
		return domain.ReviewArtifact{}, err
	}
	s.obs().ArtifactStored(len(in.Content), existing)
	return saved, nil
}

// StoreFailure keeps a refused attempt, without content, so the review can say why a drawing is missing and a repeat is not automatic.
func (s *ReviewArtifactStore) StoreFailure(ctx context.Context, requestID string, l ArtifactLookup, code, reason, createdBy string) error {
	model, err := compactModel(l.Model)
	if err != nil {
		return err
	}
	a := domain.ReviewArtifact{
		ID: uuid.NewString(), RequestID: requestID, Key: l.Key, Type: l.Model.Type, State: domain.ArtifactFailed, Origin: domain.ArtifactOriginAuto,
		ErrorCode: code, Reason: clipReason(reason), Model: model, CreatedBy: createdBy,
	}
	_, err = s.Repo.Save(ctx, a, nil)
	return err
}

func originOrAuto(o domain.ReviewArtifactOrigin) domain.ReviewArtifactOrigin {
	if o == "" {
		return domain.ArtifactOriginAuto
	}
	return o
}

func clipReason(s string) string {
	r := []rune(s)
	if len(r) > 200 {
		return string(r[:200])
	}
	return s
}

// compactModel is the model as it is kept with the drawing, within the size the table allows.
func compactModel(m domain.DiagramModel) ([]byte, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	if len(raw) > domain.MaxReviewArtifactModel {
		return nil, domain.ErrReviewArtifactTooLarge(len(raw), domain.MaxReviewArtifactModel)
	}
	return raw, nil
}

// ArtifactContent is a drawing and its content as a read returns them.
type ArtifactContent struct {
	Artifact    domain.ReviewArtifact
	Blob        domain.ReviewBlob
	NotModified bool
}

// Read returns a drawing and its content. The content is checked against its digest every time: a drawing that was changed after
// it was checked is refused, not served. With ifDigest equal to the digest the caller already holds, no content is returned.
func (s *ReviewArtifactStore) Read(ctx context.Context, requestID, id, ifDigest string) (ArtifactContent, error) {
	a, err := s.Repo.Get(ctx, requestID, id)
	if err != nil {
		return ArtifactContent{}, err
	}
	if a.State == domain.ArtifactFailed || a.BlobDigest == "" {
		return ArtifactContent{Artifact: a}, nil
	}
	if ifDigest != "" && ifDigest == a.BlobDigest {
		return ArtifactContent{Artifact: a, NotModified: true}, nil
	}
	blob, ok, err := s.Repo.Blob(ctx, requestID, a.BlobDigest)
	if err != nil {
		return ArtifactContent{}, err
	}
	if !ok {
		s.obs().ArtifactLookup("corrupt")
		return ArtifactContent{}, domain.ErrReviewArtifactNotFound(id)
	}
	if err := domain.VerifyBlob(blob); err != nil {
		s.obs().ArtifactLookup("corrupt")
		return ArtifactContent{}, err
	}
	return ArtifactContent{Artifact: a, Blob: blob}, nil
}

// Revisions lists every drawing of the slot of a model, newest first.
func (s *ReviewArtifactStore) Revisions(ctx context.Context, requestID string, section domain.ReviewSection, variant, name string) ([]domain.ReviewArtifact, error) {
	return s.Repo.ListRevisions(ctx, requestID, section, variant, name)
}

// Annotate sets the status of each diagram of a review from what the store holds, so a screen knows which drawings exist without
// asking for them. It reads only metadata. The skill version is left out of the question: a drawing made by another version of the
// skill still stands until a person asks for a new one.
func (s *ReviewArtifactStore) Annotate(ctx context.Context, requestID string, refs []domain.DiagramRef, profile domain.ResolvedProfile) ([]domain.DiagramRef, error) {
	if len(refs) == 0 {
		return refs, nil
	}
	out := make([]domain.DiagramRef, len(refs))
	copy(out, refs)
	for i, ref := range out {
		if !profile.Draws(ref.Section, ref.Variant) {
			out[i] = ref.WithStatus("disabled", "")
			continue
		}
		preset, err := profile.PresetDigest(ref.Section, ref.Variant)
		if err != nil {
			return nil, err
		}
		act, ok, err := s.Repo.Active(ctx, requestID, ref.Section, ref.Variant, ref.Name)
		if err != nil {
			return nil, err
		}
		switch {
		case ok && act.Key.InputDigest == ref.InputDigest && act.Key.PresetDigest == preset:
			out[i] = ref.WithStatus("cached", act.ID)
		default:
			out[i] = ref.WithStatus("missing", "")
		}
	}
	return out, nil
}
