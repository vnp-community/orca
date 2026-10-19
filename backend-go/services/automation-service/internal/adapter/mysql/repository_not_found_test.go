//go:build integration

package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// TASK-E2E-017: repo trả sentinel miền (errors.Is) khi không có hàng. Cột id
// là CHAR nên id sai định dạng cũng chỉ là "không khớp hàng".
func TestAutomationRepository_NotFoundReturnsDomainSentinel(t *testing.T) {
	automations, runs := setupRepositories(t)
	ctx := context.Background()
	tenant := "11111111-1111-1111-1111-111111111111"
	missing := "00000000-0000-0000-0000-00000000dead"

	a := newAutomation(t, missing, tenant)
	for _, id := range []string{missing, "abc"} {
		if _, err := automations.Get(ctx, tenant, id); !errors.Is(err, domain.ErrAutomationNotFound) {
			t.Errorf("Get(%q): want ErrAutomationNotFound, got %v", id, err)
		}
		if err := automations.Delete(ctx, tenant, id); !errors.Is(err, domain.ErrAutomationNotFound) {
			t.Errorf("Delete(%q): want ErrAutomationNotFound, got %v", id, err)
		}
		a.ID = id
		if err := automations.Update(ctx, tenant, a); !errors.Is(err, domain.ErrAutomationNotFound) {
			t.Errorf("Update(%q): want ErrAutomationNotFound, got %v", id, err)
		}
	}

	now := time.Now().UTC().Truncate(time.Second)
	run, err := domain.NewPendingRun(missing, missing, tenant, "req-nf", domain.StepTypeAgent, domain.RunTriggerManual, `{"prompt":"x"}`, now)
	if err != nil {
		t.Fatalf("building run: %v", err)
	}
	if err := runs.UpdateStatus(ctx, run); !errors.Is(err, domain.ErrAutomationRunNotFound) {
		t.Errorf("UpdateStatus: want ErrAutomationRunNotFound, got %v", err)
	}
}
