package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// changesPresence is WorktreePresence over the same change table the reservation writes, with the rule of the real stores:
// an abandoned change does not lock, any other does.
type changesPresence struct {
	changes *fakeChanges
	store   *baseRefStore
}

func (p changesPresence) Has(_ context.Context, id string) (bool, domain.BaseRefLockReason, error) {
	r, ok := p.store.read(id)
	if !ok {
		return false, domain.BaseRefUnlocked, domain.ErrRequestNotFound(id)
	}
	c, found, _ := p.changes.GetByRequest(context.Background(), id)
	reason := domain.StoredBaseRefLock(r.Status, found && c.Status != domain.ChangeStatusAbandoned)
	return reason != domain.BaseRefUnlocked, reason, nil
}

func (p changesPresence) HasMany(ctx context.Context, ids []string) (map[string]domain.BaseRefLockReason, error) {
	out := map[string]domain.BaseRefLockReason{}
	for _, id := range ids {
		if ok, reason, _ := p.Has(ctx, id); ok {
			out[id] = reason
		}
	}
	return out, nil
}

// gitStandIn is the real ProposalWorkspace as far as Ensure is concerned: it is told which ref to create from, and what
// WorktreePresence said at that moment (the worktree does not exist yet, but the marker must).
type gitStandIn struct {
	ProposalWorkspace
	presence WorktreePresence
	// during runs while "git" works, outside any transaction, like the real create.
	during func(in EnsureWorkspaceInput)
	err    error

	mu        sync.Mutex
	calls     []EnsureWorkspaceInput
	lockedAt  []bool
	reasonsAt []domain.BaseRefLockReason
}

func (g *gitStandIn) Ensure(ctx context.Context, in EnsureWorkspaceInput) (Workspace, error) {
	locked, reason, _ := g.presence.Has(ctx, in.RequestID)
	g.mu.Lock()
	g.calls = append(g.calls, in)
	g.lockedAt = append(g.lockedAt, locked)
	g.reasonsAt = append(g.reasonsAt, reason)
	g.mu.Unlock()
	if g.during != nil {
		g.during(in)
	}
	if g.err != nil {
		return Workspace{}, g.err
	}
	return Workspace{WorktreeID: "wt-" + in.RequestID, Path: "/srv/wt", Branch: domain.ProposalBranch(in.RequestNumber), BaseRef: in.BaseRef}, nil
}

type reservationFixture struct {
	store   *baseRefStore
	changes *fakeChanges
	git     *gitStandIn
	ws      *ReservingProposalWorkspace
	set     *SetRequestBaseRef
}

func newReservationFixture(t *testing.T) *reservationFixture {
	t.Helper()
	f := &reservationFixture{store: newBaseRefStore(), changes: newFakeChanges()}
	presence := changesPresence{changes: f.changes, store: f.store}
	f.git = &gitStandIn{presence: presence}
	ws, err := NewReservingProposalWorkspace(f.git, f.store, f.changes, f.store)
	if err != nil {
		t.Fatal(err)
	}
	f.ws = ws
	set, err := NewSetRequestBaseRef(SetRequestBaseRefDeps{
		Requests: f.store, Locker: f.store, Writer: f.store, Presence: presence, Checker: &fakeRefChecker{}, Tx: f.store, Outbox: f.store,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.set = set
	return f
}

func (f *reservationFixture) seed(mod func(r *domain.Request)) domain.Request {
	r := domain.Request{
		ID: uuid.NewString(), TenantID: testTenant, ProjectID: uuid.NewString(), Number: 42, Title: "T", Body: "B",
		SourceProvider: domain.SourceProviderManual, Status: domain.RequestStatusPlanning, Urgency: domain.UrgencyNormal, Version: 3,
	}
	if mod != nil {
		mod(&r)
	}
	f.store.put(r)
	return r
}

func ensureFor(r domain.Request) EnsureWorkspaceInput {
	return EnsureWorkspaceInput{TenantID: r.TenantID, ProjectID: r.ProjectID, RequestID: r.ID, RequestNumber: r.Number, ChangeID: "req-42-add-ai"}
}

func TestReservingEnsure_UsesTheStoredRefNotTheCallersCopy(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(func(r *domain.Request) { r.BaseRef = "release/x" })
	in := ensureFor(r)
	in.BaseRef = "main" // the caller's struct was read before the person changed the ref
	ws, err := f.ws.Ensure(tctx(), in)
	if err != nil || ws.BaseRef != "release/x" || f.git.calls[0].BaseRef != "release/x" {
		t.Fatalf("ws=%+v calls=%+v err=%v", ws, f.git.calls, err)
	}
}

func TestReservingEnsure_EmptyRefStaysEmptyForTheDefault(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(nil)
	if _, err := f.ws.Ensure(tctx(), ensureFor(r)); err != nil || f.git.calls[0].BaseRef != "" {
		t.Fatalf("calls=%+v err=%v", f.git.calls, err)
	}
}

func TestReservingEnsure_MarkerExistsBeforeGitIsAsked(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(nil)
	if _, err := f.ws.Ensure(tctx(), ensureFor(r)); err != nil {
		t.Fatal(err)
	}
	if !f.git.lockedAt[0] || f.git.reasonsAt[0] != domain.BaseRefLockedProposal {
		t.Fatalf("WorktreePresence must already be true when git is asked: %v %q", f.git.lockedAt, f.git.reasonsAt)
	}
	c, found, _ := f.changes.GetByRequest(tctx(), r.ID)
	if !found || c.Status != domain.ChangeStatusPreparing || c.Branch != "request/42-proposal" || c.ChangeID != "req-42-add-ai" || c.WorkspaceID != "" {
		t.Fatalf("marker = %+v", c)
	}
}

func TestReservingEnsure_ReusesAnExistingMarker(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(nil)
	existing := domain.OpenSpecChange{ID: uuid.NewString(), RequestID: r.ID, ChangeID: "req-42-add-ai", Branch: "request/42-proposal",
		Status: domain.ChangeStatusReady, SyncState: domain.SyncStateInSync, WorkspaceID: "wt-old"}
	if _, err := f.changes.Upsert(tctx(), existing); err != nil {
		t.Fatal(err)
	}
	before := f.changes.upsert
	if _, err := f.ws.Ensure(tctx(), ensureFor(r)); err != nil {
		t.Fatal(err)
	}
	got, _, _ := f.changes.GetByRequest(tctx(), r.ID)
	if f.changes.upsert != before || got.Status != domain.ChangeStatusReady || got.WorkspaceID != "wt-old" {
		t.Fatalf("a retry or recreate must not touch the marker: upserts %d->%d %+v", before, f.changes.upsert, got)
	}
}

func TestReservingEnsure_AbandonedMarkerIsReopened(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(nil)
	if _, err := f.changes.Upsert(tctx(), domain.OpenSpecChange{RequestID: r.ID, ChangeID: "req-42-add-ai", Status: domain.ChangeStatusAbandoned, SyncState: domain.SyncStateInSync}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ws.Ensure(tctx(), ensureFor(r)); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := f.changes.GetByRequest(tctx(), r.ID); got.Status != domain.ChangeStatusPreparing || !f.git.lockedAt[0] {
		t.Fatalf("a reopened marker locks again: %+v locked=%v", got, f.git.lockedAt)
	}
}

func TestReservingEnsure_FinishedRequestRecordsNothing(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(func(r *domain.Request) { r.Status = domain.RequestStatusCompleted; r.BaseRef = "release/x" })
	if _, err := f.ws.Ensure(tctx(), ensureFor(r)); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := f.changes.GetByRequest(tctx(), r.ID); found || f.git.calls[0].BaseRef != "release/x" {
		t.Fatalf("a finished Request has nothing to protect: found=%v calls=%+v", found, f.git.calls)
	}
}

func TestReservingEnsure_InvalidChangeIDFailsBeforeGit(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(nil)
	in := ensureFor(r)
	in.ChangeID = "Not A Change"
	if _, err := f.ws.Ensure(tctx(), in); !errors.Is(err, domain.ErrOpenSpecInvalidOutput) || len(f.git.calls) != 0 {
		t.Fatalf("err=%v calls=%d", err, len(f.git.calls))
	}
	if _, found, _ := f.changes.GetByRequest(tctx(), r.ID); found {
		t.Fatal("no marker for a refused reservation")
	}
}

func TestReservingEnsure_UnknownRequestAndStoreFailures(t *testing.T) {
	f := newReservationFixture(t)
	if _, err := f.ws.Ensure(tctx(), EnsureWorkspaceInput{RequestID: uuid.NewString(), ChangeID: "req-1-x", RequestNumber: 1}); codeOf(err) != "REQUEST_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
	r := f.seed(nil)
	boom := errors.New("db down")
	f.changes.err = boom
	if _, err := f.ws.Ensure(tctx(), ensureFor(r)); !errors.Is(err, boom) || len(f.git.calls) != 0 {
		t.Fatalf("a store failure must stop before git: %v calls=%d", err, len(f.git.calls))
	}
}

func TestReservingEnsure_BaseRefGone_ReleasesTheMarkerSoTheRefCanBeFixed(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(func(r *domain.Request) { r.BaseRef = "release/gone" })
	f.git.err = domain.ErrRequestBaseRefNotFound("release/gone", []string{"main"})

	_, err := f.ws.Ensure(tctx(), ensureFor(r))
	if !domain.IsBaseRefNotFound(err) {
		t.Fatalf("the typed error must reach the caller: %v", err)
	}
	if c, _, _ := f.changes.GetByRequest(tctx(), r.ID); c.Status != domain.ChangeStatusAbandoned {
		t.Fatalf("no worktree was made, so the marker is released: %+v", c)
	}
	cur, _ := f.store.read(r.ID)
	res, err := f.set.Execute(tctx(), SetRequestBaseRefInput{RequestID: r.ID, BaseRef: "main", ExpectedVersion: cur.Version, ActorID: "u", ActorKind: domain.ActorKindUser})
	if err != nil || res.Request.BaseRef != "main" {
		t.Fatalf("the person must be able to choose another ref now: %+v %v", res, err)
	}

	// The next run reserves anew and creates from the new ref.
	f.git.err = nil
	ws, err := f.ws.Ensure(tctx(), ensureFor(res.Request))
	if err != nil || ws.BaseRef != "main" {
		t.Fatalf("ws=%+v err=%v", ws, err)
	}
	if c, _, _ := f.changes.GetByRequest(tctx(), r.ID); c.Status != domain.ChangeStatusPreparing || !f.git.lockedAt[1] {
		t.Fatalf("the marker locks again: %+v %v", c, f.git.lockedAt)
	}
}

func TestReservingEnsure_OtherFailuresKeepTheMarker(t *testing.T) {
	for name, cause := range map[string]error{
		"git-gateway unavailable": domain.ErrEngineNoConnection,
		"generic create failure":  domain.ErrOpenSpecAgentFailed,
		"context cancelled":       context.Canceled,
	} {
		t.Run(name, func(t *testing.T) {
			f := newReservationFixture(t)
			r := f.seed(nil)
			f.git.err = cause
			if _, err := f.ws.Ensure(tctx(), ensureFor(r)); !errors.Is(err, cause) {
				t.Fatalf("got %v", err)
			}
			if c, _, _ := f.changes.GetByRequest(tctx(), r.ID); c.Status != domain.ChangeStatusPreparing {
				t.Fatalf("the worktree may exist, so the lock stays: %+v", c)
			}
		})
	}
}

func TestReservingEnsure_BaseRefGone_NeverAbandonsAMarkerThatHoldsAWorktree(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(nil)
	f.git.during = func(EnsureWorkspaceInput) {
		// A worktree was recorded meanwhile (an earlier attempt finishing late).
		c, _, _ := f.changes.GetByRequest(context.Background(), r.ID)
		c.WorkspaceID = "wt-late"
		_, _ = f.changes.Upsert(context.Background(), c)
	}
	f.git.err = domain.ErrRequestBaseRefNotFound("x", nil)
	if _, err := f.ws.Ensure(tctx(), ensureFor(r)); !domain.IsBaseRefNotFound(err) {
		t.Fatal(err)
	}
	if c, _, _ := f.changes.GetByRequest(tctx(), r.ID); c.Status != domain.ChangeStatusPreparing || c.WorkspaceID != "wt-late" {
		t.Fatalf("%+v", c)
	}
}

func TestReservingEnsure_ReleaseRunsEvenWhenTheCallersContextIsDone(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(nil)
	ctx, cancel := context.WithCancel(tctx())
	f.git.during = func(EnsureWorkspaceInput) { cancel() }
	f.git.err = domain.ErrRequestBaseRefNotFound("x", nil)
	if _, err := f.ws.Ensure(ctx, ensureFor(r)); !domain.IsBaseRefNotFound(err) {
		t.Fatal(err)
	}
	if c, _, _ := f.changes.GetByRequest(tctx(), r.ID); c.Status != domain.ChangeStatusAbandoned {
		t.Fatalf("the cleanup must not depend on the cancelled context: %+v", c)
	}
}

func TestReservingEnsure_ReleaseFailureIsLoggedNotReturned(t *testing.T) {
	f := newReservationFixture(t)
	r := f.seed(nil)
	f.git.during = func(EnsureWorkspaceInput) { f.changes.err = errors.New("db down") }
	f.git.err = domain.ErrRequestBaseRefNotFound("x", nil)
	if _, err := f.ws.Ensure(tctx(), ensureFor(r)); !domain.IsBaseRefNotFound(err) {
		t.Fatalf("the caller still gets the typed cause: %v", err)
	}
	f.changes.err = nil
	if c, _, _ := f.changes.GetByRequest(tctx(), r.ID); c.Status != domain.ChangeStatusPreparing {
		t.Fatalf("a marker that could not be released stays, which is the safe side: %+v", c)
	}
}

func TestReservingEnsure_OtherMethodsPassThrough(t *testing.T) {
	inner := newFakeWorkspace()
	f := newReservationFixture(t)
	ws, err := NewReservingProposalWorkspace(inner, f.store, f.changes, f.store)
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.Remove(context.Background(), Workspace{WorktreeID: "w"}); err != nil || inner.removed != 1 {
		t.Fatalf("err=%v removed=%d", err, inner.removed)
	}
}

func TestNewReservingProposalWorkspace_RefusesAMissingPort(t *testing.T) {
	f := newReservationFixture(t)
	for name, build := range map[string]func() (*ReservingProposalWorkspace, error){
		"workspace": func() (*ReservingProposalWorkspace, error) {
			return NewReservingProposalWorkspace(nil, f.store, f.changes, f.store)
		},
		"locker": func() (*ReservingProposalWorkspace, error) {
			return NewReservingProposalWorkspace(f.git, nil, f.changes, f.store)
		},
		"changes": func() (*ReservingProposalWorkspace, error) {
			return NewReservingProposalWorkspace(f.git, f.store, nil, f.store)
		},
		"tx": func() (*ReservingProposalWorkspace, error) {
			return NewReservingProposalWorkspace(f.git, f.store, f.changes, nil)
		},
	} {
		if w, err := build(); err == nil || w != nil {
			t.Errorf("%s: a missing port must stop startup", name)
		}
	}
}

// The race the reservation exists for: SetRequestBaseRef against an Ensure whose git call takes time. Either the change
// commits first and git is asked for the new ref, or Ensure's marker commits first and the change is refused. Never "the change
// succeeded but the worktree was made from the old ref".
func TestReservingEnsure_RacesSetRequestBaseRef_NeverCreatesFromAStaleRef(t *testing.T) {
	var changeFirst, ensureFirst int
	for i := 0; i < 300; i++ {
		f := newReservationFixture(t)
		r := f.seed(nil)
		f.git.during = func(EnsureWorkspaceInput) { time.Sleep(200 * time.Microsecond) }
		var setErr, ensureErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, ensureErr = f.ws.Ensure(tctx(), ensureFor(r))
		}()
		go func() {
			defer wg.Done()
			<-start
			_, setErr = f.set.Execute(tctx(), setIn(r, "release/race"))
		}()
		close(start)
		wg.Wait()
		if ensureErr != nil {
			t.Fatalf("run %d: %v", i, ensureErr)
		}
		final, _ := f.store.read(r.ID)
		used := f.git.calls[0].BaseRef
		switch {
		case setErr == nil:
			changeFirst++
			if used != "release/race" || final.BaseRef != "release/race" {
				t.Fatalf("run %d: the change succeeded but the worktree was cut from %q (stored %q)", i, used, final.BaseRef)
			}
		case codeOf(setErr) == "REQUEST_BASE_REF_LOCKED":
			ensureFirst++
			if used != "" || final.BaseRef != "" {
				t.Fatalf("run %d: the change was refused but the ref moved: used %q, stored %q", i, used, final.BaseRef)
			}
		default:
			t.Fatalf("run %d: %v", i, setErr)
		}
	}
	if changeFirst == 0 || ensureFirst == 0 {
		t.Logf("one order never happened (change first %d, ensure first %d): the scheduler decides, the invariant held in every run", changeFirst, ensureFirst)
	}
}
