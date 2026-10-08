package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ErrIssueNotFound tells a missing source issue (NotFound to the caller) apart from a failed fetch.
var ErrIssueNotFound = errors.New("source issue not found")

type IssueSnapshot struct {
	Title string
	Body  string
	URL   string
	Hints domain.SourceHints
}

// IssueFetcher reads one issue from the tracker named by provider (jira, linear only).
type IssueFetcher interface {
	GetIssue(ctx context.Context, provider domain.SourceProvider, ref, site string) (IssueSnapshot, error)
}

// RequestCreationRecorder runs inside the creation transaction. The content-revision
// feature (CR-REQ-027) plugs revision 1 in here; until then nothing is recorded.
type RequestCreationRecorder interface {
	RecordCreated(ctx context.Context, r domain.Request) error
}

type NoopRequestCreationRecorder struct{}

func (NoopRequestCreationRecorder) RecordCreated(context.Context, domain.Request) error { return nil }
