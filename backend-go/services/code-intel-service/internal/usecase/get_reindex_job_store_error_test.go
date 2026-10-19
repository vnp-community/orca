package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

type failingReindexJobStore struct {
	*inMemoryReindexJobStore
	err error
}

func (s failingReindexJobStore) Get(context.Context, string, string) (*ReindexJob, error) {
	return nil, s.err
}

// TASK-E2E-017: a store outage is Internal; only a genuinely missing job is NotFound.
func TestGetReindexJob_StoreFailureIsInternalNotNotFound(t *testing.T) {
	resolver := &staticResolver{target: AgentTarget{WorkspaceRoot: "/workspace"}, binding: "binding-1"}
	req := &codeintelv1.GetReindexJobRequest{Selector: &codeintelv1.WorktreeSelector{ProjectId: "p1"}, JobId: "job-1"}

	uc := NewGetReindexJobUseCase(resolver, failingReindexJobStore{newInMemoryReindexJobStore(), errors.New("connection refused")}, nil, 0)
	_, err := uc.Execute(context.Background(), "tenant-A", req)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != apperrors.KindInternal {
		t.Fatalf("store failure: want Internal AppError, got %v", err)
	}

	uc = NewGetReindexJobUseCase(resolver, newInMemoryReindexJobStore(), nil, 0)
	_, err = uc.Execute(context.Background(), "tenant-A", req)
	if !errors.As(err, &appErr) || appErr.Kind != apperrors.KindNotFound || appErr.Code != "CODEINTEL_NOT_FOUND" {
		t.Fatalf("unknown job: want NotFound CODEINTEL_NOT_FOUND, got %v", err)
	}
}
