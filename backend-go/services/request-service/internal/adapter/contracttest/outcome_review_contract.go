package contracttest

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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

	t.Run("approval is recorded with who and when and any other change of state clears it", func(t *testing.T) {
		rev, err := reviews.Create(ctxA, mk(domain.ReviewPhaseDelivered, "heads-approve"))
		if err != nil {
			t.Fatal(err)
		}
		person := uuid.NewString()
		if _, err := reviews.Approve(ctxA, rev.ID, person, "looks right", rev.Version+5); appCode(err) != domain.CodeReviewStaleRevision {
			t.Fatalf("approve with a stale version: %v", err)
		}
		ok, err := reviews.Approve(ctxA, rev.ID, person, "looks right", rev.Version)
		if err != nil || ok.State != domain.ReviewStateApproved || ok.ApprovedBy != person || ok.ApprovalNote != "looks right" || ok.ApprovedAt == nil || ok.Version != rev.Version+1 {
			t.Fatalf("approve: %+v err=%v", ok, err)
		}
		back, err := reviews.SetState(ctxA, rev.ID, domain.ReviewStateReady, ok.Version)
		if err != nil || back.ApprovedBy != "" || back.ApprovedAt != nil || back.ApprovalNote != "" {
			t.Fatalf("leaving approved clears it: %+v err=%v", back, err)
		}
		again, _ := reviews.Approve(ctxA, rev.ID, person, "again", back.Version)
		up := again
		up.State = domain.ReviewStateReady
		changed, err := reviews.Update(ctxA, up, again.Version)
		if err != nil || changed.ApprovedBy != "" || changed.ApprovalNote != "" {
			t.Fatalf("a rebuild that makes the review ready clears the approval: %+v err=%v", changed, err)
		}
		if _, err := reviews.Approve(ctxB, rev.ID, person, "", changed.Version); appCode(err) != domain.CodeReviewNotFound {
			t.Fatalf("another tenant approves: %v", err)
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

	t.Run("a run of a regeneration carries its reason and client key", func(t *testing.T) {
		r2 := uuid.NewString()
		review := mk(domain.ReviewPhaseDelivered, "heads-regen")
		review.RequestID = r2
		rev, err := reviews.Create(ctxA, review)
		if err != nil {
			t.Fatal(err)
		}
		person := uuid.NewString()
		first, err := runs.Start(ctxA, usecase.ReviewRun{ID: uuid.NewString(), ReviewID: rev.ID, RequestID: r2, Kind: usecase.ReviewRunDraw, Scope: "delivered", Reason: "looks_wrong", ClientKey: "key-1", CreatedBy: person})
		if err != nil || first.Reason != "looks_wrong" || first.ClientKey != "key-1" || first.CreatedBy != person {
			t.Fatalf("start: %+v err=%v", first, err)
		}
		found, ok, err := runs.FindByClientKey(ctxA, r2, "key-1")
		if err != nil || !ok || found.ID != first.ID {
			t.Fatalf("by client key: %+v ok=%v err=%v", found, ok, err)
		}
		if _, ok, _ := runs.FindByClientKey(ctxA, r2, "other"); ok {
			t.Error("another key finds nothing")
		}
		if _, ok, _ := runs.FindByClientKey(ctxB, r2, "key-1"); ok {
			t.Error("another tenant finds the run")
		}
		if err := runs.Finish(ctxA, first.ID, usecase.ReviewRunSucceeded, "", ""); err != nil {
			t.Fatal(err)
		}
		if _, err := runs.Start(ctxA, usecase.ReviewRun{ID: uuid.NewString(), ReviewID: rev.ID, RequestID: r2, Kind: usecase.ReviewRunDraw, ClientKey: "key-1"}); appCode(err) != "REQUEST_REVIEW_DUPLICATE_REQUEST" {
			t.Fatalf("a repeated client key: %v", err)
		}
		auto, err := runs.Start(ctxA, usecase.ReviewRun{ID: uuid.NewString(), ReviewID: rev.ID, RequestID: r2, Kind: usecase.ReviewRunContext})
		if err != nil {
			t.Fatal(err)
		}
		since := time.Now().Add(-time.Hour)
		if n, err := runs.CountRegenerations(ctxA, r2, since); err != nil || n != 1 {
			t.Fatalf("only the run a person asked for counts: %d %v", n, err)
		}
		if n, err := runs.CountRegenerations(ctxA, r2, time.Now().Add(time.Hour)); err != nil || n != 0 {
			t.Fatalf("nothing since the future: %d %v", n, err)
		}
		if n, err := runs.CountRegenerations(ctxB, r2, since); err != nil || n != 0 {
			t.Fatalf("another tenant: %d %v", n, err)
		}
		if n, err := runs.CountActive(ctxA); err != nil || n < 1 {
			t.Fatalf("the context run is active: %d %v", n, err)
		}
		_ = runs.Finish(ctxA, auto.ID, usecase.ReviewRunFailed, "x", "")
		if n, _ := runs.CountActive(ctxB); n != 0 {
			t.Fatalf("tenant B has no active runs: %d", n)
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

// RunReviewContextStoreContract is what both dialects must do for the locked design and the cached context results: replace on the
// same (request, kind, digest), serve only unexpired records, return the newest, and never cross tenants.
func RunReviewContextStoreContract(t *testing.T, store usecase.ReviewContextStore) {
	t.Helper()
	tenantA, tenantB := newTenant(), newTenant()
	ctxA, ctxB := CtxForTenant(tenantA), CtxForTenant(tenantB)
	req := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Second)
	rec := func(kind usecase.ReviewContextKind, digest, payload string, ttl time.Duration) usecase.ReviewContextRecord {
		return usecase.ReviewContextRecord{ID: uuid.NewString(), RequestID: req, Kind: kind, Digest: digest, PromptVersion: "p/1", Payload: []byte(payload), ExpiresAt: now.Add(ttl)}
	}

	t.Run("put and read back", func(t *testing.T) {
		got, err := store.Put(ctxA, rec(usecase.ReviewContextDesign, "d1", `{"design":{"available":true}}`, time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if got.TenantID != tenantA || got.Kind != usecase.ReviewContextDesign || got.Digest != "d1" || got.PromptVersion != "p/1" || got.CreatedAt.Location() != time.UTC {
			t.Fatalf("put returned %+v", got)
		}
		read, ok, err := store.Get(ctxA, req, usecase.ReviewContextDesign, "d1", now)
		if err != nil || !ok || read.ID != got.ID || !jsonEqual(read.Payload, got.Payload) {
			t.Fatalf("get: %+v ok=%v err=%v", read, ok, err)
		}
		if _, ok, _ := store.Get(ctxA, req, usecase.ReviewContextDesign, "other", now); ok {
			t.Error("a digest that was never stored must not be found")
		}
		if _, ok, _ := store.Get(ctxA, req, usecase.ReviewContextResult, "d1", now); ok {
			t.Error("the kind is part of the key")
		}
	})

	t.Run("the same key is replaced, not duplicated", func(t *testing.T) {
		first, _ := store.Put(ctxA, rec(usecase.ReviewContextResult, "k1", `{"v":1}`, time.Hour))
		second, err := store.Put(ctxA, rec(usecase.ReviewContextResult, "k1", `{"v":2}`, 2*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if second.ID != first.ID || !jsonEqual(second.Payload, []byte(`{"v":2}`)) || !second.ExpiresAt.After(first.ExpiresAt) {
			t.Errorf("the second put must update the row: %+v then %+v", first, second)
		}
	})

	t.Run("an expired record is not served", func(t *testing.T) {
		if _, err := store.Put(ctxA, rec(usecase.ReviewContextResult, "old", `{"v":0}`, -time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := store.Get(ctxA, req, usecase.ReviewContextResult, "old", now); ok {
			t.Error("expired record served by Get")
		}
	})

	t.Run("latest is the newest unexpired of its kind", func(t *testing.T) {
		r2 := uuid.NewString()
		mk := func(digest string, ttl time.Duration) usecase.ReviewContextRecord {
			x := rec(usecase.ReviewContextDesign, digest, `{"d":"`+digest+`"}`, ttl)
			x.RequestID = r2
			return x
		}
		if _, err := store.Put(ctxA, mk("a", time.Hour)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
		if _, err := store.Put(ctxA, mk("b", time.Hour)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
		if _, err := store.Put(ctxA, mk("c", -time.Hour)); err != nil {
			t.Fatal(err)
		}
		got, ok, err := store.Latest(ctxA, r2, usecase.ReviewContextDesign, now)
		if err != nil || !ok || got.Digest != "b" {
			t.Fatalf("latest = %+v ok=%v err=%v, want digest b (c is expired)", got, ok, err)
		}
		if _, ok, _ := store.Latest(ctxA, r2, usecase.ReviewContextResult, now); ok {
			t.Error("latest looks only at its kind")
		}
		if _, ok, _ := store.Latest(ctxA, "not-a-uuid", usecase.ReviewContextDesign, now); ok {
			t.Error("a malformed id finds nothing")
		}
	})

	t.Run("tenants do not see each other", func(t *testing.T) {
		if _, ok, _ := store.Get(ctxB, req, usecase.ReviewContextDesign, "d1", now); ok {
			t.Error("tenant B read tenant A's record")
		}
		if _, ok, _ := store.Latest(ctxB, req, usecase.ReviewContextDesign, now); ok {
			t.Error("tenant B saw tenant A's latest")
		}
		if _, err := store.Put(ctxB, usecase.ReviewContextRecord{ID: uuid.NewString(), RequestID: req, Kind: usecase.ReviewContextDesign, Digest: "d1", Payload: []byte(`{"b":1}`), ExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatalf("the same key in another tenant is a different row: %v", err)
		}
		a, _, _ := store.Get(ctxA, req, usecase.ReviewContextDesign, "d1", now)
		if jsonEqual(a.Payload, []byte(`{"b":1}`)) {
			t.Error("tenant B's put changed tenant A's row")
		}
	})
}

// jsonEqual compares two JSON documents by value, since a JSON column may be stored with other spacing and key order.
func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// RunReviewDecisionContract is what both dialects must do for decisions: one in force for each (review, item), history kept, a
// send_back decision linked to the Request it was handed to, and no reads across tenants.
func RunReviewDecisionContract(t *testing.T, decisions usecase.OutcomeReviewDecisionRepository) {
	t.Helper()
	tenantA, tenantB := newTenant(), newTenant()
	ctxA, ctxB := CtxForTenant(tenantA), CtxForTenant(tenantB)
	review, req, person := uuid.NewString(), uuid.NewString(), uuid.NewString()
	rec := func(ctx context.Context, item string, d domain.ReviewDecision, note string) domain.ItemDecision {
		t.Helper()
		got, err := decisions.Record(ctx, domain.ItemDecision{ReviewID: review, RequestID: req, ItemKey: item, Decision: d, Note: note, DecidedBy: person})
		if err != nil {
			t.Fatalf("record %s %s: %v", item, d, err)
		}
		return got
	}

	t.Run("a decision is in force until a later one replaces it", func(t *testing.T) {
		first := rec(ctxA, "item-1", domain.DecisionAcceptException, "ADR-09 allows it")
		if !first.Active || first.Decision != domain.DecisionAcceptException || first.Note != "ADR-09 allows it" || first.DecidedBy != person || first.DecidedAt.Location() != time.UTC {
			t.Fatalf("recorded %+v", first)
		}
		second := rec(ctxA, "item-1", domain.DecisionSendBack, "")
		rec(ctxA, "item-2", domain.DecisionDismiss, "the claim is wrong")
		inForce, err := decisions.InForce(ctxA, review)
		if err != nil || len(inForce) != 2 {
			t.Fatalf("in force: %+v %v", inForce, err)
		}
		for _, d := range inForce {
			if d.ItemKey == "item-1" && (d.ID != second.ID || d.Decision != domain.DecisionSendBack) {
				t.Errorf("the later decision is the one in force: %+v", d)
			}
		}
		hist, err := decisions.History(ctxA, review, "item-1")
		if err != nil || len(hist) != 2 || hist[0].ID != second.ID || hist[1].ID != first.ID || hist[1].Active || !hist[0].Active {
			t.Fatalf("history is newest first and the old one is not in force: %+v %v", hist, err)
		}
	})

	t.Run("a send_back decision can be linked to the Request that took it", func(t *testing.T) {
		child := uuid.NewString()
		rec(ctxA, "item-3", domain.DecisionSendBack, "")
		rec(ctxA, "item-4", domain.DecisionAcceptException, "fine")
		n, err := decisions.LinkChild(ctxA, review, []string{"item-1", "item-3", "item-4", "missing"}, child)
		if err != nil || n != 2 {
			t.Fatalf("only send_back decisions in force are linked: %d %v", n, err)
		}
		inForce, _ := decisions.InForce(ctxA, review)
		linked := map[string]string{}
		for _, d := range inForce {
			linked[d.ItemKey] = d.ChildRequestID
		}
		if linked["item-1"] != child || linked["item-3"] != child || linked["item-4"] != "" || linked["item-2"] != "" {
			t.Errorf("linked: %v", linked)
		}
		if n, _ := decisions.LinkChild(ctxA, review, nil, child); n != 0 {
			t.Errorf("nothing to link: %d", n)
		}
		if n, _ := decisions.LinkChild(ctxA, review, []string{"item-1"}, "not-a-uuid"); n != 0 {
			t.Errorf("a malformed child: %d", n)
		}
	})

	t.Run("racing decisions on one item leave one in force", func(t *testing.T) {
		race := uuid.NewString()
		var wg sync.WaitGroup
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				d := domain.DecisionSendBack
				if i%2 == 0 {
					d = domain.DecisionDismiss
				}
				_, _ = decisions.Record(ctxA, domain.ItemDecision{ReviewID: race, RequestID: req, ItemKey: "same", Decision: d, Note: "n", DecidedBy: person})
			}(i)
		}
		wg.Wait()
		inForce, err := decisions.InForce(ctxA, race)
		if err != nil || len(inForce) != 1 {
			t.Fatalf("in force: %d %v", len(inForce), err)
		}
	})

	t.Run("bad ids and other tenants", func(t *testing.T) {
		if _, err := decisions.Record(ctxA, domain.ItemDecision{ReviewID: "x", RequestID: req, ItemKey: "k", Decision: domain.DecisionSendBack, DecidedBy: person}); appCode(err) != domain.CodeReviewDecisionInvalid {
			t.Errorf("bad review id: %v", err)
		}
		if got, err := decisions.InForce(ctxA, "not-a-uuid"); err != nil || len(got) != 0 {
			t.Errorf("malformed review id: %v %v", got, err)
		}
		if got, _ := decisions.InForce(ctxB, review); len(got) != 0 {
			t.Errorf("tenant B sees tenant A's decisions: %+v", got)
		}
		if got, _ := decisions.History(ctxB, review, "item-1"); len(got) != 0 {
			t.Errorf("tenant B sees history: %+v", got)
		}
		if n, _ := decisions.LinkChild(ctxB, review, []string{"item-1"}, uuid.NewString()); n != 0 {
			t.Errorf("tenant B linked tenant A's decision: %d", n)
		}
	})
}

// RunReviewProfileContract is what both dialects must do for drawing profiles: one layer per tenant or project, optimistic
// versions, and no reads across tenants.
func RunReviewProfileContract(t *testing.T, profiles usecase.ReviewDrawingProfileRepository) {
	t.Helper()
	tenantA, tenantB := newTenant(), newTenant()
	ctxA, ctxB := CtxForTenant(tenantA), CtxForTenant(tenantB)
	person, project := uuid.NewString(), uuid.NewString()
	spec := func(lang string) []byte {
		return []byte(`{"schema":"review-drawing-profile/1","language":"` + lang + `"}`)
	}
	save := func(ctx context.Context, key, lang string, expected int64) (usecase.StoredDrawingProfile, error) {
		return profiles.Save(ctx, usecase.StoredDrawingProfile{ProjectKey: key, Spec: spec(lang), SpecDigest: "digest-" + lang, UpdatedBy: person}, expected)
	}

	t.Run("create, read, replace under the version", func(t *testing.T) {
		if _, ok, err := profiles.Get(ctxA, project); err != nil || ok {
			t.Fatalf("nothing yet: ok=%v err=%v", ok, err)
		}
		created, err := save(ctxA, project, "vi", 0)
		if err != nil || created.Version != 1 || created.ProjectKey != project || created.SpecDigest != "digest-vi" || created.UpdatedBy != person || created.UpdatedAt.Location() != time.UTC {
			t.Fatalf("create: %+v err=%v", created, err)
		}
		if !jsonEqual(created.Spec, spec("vi")) {
			t.Errorf("spec %s", created.Spec)
		}
		if _, err := save(ctxA, project, "en", 0); appCode(err) != domain.CodeReviewStaleRevision {
			t.Fatalf("creating twice: %v", err)
		}
		if _, err := save(ctxA, project, "en", 7); appCode(err) != domain.CodeReviewStaleRevision {
			t.Fatalf("a version that is not the stored one: %v", err)
		}
		next, err := save(ctxA, project, "en", created.Version)
		if err != nil || next.Version != 2 || next.SpecDigest != "digest-en" || next.ID != created.ID {
			t.Fatalf("replace: %+v err=%v", next, err)
		}
		if _, err := save(ctxA, project, "vi", created.Version); appCode(err) != domain.CodeReviewStaleRevision {
			t.Fatalf("the second writer on the old version loses: %v", err)
		}
	})

	t.Run("the tenant layer and a project layer are separate", func(t *testing.T) {
		if _, err := save(ctxA, "", "vi", 0); err != nil {
			t.Fatal(err)
		}
		tenantLayer, ok, _ := profiles.Get(ctxA, "")
		projectLayer, ok2, _ := profiles.Get(ctxA, project)
		if !ok || !ok2 || tenantLayer.ID == projectLayer.ID || tenantLayer.SpecDigest == projectLayer.SpecDigest {
			t.Fatalf("layers: %+v %+v", tenantLayer, projectLayer)
		}
	})

	t.Run("two writers creating at once leave one winner", func(t *testing.T) {
		key := uuid.NewString()
		var wg sync.WaitGroup
		results := make(chan error, 6)
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := save(ctxA, key, "vi", 0)
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		wins := 0
		for err := range results {
			switch {
			case err == nil:
				wins++
			case appCode(err) != domain.CodeReviewStaleRevision:
				t.Errorf("a loser must get a version error: %v", err)
			}
		}
		if wins != 1 {
			t.Errorf("winners = %d", wins)
		}
	})

	t.Run("delete is versioned", func(t *testing.T) {
		cur, _, _ := profiles.Get(ctxA, project)
		if err := profiles.Delete(ctxA, project, cur.Version+3); appCode(err) != domain.CodeReviewStaleRevision {
			t.Fatalf("delete with a stale version: %v", err)
		}
		if err := profiles.Delete(ctxA, project, cur.Version); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := profiles.Get(ctxA, project); ok {
			t.Error("deleted")
		}
		if err := profiles.Delete(ctxA, project, cur.Version); appCode(err) != domain.CodeReviewStaleRevision {
			t.Fatalf("deleting twice: %v", err)
		}
	})

	t.Run("tenants do not see each other", func(t *testing.T) {
		if _, ok, _ := profiles.Get(ctxB, ""); ok {
			t.Error("tenant B read tenant A's layer")
		}
		if _, err := save(ctxB, "", "en", 1); appCode(err) != domain.CodeReviewStaleRevision {
			t.Errorf("tenant B updates tenant A's layer: %v", err)
		}
		if _, err := save(ctxB, "", "en", 0); err != nil {
			t.Errorf("the same scope in another tenant is another row: %v", err)
		}
		a, _, _ := profiles.Get(ctxA, "")
		if a.SpecDigest != "digest-vi" {
			t.Errorf("tenant B's save changed tenant A's layer: %+v", a)
		}
	})
}
