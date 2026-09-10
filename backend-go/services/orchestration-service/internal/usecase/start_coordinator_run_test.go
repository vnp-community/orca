package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

func TestStartCoordinatorRun_HappyPath(t *testing.T) {
	repo := newFakeCoordinatorRunRepository()
	uc := NewStartCoordinatorRun(repo, &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")

	specJSON := []byte(`[{"tempId":"a","title":"Root","spec":{},"deps":[]}]`)
	run, err := uc.Execute(ctx, StartCoordinatorRunInput{OriginTaskID: "origin-1", SpecJSON: specJSON})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != domain.RunStatusRunning {
		t.Errorf("expected RunStatusRunning, got %s", run.Status)
	}
	if run.ID == "" {
		t.Error("expected a minted run id")
	}
}

func TestStartCoordinatorRun_InvalidSpecFailsFastWithoutCreateWithTasks(t *testing.T) {
	cases := []struct {
		name     string
		specJSON []byte
	}{
		{"empty spec", []byte(`[]`)},
		{"duplicate tempId", []byte(`[{"tempId":"a","title":"1","spec":{},"deps":[]},{"tempId":"a","title":"2","spec":{},"deps":[]}]`)},
		{"dangling dep", []byte(`[{"tempId":"a","title":"1","spec":{},"deps":["missing"]}]`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeCoordinatorRunRepository()
			createCalled := false
			repo.createWithTasks = func(tenantID string, run domain.CoordinatorRun) (domain.CoordinatorRun, error) {
				createCalled = true
				return run, nil
			}
			uc := NewStartCoordinatorRun(repo, &synchronousSerializer{})
			ctx := withTenant(context.Background(), "tenant-1")

			_, err := uc.Execute(ctx, StartCoordinatorRunInput{OriginTaskID: "origin-1", SpecJSON: tc.specJSON})
			if err == nil {
				t.Fatal("expected an error for invalid spec_json")
			}
			if createCalled {
				t.Error("expected CreateWithTasks NOT to be called for an invalid spec (fail fast)")
			}
		})
	}
}

func TestStartCoordinatorRun_RequiresTenantContext(t *testing.T) {
	uc := NewStartCoordinatorRun(newFakeCoordinatorRunRepository(), &synchronousSerializer{})
	_, err := uc.Execute(context.Background(), StartCoordinatorRunInput{OriginTaskID: "origin-1", SpecJSON: []byte(`[{"tempId":"a","title":"1","spec":{},"deps":[]}]`)})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestStartCoordinatorRun_RequiresOriginTaskID(t *testing.T) {
	uc := NewStartCoordinatorRun(newFakeCoordinatorRunRepository(), &synchronousSerializer{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, StartCoordinatorRunInput{OriginTaskID: "", SpecJSON: []byte(`[{"tempId":"a","title":"1","spec":{},"deps":[]}]`)})
	if err == nil {
		t.Fatal("expected an error for empty origin_task_id")
	}
}
