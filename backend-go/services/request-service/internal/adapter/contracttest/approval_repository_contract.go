package contracttest

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// ApprovalEnv is the approval-side adapter set on top of the lifecycle environment; each dialect fills it.
type ApprovalEnv struct {
	LifecycleEnv
	Approvals usecase.ApprovalRepository
	Approvers usecase.ApprovalApproverRepository
	Policies  usecase.ApprovalPolicyRepository
	Locker    usecase.RequestLocker
}

func usec() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// seedApproval inserts a pending approval for a fresh Request of tenantID and returns both.
func seedApproval(t *testing.T, env ApprovalEnv, tenantID string, mod func(a *domain.Approval, r *domain.Request)) (domain.Approval, domain.Request) {
	t.Helper()
	ctx := tenOf(tenantID)
	r := createRequest(t, env.Env, ctx, func(r *domain.Request) {
		r.Status, r.Type, r.Size = domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeChangeRequest, domain.RequestSizeM
	})
	now := usec()
	a := domain.Approval{
		ID: uuid.NewString(), TenantID: tenantID, RequestID: r.ID, SubjectType: domain.SubjectPlan, SubjectID: uuid.NewString(),
		Stage: string(r.Status), Status: domain.ApprovalStatusPending, RequestedBy: uuid.NewString(), Version: 1,
		SubjectDigest: domain.SubjectDigest(domain.SubjectPlan, "x"), SelfApprovalAllowed: true, CreatedAt: now, UpdatedAt: now,
	}
	if mod != nil {
		mod(&a, &r)
	}
	if err := env.Approvals.Insert(ctx, a); err != nil {
		t.Fatalf("insert approval: %v", err)
	}
	return a, r
}

// RunApprovalRepositoryContract covers TASK-REQ-009-03 and the approver/claim parts of 010-03 and 010-06.
func RunApprovalRepositoryContract(t *testing.T, newEnv func(t *testing.T) ApprovalEnv) {
	env := newEnv(t)
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env ApprovalEnv)
	}{
		{"InsertGetRoundTrip", approvalInsertGetRoundTrip},
		{"DuplicatePendingAndIdempotencyKeyAreDistinguished", approvalDuplicateKeys},
		{"PendingSlotFreesAfterDecision", approvalPendingSlotFrees},
		{"ConcurrentUpdateDecisionExactlyOneWins", approvalConcurrentUpdateDecision},
		{"UpdatePendingDigestOnlyWhilePending", approvalUpdateDigestOnlyPending},
		{"CancelPendingForRequestReturnsChangedRows", approvalCancelPendingForRequest},
		{"UpdateDueAndMarkReminded", approvalUpdateDueAndMarkReminded},
		{"ListFiltersAndStablePaginationOnEqualCreatedAt", approvalListPagination},
		{"TenantIsolation", approvalTenantIsolation},
		{"ApproverSnapshotRoundTripAndIsolation", approvalApproverSnapshot},
		{"ListPendingForUserMatchesPrincipals", approvalListPendingForUser},
		{"ClaimDueAndReminderSpanTenants", approvalClaimsSpanTenants},
		{"NowDBIsDatabaseClock", approvalNowDB},
		{"DBChecksRejectUnknownValues", approvalChecksRejectUnknownValues},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

func approvalInsertGetRoundTrip(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	key := "idem-" + uuid.NewString()
	due := usec().Add(time.Hour)
	a, _ := seedApproval(t, env, tenantID, func(a *domain.Approval, _ *domain.Request) { a.IdempotencyKey, a.DueAt = &key, &due })
	got, err := env.Approvals.Get(tenOf(tenantID), tenantID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubjectType != a.SubjectType || got.SubjectID != a.SubjectID || got.Stage != a.Stage || got.Status != domain.ApprovalStatusPending ||
		got.SubjectDigest != a.SubjectDigest || !got.SelfApprovalAllowed || got.IdempotencyKey == nil || *got.IdempotencyKey != key ||
		got.DueAt == nil || !got.DueAt.Equal(due) || got.Version != 1 || !got.CreatedAt.Equal(a.CreatedAt) || got.DecidedBy != nil {
		t.Fatalf("round trip: %+v vs %+v", got, a)
	}
	if byKey, err := env.Approvals.FindByIdempotencyKey(tenOf(tenantID), tenantID, key); err != nil || byKey == nil || byKey.ID != a.ID {
		t.Fatalf("by key: %v %v", byKey, err)
	}
	if byKey, _ := env.Approvals.FindByIdempotencyKey(tenOf(tenantID), tenantID, "nope"); byKey != nil {
		t.Fatal("unknown key must find nothing")
	}
	if _, err := env.Approvals.Get(tenOf(tenantID), tenantID, uuid.NewString()); !errors.Is(err, usecase.ErrApprovalNotFound) {
		t.Fatalf("missing id = %v", err)
	}
	if _, err := env.Approvals.Get(tenOf(tenantID), tenantID, "not-a-uuid"); !errors.Is(err, usecase.ErrApprovalNotFound) {
		t.Fatalf("malformed id must be NotFound, got %v", err)
	}
}

func approvalDuplicateKeys(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	key := "k-" + uuid.NewString()
	first, r := seedApproval(t, env, tenantID, func(a *domain.Approval, _ *domain.Request) { a.IdempotencyKey = &key })
	dup := first
	dup.ID, dup.IdempotencyKey = uuid.NewString(), nil
	if err := env.Approvals.Insert(ctx, dup); !errors.Is(err, usecase.ErrPendingExists) {
		t.Fatalf("second pending for the same subject = %v", err)
	}
	other := first
	other.ID, other.SubjectID = uuid.NewString(), uuid.NewString()
	if err := env.Approvals.Insert(ctx, other); !errors.Is(err, usecase.ErrIdempotencyConflict) {
		t.Fatalf("reused idempotency key = %v", err)
	}
	other.IdempotencyKey = nil
	if err := env.Approvals.Insert(ctx, other); err != nil {
		t.Fatalf("different subject must be allowed: %v", err)
	}
	if p, err := env.Approvals.FindPendingBySubject(ctx, tenantID, domain.SubjectPlan, first.SubjectID); err != nil || p == nil || p.ID != first.ID {
		t.Fatalf("find pending: %v %v", p, err)
	}
	_ = r
}

func approvalPendingSlotFrees(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	a, _ := seedApproval(t, env, tenantID, nil)
	decided := a
	by := uuid.NewString()
	now := usec()
	decided.Status, decided.DecidedBy, decided.DecidedAt, decided.UpdatedAt, decided.Comment = domain.ApprovalStatusRejected, &by, &now, now, "no"
	if ok, err := env.Approvals.UpdateDecision(ctx, decided, 1); err != nil || !ok {
		t.Fatalf("decide: %v %v", ok, err)
	}
	again := a
	again.ID = uuid.NewString()
	if err := env.Approvals.Insert(ctx, again); err != nil {
		t.Fatalf("a closed approval must free the subject for a new pending one: %v", err)
	}
	got, _ := env.Approvals.Get(ctx, tenantID, a.ID)
	if got.Status != domain.ApprovalStatusRejected || got.Version != 2 || got.DecidedBy == nil || *got.DecidedBy != by || got.Comment != "no" || got.DecidedAt == nil {
		t.Fatalf("stored decision = %+v", got)
	}
}

func approvalConcurrentUpdateDecision(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	a, _ := seedApproval(t, env, tenantID, nil)
	const workers = 8
	wins := make(chan bool, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := a
			by := uuid.NewString()
			now := usec()
			d.Status, d.DecidedBy, d.DecidedAt, d.UpdatedAt = domain.ApprovalStatusApproved, &by, &now, now
			var ok bool
			err := retryDeadlock(func() (e error) { ok, e = env.Approvals.UpdateDecision(ctx, d, 1); return })
			if err != nil {
				t.Errorf("update: %v", err)
			}
			wins <- ok
		}()
	}
	wg.Wait()
	close(wins)
	n := 0
	for ok := range wins {
		if ok {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("exactly one UpdateDecision must win, got %d", n)
	}
}

func approvalUpdateDigestOnlyPending(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	a, _ := seedApproval(t, env, tenantID, nil)
	if ok, err := env.Approvals.UpdatePendingDigest(ctx, tenantID, a.SubjectType, a.SubjectID, "newdigest"); err != nil || !ok {
		t.Fatalf("pending digest update: %v %v", ok, err)
	}
	approved := a
	by := "u"
	now := usec()
	approved.Status, approved.DecidedBy, approved.DecidedAt, approved.UpdatedAt = domain.ApprovalStatusApproved, &by, &now, now
	if ok, _ := env.Approvals.UpdateDecision(ctx, approved, 1); !ok {
		t.Fatal("approve")
	}
	if ok, err := env.Approvals.UpdatePendingDigest(ctx, tenantID, a.SubjectType, a.SubjectID, "late"); err != nil || ok {
		t.Fatalf("an approved approval's digest is frozen: %v %v", ok, err)
	}
	got, _ := env.Approvals.Get(ctx, tenantID, a.ID)
	if got.SubjectDigest != "newdigest" {
		t.Fatalf("digest = %s", got.SubjectDigest)
	}
}

func approvalCancelPendingForRequest(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	a1, r := seedApproval(t, env, tenantID, nil)
	a2 := a1
	a2.ID, a2.SubjectType, a2.SubjectID = uuid.NewString(), domain.SubjectPhase, uuid.NewString()
	if err := env.Approvals.Insert(ctx, a2); err != nil {
		t.Fatal(err)
	}
	a3 := a1
	a3.ID, a3.SubjectID, a3.Status = uuid.NewString(), uuid.NewString(), domain.ApprovalStatusApproved
	if err := env.Approvals.Insert(ctx, a3); err != nil {
		t.Fatal(err)
	}
	now := usec()
	list, err := env.Approvals.CancelPendingForRequest(ctx, tenantID, r.ID, "returned", now)
	if err != nil || len(list) != 2 {
		t.Fatalf("cancelled %d rows: %v", len(list), err)
	}
	for _, c := range list {
		if c.Status != domain.ApprovalStatusCancelled || c.Comment != "returned" || c.Version != 2 {
			t.Fatalf("returned row must show the new state: %+v", c)
		}
	}
	for _, id := range []string{a1.ID, a2.ID} {
		got, _ := env.Approvals.Get(ctx, tenantID, id)
		if got.Status != domain.ApprovalStatusCancelled || got.Comment != "returned" || got.DecidedAt == nil {
			t.Fatalf("stored = %+v", got)
		}
	}
	if got, _ := env.Approvals.Get(ctx, tenantID, a3.ID); got.Status != domain.ApprovalStatusApproved {
		t.Fatal("decided approvals are untouched")
	}
	if again, err := env.Approvals.CancelPendingForRequest(ctx, tenantID, r.ID, "again", now); err != nil || len(again) != 0 {
		t.Fatalf("nothing left to cancel: %v %v", again, err)
	}
}

func approvalUpdateDueAndMarkReminded(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	due := usec().Add(time.Hour)
	a, _ := seedApproval(t, env, tenantID, func(a *domain.Approval, _ *domain.Request) { a.DueAt = &due })
	at := usec()
	if ok, err := env.Approvals.MarkReminded(ctx, tenantID, a.ID, at); err != nil || !ok {
		t.Fatalf("first reminder: %v %v", ok, err)
	}
	if ok, _ := env.Approvals.MarkReminded(ctx, tenantID, a.ID, at); ok {
		t.Fatal("a second reminder must lose")
	}
	got, _ := env.Approvals.Get(ctx, tenantID, a.ID)
	if got.RemindedAt == nil || got.Version != 1 {
		t.Fatalf("reminding must not bump the version: %+v", got)
	}
	ext := a
	newDue := due.Add(time.Hour)
	ext.DueAt, ext.UpdatedAt = &newDue, usec()
	if ok, err := env.Approvals.UpdateDue(ctx, ext, 1); err != nil || !ok {
		t.Fatalf("extend: %v %v", ok, err)
	}
	if ok, _ := env.Approvals.UpdateDue(ctx, ext, 1); ok {
		t.Fatal("stale version must lose")
	}
	got, _ = env.Approvals.Get(ctx, tenantID, a.ID)
	if got.RemindedAt != nil || got.Version != 2 || !got.DueAt.Equal(newDue) {
		t.Fatalf("extend must clear reminded_at and bump version: %+v", got)
	}
}

func approvalListPagination(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	same := usec()
	first, r := seedApproval(t, env, tenantID, func(a *domain.Approval, _ *domain.Request) { a.CreatedAt = same })
	ids := map[string]bool{first.ID: true}
	for i := 0; i < 4; i++ {
		a := first
		a.ID, a.SubjectID = uuid.NewString(), uuid.NewString()
		if i == 3 {
			a.Status, a.SubjectType = domain.ApprovalStatusRejected, domain.SubjectSolution
		}
		if err := env.Approvals.Insert(ctx, a); err != nil {
			t.Fatal(err)
		}
		ids[a.ID] = true
	}
	var seen []string
	token := ""
	for page := 0; page < 10; page++ {
		list, next, err := env.Approvals.List(ctx, tenantID, usecase.ApprovalListFilter{RequestID: r.ID, PageSize: 2, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range list {
			seen = append(seen, a.ID)
		}
		if next == "" {
			break
		}
		token = next
	}
	if len(seen) != 5 {
		t.Fatalf("paging over equal created_at lost or repeated rows: %v", seen)
	}
	uniq := map[string]bool{}
	for _, id := range seen {
		if !ids[id] || uniq[id] {
			t.Fatalf("unexpected or duplicate id %s in %v", id, seen)
		}
		uniq[id] = true
	}
	if !sort.SliceIsSorted(seen, func(i, j int) bool { return seen[i] > seen[j] }) {
		t.Fatalf("ties must order by id descending: %v", seen)
	}
	pend, _, _ := env.Approvals.List(ctx, tenantID, usecase.ApprovalListFilter{RequestID: r.ID, Status: domain.ApprovalStatusPending, PageSize: 50})
	sol, _, _ := env.Approvals.List(ctx, tenantID, usecase.ApprovalListFilter{RequestID: r.ID, SubjectType: domain.SubjectSolution, PageSize: 50})
	if len(pend) != 4 || len(sol) != 1 {
		t.Fatalf("filters: pending=%d solution=%d", len(pend), len(sol))
	}
	if _, _, err := env.Approvals.List(ctx, tenantID, usecase.ApprovalListFilter{PageSize: 2, PageToken: "garbage"}); err == nil {
		t.Fatal("a bad page token must be rejected")
	}
}

func approvalTenantIsolation(t *testing.T, env ApprovalEnv) {
	a, r := seedApproval(t, env, newTenant(), nil)
	other := newTenant()
	octx := tenOf(other)
	if _, err := env.Approvals.Get(octx, other, a.ID); !errors.Is(err, usecase.ErrApprovalNotFound) {
		t.Fatalf("Get across tenants = %v", err)
	}
	if _, err := env.Approvals.Get(octx, a.TenantID, a.ID); !errors.Is(err, usecase.ErrApprovalNotFound) {
		t.Fatalf("asking for another tenant's id under my ctx = %v", err)
	}
	if list, _, _ := env.Approvals.List(octx, other, usecase.ApprovalListFilter{RequestID: r.ID, PageSize: 10}); len(list) != 0 {
		t.Fatalf("List leaked %d rows", len(list))
	}
	d := a
	by := "x"
	now := usec()
	d.Status, d.DecidedBy, d.DecidedAt, d.UpdatedAt, d.TenantID = domain.ApprovalStatusApproved, &by, &now, now, other
	if ok, _ := env.Approvals.UpdateDecision(octx, d, 1); ok {
		t.Fatal("UpdateDecision crossed tenants")
	}
	if got, _ := env.Approvals.CancelPendingForRequest(octx, other, r.ID, "x", now); len(got) != 0 {
		t.Fatal("CancelPendingForRequest crossed tenants")
	}
	foreign := a
	foreign.ID, foreign.SubjectID = uuid.NewString(), uuid.NewString()
	if err := env.Approvals.Insert(octx, foreign); err == nil {
		t.Fatal("inserting a row of another tenant under my ctx must fail")
	}
	if err := env.Approvals.Insert(context.Background(), foreign); err == nil {
		t.Fatal("no tenant in ctx must fail")
	}
}

func approvalApproverSnapshot(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	a, _ := seedApproval(t, env, tenantID, nil)
	snap := []domain.Principal{{Kind: domain.PrincipalKindReporter}, {Kind: domain.PrincipalKindUser, ID: "u1"}, {Kind: domain.PrincipalKindTeam, ID: "t1"}, {Kind: domain.PrincipalKindRole, ID: "admin"}}
	if err := env.Approvers.InsertSnapshot(ctx, a.ID, tenantID, snap); err != nil {
		t.Fatal(err)
	}
	if err := env.Approvers.InsertSnapshot(ctx, a.ID, tenantID, snap[:1]); err != nil {
		t.Fatalf("re-inserting the same principal must be harmless: %v", err)
	}
	got, err := env.Approvers.ListForApproval(ctx, tenantID, a.ID)
	if err != nil || len(got) != 4 {
		t.Fatalf("%v %v", got, err)
	}
	kinds := map[domain.PrincipalKind]string{}
	for _, p := range got {
		kinds[p.Kind] = p.ID
	}
	if kinds[domain.PrincipalKindUser] != "u1" || kinds[domain.PrincipalKindTeam] != "t1" || kinds[domain.PrincipalKindRole] != "admin" {
		t.Fatalf("snapshot = %v", got)
	}
	other := newTenant()
	if got, _ := env.Approvers.ListForApproval(tenOf(other), other, a.ID); len(got) != 0 {
		t.Fatal("snapshot leaked across tenants")
	}
}

func approvalListPendingForUser(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	u1, outsider := uuid.NewString(), uuid.NewString()
	mk := func(snap []domain.Principal, mod func(a *domain.Approval, r *domain.Request)) domain.Approval {
		a, _ := seedApproval(t, env, tenantID, mod)
		if err := env.Approvers.InsertSnapshot(ctx, a.ID, tenantID, snap); err != nil {
			t.Fatal(err)
		}
		return a
	}
	byUser := mk([]domain.Principal{{Kind: domain.PrincipalKindUser, ID: u1}}, nil)
	byTeam := mk([]domain.Principal{{Kind: domain.PrincipalKindTeam, ID: "team-7"}}, nil)
	byRole := mk([]domain.Principal{{Kind: domain.PrincipalKindRole, ID: "admin"}}, nil)
	byReporter := mk([]domain.Principal{{Kind: domain.PrincipalKindReporter}}, func(a *domain.Approval, r *domain.Request) {})
	blocked := mk([]domain.Principal{{Kind: domain.PrincipalKindUser, ID: u1}}, func(a *domain.Approval, r *domain.Request) {
		a.SelfApprovalAllowed, a.RequestedBy = false, u1
	})
	past := usec().Add(-time.Hour)
	overdue := mk([]domain.Principal{{Kind: domain.PrincipalKindUser, ID: u1}}, func(a *domain.Approval, r *domain.Request) { a.DueAt = &past })
	closed := mk([]domain.Principal{{Kind: domain.PrincipalKindUser, ID: u1}}, func(a *domain.Approval, r *domain.Request) { a.Status = domain.ApprovalStatusRejected })

	ids := func(f usecase.PendingForUserFilter) map[string]usecase.PendingApproval {
		f.PageSize = 50
		list, _, err := env.Approvals.ListPendingForUser(ctx, tenantID, f)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]usecase.PendingApproval{}
		for _, p := range list {
			out[p.ID] = p
		}
		return out
	}
	got := ids(usecase.PendingForUserFilter{UserID: u1, Role: "user", TeamIDs: []string{"team-7"}})
	if len(got) != 2 || got[byUser.ID].ID == "" || got[byTeam.ID].ID == "" {
		t.Fatalf("user + team member must see exactly their two: %v", keysOf(got))
	}
	if p := got[byUser.ID]; p.RequestTitle == "" || p.RequestNumber == 0 {
		t.Fatalf("inbox rows carry request columns: %+v", p)
	}
	if got := ids(usecase.PendingForUserFilter{UserID: outsider, Role: "user"}); len(got) != 0 {
		t.Fatalf("outsider = %v", keysOf(got))
	}
	adminSees := ids(usecase.PendingForUserFilter{UserID: outsider, Role: "admin"})
	want := map[string]bool{byUser.ID: true, byTeam.ID: true, byRole.ID: true, byReporter.ID: true, blocked.ID: true}
	if len(adminSees) != len(want) {
		t.Fatalf("admin sees every live pending approval: %v", keysOf(adminSees))
	}
	for id := range want {
		if adminSees[id].ID == "" {
			t.Fatalf("admin misses %s", id)
		}
	}
	if adminSees[overdue.ID].ID != "" || adminSees[closed.ID].ID != "" {
		t.Fatal("overdue and closed approvals are not inbox items")
	}
	if got := ids(usecase.PendingForUserFilter{UserID: u1, Role: "admin"}); got[blocked.ID].ID != "" {
		t.Fatal("separation of duties applies to admins too")
	}
	if got := ids(usecase.PendingForUserFilter{UserID: outsider, Role: "role-x", SubjectType: domain.SubjectSolution}); len(got) != 0 {
		t.Fatalf("subject filter: %v", keysOf(got))
	}
	// reporter principal: the Request's reporter sees it
	rep, err := env.Requests.Get(ctx, byReporter.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(usecase.PendingForUserFilter{UserID: rep.ReporterID, Role: "user"}); got[byReporter.ID].ID == "" {
		t.Fatal("reporter principal must match requests.reporter_id")
	}
	// pagination over the inbox
	var all []string
	token := ""
	for page := 0; page < 10; page++ {
		list, next, err := env.Approvals.ListPendingForUser(ctx, tenantID, usecase.PendingForUserFilter{UserID: outsider, Role: "admin", PageSize: 2, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range list {
			all = append(all, p.ID)
		}
		if next == "" {
			break
		}
		token = next
	}
	if len(all) != len(want) {
		t.Fatalf("paged inbox = %d rows, want %d", len(all), len(want))
	}
}

func keysOf(m map[string]usecase.PendingApproval) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func approvalClaimsSpanTenants(t *testing.T, env ApprovalEnv) {
	tenantA, tenantB := newTenant(), newTenant()
	past, future := usec().Add(-time.Minute), usec().Add(20*time.Second)
	dueA, _ := seedApproval(t, env, tenantA, func(a *domain.Approval, _ *domain.Request) { a.DueAt = &past })
	dueB, _ := seedApproval(t, env, tenantB, func(a *domain.Approval, _ *domain.Request) { a.DueAt = &past })
	notDue, _ := seedApproval(t, env, tenantA, func(a *domain.Approval, _ *domain.Request) { a.DueAt = &future })
	noDue, _ := seedApproval(t, env, tenantA, nil)
	remind, _ := seedApproval(t, env, tenantB, func(a *domain.Approval, _ *domain.Request) {
		c := usec().Add(-100 * time.Second)
		a.CreatedAt, a.DueAt = c, &future // 75% of the window is already behind us
	})
	already := usec()
	reminded, _ := seedApproval(t, env, tenantB, func(a *domain.Approval, _ *domain.Request) {
		c := usec().Add(-100 * time.Second)
		a.CreatedAt, a.DueAt, a.RemindedAt = c, &future, &already
	})

	claims, err := env.Approvals.ClaimDue(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range claims {
		got[c.ApprovalID] = c.TenantID
	}
	if got[dueA.ID] != tenantA || got[dueB.ID] != tenantB {
		t.Fatalf("ClaimDue must return both tenants' overdue approvals with their tenant: %v", got)
	}
	if got[notDue.ID] != "" || got[noDue.ID] != "" || got[remind.ID] != "" {
		t.Fatal("ClaimDue returned approvals that are not due")
	}
	for _, c := range claims {
		if c.RequestID == "" {
			t.Fatal("claims carry the request id for the Request-first lock")
		}
	}
	rem, err := env.Approvals.ClaimDueForReminder(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range rem {
		ids[c.ApprovalID] = true
	}
	if !ids[remind.ID] || ids[reminded.ID] || ids[dueA.ID] || ids[notDue.ID] || ids[noDue.ID] {
		t.Fatalf("reminder claims = %v", ids)
	}
	if limited, _ := env.Approvals.ClaimDue(context.Background(), 1); len(limited) != 1 {
		t.Fatalf("batch limit ignored: %d", len(limited))
	}
	// the tenant switch the sweeper performs: the claim's tenant makes the row readable and lockable
	for _, c := range claims {
		if c.ApprovalID != dueA.ID {
			continue
		}
		tctx := tenant.WithTenantID(context.Background(), c.TenantID)
		err := env.Tx.InTx(tctx, func(txCtx context.Context) error {
			if _, err := env.Locker.LockRequest(txCtx, c.RequestID); err != nil {
				return err
			}
			_, err := env.Approvals.GetForUpdate(txCtx, c.TenantID, c.ApprovalID)
			return err
		})
		if err != nil {
			t.Fatalf("locking a claim under its own tenant: %v", err)
		}
	}
}

func approvalNowDB(t *testing.T, env ApprovalEnv) {
	got, err := env.Approvals.NowDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(got); d > time.Minute || d < -time.Minute {
		t.Fatalf("database clock is %v away from this process", d)
	}
	if got.Location() != time.UTC {
		t.Fatalf("NowDB must be UTC, got %v", got.Location())
	}
}

func approvalChecksRejectUnknownValues(t *testing.T, env ApprovalEnv) {
	tenantID := newTenant()
	ctx := tenOf(tenantID)
	a, r := seedApproval(t, env, tenantID, nil)
	for name, mod := range map[string]func(x *domain.Approval){
		"subject_type": func(x *domain.Approval) { x.SubjectType = "bogus" },
		"status":       func(x *domain.Approval) { x.Status = "bogus" },
	} {
		bad := a
		bad.ID, bad.SubjectID = uuid.NewString(), uuid.NewString()
		mod(&bad)
		err := env.Approvals.Insert(ctx, bad)
		if err == nil || errors.Is(err, usecase.ErrPendingExists) || errors.Is(err, usecase.ErrIdempotencyConflict) {
			t.Errorf("%s: the database must reject it with a check violation, got %v", name, err)
		}
	}
	for name, mod := range map[string]func(p *domain.ApprovalPolicy){
		"size":    func(p *domain.ApprovalPolicy) { p.Size = strPtr("XL") },
		"urgency": func(p *domain.ApprovalPolicy) { p.Urgency = strPtr("asap") },
		"subject": func(p *domain.ApprovalPolicy) { p.SubjectType = "bogus" },
	} {
		if _, err := env.Policies.Upsert(ctx, policyOf(tenantID, mod), 0); err == nil {
			t.Errorf("policy %s: unknown value accepted", name)
		}
	}
	if list, _, _ := env.Approvals.List(ctx, tenantID, usecase.ApprovalListFilter{RequestID: r.ID, PageSize: 10}); len(list) != 1 {
		t.Errorf("rejected rows must not be stored, found %d", len(list))
	}
}
