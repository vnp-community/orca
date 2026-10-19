//go:build integration

// TASK-E2E-017: ids that name nothing answer with the usecase not-found sentinels, never a raw MySQL error.
package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"
)

func TestRepository_UnknownOrMalformedID_ReturnsNotFoundSentinel(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	for _, id := range []string{uuid.NewString(), "abc"} {
		check := func(what string, err, want error) {
			t.Helper()
			if !errors.Is(err, want) {
				t.Errorf("%s(%q): want %v, got %v", what, id, want, err)
			}
		}
		_, err := repo.Get(ctx, tenantID, id)
		check("Get", err, usecase.ErrTaskNotFound)
		_, err = repo.UpdateStatusAndPromote(ctx, tenantID, id, domain.TaskStatusReady, domain.OutboxEvent{})
		check("UpdateStatusAndPromote", err, usecase.ErrTaskNotFound)
		_, err = repo.GetLatestForTask(ctx, tenantID, id)
		check("GetLatestForTask", err, usecase.ErrDispatchContextNotFound)
		_, err = repo.GetDispatchContext(ctx, tenantID, id)
		check("GetDispatchContext", err, usecase.ErrDispatchContextNotFound)
		_, err = repo.RecordDispatchFailure(ctx, tenantID, id, "boom")
		check("RecordDispatchFailure", err, usecase.ErrDispatchContextNotFound)
		_, err = repo.RecordHeartbeat(ctx, tenantID, id)
		check("RecordHeartbeat", err, usecase.ErrDispatchContextNotFound)
		_, err = repo.CreateGate(ctx, tenantID, id, "q?", nil, domain.OutboxEvent{})
		check("CreateGate", err, usecase.ErrDispatchContextNotFound)
		_, _, err = repo.ResolveGate(ctx, tenantID, id, "yes")
		check("ResolveGate", err, usecase.ErrGateNotFound)
		_, err = repo.GetRun(ctx, tenantID, id)
		check("GetRun", err, usecase.ErrRunNotFound)
		_, err = repo.Complete(ctx, tenantID, id, nil)
		check("Complete", err, usecase.ErrRunNotFound)
		_, err = repo.Fail(ctx, tenantID, id, "boom")
		check("Fail", err, usecase.ErrRunNotFound)

		_, err = repo.CreateDispatchContext(ctx, tenantID, "", "", "h", id, "", domain.OutboxEvent{})
		check("CreateDispatchContext(run)", err, usecase.ErrRunNotFound)
	}

	runID := uuid.NewString()
	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-nf")
	for _, id := range []string{uuid.NewString(), "abc"} {
		_, err := repo.CreateDispatchContext(ctx, tenantID, "", "", "h", runID, id, domain.OutboxEvent{})
		if !errors.Is(err, usecase.ErrTaskNotFound) {
			t.Errorf("CreateDispatchContext(task %q): want ErrTaskNotFound, got %v", id, err)
		}
	}
}
