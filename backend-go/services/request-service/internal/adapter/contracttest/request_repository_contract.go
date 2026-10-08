// Package contracttest holds the repository scenarios both dialect adapters must pass.
// It is a regular (non _test) package so each adapter's thin _test file can import it.
package contracttest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// Env wires one dialect's adapters. Tx must be the same runner the repositories join.
type Env struct {
	Tx          usecase.TxRunner
	Requests    usecase.RequestRepository
	History     usecase.RequestTypeHistoryRepository
	Solutions   usecase.SolutionCoreRepository
	Links       usecase.RequestLinkRepository
	Idempotency usecase.RequestIdempotencyRepository
}

// CtxForTenant is how the gRPC interceptor would scope a call.
func CtxForTenant(tenantID string) context.Context {
	return tenant.WithTenantID(context.Background(), tenantID)
}

func newTenant() string { return uuid.NewString() }

// createRequest allocates a number and inserts in one transaction, like the CreateRequest use case will.
func createRequest(t *testing.T, env Env, ctx context.Context, mod func(r *domain.Request)) domain.Request {
	t.Helper()
	tenantID, _ := tenant.TenantID(ctx)
	r, err := domain.NewRequest(domain.NewRequestInput{
		TenantID: tenantID, ProjectID: uuid.NewString(), Title: "title " + uuid.NewString()[:8], Body: "body",
		SourceProvider: "manual", ReporterID: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if mod != nil {
		mod(&r)
	}
	err = env.Tx.InTx(ctx, func(txCtx context.Context) error {
		n, err := env.Requests.NextNumber(txCtx)
		if err != nil {
			return err
		}
		r.Number = n
		return env.Requests.Create(txCtx, r)
	})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	return r
}

func errCode(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func requireCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || errCode(err) != want {
		t.Fatalf("want error code %s, got %v", want, err)
	}
}

// retryDeadlock absorbs InnoDB/PG deadlock victims; the counter row is the only hot spot.
func retryDeadlock(fn func() error) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = fn(); err == nil {
			return nil
		}
		msg := err.Error()
		if !strings.Contains(msg, "1213") && !strings.Contains(msg, "deadlock detected") {
			return err
		}
	}
	return err
}

// RunRequestRepositoryContract runs every scenario against a fresh environment.
func RunRequestRepositoryContract(t *testing.T, newEnv func(t *testing.T) Env) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env Env)
	}{
		{"CreateGetRoundTrip", createGetRoundTrip},
		{"GetByNumber", getByNumber},
		{"NumberingConcurrent20", numberingConcurrent20},
		{"NumberingRollbackNoGap", numberingRollbackNoGap},
		{"NextNumberOutsideTxRejected", nextNumberOutsideTxRejected},
		{"MissingTenantRejected", missingTenantRejected},
		{"ClaimConcurrent", claimConcurrent},
		{"ClaimLoserSeesWinnerID", claimLoserSeesWinnerID},
		{"UpdateCASConflict", updateCASConflict},
		{"UpdateNotFound", updateNotFound},
		{"UpdateSolutionEngineCAS", updateSolutionEngineCAS},
		{"TenantIsolationRead", tenantIsolationRead},
		{"TenantIsolationUpdate", tenantIsolationUpdate},
		{"ListKeysetStableUnderInsert", listKeysetStableUnderInsert},
		{"ListFilters", listFilters},
		{"LinkRoundTripAndSelfRejected", linkRoundTripAndSelfRejected},
		{"TypeHistoryOrderedByAt", typeHistoryOrderedByAt},
		{"SolutionsCAS", solutionsCAS},
		{"EveryGoConstantAcceptedByDB", everyGoConstantAcceptedByDB},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

func createGetRoundTrip(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	conf := 0.875
	engine := domain.EngineOpenSpec
	want := createRequest(t, env, ctx, func(r *domain.Request) {
		r.Type, r.TypeSource, r.Size, r.Urgency = domain.RequestTypeBug, domain.TypeSourceAI, domain.RequestSizeM, domain.UrgencyUrgent
		r.Confidence, r.ClassificationReason = &conf, "looks like a bug"
		r.SourceProvider, r.SourceRef, r.SourceSite, r.SourceURL = domain.SourceProviderJira, "ABC-1", "acme", "https://acme/ABC-1"
		r.PlanTaskID = uuid.NewString()
		r.SolutionEngine = &engine
		r.SourceHints = domain.SourceHints{IssueType: "Bug", Labels: []string{"ệ", "backend"}, Priority: "High", TypeHint: "bug"}
		r.ClassificationAttempts = 2
	})
	got, err := env.Requests.Get(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 1 {
		t.Errorf("first number = %d, want 1", got.Number)
	}
	want.Number = got.Number
	if got.Confidence == nil || *got.Confidence != conf {
		t.Errorf("confidence = %v, want %v", got.Confidence, conf)
	}
	if got.SolutionEngine == nil || *got.SolutionEngine != engine {
		t.Errorf("solution engine = %v", got.SolutionEngine)
	}
	want.Confidence, got.Confidence, want.SolutionEngine, got.SolutionEngine = nil, nil, nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip mismatch:\n got  %+v\n want %+v", got, want)
	}

	// Optional columns stay empty (NULL), not zero values.
	bare := createRequest(t, env, ctx, func(r *domain.Request) { r.ProjectID = "" })
	gb, err := env.Requests.Get(ctx, bare.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gb.ProjectID != "" || gb.Type != "" || gb.Size != "" || gb.TypeSource != "" || gb.PlanTaskID != "" ||
		gb.ReturnedFromStage != "" || gb.Confidence != nil || gb.SolutionEngine != nil || !gb.SourceHints.IsZero() || gb.ClassificationAttempts != 0 {
		t.Errorf("optional fields should be empty: %+v", gb)
	}
}

func getByNumber(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	r := createRequest(t, env, ctx, nil)
	got, err := env.Requests.GetByNumber(ctx, 1)
	if err != nil || got.ID != r.ID {
		t.Fatalf("GetByNumber: %v %v", got.ID, err)
	}
	_, err = env.Requests.GetByNumber(ctx, 999)
	requireCode(t, err, "REQUEST_NOT_FOUND")
}

func numberingConcurrent20(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	var wg sync.WaitGroup
	var mu sync.Mutex
	var nums []int64
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := retryDeadlock(func() error {
				tenantID, _ := tenant.TenantID(ctx)
				r, err := domain.NewRequest(domain.NewRequestInput{TenantID: tenantID, Title: "c", SourceProvider: "manual", ReporterID: uuid.NewString()})
				if err != nil {
					return err
				}
				return env.Tx.InTx(ctx, func(txCtx context.Context) error {
					n, err := env.Requests.NextNumber(txCtx)
					if err != nil {
						return err
					}
					r.Number = n
					if err := env.Requests.Create(txCtx, r); err != nil {
						return err
					}
					mu.Lock()
					nums = append(nums, n)
					mu.Unlock()
					return nil
				})
			})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent create: %v", err)
	}
	// A retried transaction may append twice before rolling back; compare the committed rows instead.
	res, err := env.Requests.List(ctx, usecase.ListFilter{PageSize: 200})
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, r := range res.Requests {
		got = append(got, r.Number)
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if len(got) != 20 {
		t.Fatalf("committed %d requests, want 20 (numbers %v)", len(got), got)
	}
	for i, n := range got {
		if n != int64(i+1) {
			t.Fatalf("numbers not 1..20 contiguous: %v", got)
		}
	}
}

func numberingRollbackNoGap(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	boom := errors.New("boom")
	err := env.Tx.InTx(ctx, func(txCtx context.Context) error {
		n, err := env.Requests.NextNumber(txCtx)
		if err != nil || n != 1 {
			t.Errorf("first NextNumber = %d, %v", n, err)
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("InTx err = %v", err)
	}
	r := createRequest(t, env, ctx, nil)
	got, err := env.Requests.Get(ctx, r.ID)
	if err != nil || got.Number != 1 {
		t.Fatalf("number after rollback = %d (%v), want 1 with no gap", got.Number, err)
	}
}

func nextNumberOutsideTxRejected(t *testing.T, env Env) {
	_, err := env.Requests.NextNumber(CtxForTenant(newTenant()))
	requireCode(t, err, "REQUEST_NEXT_NUMBER_NEEDS_TX")
}

// A missing tenant must fail loudly; FORCE RLS would otherwise just return no rows.
func missingTenantRejected(t *testing.T, env Env) {
	ctx := context.Background()
	requireCode(t, env.Tx.InTx(ctx, func(context.Context) error { return nil }), "REQUEST_TENANT_REQUIRED")
	_, err := env.Requests.Get(ctx, uuid.NewString())
	requireCode(t, err, "REQUEST_TENANT_REQUIRED")
	_, err = env.Requests.List(ctx, usecase.ListFilter{})
	requireCode(t, err, "REQUEST_TENANT_REQUIRED")
	_, err = env.Idempotency.Find(ctx, "manual", "", "x")
	requireCode(t, err, "REQUEST_TENANT_REQUIRED")
	_, err = env.Links.ListChildren(ctx, uuid.NewString())
	requireCode(t, err, "REQUEST_TENANT_REQUIRED")

	// A nested InTx for another tenant must not silently reuse the outer transaction.
	err = env.Tx.InTx(CtxForTenant(newTenant()), func(txCtx context.Context) error {
		return env.Tx.InTx(tenant.WithTenantID(txCtx, newTenant()), func(context.Context) error { return nil })
	})
	requireCode(t, err, "REQUEST_TX_TENANT_MISMATCH")
}

func claimConcurrent(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	const workers = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimedBy := map[string]bool{}
	existingSeen := map[string]bool{}
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		id := uuid.NewString()
		go func() {
			defer wg.Done()
			existing, claimed, err := env.Idempotency.Claim(ctx, "jira", "acme", "ABC-7", id)
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if claimed {
				claimedBy[id] = true
			} else {
				existingSeen[existing] = true
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("claim: %v", err)
	}
	if len(claimedBy) != 1 {
		t.Fatalf("%d claimers won, want exactly 1", len(claimedBy))
	}
	for id := range claimedBy {
		if len(existingSeen) != 1 || !existingSeen[id] {
			t.Fatalf("losers saw %v, want only the winner %s", existingSeen, id)
		}
	}
}

func claimLoserSeesWinnerID(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	first, second := uuid.NewString(), uuid.NewString()
	if existing, claimed, err := env.Idempotency.Claim(ctx, "manual", "user:u1", "client-1", first); err != nil || !claimed || existing != "" {
		t.Fatalf("first claim = %q %v %v", existing, claimed, err)
	}
	existing, claimed, err := env.Idempotency.Claim(ctx, "manual", "user:u1", "client-1", second)
	if err != nil || claimed || existing != first {
		t.Fatalf("second claim = %q %v %v, want winner %s", existing, claimed, err, first)
	}
	if got, err := env.Idempotency.Find(ctx, "manual", "user:u1", "client-1"); err != nil || got != first {
		t.Fatalf("Find = %q %v", got, err)
	}
	if got, err := env.Idempotency.Find(ctx, "manual", "user:u1", "other"); err != nil || got != "" {
		t.Fatalf("Find of unknown key = %q %v, want empty", got, err)
	}
	// Same key under another tenant is independent.
	if _, claimed, err := env.Idempotency.Claim(CtxForTenant(newTenant()), "manual", "user:u1", "client-1", uuid.NewString()); err != nil || !claimed {
		t.Fatalf("other tenant claim = %v %v", claimed, err)
	}
}

func updateCASConflict(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	r := createRequest(t, env, ctx, nil)
	a, b := r, r
	a.Title, b.Title = "from A", "from B"
	a.Status = domain.RequestStatusClassifying

	updated, err := env.Requests.Update(ctx, a, r.Version)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != r.Version+1 || updated.Title != "from A" || updated.Status != domain.RequestStatusClassifying {
		t.Fatalf("Update must return the bumped row: %+v", updated)
	}
	if !updated.UpdatedAt.After(r.UpdatedAt) && !updated.UpdatedAt.Equal(r.UpdatedAt) {
		t.Errorf("updated_at went backwards")
	}
	_, err = env.Requests.Update(ctx, b, r.Version)
	requireCode(t, err, "REQUEST_VERSION_CONFLICT")
	got, _ := env.Requests.Get(ctx, r.ID)
	if got.Title != "from A" || got.Version != r.Version+1 {
		t.Fatalf("loser must not change the row: %+v", got)
	}

	// Two writers racing on the same expected version: exactly one wins.
	var wg sync.WaitGroup
	var wins, conflicts int32
	var mu sync.Mutex
	for i := 0; i < 2; i++ {
		wg.Add(1)
		c := got
		c.Body = fmt.Sprintf("writer %d", i)
		go func() {
			defer wg.Done()
			_, err := env.Requests.Update(ctx, c, got.Version)
			mu.Lock()
			defer mu.Unlock()
			switch errCode(err) {
			case "":
				wins++
			case "REQUEST_VERSION_CONFLICT":
				conflicts++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins=%d conflicts=%d, want 1 and 1", wins, conflicts)
	}
}

func updateNotFound(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	tenantID, _ := tenant.TenantID(ctx)
	_, err := env.Requests.Update(ctx, domain.Request{ID: uuid.NewString(), TenantID: tenantID, Title: "x", SourceProvider: "manual", Urgency: "normal", Status: "new", ReporterID: uuid.NewString()}, 1)
	requireCode(t, err, "REQUEST_NOT_FOUND")
}

func updateSolutionEngineCAS(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	r := createRequest(t, env, ctx, nil)
	if err := env.Requests.UpdateSolutionEngine(ctx, r.ID, domain.EngineOpenSpec, r.Version); err != nil {
		t.Fatal(err)
	}
	requireCode(t, env.Requests.UpdateSolutionEngine(ctx, r.ID, domain.EngineNative, r.Version), "REQUEST_VERSION_CONFLICT")
	requireCode(t, env.Requests.UpdateSolutionEngine(ctx, uuid.NewString(), domain.EngineNative, 1), "REQUEST_NOT_FOUND")
	got, _ := env.Requests.Get(ctx, r.ID)
	if got.SolutionEngine == nil || *got.SolutionEngine != domain.EngineOpenSpec || got.Version != r.Version+1 {
		t.Fatalf("engine not persisted: %+v", got)
	}
}

func tenantIsolationRead(t *testing.T, env Env) {
	ctxA, ctxB := CtxForTenant(newTenant()), CtxForTenant(newTenant())
	r := createRequest(t, env, ctxA, nil)
	_, err := env.Requests.Get(ctxB, r.ID)
	requireCode(t, err, "REQUEST_NOT_FOUND")
	_, err = env.Requests.GetByNumber(ctxB, 1)
	requireCode(t, err, "REQUEST_NOT_FOUND")
	res, err := env.Requests.List(ctxB, usecase.ListFilter{})
	if err != nil || len(res.Requests) != 0 {
		t.Fatalf("tenant B list = %d %v, want empty", len(res.Requests), err)
	}
	// Numbers are per tenant.
	b := createRequest(t, env, ctxB, nil)
	if got, _ := env.Requests.Get(ctxB, b.ID); got.Number != 1 {
		t.Errorf("tenant B first number = %d, want 1", got.Number)
	}
}

func tenantIsolationUpdate(t *testing.T, env Env) {
	ctxA, ctxB := CtxForTenant(newTenant()), CtxForTenant(newTenant())
	r := createRequest(t, env, ctxA, nil)
	hijack := r
	hijack.Title = "hijacked"
	_, err := env.Requests.Update(ctxB, hijack, r.Version)
	requireCode(t, err, "REQUEST_NOT_FOUND")
	requireCode(t, env.Requests.UpdateSolutionEngine(ctxB, r.ID, domain.EngineOpenSpec, r.Version), "REQUEST_NOT_FOUND")
	got, _ := env.Requests.Get(ctxA, r.ID)
	if got.Title == "hijacked" || got.Version != r.Version {
		t.Fatalf("tenant A row was modified: %+v", got)
	}
	// Creating a row for another tenant through the wrong ctx is refused.
	foreign := r
	foreign.ID = uuid.NewString()
	err = env.Tx.InTx(ctxB, func(txCtx context.Context) error {
		n, err := env.Requests.NextNumber(txCtx)
		foreign.Number = n
		if err != nil {
			return err
		}
		return env.Requests.Create(txCtx, foreign)
	})
	requireCode(t, err, "REQUEST_TENANT_REQUIRED")
}

func listKeysetStableUnderInsert(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	var all []string
	for i := 0; i < 12; i++ {
		i := i
		r := createRequest(t, env, ctx, func(r *domain.Request) {
			r.CreatedAt = base.Add(time.Duration(i) * time.Second)
			r.UpdatedAt = r.CreatedAt
		})
		all = append(all, r.ID)
	}
	p1, err := env.Requests.List(ctx, usecase.ListFilter{PageSize: 5})
	if err != nil || len(p1.Requests) != 5 || p1.NextPageToken == "" {
		t.Fatalf("page1 = %d token=%q err=%v", len(p1.Requests), p1.NextPageToken, err)
	}
	// A newer row appears between the page fetches; keyset paging must not shift or repeat.
	createRequest(t, env, ctx, func(r *domain.Request) { r.CreatedAt = base.Add(time.Hour); r.UpdatedAt = r.CreatedAt })
	seen := map[string]bool{}
	for _, r := range p1.Requests {
		seen[r.ID] = true
	}
	token := p1.NextPageToken
	for token != "" {
		pg, err := env.Requests.List(ctx, usecase.ListFilter{PageSize: 5, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range pg.Requests {
			if seen[r.ID] {
				t.Fatalf("duplicate %s across pages", r.ID)
			}
			seen[r.ID] = true
		}
		token = pg.NextPageToken
	}
	for _, id := range all {
		if !seen[id] {
			t.Fatalf("request %s missing from paging", id)
		}
	}
	if len(seen) != 12 {
		t.Fatalf("saw %d rows, want the 12 existing at page 1", len(seen))
	}
	// Order is newest first.
	if p1.Requests[0].ID != all[11] {
		t.Errorf("first row should be the newest of the original 12")
	}
	_, err = env.Requests.List(ctx, usecase.ListFilter{PageToken: "!!!"})
	requireCode(t, err, "REQUEST_INVALID_PAGE_TOKEN")
}

func listFilters(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	proj1, proj2 := uuid.NewString(), uuid.NewString()
	mk := func(proj string, typ domain.RequestType, st domain.RequestStatus, prov domain.SourceProvider, ref, site string) {
		createRequest(t, env, ctx, func(r *domain.Request) {
			r.ProjectID, r.Type, r.Status, r.SourceProvider, r.SourceRef, r.SourceSite = proj, typ, st, prov, ref, site
			if st == domain.RequestStatusRequestBacklog {
				r.ReturnedFromStage, r.ReturnedCategory = domain.ReturnStageAnalysis, domain.ReturnCategoryOther
			}
		})
	}
	mk(proj1, domain.RequestTypeBug, domain.RequestStatusNew, domain.SourceProviderJira, "A-1", "acme")
	mk(proj1, domain.RequestTypeTask, domain.RequestStatusClassifying, domain.SourceProviderJira, "A-2", "acme")
	mk(proj2, domain.RequestTypeBug, domain.RequestStatusRequestBacklog, domain.SourceProviderGithub, "9", "org/repo")
	mk(proj2, "", domain.RequestStatusNew, domain.SourceProviderManual, "", "")

	count := func(f usecase.ListFilter) int {
		res, err := env.Requests.List(ctx, f)
		if err != nil {
			t.Fatalf("list %+v: %v", f, err)
		}
		return len(res.Requests)
	}
	cases := []struct {
		name string
		f    usecase.ListFilter
		want int
	}{
		{"all", usecase.ListFilter{}, 4},
		{"project", usecase.ListFilter{ProjectID: proj1}, 2},
		{"one status", usecase.ListFilter{Statuses: []domain.RequestStatus{domain.RequestStatusNew}}, 2},
		{"two statuses", usecase.ListFilter{Statuses: []domain.RequestStatus{domain.RequestStatusNew, domain.RequestStatusClassifying}}, 3},
		{"type", usecase.ListFilter{Types: []domain.RequestType{domain.RequestTypeBug}}, 2},
		{"two types", usecase.ListFilter{Types: []domain.RequestType{domain.RequestTypeBug, domain.RequestTypeTask}}, 3},
		{"project and type", usecase.ListFilter{ProjectID: proj2, Types: []domain.RequestType{domain.RequestTypeBug}}, 1},
		{"source provider", usecase.ListFilter{SourceProvider: "jira"}, 2},
		{"source triple", usecase.ListFilter{SourceProvider: "github", SourceSite: "org/repo", SourceRef: "9"}, 1},
		{"no match", usecase.ListFilter{Statuses: []domain.RequestStatus{domain.RequestStatusCompleted}}, 0},
	}
	for _, c := range cases {
		if got := count(c.f); got != c.want {
			t.Errorf("%s: got %d rows, want %d", c.name, got, c.want)
		}
	}
}

func linkRoundTripAndSelfRejected(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	p := createRequest(t, env, ctx, nil)
	c1 := createRequest(t, env, ctx, nil)
	c2 := createRequest(t, env, ctx, nil)

	requireCode(t, env.Links.Insert(ctx, domain.RequestLink{ParentRequestID: p.ID, ChildRequestID: p.ID, Reason: domain.LinkReasonRelatesTo}), "REQUEST_LINK_SELF")
	for _, c := range []domain.Request{c1, c2} {
		if err := env.Links.Insert(ctx, domain.RequestLink{ParentRequestID: p.ID, ChildRequestID: c.ID, Reason: domain.LinkReasonBlocks}); err != nil {
			t.Fatal(err)
		}
	}
	if err := env.Links.Insert(ctx, domain.RequestLink{ParentRequestID: p.ID, ChildRequestID: c1.ID, Reason: domain.LinkReasonBlocks}); err == nil {
		t.Fatal("duplicate link must fail")
	}
	kids, err := env.Links.ListChildren(ctx, p.ID)
	if err != nil || len(kids) != 2 || kids[0].Reason != domain.LinkReasonBlocks {
		t.Fatalf("children = %+v %v", kids, err)
	}
	parents, err := env.Links.ListParents(ctx, c1.ID)
	if err != nil || len(parents) != 1 || parents[0].ParentRequestID != p.ID {
		t.Fatalf("parents = %+v %v", parents, err)
	}
	other, _ := env.Links.ListChildren(CtxForTenant(newTenant()), p.ID)
	if len(other) != 0 {
		t.Fatalf("other tenant sees %d links", len(other))
	}
	if err := env.Links.Delete(ctx, p.ID, c1.ID); err != nil {
		t.Fatal(err)
	}
	if kids, _ = env.Links.ListChildren(ctx, p.ID); len(kids) != 1 {
		t.Fatalf("after delete: %d children", len(kids))
	}
}

func typeHistoryOrderedByAt(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	r := createRequest(t, env, ctx, nil)
	actor := uuid.NewString()
	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	steps := []domain.RequestTypeChange{
		{RequestID: r.ID, At: base.Add(2 * time.Second), FromType: domain.RequestTypeBug, ToType: domain.RequestTypeHotfix, ActorID: actor, ActorKind: domain.ActorKindUser, Reason: "prod down"},
		{RequestID: r.ID, At: base, FromType: "", ToType: domain.RequestTypeBug, ActorID: actor, ActorKind: domain.ActorKindAgent},
		{RequestID: r.ID, At: base.Add(time.Second), FromType: domain.RequestTypeBug, ToType: domain.RequestTypeTask, ActorID: actor, ActorKind: domain.ActorKindSystem},
	}
	for _, s := range steps {
		if err := env.History.Append(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := env.History.List(ctx, r.ID)
	if err != nil || len(got) != 3 {
		t.Fatalf("history = %d %v", len(got), err)
	}
	want := []domain.RequestType{domain.RequestTypeBug, domain.RequestTypeTask, domain.RequestTypeHotfix}
	for i, w := range want {
		if got[i].ToType != w {
			t.Fatalf("row %d to_type = %s, want %s (ordered by at)", i, got[i].ToType, w)
		}
	}
	if got[0].FromType != "" || got[0].ActorKind != domain.ActorKindAgent || got[2].Reason != "prod down" {
		t.Errorf("fields not preserved: %+v", got)
	}
	if other, _ := env.History.List(CtxForTenant(newTenant()), r.ID); len(other) != 0 {
		t.Errorf("other tenant sees history")
	}
}

func solutionsCAS(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	tenantID, _ := tenant.TenantID(ctx)
	r := createRequest(t, env, ctx, nil)
	now := time.Now().UTC().Truncate(time.Microsecond)
	mk := func(opts string, at time.Time) domain.Solution {
		return domain.Solution{ID: uuid.NewString(), TenantID: tenantID, RequestID: r.ID, OptionsJSON: []byte(opts), Version: 1, CreatedAt: at, UpdatedAt: at}
	}
	empty := mk("", now)
	full := mk(`[{"title":"A"},{"title":"B"}]`, now.Add(time.Second))
	for _, s := range []domain.Solution{empty, full} {
		if err := env.Solutions.Insert(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := env.Solutions.Get(ctx, empty.ID)
	if err != nil || strings.ReplaceAll(string(got.OptionsJSON), " ", "") != "[]" || got.ChosenOption != nil {
		t.Fatalf("empty options round trip = %s %v %v", got.OptionsJSON, got.ChosenOption, err)
	}
	list, err := env.Solutions.ListByRequestID(ctx, r.ID)
	if err != nil || len(list) != 2 || list[0].ID != empty.ID {
		t.Fatalf("list = %d %v (want ordered by created_at)", len(list), err)
	}

	pick := 1
	full.ChosenOption = &pick
	upd, err := env.Solutions.Update(ctx, full, 1)
	if err != nil || upd.Version != 2 || upd.ChosenOption == nil || *upd.ChosenOption != 1 {
		t.Fatalf("update = %+v %v", upd, err)
	}
	_, err = env.Solutions.Update(ctx, full, 1)
	requireCode(t, err, "SOLUTION_VERSION_CONFLICT")
	missing := full
	missing.ID = uuid.NewString()
	_, err = env.Solutions.Update(ctx, missing, 1)
	requireCode(t, err, "SOLUTION_NOT_FOUND")
	_, err = env.Solutions.Get(CtxForTenant(newTenant()), full.ID)
	requireCode(t, err, "SOLUTION_NOT_FOUND")
	_, err = env.Solutions.Get(ctx, "not-a-uuid")
	requireCode(t, err, "SOLUTION_NOT_FOUND")
}

// The DB CHECK lists must accept every Go constant (the reverse direction is covered per dialect).
func everyGoConstantAcceptedByDB(t *testing.T, env Env) {
	ctx := CtxForTenant(newTenant())
	for _, typ := range domain.AllRequestTypes() {
		typ := typ
		createRequest(t, env, ctx, func(r *domain.Request) { r.Type = typ })
	}
	for _, st := range domain.AllRequestStatuses() {
		st := st
		createRequest(t, env, ctx, func(r *domain.Request) {
			r.Status = st
			if st == domain.RequestStatusRequestBacklog {
				r.ReturnedFromStage, r.ReturnedCategory = domain.ReturnStageTask, domain.ReturnCategoryOther
			}
		})
	}
	for _, rs := range []domain.ReturnStage{domain.ReturnStageClassification, domain.ReturnStageAnalysis, domain.ReturnStagePlan, domain.ReturnStagePhase, domain.ReturnStageTask} {
		rs := rs
		createRequest(t, env, ctx, func(r *domain.Request) {
			r.Status = domain.RequestStatusRequestBacklog
			r.ReturnedFromStage, r.ReturnedCategory = rs, domain.ReturnCategoryOther
		})
	}
	for _, p := range []domain.SourceProvider{domain.SourceProviderJira, domain.SourceProviderGithub, domain.SourceProviderGitlab, domain.SourceProviderLinear, domain.SourceProviderMCP, domain.SourceProviderManual, domain.SourceProviderWebhook} {
		p := p
		createRequest(t, env, ctx, func(r *domain.Request) { r.SourceProvider = p })
	}
	for _, sz := range []domain.RequestSize{domain.RequestSizeS, domain.RequestSizeM, domain.RequestSizeL} {
		sz := sz
		createRequest(t, env, ctx, func(r *domain.Request) { r.Size = sz })
	}
	for _, u := range []domain.Urgency{domain.UrgencyNormal, domain.UrgencyUrgent} {
		u := u
		createRequest(t, env, ctx, func(r *domain.Request) { r.Urgency = u })
	}
	for _, ts := range []domain.TypeSource{domain.TypeSourceAI, domain.TypeSourceHuman} {
		ts := ts
		createRequest(t, env, ctx, func(r *domain.Request) { r.TypeSource = ts })
	}
	for _, lr := range []domain.LinkReason{domain.LinkReasonRelatesTo, domain.LinkReasonBlocks, domain.LinkReasonIsBlockedBy, domain.LinkReasonDuplicates} {
		a, b := createRequest(t, env, ctx, nil), createRequest(t, env, ctx, nil)
		if err := env.Links.Insert(ctx, domain.RequestLink{ParentRequestID: a.ID, ChildRequestID: b.ID, Reason: lr}); err != nil {
			t.Fatalf("link reason %s rejected: %v", lr, err)
		}
	}
	r := createRequest(t, env, ctx, nil)
	for _, k := range []domain.ActorKind{domain.ActorKindAgent, domain.ActorKindUser, domain.ActorKindSystem} {
		if err := env.History.Append(ctx, domain.RequestTypeChange{RequestID: r.ID, ToType: domain.RequestTypeBug, ActorID: uuid.NewString(), ActorKind: k}); err != nil {
			t.Fatalf("actor kind %s rejected: %v", k, err)
		}
	}
}
