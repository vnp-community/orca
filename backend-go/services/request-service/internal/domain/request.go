package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Request struct {
	ID                     string
	TenantID               string
	ProjectID              string
	Number                 int64
	Title                  string
	Body                   string
	SourceProvider         SourceProvider
	SourceRef              string
	SourceURL              string
	SourceSite             string
	Type                   RequestType // empty means not set
	TypeSource             TypeSource
	Size                   RequestSize // empty means not set
	Urgency                Urgency
	SolutionEngine         *EngineName
	Confidence             *float64
	ClassificationReason   string
	Stage                  string
	Status                 RequestStatus
	ReturnedFromStage      ReturnStage
	ReturnedCategory       ReturnCategory
	ReturnReason           string
	PlanTaskID             string
	ReporterID             string
	SourceHints            SourceHints
	ClassificationAttempts int // AI classification calls, failures included; caps cost
	// Content columns (CR-REQ-027): written only by the append-revision use case after creation.
	ContentSchemaVersion   int
	ContentRevision        int
	AcceptanceCriteriaJSON []byte
	TypeFieldsJSON         []byte
	ContentDigest          string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Version                int64
}

func (r Request) HasType() bool {
	return r.Type != ""
}

type NewRequestInput struct {
	TenantID       string
	ProjectID      string
	Title          string
	Body           string
	SourceProvider string
	SourceRef      string
	SourceURL      string
	SourceSite     string
	ReporterID     string
	SourceHints    SourceHints
}

func NewRequest(in NewRequestInput) (Request, error) {
	if in.TenantID == "" {
		return Request{}, ErrRequestTenantRequired()
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Request{}, ErrRequestTitleRequired()
	}
	if in.ReporterID == "" {
		return Request{}, ErrRequestReporterRequired()
	}
	provider, err := ParseSourceProvider(in.SourceProvider)
	if err != nil {
		return Request{}, err
	}

	now := time.Now().UTC().Truncate(time.Microsecond) // DB columns keep microseconds; round-trips must compare equal
	r := Request{
		ID:             uuid.NewString(),
		TenantID:       in.TenantID,
		ProjectID:      in.ProjectID,
		Title:          title,
		Body:           in.Body,
		SourceProvider: provider,
		SourceRef:      in.SourceRef,
		SourceURL:      in.SourceURL,
		SourceSite:     in.SourceSite,
		Status:         RequestStatusNew,
		Urgency:        UrgencyNormal,
		ReporterID:     in.ReporterID,
		SourceHints:    in.SourceHints,
		CreatedAt:      now,
		UpdatedAt:      now,
		Version:        1,

		ContentSchemaVersion:   LatestSchemaVersion(ArtifactKindRequest),
		ContentRevision:        1,
		AcceptanceCriteriaJSON: []byte("[]"),
		TypeFieldsJSON:         []byte("{}"),
	}
	c, err := ContentFromRequest(r)
	if err != nil {
		return Request{}, err
	}
	if r.ContentDigest, err = c.Digest(); err != nil {
		return Request{}, err
	}
	return r, nil
}
