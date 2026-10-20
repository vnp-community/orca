package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Limits of a stored drawing. The agent returns every drawing of a run inside one result block of at most 256 KiB, so a
// drawing has to be a fraction of that; a run draws at most MaxDrawingsPerRun diagrams for the same reason.
const (
	MaxReviewArtifactBytes   = 56 * 1024
	MaxDrawingsPerRun        = 4
	MaxReviewArtifactModel   = 32 * 1024
	ReviewArtifactIDPrefix   = "rv-"
	DefaultArtifactQuotaMB   = 512
	ReviewArtifactMediaType  = "text/html"
	ReviewArtifactSchemaName = "outcome-review-draw/1"
)

type ReviewArtifactState string

const (
	ArtifactActive     ReviewArtifactState = "active"
	ArtifactSuperseded ReviewArtifactState = "superseded"
	ArtifactFailed     ReviewArtifactState = "failed"
)

func (s ReviewArtifactState) Valid() bool {
	return s == ArtifactActive || s == ArtifactSuperseded || s == ArtifactFailed
}

type ReviewArtifactOrigin string

const (
	ArtifactOriginAuto        ReviewArtifactOrigin = "auto"
	ArtifactOriginRegenerated ReviewArtifactOrigin = "regenerated"
)

// ArtifactKey is what makes two drawings the same drawing. The diagram model digest does not depend on the commit, the review or
// the time, so a drawing is paid for once and reused wherever the same model appears. PresetDigest is per section: changing the
// knobs of one section redraws only that section. SkillVersion is the version of the skill that drew it.
type ArtifactKey struct {
	Section      ReviewSection
	Variant      string
	Name         string
	InputDigest  string
	PresetDigest string
	SkillVersion string
}

func (k ArtifactKey) Valid() error {
	switch {
	case !k.Section.Drawn():
		return fmt.Errorf("section %q is not drawn", k.Section)
	case k.Variant != DiagramVariantDesign && k.Variant != DiagramVariantDelta:
		return fmt.Errorf("variant %q is not design or delta", k.Variant)
	case k.InputDigest == "" || k.PresetDigest == "":
		return fmt.Errorf("a drawing key needs the model digest and the preset digest")
	}
	return nil
}

// Slot is the place in a review the drawing fills: only one drawing of a slot is active at a time.
func (k ArtifactKey) Slot() string {
	return string(k.Section) + "|" + k.Variant + "|" + k.Name
}

// ReviewArtifact is the metadata of one stored drawing. The HTML is a blob shared by every row with the same digest.
type ReviewArtifact struct {
	ID         string
	TenantID   string
	RequestID  string
	Key        ArtifactKey
	Type       DiagramType
	Revision   int
	State      ReviewArtifactState
	Origin     ReviewArtifactOrigin
	Reason     string
	ErrorCode  string
	BlobDigest string
	SizeBytes  int
	ViewBox    string
	Title      string
	Desc       string
	SelfCheck  string
	Model      []byte
	CreatedBy  string
	CreatedAt  time.Time
}

// ReviewBlob is a drawing's content, stored once per tenant and digest.
type ReviewBlob struct {
	Digest    string
	MediaType string
	Content   []byte
	CreatedAt time.Time
}

// BlobDigest is the digest a drawing is stored and checked under.
func BlobDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// VerifyBlob is how a read refuses content that does not match its digest: the stored drawing was changed after it was checked.
func VerifyBlob(b ReviewBlob) error {
	if BlobDigest(b.Content) != b.Digest {
		return ErrReviewArtifactCorrupt(b.Digest)
	}
	return nil
}

// CacheStatus is what a lookup of a drawing found.
type CacheStatus string

const (
	CacheHit    CacheStatus = "cached"
	CacheMiss   CacheStatus = "missing"
	CacheFailed CacheStatus = "failed"
)
