package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// designLockPayload is what the design lock stores. The prompt version and the contract version are inside it, so a lock made
// with other wording is not used.
type designLockPayload struct {
	PromptVersion string             `json:"promptVersion"`
	Schema        int                `json:"schema"`
	Design        domain.DesignModel `json:"design"`
}

// LockedDesign is the design extracted earlier for the Request and its digest.
type LockedDesign struct {
	Design domain.DesignModel
	Digest string
}

// ReviewDesignLock keeps one extracted design per Request, so the delivered phase and a later run read the knowledge graph and the
// SAD once. It does not probe the graph: a changed snapshot is noticed when a person regenerates, not before.
type ReviewDesignLock struct {
	Store ReviewContextStore
	TTL   time.Duration
	Now   func() time.Time
}

func (l ReviewDesignLock) now() time.Time {
	if l.Now != nil {
		return l.Now().UTC()
	}
	return time.Now().UTC()
}

// Load returns the newest lock that has not expired and was made with the current prompt and contract.
func (l ReviewDesignLock) Load(ctx context.Context, requestID string) (LockedDesign, bool, error) {
	rec, ok, err := l.Store.Latest(ctx, requestID, ReviewContextDesign, l.now())
	if err != nil || !ok {
		return LockedDesign{}, false, err
	}
	var p designLockPayload
	if err := json.Unmarshal(rec.Payload, &p); err != nil || p.PromptVersion != ReviewContextPromptVersion || p.Schema != domain.ContextSchemaVersion || !p.Design.Available {
		return LockedDesign{}, false, nil // a lock that cannot be read or was made with other wording is no lock
	}
	return LockedDesign{Design: p.Design, Digest: rec.Digest}, true, nil
}

// Save stores the design under the digest of its snapshot and documents.
func (l ReviewDesignLock) Save(ctx context.Context, requestID string, d domain.DesignModel) (LockedDesign, error) {
	digest, err := domain.DesignDigest(d.Snapshot, d.DocSha256, ReviewContextPromptVersion)
	if err != nil {
		return LockedDesign{}, err
	}
	raw, err := json.Marshal(designLockPayload{PromptVersion: ReviewContextPromptVersion, Schema: domain.ContextSchemaVersion, Design: d})
	if err != nil {
		return LockedDesign{}, err
	}
	if _, err := l.Store.Put(ctx, ReviewContextRecord{
		ID: uuid.NewString(), RequestID: requestID, Kind: ReviewContextDesign, Digest: digest, PromptVersion: ReviewContextPromptVersion,
		Payload: raw, ExpiresAt: l.now().Add(l.TTL),
	}); err != nil {
		return LockedDesign{}, err
	}
	return LockedDesign{Design: d, Digest: digest}, nil
}
