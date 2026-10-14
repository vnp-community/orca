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

func fullChange(requestID string) domain.OpenSpecChange {
	return domain.OpenSpecChange{
		ID: uuid.NewString(), RequestID: requestID, ChangeID: "req-42-add-ai", Branch: "request/42-proposal", Status: domain.ChangeStatusReady,
		SyncState: domain.SyncStatePending, Commit: "abc1234", TasksMDDigest: "digest-1", KGAvailable: true, KGSnapshot: "snap-000001",
		KGReadIDs: []string{"FR-SYS-07", "ADR-12"}, KGGaps: []string{"no quota entity"}, KGConflicts: []string{"ADR-12 vs AC-AI-05"},
		SkillHash: "skills-1", Bundle: map[string]string{"tasks.md": "# t\n", "specs/ai/spec.md": "### Requirement: x\n"}, WorkspaceID: "wt-1",
	}
}

// RunOpenSpecChangeContract is what both dialects must do for the stored OpenSpec change of a Request.
func RunOpenSpecChangeContract(t *testing.T, env Env, repo usecase.OpenSpecChangeRepository) {
	t.Helper()
	ctx := CtxForTenant(newTenant())
	req := createRequest(t, env, ctx, nil)

	t.Run("a Request without a change reads as not found", func(t *testing.T) {
		if _, found, err := repo.GetByRequest(ctx, req.ID); err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	var first domain.OpenSpecChange
	t.Run("insert then read back every field", func(t *testing.T) {
		want := fullChange(req.ID)
		got, err := repo.Upsert(ctx, want)
		if err != nil {
			t.Fatal(err)
		}
		first = got
		if got.ID != want.ID || got.ChangeID != want.ChangeID || got.Branch != want.Branch || got.Status != want.Status || got.SyncState != want.SyncState ||
			got.Commit != want.Commit || got.TasksMDDigest != want.TasksMDDigest || !got.KGAvailable || got.KGSnapshot != want.KGSnapshot ||
			got.SkillHash != want.SkillHash || got.WorkspaceID != want.WorkspaceID || got.Version != 1 {
			t.Fatalf("scalar fields: %+v", got)
		}
		if len(got.KGReadIDs) != 2 || got.KGReadIDs[1] != "ADR-12" || len(got.KGGaps) != 1 || len(got.KGConflicts) != 1 {
			t.Fatalf("lists: %+v", got)
		}
		if len(got.Bundle) != 2 || got.Bundle["tasks.md"] != "# t\n" || got.BundleDigest() != want.BundleDigest() {
			t.Fatalf("bundle: %+v", got.Bundle)
		}
		read, found, err := repo.GetByRequest(ctx, req.ID)
		if err != nil || !found || read.ID != got.ID || read.BundleDigest() != got.BundleDigest() {
			t.Fatalf("read back: %+v found=%v %v", read, found, err)
		}
	})

	t.Run("a second upsert replaces the fields, keeps the row and bumps the version", func(t *testing.T) {
		next := fullChange(req.ID) // a fresh id: the existing row must win
		next.Commit, next.KGAvailable, next.KGReadIDs, next.Bundle = "def5678", false, nil, nil
		got, err := repo.Upsert(ctx, next)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != first.ID || got.Version != 2 || got.Commit != "def5678" || got.KGAvailable || len(got.KGReadIDs) != 0 || len(got.Bundle) != 0 {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("optional text columns come back empty, not as the string NULL", func(t *testing.T) {
		req2 := createRequest(t, env, ctx, nil)
		got, err := repo.Upsert(ctx, domain.OpenSpecChange{ID: uuid.NewString(), RequestID: req2.ID, ChangeID: "req-43-x", Status: domain.ChangeStatusPreparing, SyncState: domain.SyncStateInSync})
		if err != nil {
			t.Fatal(err)
		}
		if got.Branch != "" || got.Commit != "" || got.KGSnapshot != "" || got.SkillHash != "" || got.WorkspaceID != "" || got.TasksMDDigest != "" || got.Status != domain.ChangeStatusPreparing {
			t.Fatalf("%+v", got)
		}
	})

	t.Run("another tenant cannot read it", func(t *testing.T) {
		other := tenant.WithTenantID(ctx, newTenant())
		if _, found, err := repo.GetByRequest(other, req.ID); err != nil || found {
			t.Fatalf("found=%v err=%v", found, err)
		}
	})

	t.Run("UpdateSync is a compare-and-set on the version", func(t *testing.T) {
		cur, _, _ := repo.GetByRequest(ctx, req.ID)
		retry := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
		if err := repo.UpdateSync(ctx, cur.ID, domain.SyncStateFailed, "d2", &retry, cur.Version); err != nil {
			t.Fatal(err)
		}
		if err := repo.UpdateSync(ctx, cur.ID, domain.SyncStateInSync, "d3", nil, cur.Version); !errors.Is(err, domain.ErrOpenSpecChangeVersionConflict) {
			t.Fatalf("a stale version must conflict: %v", err)
		}
		if err := repo.UpdateSync(ctx, uuid.NewString(), domain.SyncStateInSync, "", nil, 1); !errors.Is(err, domain.ErrOpenSpecChangeNotFound) {
			t.Fatalf("an unknown id: %v", err)
		}
		after, _, _ := repo.GetByRequest(ctx, req.ID)
		if after.SyncState != domain.SyncStateFailed || after.Version != cur.Version+1 {
			t.Fatalf("%+v", after)
		}
	})

	t.Run("ListPendingSync returns only ready changes that are due", func(t *testing.T) {
		tctx := CtxForTenant(newTenant())
		mk := func(state domain.SyncState, status domain.ChangeStatus) domain.OpenSpecChange {
			r := createRequest(t, env, tctx, nil)
			c, err := repo.Upsert(tctx, domain.OpenSpecChange{ID: uuid.NewString(), RequestID: r.ID, ChangeID: "req-1-x", Status: status, SyncState: state})
			if err != nil {
				t.Fatal(err)
			}
			return c
		}
		pending := mk(domain.SyncStatePending, domain.ChangeStatusReady)
		inSync := mk(domain.SyncStateInSync, domain.ChangeStatusReady)
		archived := mk(domain.SyncStatePending, domain.ChangeStatusArchived)
		later := mk(domain.SyncStateFailed, domain.ChangeStatusReady)
		future := time.Now().UTC().Add(time.Hour)
		if err := repo.UpdateSync(tctx, later.ID, domain.SyncStateFailed, "x", &future, later.Version); err != nil {
			t.Fatal(err)
		}
		due := mk(domain.SyncStateFailed, domain.ChangeStatusReady)
		past := time.Now().UTC().Add(-time.Minute)
		if err := repo.UpdateSync(tctx, due.ID, domain.SyncStateFailed, "x", &past, due.Version); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ListPendingSync(tctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		for _, c := range got {
			ids[c.ID] = true
		}
		if !ids[pending.ID] || !ids[due.ID] || ids[inSync.ID] || ids[archived.ID] || ids[later.ID] {
			t.Fatalf("got %v; want pending %s and due %s only", ids, pending.ID, due.ID)
		}
		if limited, _ := repo.ListPendingSync(tctx, 1); len(limited) != 1 {
			t.Fatalf("limit not applied: %d", len(limited))
		}
	})
}
