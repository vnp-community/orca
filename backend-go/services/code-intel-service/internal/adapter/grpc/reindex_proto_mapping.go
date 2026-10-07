package grpc

import (
	"time"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ToProtoReindexJob converts usecase.ReindexJob to proto codeintelv1.ReindexJob.
func ToProtoReindexJob(j *usecase.ReindexJob) *codeintelv1.ReindexJob {
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

// FromProtoReindexJob converts proto codeintelv1.ReindexJob to usecase.ReindexJob.
func FromProtoReindexJob(p *codeintelv1.ReindexJob) *usecase.ReindexJob {
	if p == nil {
		return nil
	}
	var startedAt, createdAt time.Time
	var finishedAt *time.Time
	if p.StartedAt != nil {
		startedAt = p.StartedAt.AsTime()
	}
	if p.FinishedAt != nil {
		t := p.FinishedAt.AsTime()
		finishedAt = &t
	}
	if p.CreatedAt != nil {
		createdAt = p.CreatedAt.AsTime()
	}

	return &usecase.ReindexJob{
		JobID:         p.JobId,
		RepoBindingID: p.RepoBindingId,
		Mode:          p.Mode,
		Status:        p.Status,
		Trigger:       p.Trigger,
		Stage:         p.Stage,
		Percent:       p.Percent,
		Message:       p.Message,
		Outcome:       p.Outcome,
		ErrorCode:     p.ErrorCode,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		CreatedAt:     createdAt,
	}
}
