//go:build integration

// TASK-E2E-017: an unknown or malformed account id answers ErrAccountNotFound, never a raw
// Postgres error that the usecase would turn into Internal.
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

func TestRepository_UnknownOrMalformedAccountID_IsNotFound(t *testing.T) {
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

	// A scope ref that cannot be a uuid owns no accounts: empty, not an error.
	accounts, err := repo.List(ctx, usecase.ListAccountsFilter{TenantID: tenantID, Scope: domain.ScopeUser, ScopeRefID: "abc"})
	if err != nil || len(accounts) != 0 {
		t.Errorf("List(malformed scope ref) = %v, %v; want empty, nil", accounts, err)
	}
}

func assertNotFound(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("%s: want ErrAccountNotFound, got %v", what, err)
	}
}
