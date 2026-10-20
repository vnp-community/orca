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
		SkillHash: "skills-1", RedactedSecrets: 2, Bundle: map[string]string{"tasks.md": "# t\n", "specs/ai/spec.md": "### Requirement: x\n"}, WorkspaceID: "wt-1",
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
			got.SkillHash != want.SkillHash || got.WorkspaceID != want.WorkspaceID || got.RedactedSecrets != 2 || got.Version != 1 {
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
		next.Commit, next.KGAvailable, next.KGReadIDs, next.Bundle, next.RedactedSecrets = "def5678", false, nil, nil, 0
		got, err := repo.Upsert(ctx, next)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != first.ID || got.Version != 2 || got.Commit != "def5678" || got.KGAvailable || len(got.KGReadIDs) != 0 || len(got.Bundle) != 0 || got.RedactedSecrets != 0 {
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

	t.Run("UpdateSync is a compare-and-set on the version and stores the whole sync progress", func(t *testing.T) {
		cur, _, _ := repo.GetByRequest(ctx, req.ID)
		retry := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
		up := domain.SyncUpdate{State: domain.SyncStatePending, Digest: "d2", Error: "archive exited 1", Attempts: 2, NextRetryAt: &retry}
		if err := repo.UpdateSync(ctx, cur.ID, up, cur.Version); err != nil {
			t.Fatal(err)
		}
		if err := repo.UpdateSync(ctx, cur.ID, domain.SyncUpdate{State: domain.SyncStateInSync, Digest: "d3"}, cur.Version); !errors.Is(err, domain.ErrOpenSpecChangeVersionConflict) {
			t.Fatalf("a stale version must conflict: %v", err)
		}
		if err := repo.UpdateSync(ctx, uuid.NewString(), domain.SyncUpdate{State: domain.SyncStateInSync}, 1); !errors.Is(err, domain.ErrOpenSpecChangeNotFound) {
			t.Fatalf("an unknown id: %v", err)
		}
		after, _, _ := repo.GetByRequest(ctx, req.ID)
		if after.SyncState != domain.SyncStatePending || after.Version != cur.Version+1 || after.SyncDigest != "d2" || after.SyncError != "archive exited 1" ||
			after.SyncAttempts != 2 || after.SyncNextRetryAt == nil || !after.SyncNextRetryAt.Equal(retry) {
			t.Fatalf("%+v", after)
		}
		// Clearing the progress reads back as empty, not as NULL text.
		if err := repo.UpdateSync(ctx, after.ID, domain.SyncUpdate{State: domain.SyncStateInSync}, after.Version); err != nil {
			t.Fatal(err)
		}
		clear, _, _ := repo.GetByRequest(ctx, req.ID)
		if clear.SyncDigest != "" || clear.SyncError != "" || clear.SyncAttempts != 0 || clear.SyncNextRetryAt != nil {
			t.Fatalf("%+v", clear)
		}
	})

	t.Run("an upsert carries the sync progress it was given", func(t *testing.T) {
		cur, _, _ := repo.GetByRequest(ctx, req.ID)
		cur.SyncDigest, cur.SyncAttempts = "dd", 3
		got, err := repo.Upsert(ctx, cur)
		if err != nil || got.SyncDigest != "dd" || got.SyncAttempts != 3 {
			t.Fatalf("%+v %v", got, err)
		}
	})

	t.Run("ListSyncWork returns the ready changes that are due, and the completed Requests waiting to be archived", func(t *testing.T) {
		tctx := CtxForTenant(newTenant())
		mkFor := func(state domain.SyncState, status domain.ChangeStatus, reqStatus domain.RequestStatus) (domain.OpenSpecChange, domain.Request) {
			r := createRequest(t, env, tctx, func(r *domain.Request) { r.Status = reqStatus })
			c, err := repo.Upsert(tctx, domain.OpenSpecChange{ID: uuid.NewString(), RequestID: r.ID, ChangeID: "req-1-x", Status: status, SyncState: state})
			if err != nil {
				t.Fatal(err)
			}
			return c, r
		}
		mk := func(state domain.SyncState, status domain.ChangeStatus) (domain.OpenSpecChange, domain.Request) {
			return mkFor(state, status, domain.RequestStatusExecuting)
		}
		toArchive, _ := mkFor(domain.SyncStateInSync, domain.ChangeStatusReady, domain.RequestStatusCompleted)
		completedGaveUp, _ := mkFor(domain.SyncStateFailed, domain.ChangeStatusReady, domain.RequestStatusCompleted)
		pending, _ := mk(domain.SyncStatePending, domain.ChangeStatusReady)
		inSync, _ := mk(domain.SyncStateInSync, domain.ChangeStatusReady)
		archived, _ := mk(domain.SyncStatePending, domain.ChangeStatusArchived)
		gaveUp, _ := mk(domain.SyncStateFailed, domain.ChangeStatusReady)
		later, _ := mk(domain.SyncStatePending, domain.ChangeStatusReady)
		future := time.Now().UTC().Add(time.Hour)
		if err := repo.UpdateSync(tctx, later.ID, domain.SyncUpdate{State: domain.SyncStatePending, Attempts: 1, NextRetryAt: &future}, later.Version); err != nil {
			t.Fatal(err)
		}
		due, _ := mk(domain.SyncStatePending, domain.ChangeStatusReady)
		past := time.Now().UTC().Add(-time.Minute)
		if err := repo.UpdateSync(tctx, due.ID, domain.SyncUpdate{State: domain.SyncStatePending, Attempts: 1, NextRetryAt: &past}, due.Version); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ListSyncWork(tctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		for _, c := range got {
			ids[c.ID] = true
		}
		if !ids[pending.ID] || !ids[due.ID] || !ids[toArchive.ID] || ids[completedGaveUp.ID] || ids[inSync.ID] || ids[archived.ID] || ids[later.ID] || ids[gaveUp.ID] {
			t.Fatalf("got %v; want pending %s, due %s and the completed one %s only", ids, pending.ID, due.ID, toArchive.ID)
		}
		if limited, _ := repo.ListSyncWork(tctx, 1); len(limited) != 1 {
			t.Fatalf("limit not applied: %d", len(limited))
		}
	})

	t.Run("ListWithWorkspace returns only the tenant's changes that still hold a worktree, with the Request status", func(t *testing.T) {
		tctx := CtxForTenant(newTenant())
		mk := func(workspace string) (domain.OpenSpecChange, domain.Request) {
			r := createRequest(t, env, tctx, nil)
			c, err := repo.Upsert(tctx, domain.OpenSpecChange{ID: uuid.NewString(), RequestID: r.ID, ChangeID: "req-1-x", Status: domain.ChangeStatusPreparing,
				SyncState: domain.SyncStateInSync, WorkspaceID: workspace})
			if err != nil {
				t.Fatal(err)
			}
			return c, r
		}
		held, heldReq := mk("wt-held")
		mk("") // released: no worktree
		other := CtxForTenant(newTenant())
		otherReq := createRequest(t, env, other, nil)
		if _, err := repo.Upsert(other, domain.OpenSpecChange{ID: uuid.NewString(), RequestID: otherReq.ID, ChangeID: "req-9-x", Status: domain.ChangeStatusPreparing,
			SyncState: domain.SyncStateInSync, WorkspaceID: "wt-other"}); err != nil {
			t.Fatal(err)
		}
		got, err := repo.ListWithWorkspace(tctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Change.ID != held.ID || got[0].Change.WorkspaceID != "wt-held" || got[0].RequestStatus != heldReq.Status || got[0].UpdatedAt.IsZero() {
			t.Fatalf("got %+v, want only the held change of this tenant (request status %q)", got, heldReq.Status)
		}
		// Releasing the worktree takes the change out of the list.
		held.WorkspaceID = ""
		if _, err := repo.Upsert(tctx, held); err != nil {
			t.Fatal(err)
		}
		if again, _ := repo.ListWithWorkspace(tctx, 10); len(again) != 0 {
			t.Fatalf("a released change is still listed: %+v", again)
		}
	})
}
