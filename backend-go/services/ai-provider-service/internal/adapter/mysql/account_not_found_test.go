//go:build integration

// TASK-E2E-017: an unknown account id answers ErrAccountNotFound on every dialect.
package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

func TestRepository_UnknownAccountID_IsNotFound(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	for _, id := range []string{uuid.NewString(), "abc"} {
		_, err := repo.Get(ctx, tenantID, id)
		assertNotFound(t, "Get "+id, err)
		_, err = repo.UpdateStatus(ctx, usecase.UpdateStatusInput{TenantID: tenantID, AccountID: id, Status: domain.AccountStatusActive})
		assertNotFound(t, "UpdateStatus "+id, err)
		_, err = repo.Update(ctx, usecase.UpdateFields{TenantID: tenantID, AccountID: id, Label: "x"})
		assertNotFound(t, "Update "+id, err)
		assertNotFound(t, "Delete "+id, repo.Delete(ctx, tenantID, id))

		state, err := repo.GetToday(ctx, tenantID, id, domain.DayKey(time.Now()))
		if err != nil || state.TokensUsed != 0 {
			t.Errorf("GetToday(%q) = %+v, %v; want zero usage, nil", id, state, err)
		}
	}
}

func assertNotFound(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("%s: want ErrAccountNotFound, got %v", what, err)
	}
}
