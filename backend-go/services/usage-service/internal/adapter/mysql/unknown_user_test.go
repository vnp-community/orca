//go:build integration

// TASK-E2E-017: a user id that owns nothing (unknown, or not even a uuid) reads as empty usage, not an error.
package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/usage-service/internal/domain"
)

func TestRepository_UnknownOrMalformedUserID_ReadsEmpty(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	for _, userID := range []string{uuid.NewString(), "abc"} {
		sessions, next, err := repo.ListSessions(ctx, tenantID, userID, "", 10)
		if err != nil || len(sessions) != 0 || next != "" {
			t.Errorf("ListSessions(%q) = %v, %q, %v; want empty, \"\", nil", userID, sessions, next, err)
		}
		rollup, err := repo.GetDailyRollup(ctx, tenantID, userID, domain.ProviderClaude, time.Now().UTC().Truncate(24*time.Hour))
		if err != nil || rollup.SessionCount != 0 || rollup.TotalInputTokens != 0 {
			t.Errorf("GetDailyRollup(%q) = %+v, %v; want zeros, nil", userID, rollup, err)
		}
	}
}
