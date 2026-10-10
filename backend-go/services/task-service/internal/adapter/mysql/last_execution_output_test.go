//go:build integration

package mysql

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// Agent stdout is arbitrary bytes: a NUL, a bad byte or a cut inside a multi-byte character used to make the whole
// UPDATE fail (the caller ignores the error), so a long non-ASCII run left no output to show.
func TestRepository_UpdateLastExecutionOutput_StoresHostileStdout(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	task, err := repo.Create(ctx, domain.Task{ID: uuid.NewString(), TenantID: tenantID, Title: "t", Status: domain.StatusOpen})
	if err != nil {
		t.Fatal(err)
	}

	out := strings.Repeat("ế", 5000) + "a\x00b\xff"
	if err := repo.UpdateLastExecutionOutput(ctx, tenantID, task.ID, out); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := repo.Get(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastExecutionOutput == "" || len(got.LastExecutionOutput) > maxLastExecutionOutputBytes || !utf8.ValidString(got.LastExecutionOutput) {
		t.Fatalf("stored %d bytes, valid=%v", len(got.LastExecutionOutput), utf8.ValidString(got.LastExecutionOutput))
	}
}
