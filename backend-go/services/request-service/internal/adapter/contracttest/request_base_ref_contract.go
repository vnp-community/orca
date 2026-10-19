package contracttest

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type baseRefScenario struct {
	name string
	fn   func(t *testing.T, env Env)
}

// baseRefScenarios are the CR-REQ-039 storage scenarios; RunRequestRepositoryContract runs them with the others.
func baseRefScenarios() []baseRefScenario {
	return []baseRefScenario{
		{"BaseRefDefaultsToEmptyAndRoundTrips", baseRefDefaultsAndRoundTrips},
		{"SetBaseRefOptimisticLock", setBaseRefOptimisticLock},
		{"SetBaseRefNotFound", setBaseRefNotFound},
		{"SetBaseRefConcurrentSameVersion", setBaseRefConcurrentSameVersion},
		{"SetBaseRefRejectsOverlongBeforeWriting", setBaseRefRejectsOverlong},
		{"UpdateKeepsBaseRef", updateKeepsBaseRef},
	}
}

func baseRefWriter(t *testing.T, env Env) usecase.RequestBaseRefWriter {
	t.Helper()
	w, ok := env.Requests.(usecase.RequestBaseRefWriter)
	if !ok {
		t.Fatalf("%T does not implement usecase.RequestBaseRefWriter", env.Requests)
	}
	return w
}

func baseRefDefaultsAndRoundTrips(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	plain := createRequest(t, env, ctx, nil)
	got, err := env.Requests.Get(ctx, plain.ID)
	if err != nil || got.BaseRef != "" {
		t.Fatalf("a Request created without a ref must read back empty: %q %v", got.BaseRef, err)
	}
	pinned := createRequest(t, env, ctx, func(r *domain.Request) { r.BaseRef = "origin/release/1.4" })
	got, err = env.Requests.Get(ctx, pinned.ID)
	if err != nil || got.BaseRef != "origin/release/1.4" {
		t.Fatalf("base ref must round trip: %q %v", got.BaseRef, err)
	}
	// The ref also rides the lock read, the backlog and the content paths that share requestColumns.
	if list, err := env.Requests.List(ctx, usecase.ListFilter{}); err != nil || !listHasBaseRef(list.Requests, pinned.ID, "origin/release/1.4") {
		t.Fatalf("List must return the base ref: %v", err)
	}
}

func listHasBaseRef(rs []domain.Request, id, want string) bool {
	for _, r := range rs {
		if r.ID == id {
			return r.BaseRef == want
		}
	}
	return false
}

func setBaseRefOptimisticLock(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	w := baseRefWriter(t, env)
	r := createRequest(t, env, ctx, nil)

	got, err := w.SetBaseRef(ctx, r.ID, "release/1.4", r.Version)
	if err != nil {
		t.Fatalf("SetBaseRef at the right version: %v", err)
	}
	if got.BaseRef != "release/1.4" || got.Version != r.Version+1 {
		t.Fatalf("SetBaseRef must return the bumped row: %+v", got)
	}

	_, err = w.SetBaseRef(ctx, r.ID, "main", r.Version)
	requireCode(t, err, "REQUEST_VERSION_CONFLICT")
	cur, _ := env.Requests.Get(ctx, r.ID)
	if cur.BaseRef != "release/1.4" || cur.Version != r.Version+1 {
		t.Fatalf("a stale write must change nothing: %+v", cur)
	}

	// Setting it back to empty restores "use the repository default" and still bumps the version.
	back, err := w.SetBaseRef(ctx, r.ID, "", cur.Version)
	if err != nil || back.BaseRef != "" || back.Version != cur.Version+1 {
		t.Fatalf("reset to empty: %+v %v", back, err)
	}
}

func setBaseRefNotFound(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	w := baseRefWriter(t, env)
	r := createRequest(t, env, ctx, nil)
	_, err := w.SetBaseRef(ctx, uuid.NewString(), "main", 1)
	requireCode(t, err, "REQUEST_NOT_FOUND")
	_, err = w.SetBaseRef(ctx, "abc", "main", 1)
	requireCode(t, err, "REQUEST_NOT_FOUND")
	_, err = w.SetBaseRef(CtxForTenant(newTenant()), r.ID, "main", r.Version)
	requireCode(t, err, "REQUEST_NOT_FOUND")
	cur, _ := env.Requests.Get(ctx, r.ID)
	if cur.BaseRef != "" || cur.Version != r.Version {
		t.Fatalf("a foreign tenant must not change the row: %+v", cur)
	}
}

func setBaseRefConcurrentSameVersion(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	w := baseRefWriter(t, env)
	r := createRequest(t, env, ctx, nil)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var wins, conflicts int
	for _, ref := range []string{"a", "b", "c", "d"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.SetBaseRef(ctx, r.ID, ref, r.Version)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errCode(err) == "REQUEST_VERSION_CONFLICT":
				conflicts++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 || conflicts != 3 {
		t.Fatalf("wins=%d conflicts=%d, want 1 and 3", wins, conflicts)
	}
	cur, _ := env.Requests.Get(ctx, r.ID)
	if cur.Version != r.Version+1 {
		t.Fatalf("exactly one write may land, version %d -> %d", r.Version, cur.Version)
	}
}

func setBaseRefRejectsOverlong(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	w := baseRefWriter(t, env)
	r := createRequest(t, env, ctx, nil)
	long := strings.Repeat("a", 256)
	_, err := w.SetBaseRef(ctx, r.ID, long, r.Version)
	requireCode(t, err, "REQUEST_BASE_REF_INVALID")
	cur, _ := env.Requests.Get(ctx, r.ID)
	if cur.BaseRef != "" || cur.Version != r.Version {
		t.Fatalf("an overlong ref must not be stored or truncated: %d bytes, version %d", len(cur.BaseRef), cur.Version)
	}
	tenantID := r.TenantID
	over, err := domain.NewRequest(domain.NewRequestInput{TenantID: tenantID, Title: "t", SourceProvider: "manual", ReporterID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	over.BaseRef = long
	err = env.Tx.InTx(ctx, func(txCtx context.Context) error {
		n, err := env.Requests.NextNumber(txCtx)
		if err != nil {
			return err
		}
		over.Number = n
		return env.Requests.Create(txCtx, over)
	})
	requireCode(t, err, "REQUEST_BASE_REF_INVALID")
	exact := createRequest(t, env, ctx, func(r *domain.Request) { r.BaseRef = strings.Repeat("b", 255) })
	if got, err := env.Requests.Get(ctx, exact.ID); err != nil || len(got.BaseRef) != 255 {
		t.Fatalf("255 bytes is the limit and must be stored whole: %d %v", len(got.BaseRef), err)
	}
}

// Update writes the classification and lifecycle columns from a Request read earlier; it must never rewrite base_ref with
// the stale value that struct carries.
func updateKeepsBaseRef(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	w := baseRefWriter(t, env)
	r := createRequest(t, env, ctx, nil)
	stale := r
	set, err := w.SetBaseRef(ctx, r.ID, "release/1.4", r.Version)
	if err != nil {
		t.Fatal(err)
	}
	stale.Title = "renamed"
	if _, err := env.Requests.Update(ctx, stale, set.Version); err != nil {
		t.Fatal(err)
	}
	cur, _ := env.Requests.Get(ctx, r.ID)
	if cur.BaseRef != "release/1.4" || cur.Title != "renamed" {
		t.Fatalf("Update must leave base_ref alone: %+v", cur)
	}
}
