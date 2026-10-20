package contracttest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RunEngineSettingsContract is what both dialects must do for the per-project engine choice: optimistic versions, no
// silent overwrite, and no read across tenants.
func RunEngineSettingsContract(t *testing.T, repo usecase.EngineSettingsRepository) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	tenantID, project, actor := newTenant(), uuid.NewString(), uuid.NewString()
	ctx := CtxForTenant(tenantID)
	mk := func(engine domain.EngineName, minVersion string) domain.ProjectEngineSettings {
		s, err := domain.NewProjectEngineSettings(tenantID, project, engine, minVersion, actor, now)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	t.Run("a project without a row reads as not found, not as an error", func(t *testing.T) {
		if _, found, err := repo.Get(ctx, project); err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})
	t.Run("insert then read back", func(t *testing.T) {
		got, err := repo.Upsert(ctx, mk(domain.EngineOpenSpec, "1.2.0"), 0)
		if err != nil || got.Version != 1 {
			t.Fatalf("insert: %+v %v", got, err)
		}
		read, found, err := repo.Get(ctx, project)
		if err != nil || !found || read.Engine != domain.EngineOpenSpec || read.MinVersion != "1.2.0" || read.Version != 1 || read.UpdatedBy != actor {
			t.Fatalf("read: %+v found=%v %v", read, found, err)
		}
	})
	t.Run("a second insert is a version conflict", func(t *testing.T) {
		if _, err := repo.Upsert(ctx, mk(domain.EngineNative, ""), 0); !errors.Is(err, domain.ErrEngineSettingsVersionConflict) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("update at the right version bumps it and may clear the minimum version", func(t *testing.T) {
		got, err := repo.Upsert(ctx, mk(domain.EngineNative, ""), 1)
		if err != nil || got.Version != 2 {
			t.Fatalf("update: %+v %v", got, err)
		}
		read, _, _ := repo.Get(ctx, project)
		if read.Engine != domain.EngineNative || read.MinVersion != "" || read.Version != 2 {
			t.Fatalf("read: %+v", read)
		}
	})
	t.Run("the solution-agent switch is stored, read back and can be turned off again", func(t *testing.T) {
		on := mk(domain.EngineNative, "")
		on.SolutionAgent = true
		if _, err := repo.Upsert(ctx, on, 2); err != nil {
			t.Fatal(err)
		}
		if read, _, _ := repo.Get(ctx, project); !read.SolutionAgent || read.Version != 3 {
			t.Fatalf("read: %+v", read)
		}
		if _, err := repo.Upsert(ctx, mk(domain.EngineNative, ""), 3); err != nil {
			t.Fatal(err)
		}
		if read, _, _ := repo.Get(ctx, project); read.SolutionAgent || read.Version != 4 {
			t.Fatalf("read: %+v", read)
		}
	})
	t.Run("the execution mode defaults to step_by_step and is stored, read back and changed", func(t *testing.T) {
		if read, _, _ := repo.Get(ctx, project); read.ExecutionMode != domain.ExecutionStepByStep {
			t.Fatalf("a row written without a mode reads as step_by_step: %q", read.ExecutionMode)
		}
		continuous := mk(domain.EngineNative, "")
		continuous.ExecutionMode = domain.ExecutionContinuous
		if got, err := repo.Upsert(ctx, continuous, 4); err != nil || got.Version != 5 {
			t.Fatalf("set continuous: %+v %v", got, err)
		}
		if read, _, _ := repo.Get(ctx, project); read.ExecutionMode != domain.ExecutionContinuous || read.Version != 5 {
			t.Fatalf("read: %+v", read)
		}
		if _, err := repo.Upsert(ctx, mk(domain.EngineNative, ""), 5); err != nil {
			t.Fatal(err)
		}
		if read, _, _ := repo.Get(ctx, project); read.ExecutionMode != domain.ExecutionStepByStep || read.Version != 6 {
			t.Fatalf("back to step_by_step: %+v", read)
		}
	})
	t.Run("automatic Plan generation is on by default and is stored, read back and turned off again", func(t *testing.T) {
		if read, _, _ := repo.Get(ctx, project); !read.PlanAutoGenerate || read.Version != 6 {
			t.Fatalf("a project that never chose reads as on: %+v", read)
		}
		off := mk(domain.EngineNative, "")
		off.PlanAutoGenerate = false
		if got, err := repo.Upsert(ctx, off, 6); err != nil || got.Version != 7 {
			t.Fatalf("turn off: %+v %v", got, err)
		}
		if read, _, _ := repo.Get(ctx, project); read.PlanAutoGenerate || read.Version != 7 {
			t.Fatalf("read: %+v", read)
		}
		if got, err := repo.Upsert(ctx, mk(domain.EngineNative, ""), 7); err != nil || got.Version != 8 {
			t.Fatalf("turn on: %+v %v", got, err)
		}
		if read, _, _ := repo.Get(ctx, project); !read.PlanAutoGenerate || read.Version != 8 {
			t.Fatalf("back on: %+v", read)
		}
	})
	t.Run("the outcome review choice is empty by default and is stored, read back and cleared", func(t *testing.T) {
		if read, _, _ := repo.Get(ctx, project); read.OutcomeReview != "" || read.Version != 8 {
			t.Fatalf("a project that never chose has no review choice: %+v", read)
		}
		manual := mk(domain.EngineNative, "")
		manual.OutcomeReview = domain.OutcomeReviewManual
		if got, err := repo.Upsert(ctx, manual, 8); err != nil || got.Version != 9 {
			t.Fatalf("choose manual: %+v %v", got, err)
		}
		if read, _, _ := repo.Get(ctx, project); read.OutcomeReview != domain.OutcomeReviewManual || read.Version != 9 {
			t.Fatalf("read: %+v", read)
		}
		off := mk(domain.EngineNative, "")
		off.OutcomeReview = domain.OutcomeReviewOff
		if _, err := repo.Upsert(ctx, off, 9); err != nil {
			t.Fatal(err)
		}
		if read, _, _ := repo.Get(ctx, project); read.OutcomeReview != domain.OutcomeReviewOff {
			t.Fatalf("an explicit off is stored and differs from no choice: %+v", read)
		}
		if got, err := repo.Upsert(ctx, mk(domain.EngineNative, ""), 10); err != nil || got.Version != 11 {
			t.Fatalf("clear the choice: %+v %v", got, err)
		}
		if read, _, _ := repo.Get(ctx, project); read.OutcomeReview != "" || read.Version != 11 {
			t.Fatalf("cleared: %+v", read)
		}
	})
	t.Run("update at a stale version never overwrites", func(t *testing.T) {
		if _, err := repo.Upsert(ctx, mk(domain.EngineOpenSpec, ""), 1); !errors.Is(err, domain.ErrEngineSettingsVersionConflict) {
			t.Fatalf("got %v", err)
		}
		if read, _, _ := repo.Get(ctx, project); read.Engine != domain.EngineNative {
			t.Fatalf("a stale write changed the row: %+v", read)
		}
	})
	t.Run("another tenant cannot read the row", func(t *testing.T) {
		other := tenant.WithTenantID(ctx, newTenant())
		if _, found, err := repo.Get(other, project); err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})
}
