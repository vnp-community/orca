package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ReindexJob represents a persisted code intelligence reindex execution record.
type ReindexJob struct {
	JobID         string     `json:"jobId"`
	TenantID      string     `json:"tenantId"`
	RepoBindingID string     `json:"repoBindingId"`
	ActiveKey     string     `json:"activeKey,omitempty"`
	Mode          string     `json:"mode"`
	Status        string     `json:"status"`
	Trigger       string     `json:"trigger"`
	Stage         string     `json:"stage"`
	Percent       *int32     `json:"percent,omitempty"`
	Message       string     `json:"message"`
	Outcome       string     `json:"outcome"`
	ErrorCode     string     `json:"errorCode"`
	AgentJobID    string     `json:"agentJobId"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// OutboxEvent captures transactional outbox messages (e.g. "reindex.started").
type OutboxEvent struct {
	EventID   string    `json:"eventId"`
	TenantID  string    `json:"tenantId"`
	EventType string    `json:"eventType"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"createdAt"`
}

// ReindexJobStore defines database persistence operations for Reindex jobs with active key exclusivity.
type ReindexJobStore interface {
	CreateActive(ctx context.Context, job ReindexJob, outbox OutboxEvent) error
	Get(ctx context.Context, tenantID, jobID string) (*ReindexJob, error)
	GetActiveByBinding(ctx context.Context, tenantID, repoBindingID string) (*ReindexJob, error)
	UpdateProgress(ctx context.Context, tenantID, jobID, stage string, percent *int32, message string) error
	Finish(ctx context.Context, tenantID, jobID, status, outcome, errorCode string, finishedAt time.Time) error
	UpdateAgentJobID(ctx context.Context, tenantID, jobID, agentJobID string) error
}

// ReindexAdmission checks cooldown and quota before admitting a reindex execution.
type ReindexAdmission interface {
	Admit(ctx context.Context, tenantID, repoBindingID string) error
}

// DefaultNoopAdmission allows all requests when no admission controller is configured.
type DefaultNoopAdmission struct{}

func (a *DefaultNoopAdmission) Admit(ctx context.Context, tenantID, repoBindingID string) error {
	return nil
}

// WorktreeTargetResolver resolves a WorktreeSelector into an AgentTarget and repository binding identifier.
type WorktreeTargetResolver interface {
	ResolveTarget(ctx context.Context, tenantID string, sel *codeintelv1.WorktreeSelector) (AgentTarget, string, error)
}

// ToProtoReindexJob converts domain ReindexJob to proto codeintelv1.ReindexJob.
func ToProtoReindexJob(j *ReindexJob) *codeintelv1.ReindexJob {
	if j == nil {
		return nil
	}
	var startedAt, finishedAt, createdAt *timestamppb.Timestamp
	if !j.StartedAt.IsZero() {
		startedAt = timestamppb.New(j.StartedAt)
	}
	if j.FinishedAt != nil && !j.FinishedAt.IsZero() {
		finishedAt = timestamppb.New(*j.FinishedAt)
	}
	if !j.CreatedAt.IsZero() {
		createdAt = timestamppb.New(j.CreatedAt)
	}

	return &codeintelv1.ReindexJob{
		JobId:         j.JobID,
		RepoBindingId: j.RepoBindingID,
		Mode:          j.Mode,
		Status:        j.Status,
		Trigger:       j.Trigger,
		Stage:         j.Stage,
		Percent:       j.Percent,
		Message:       j.Message,
		Outcome:       j.Outcome,
		ErrorCode:     j.ErrorCode,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		CreatedAt:     createdAt,
	}
}

// ErrActiveJobExists is returned when an active job already exists for the repository binding.
var ErrActiveJobExists = apperrors.New(apperrors.KindFailedPrecondition, "CODEINTEL_REINDEX_IN_PROGRESS", "reindex job is already active for this repository", nil)
