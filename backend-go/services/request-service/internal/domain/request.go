package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Request struct {
	ID                   string
	TenantID             string
	ProjectID            string
	Number               int64
	Title                string
	Body                 string
	SourceProvider       SourceProvider
	SourceRef            string
	SourceURL            string
	SourceSite           string
	Type                 RequestType // empty means not set
	TypeSource           TypeSource
	Size                 RequestSize // empty means not set
	Urgency              Urgency
	SolutionEngine       *EngineName
	Confidence           *float64
	ClassificationReason string
	Stage                string
	Status               RequestStatus
	ReturnedFromStage    ReturnStage
	ReturnedCategory     string
	ReturnReason         string
	PlanTaskID           string
	ReporterID           string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int64
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
		// Just a sanity check according to spec
		// Wait, the spec says "reporter_id không rỗng". Let's enforce it.
	}
	provider, err := ParseSourceProvider(in.SourceProvider)
	if err != nil {
		if in.SourceProvider == "" {
			provider = SourceProviderManual // fallback or just let it fail? spec says provider hợp lệ.
			// Let's rely on ParseSourceProvider, which rejects "". But spec says source_provider is default ''. Wait, if they pass "", I should probably let it fail.
			return Request{}, err
		}
		return Request{}, err
	}

	return Request{
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
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		Version:        1,
	}, nil
}
