package contracttest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func appCode(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

// RunOutcomeReviewContract is what both dialects must do for stored reviews and their runs: one row per (request, phase, head),
// version-checked updates, supersession, cursor paging, one active run per kind, and no reads across tenants.
func RunOutcomeReviewContract(t *testing.T, reviews usecase.OutcomeReviewRepository, runs usecase.OutcomeReviewRunRepository) {
	t.Helper()
	tenantA, tenantB := newTenant(), newTenant()
	ctxA, ctxB := CtxForTenant(tenantA), CtxForTenant(tenantB)
	req := uuid.NewString()

	mk := func(phase domain.ReviewPhase, headKey string) usecase.StoredReview {
		return usecase.StoredReview{ID: uuid.NewString(), RequestID: req, Phase: phase, HeadKey: headKey, InputsDigest: "d1", State: domain.ReviewStateReady,
			Model: []byte(`{"requestId":"` + req + `","items":[]}`), KGSnapshot: "snap-1", DesignDigest: "dd", ContextDigest: "cd"}
	}

	t.Run("create, read back and refuse a second row for the same key", func(t *testing.T) {
		in := mk(domain.ReviewPhaseDesign, domain.ReviewDesignHeadKey)
		got, err := reviews.Create(ctxA, in)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != in.ID || got.TenantID != tenantA || got.Version != 1 || got.State != domain.ReviewStateReady || got.KGSnapshot != "snap-1" {
			t.Fatalf("created %+v", got)
		}
		if got.CreatedAt.IsZero() || got.CreatedAt.Location() != time.UTC {
			t.Fatalf("created_at %v", got.CreatedAt)
		}
		dup := mk(domain.ReviewPhaseDesign, domain.ReviewDesignHeadKey)
		if _, err := reviews.Create(ctxA, dup); appCode(err) != "REQUEST_REVIEW_ALREADY_EXISTS" {
			t.Fatalf("duplicate key: %v", err)
		}
		byKey, ok, err := reviews.GetByKey(ctxA, req, domain.ReviewPhaseDesign, domain.ReviewDesignHeadKey)
		if err != nil || !ok || byKey.ID != in.ID {
			t.Fatalf("by key: %+v ok=%v err=%v", byKey, ok, err)
		}
		if _, ok, _ := reviews.GetByKey(ctxA, req, domain.ReviewPhaseDelivered, "x"); ok {
			t.Fatalf("another key must not match")
		}
	})

	t.Run("a missing or malformed id is not found", func(t *testing.T) {
		for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
			if _, err := reviews.Get(ctxA, id); appCode(err) != domain.CodeReviewNotFound {
				t.Fatalf("get %q: %v", id, err)
			}
		}
		if _, ok, err := reviews.Latest(ctxA, "not-a-uuid"); err != nil || ok {
			t.Fatalf("latest of a malformed id: ok=%v err=%v", ok, err)
		}
	})

	t.Run("update needs the stored version", func(t *testing.T) {
		cur, _, _ := reviews.GetByKey(ctxA, req, domain.ReviewPhaseDesign, domain.ReviewDesignHeadKey)
		cur.InputsDigest, cur.State, cur.Model = "d2", domain.ReviewStateStale, []byte(`{"items":[1]}`)
		up, err := reviews.Update(ctxA, cur, cur.Version)
		if err != nil || up.Version != cur.Version+1 || up.InputsDigest != "d2" || up.State != domain.ReviewStateStale {
			t.Fatalf("update: %+v err=%v", up, err)
		}
		if _, err := reviews.Update(ctxA, cur, cur.Version); appCode(err) != domain.CodeReviewStaleRevision {
			t.Fatalf("a stale version must lose: %v", err)
		}
		if _, err := reviews.SetState(ctxA, cur.ID, domain.ReviewStateApproved, cur.Version); appCode(err) != domain.CodeReviewStaleRevision {
			t.Fatalf("set state with a stale version: %v", err)
		}
		ok, err := reviews.SetState(ctxA, cur.ID, domain.ReviewStateApproved, up.Version)
		if err != nil || ok.State != domain.ReviewStateApproved || ok.Version != up.Version+1 {
			t.Fatalf("set state: %+v err=%v", ok, err)
		}
		if _, err := reviews.SetState(ctxA, uuid.NewString(), domain.ReviewStateReady, 1); appCode(err) != domain.CodeReviewNotFound {
			t.Fatalf("set state of a missing row: %v", err)
		}
	})

	t.Run("a new review supersedes the others and paging walks newest first", func(t *testing.T) {
		delivered, err := reviews.Create(ctxA, mk(domain.ReviewPhaseDelivered, "heads-1"))
		if err != nil {
			t.Fatal(err)
		}
		if err := reviews.SupersedeOthers(ctxA, req, delivered.ID); err != nil {
			t.Fatal(err)
		}
		latest, ok, err := reviews.Latest(ctxA, req)
		if err != nil || !ok || latest.ID != delivered.ID {
			t.Fatalf("latest must be the delivered review: %+v ok=%v err=%v", latest, ok, err)
		}
		design, _, _ := reviews.GetByKey(ctxA, req, domain.ReviewPhaseDesign, domain.ReviewDesignHeadKey)
		if design.State != domain.ReviewStateSuperseded {
			t.Fatalf("the design review must be superseded, got %s", design.State)
		}
		page1, err := reviews.List(ctxA, req, 1, "")
		if err != nil || len(page1) != 1 || page1[0].ID != delivered.ID {
			t.Fatalf("page 1: %+v err=%v", page1, err)
		}
		page2, err := reviews.List(ctxA, req, 1, page1[0].ID)
		if err != nil || len(page2) != 1 || page2[0].ID != design.ID {
			t.Fatalf("page 2: %+v err=%v", page2, err)
		}
		page3, err := reviews.List(ctxA, req, 1, page2[0].ID)
		if err != nil || len(page3) != 0 {
			t.Fatalf("page 3 must be empty: %+v err=%v", page3, err)
		}
	})

	t.Run("another tenant sees nothing", func(t *testing.T) {
		if _, ok, err := reviews.Latest(ctxB, req); err != nil || ok {
			t.Fatalf("latest across tenants: ok=%v err=%v", ok, err)
		}
		cur, _, _ := reviews.GetByKey(ctxA, req, domain.ReviewPhaseDesign, domain.ReviewDesignHeadKey)
		if _, err := reviews.Get(ctxB, cur.ID); appCode(err) != domain.CodeReviewNotFound {
			t.Fatalf("get across tenants: %v", err)
		}
		if _, err := reviews.SetState(ctxB, cur.ID, domain.ReviewStateReady, cur.Version); appCode(err) != domain.CodeReviewNotFound {
			t.Fatalf("write across tenants: %v", err)
		}
		if l, err := reviews.List(ctxB, req, 10, ""); err != nil || len(l) != 0 {
			t.Fatalf("list across tenants: %v err=%v", l, err)
		}
	})

	if runs == nil {
		return
	}
	reqRun := uuid.NewString()
	review := uuid.NewString()
	t.Run("only one run of a kind is active per request", func(t *testing.T) {
		first, err := runs.Start(ctxA, usecase.ReviewRun{ID: uuid.NewString(), ReviewID: review, RequestID: reqRun, Kind: usecase.ReviewRunContext, Scope: "design"})
		if err != nil || first.Status != usecase.ReviewRunRunning {
			t.Fatalf("first: %+v err=%v", first, err)
		}
		_, err = runs.Start(ctxA, usecase.ReviewRun{ID: uuid.NewString(), ReviewID: review, RequestID: reqRun, Kind: usecase.ReviewRunContext})
		if appCode(err) != domain.CodeReviewRunInProgress || err == nil || !contains(err.Error(), first.ID) {
			t.Fatalf("second start must name the active run %s: %v", first.ID, err)
		}
		other, err := runs.Start(ctxA, usecase.ReviewRun{ID: uuid.NewString(), ReviewID: review, RequestID: reqRun, Kind: usecase.ReviewRunDraw})
		if err != nil {
			t.Fatalf("another kind may run beside it: %v", err)
		}
		if act, ok, err := runs.ActiveForRequest(ctxA, reqRun, usecase.ReviewRunContext); err != nil || !ok || act.ID != first.ID {
			t.Fatalf("active: %+v ok=%v err=%v", act, ok, err)
		}
		if err := runs.Finish(ctxA, first.ID, usecase.ReviewRunSucceeded, "", "0.42"); err != nil {
			t.Fatal(err)
		}
		if err := runs.Finish(ctxA, first.ID, usecase.ReviewRunFailed, "X", ""); appCode(err) != domain.CodeReviewNotFound {
			t.Fatalf("finishing twice must not rewrite history: %v", err)
		}
		done, _ := runs.Get(ctxA, first.ID)
		if done.Status != usecase.ReviewRunSucceeded || done.CostActual != "0.42" || done.FinishedAt == nil {
			t.Fatalf("finished run %+v", done)
		}
		again, err := runs.Start(ctxA, usecase.ReviewRun{ID: uuid.NewString(), ReviewID: review, RequestID: reqRun, Kind: usecase.ReviewRunContext})
		if err != nil || again.ID == first.ID {
			t.Fatalf("the slot must be free after finish: %+v err=%v", again, err)
		}
		_ = runs.Finish(ctxA, again.ID, usecase.ReviewRunCancelled, "", "")
		_ = runs.Finish(ctxA, other.ID, usecase.ReviewRunFailed, "REQUEST_REVIEW_ARTIFACT_UNSAFE", "")
		if act, ok, _ := runs.ActiveForRequest(ctxA, reqRun, usecase.ReviewRunDraw); ok {
			t.Fatalf("no draw run should still be active: %+v", act)
		}
	})

	t.Run("counting and listing runs", func(t *testing.T) {
		n, err := runs.CountSince(ctxA, reqRun, usecase.ReviewRunContext, time.Now().Add(-time.Hour))
		if err != nil || n != 2 {
			t.Fatalf("count = %d err=%v", n, err)
		}
		if n, _ := runs.CountSince(ctxA, reqRun, usecase.ReviewRunContext, time.Now().Add(time.Hour)); n != 0 {
			t.Fatalf("future cutoff counts %d", n)
		}
		l, err := runs.ListByReview(ctxA, review)
		if err != nil || len(l) != 3 {
			t.Fatalf("list = %d err=%v", len(l), err)
		}
		if l, _ := runs.ListByReview(ctxB, review); len(l) != 0 {
			t.Fatalf("another tenant lists %d runs", len(l))
		}
	})

	t.Run("concurrent starts admit exactly one", func(t *testing.T) {
		r := uuid.NewString()
		var wg sync.WaitGroup
		var mu sync.Mutex
		wins := 0
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := runs.Start(context.WithValue(ctxA, struct{}{}, 1), usecase.ReviewRun{ID: uuid.NewString(), ReviewID: review, RequestID: r, Kind: usecase.ReviewRunContext})
				mu.Lock()
				defer mu.Unlock()
				if err == nil {
					wins++
				} else if appCode(err) != domain.CodeReviewRunInProgress {
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Fatalf("%d concurrent starts won; want exactly 1", wins)
		}
	})
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
