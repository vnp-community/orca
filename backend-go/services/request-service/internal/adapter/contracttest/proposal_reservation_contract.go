package contracttest

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// reservationGit stands in for git-gateway behind ReservingProposalWorkspace. It asks the real WorktreePresence what the
// database says at the moment "git" is asked, before any worktree exists.
type reservationGit struct {
	usecase.ProposalWorkspace
	presence usecase.WorktreePresence
	// during simulates git's latency, outside any transaction.
	during func()
	err    error

	mu      sync.Mutex
	seenRef string
	locked  bool
	reason  domain.BaseRefLockReason
}

func (g *reservationGit) Ensure(ctx context.Context, in usecase.EnsureWorkspaceInput) (usecase.Workspace, error) {
	locked, reason, err := g.presence.Has(ctx, in.RequestID)
	g.mu.Lock()
	g.seenRef, g.locked, g.reason = in.BaseRef, locked && err == nil, reason
	g.mu.Unlock()
	if g.during != nil {
		g.during()
	}
	if g.err != nil {
		return usecase.Workspace{}, g.err
	}
	return usecase.Workspace{WorktreeID: "wt-" + in.RequestID, Branch: domain.ProposalBranch(in.RequestNumber), BaseRef: in.BaseRef}, nil
}

func (d BaseRefLockDeps) reserving(t *testing.T, git *reservationGit) *usecase.ReservingProposalWorkspace {
	t.Helper()
	git.presence = d.Presence
	ws, err := usecase.NewReservingProposalWorkspace(git, d.Locker, d.Changes, d.Tx)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func ensureInput(r domain.Request) usecase.EnsureWorkspaceInput {
	return usecase.EnsureWorkspaceInput{TenantID: r.TenantID, ProjectID: r.ProjectID, RequestID: r.ID, RequestNumber: r.Number, ChangeID: "req-1-add-ai"}
}

// RunProposalReservationContract proves, on a real database, that creating the proposal worktree and changing the base ref
// cannot interleave: the marker is committed before git is asked, the ref comes from the locked row, and a creation that failed
// because the ref is gone leaves nothing that freezes it.
func RunProposalReservationContract(t *testing.T, d BaseRefLockDeps) {
	t.Run("MarkerIsCommittedBeforeGitIsAsked", func(t *testing.T) { reservationMarkerFirst(t, d) })
	t.Run("RacesSetRequestBaseRef", func(t *testing.T) { reservationRacesSet(t, d) })
	t.Run("BaseRefGoneReleasesTheMarkerAndTheNextRunReservesAgain", func(t *testing.T) { reservationReleasedOnMissingRef(t, d) })
	t.Run("OtherFailuresKeepTheMarker", func(t *testing.T) { reservationKeptOnOutage(t, d) })
	t.Run("RetryReusesTheMarkerAndFinishedRequestsRecordNothing", func(t *testing.T) { reservationReuseAndTerminal(t, d) })
}

func reservationMarkerFirst(t *testing.T, d BaseRefLockDeps) {
	ctx := userCtx(newTenant())
	r := createRequest(t, d.Env, ctx, func(r *domain.Request) { r.BaseRef = "release/x" })
	git := &reservationGit{}
	in := ensureInput(r)
	in.BaseRef = "stale-value-from-a-struct-read-earlier"
	if _, err := d.reserving(t, git).Ensure(ctx, in); err != nil {
		t.Fatal(err)
	}
	if !git.locked || git.reason != domain.BaseRefLockedProposal || git.seenRef != "release/x" {
		t.Fatalf("at the moment git was asked: locked=%v reason=%q ref=%q", git.locked, git.reason, git.seenRef)
	}
	c, found, err := d.Changes.GetByRequest(ctx, r.ID)
	if err != nil || !found || c.Status != domain.ChangeStatusPreparing || c.WorkspaceID != "" || c.Branch != domain.ProposalBranch(r.Number) || c.ChangeID != "req-1-add-ai" {
		t.Fatalf("marker = %+v found=%v err=%v", c, found, err)
	}
	_, err = d.setUseCase(t).Execute(ctx, usecase.SetRequestBaseRefInput{RequestID: r.ID, BaseRef: "main", ActorID: uuid.NewString(), ActorKind: domain.ActorKindUser})
	requireLocked(t, err, "proposal")
}

// reservationRacesSet is the property the whole design exists for, on the real database: either the change commits first and
// git is asked for the new ref, or the marker commits first and the change is refused. Never "changed, but cut from the old ref".
func reservationRacesSet(t *testing.T, d BaseRefLockDeps) {
	uc := d.setUseCase(t)
	ctx := userCtx(newTenant())
	var changeFirst, ensureFirst int
	for i := 0; i < 40; i++ {
		r := createRequest(t, d.Env, ctx, nil)
		git := &reservationGit{during: func() { time.Sleep(time.Duration(rand.Intn(2000)) * time.Microsecond) }}
		ws := d.reserving(t, git)
		var setErr, ensureErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			time.Sleep(time.Duration(rand.Intn(3000)) * time.Microsecond)
			_, ensureErr = ws.Ensure(ctx, ensureInput(r))
		}()
		go func() {
			defer wg.Done()
			<-start
			time.Sleep(time.Duration(rand.Intn(3000)) * time.Microsecond)
			_, setErr = uc.Execute(ctx, usecase.SetRequestBaseRefInput{RequestID: r.ID, BaseRef: "release/race", ActorID: uuid.NewString(), ActorKind: domain.ActorKindUser})
		}()
		close(start)
		wg.Wait()
		if ensureErr != nil {
			t.Fatalf("iteration %d: Ensure: %v", i, ensureErr)
		}
		final, err := d.Requests.Get(ctx, r.ID)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case setErr == nil:
			changeFirst++
			if git.seenRef != "release/race" || final.BaseRef != "release/race" {
				t.Fatalf("iteration %d: the change succeeded but git was asked for %q (stored %q)", i, git.seenRef, final.BaseRef)
			}
		case isLocked(setErr):
			ensureFirst++
			if git.seenRef != "" || final.BaseRef != "" {
				t.Fatalf("iteration %d: the change was refused but the ref moved: git %q, stored %q", i, git.seenRef, final.BaseRef)
			}
		default:
			t.Fatalf("iteration %d: unexpected error %v", i, setErr)
		}
	}
	t.Logf("change first: %d, reservation first: %d", changeFirst, ensureFirst)
}

func reservationReleasedOnMissingRef(t *testing.T, d BaseRefLockDeps) {
	uc := d.setUseCase(t)
	ctx := userCtx(newTenant())
	r := createRequest(t, d.Env, ctx, func(r *domain.Request) { r.BaseRef = "release/gone" })
	git := &reservationGit{err: domain.ErrRequestBaseRefNotFound("release/gone", []string{"main"})}
	ws := d.reserving(t, git)

	if _, err := ws.Ensure(ctx, ensureInput(r)); !domain.IsBaseRefNotFound(err) {
		t.Fatalf("the typed error must reach the caller: %v", err)
	}
	c, _, _ := d.Changes.GetByRequest(ctx, r.ID)
	present, _, err := d.Presence.Has(ctx, r.ID)
	if c.Status != domain.ChangeStatusAbandoned || present || err != nil {
		t.Fatalf("no worktree exists, so nothing may freeze the ref: change=%+v present=%v err=%v", c, present, err)
	}
	cur, _ := d.Requests.Get(ctx, r.ID)
	res, err := uc.Execute(ctx, usecase.SetRequestBaseRefInput{RequestID: r.ID, BaseRef: "main", ExpectedVersion: cur.Version, ActorID: uuid.NewString(), ActorKind: domain.ActorKindUser})
	if err != nil || res.Request.BaseRef != "main" {
		t.Fatalf("the person must be able to pick another ref: %+v %v", res, err)
	}

	git.err = nil
	if _, err := ws.Ensure(ctx, ensureInput(res.Request)); err != nil {
		t.Fatal(err)
	}
	c, _, _ = d.Changes.GetByRequest(ctx, r.ID)
	if git.seenRef != "main" || !git.locked || c.Status != domain.ChangeStatusPreparing {
		t.Fatalf("the next run reserves again and cuts from the new ref: ref=%q locked=%v change=%+v", git.seenRef, git.locked, c)
	}
}

func reservationKeptOnOutage(t *testing.T, d BaseRefLockDeps) {
	ctx := userCtx(newTenant())
	r := createRequest(t, d.Env, ctx, nil)
	git := &reservationGit{err: domain.ErrEngineNoConnection}
	if _, err := d.reserving(t, git).Ensure(ctx, ensureInput(r)); err == nil {
		t.Fatal("the outage must be returned")
	}
	if present, reason, err := d.Presence.Has(ctx, r.ID); err != nil || !present || reason != domain.BaseRefLockedProposal {
		t.Fatalf("the worktree may exist, so the lock stays: %v %q %v", present, reason, err)
	}
}

func reservationReuseAndTerminal(t *testing.T, d BaseRefLockDeps) {
	ctx := userCtx(newTenant())
	r := createRequest(t, d.Env, ctx, nil)
	ready := fullChange(r.ID)
	ready.Status, ready.ChangeID, ready.Branch = domain.ChangeStatusReady, "req-1-add-ai", domain.ProposalBranch(r.Number)
	stored, err := d.Changes.Upsert(ctx, ready)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.reserving(t, &reservationGit{}).Ensure(ctx, ensureInput(r)); err != nil {
		t.Fatal(err)
	}
	after, _, _ := d.Changes.GetByRequest(ctx, r.ID)
	if after.Version != stored.Version || after.Status != domain.ChangeStatusReady || after.WorkspaceID != stored.WorkspaceID {
		t.Fatalf("a recreate must leave the existing row alone: %+v vs %+v", after, stored)
	}

	done := createRequest(t, d.Env, ctx, func(r *domain.Request) { r.Status = domain.RequestStatusCompleted; r.BaseRef = "release/x" })
	git := &reservationGit{}
	if _, err := d.reserving(t, git).Ensure(ctx, ensureInput(done)); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := d.Changes.GetByRequest(ctx, done.ID); found || git.seenRef != "release/x" {
		t.Fatalf("a finished Request records nothing and still reads its ref: found=%v ref=%q", found, git.seenRef)
	}
}
