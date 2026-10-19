package contracttest

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// BaseRefLockDeps are the dialect adapters the CR-REQ-039 lock scenarios need beyond the repository Env.
type BaseRefLockDeps struct {
	LifecycleEnv
	Locker   usecase.RequestLocker
	Presence usecase.WorktreePresence
	Changes  usecase.OpenSpecChangeRepository
}

type foundChecker struct{}

func (foundChecker) Check(context.Context, string, string) (usecase.BaseRefCheck, error) {
	return usecase.BaseRefCheck{Status: usecase.BaseRefFound}, nil
}

func (d BaseRefLockDeps) setUseCase(t *testing.T) *usecase.SetRequestBaseRef {
	t.Helper()
	writer, ok := d.Requests.(usecase.RequestBaseRefWriter)
	if !ok {
		t.Fatalf("%T does not implement usecase.RequestBaseRefWriter", d.Requests)
	}
	uc, err := usecase.NewSetRequestBaseRef(usecase.SetRequestBaseRefDeps{
		Requests: d.Requests, Locker: d.Locker, Writer: writer, Presence: d.Presence, Checker: foundChecker{}, Tx: d.Tx, Outbox: d.Outbox,
	})
	if err != nil {
		t.Fatal(err)
	}
	return uc
}

func (d BaseRefLockDeps) recordChange(t *testing.T, ctx context.Context, requestID string, status domain.ChangeStatus) {
	t.Helper()
	c := fullChange(requestID)
	c.Status = status
	if _, err := d.Changes.Upsert(ctx, c); err != nil {
		t.Fatalf("record %s change: %v", status, err)
	}
}

// RunBaseRefLockContract proves the lock rules and the no-gap guarantee on a real database.
func RunBaseRefLockContract(t *testing.T, d BaseRefLockDeps) {
	t.Run("WorktreePresence", func(t *testing.T) { worktreePresenceScenarios(t, d) })
	t.Run("SetRequestBaseRefEndToEnd", func(t *testing.T) { setBaseRefEndToEnd(t, d) })
	t.Run("ConcurrentWithWorktreeCreationNoLostLock", func(t *testing.T) { setBaseRefRacesWorktree(t, d) })
	t.Run("ProposalReservation", func(t *testing.T) { RunProposalReservationContract(t, d) })
}

func worktreePresenceScenarios(t *testing.T, d BaseRefLockDeps) {
	tenantID := newTenant()
	ctx := CtxForTenant(tenantID)
	mk := func(mod func(r *domain.Request)) domain.Request { return createRequest(t, d.Env, ctx, mod) }

	plain := mk(nil)
	proposalPreparing, proposalReady, proposalArchived, proposalAbandoned := mk(nil), mk(nil), mk(nil), mk(nil)
	completed := mk(func(r *domain.Request) { r.Status = domain.RequestStatusCompleted })
	cancelled := mk(func(r *domain.Request) { r.Status = domain.RequestStatusCancelled })
	completedWithChange := mk(func(r *domain.Request) { r.Status = domain.RequestStatusCompleted })
	backlog := mk(func(r *domain.Request) {
		r.Status, r.ReturnedFromStage, r.ReturnedCategory = domain.RequestStatusRequestBacklog, domain.ReturnStageTask, domain.ReturnCategoryOther
	})
	executing := mk(func(r *domain.Request) { r.Status = domain.RequestStatusExecuting })
	executingWithChange := mk(func(r *domain.Request) { r.Status = domain.RequestStatusExecuting })
	d.recordChange(t, ctx, executingWithChange.ID, domain.ChangeStatusReady)
	d.recordChange(t, ctx, proposalPreparing.ID, domain.ChangeStatusPreparing)
	d.recordChange(t, ctx, proposalReady.ID, domain.ChangeStatusReady)
	d.recordChange(t, ctx, proposalArchived.ID, domain.ChangeStatusArchived)
	d.recordChange(t, ctx, proposalAbandoned.ID, domain.ChangeStatusAbandoned)
	d.recordChange(t, ctx, completedWithChange.ID, domain.ChangeStatusReady)

	want := map[string]domain.BaseRefLockReason{
		plain.ID: domain.BaseRefUnlocked, proposalPreparing.ID: domain.BaseRefLockedProposal, proposalReady.ID: domain.BaseRefLockedProposal,
		proposalArchived.ID: domain.BaseRefLockedProposal, proposalAbandoned.ID: domain.BaseRefUnlocked,
		completed.ID: domain.BaseRefLockedTerminal, cancelled.ID: domain.BaseRefLockedTerminal,
		completedWithChange.ID: domain.BaseRefLockedTerminal, backlog.ID: domain.BaseRefUnlocked,
		// Task worktrees appear only while executing, so the whole stage is locked before task-service has recorded any.
		executing.ID: domain.BaseRefLockedExecution, executingWithChange.ID: domain.BaseRefLockedProposal,
	}
	for id, reason := range want {
		present, got, err := d.Presence.Has(ctx, id)
		if err != nil || got != reason || present != (reason != domain.BaseRefUnlocked) {
			t.Errorf("Has(%s) = %v %q %v, want %q", id, present, got, err, reason)
		}
	}

	t.Run("HasMany answers a page in one call and skips unknown ids", func(t *testing.T) {
		ids := []string{"not-a-uuid", uuid.NewString()}
		for id := range want {
			ids = append(ids, id)
		}
		got, err := d.Presence.HasMany(ctx, ids)
		if err != nil {
			t.Fatal(err)
		}
		for id, reason := range want {
			if got[id] != reason {
				t.Errorf("HasMany[%s] = %q, want %q", id, got[id], reason)
			}
		}
		for id, reason := range got {
			if _, known := want[id]; known && reason == domain.BaseRefUnlocked {
				t.Errorf("an unlocked Request must be absent from the map: %s", id)
			}
			if !known(want, id) {
				t.Errorf("unknown id %s must not appear", id)
			}
		}
		if empty, err := d.Presence.HasMany(ctx, nil); err != nil || len(empty) != 0 {
			t.Fatalf("empty input: %v %v", empty, err)
		}
	})

	t.Run("a large page", func(t *testing.T) {
		var ids []string
		for i := 0; i < 60; i++ {
			r := mk(nil)
			if i%3 == 0 {
				d.recordChange(t, ctx, r.ID, domain.ChangeStatusReady)
			}
			ids = append(ids, r.ID)
		}
		got, err := d.Presence.HasMany(ctx, ids)
		if err != nil {
			t.Fatal(err)
		}
		for i, id := range ids {
			if (i%3 == 0) != (got[id] == domain.BaseRefLockedProposal) {
				t.Fatalf("request %d: %q", i, got[id])
			}
		}
	})

	t.Run("unknown, malformed and foreign ids", func(t *testing.T) {
		for _, id := range []string{uuid.NewString(), "abc"} {
			_, _, err := d.Presence.Has(ctx, id)
			requireCode(t, err, "REQUEST_NOT_FOUND")
		}
		_, _, err := d.Presence.Has(CtxForTenant(newTenant()), proposalReady.ID)
		requireCode(t, err, "REQUEST_NOT_FOUND")
		got, err := d.Presence.HasMany(CtxForTenant(newTenant()), []string{proposalReady.ID})
		if err != nil || len(got) != 0 {
			t.Fatalf("another tenant must see nothing: %v %v", got, err)
		}
		_, _, err = d.Presence.Has(context.Background(), plain.ID)
		requireCode(t, err, "REQUEST_TENANT_REQUIRED")
	})
}

func known(m map[string]domain.BaseRefLockReason, id string) bool { _, ok := m[id]; return ok }

func setBaseRefEndToEnd(t *testing.T, d BaseRefLockDeps) {
	uc := d.setUseCase(t)
	tenantID := newTenant()
	ctx := userCtx(tenantID)
	in := func(r domain.Request, ref string) usecase.SetRequestBaseRefInput {
		return usecase.SetRequestBaseRefInput{RequestID: r.ID, BaseRef: ref, ExpectedVersion: r.Version, ActorID: uuid.NewString(), ActorKind: domain.ActorKindUser}
	}

	t.Run("unlocked: stores the ref, bumps the version and writes one event without content", func(t *testing.T) {
		r := createRequest(t, d.Env, ctx, func(r *domain.Request) { r.Title, r.Body = "SECRET-TITLE-VALUE", "SECRET-BODY-VALUE" })
		res, err := uc.Execute(ctx, in(r, "release/1.4"))
		if err != nil || res.Request.BaseRef != "release/1.4" || res.Request.Version != r.Version+1 || res.Unverified {
			t.Fatalf("%+v %v", res, err)
		}
		got, _ := d.Requests.Get(ctx, r.ID)
		if got.BaseRef != "release/1.4" {
			t.Fatalf("not stored: %+v", got)
		}
		var events []OutboxRow
		for _, row := range d.OutboxRows(t, tenantID) {
			if row.Subject == domain.SubjectRequestBaseRefChanged {
				events = append(events, row)
			}
		}
		if len(events) != 1 {
			t.Fatalf("%d base_ref_changed events, want 1", len(events))
		}
		var payload map[string]any
		if err := json.Unmarshal(events[0].Payload, &payload); err != nil || payload["request_id"] != r.ID || payload["to"] != "release/1.4" || payload["from"] != "" {
			t.Fatalf("payload %s %v", events[0].Payload, err)
		}
		if strings.Contains(string(events[0].Payload), "SECRET") {
			t.Fatalf("the event must not carry Request content: %s", events[0].Payload)
		}
	})

	t.Run("a proposal worktree locks it and nothing changes", func(t *testing.T) {
		r := createRequest(t, d.Env, ctx, nil)
		d.recordChange(t, ctx, r.ID, domain.ChangeStatusPreparing)
		_, err := uc.Execute(ctx, in(r, "release/1.4"))
		requireLocked(t, err, "proposal")
		got, _ := d.Requests.Get(ctx, r.ID)
		if got.BaseRef != "" || got.Version != r.Version {
			t.Fatalf("a refused write must change nothing: %+v", got)
		}
	})

	t.Run("an abandoned proposal does not lock", func(t *testing.T) {
		r := createRequest(t, d.Env, ctx, nil)
		d.recordChange(t, ctx, r.ID, domain.ChangeStatusAbandoned)
		if _, err := uc.Execute(ctx, in(r, "main")); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("an executing Request is locked, whatever task-service has recorded so far", func(t *testing.T) {
		r := createRequest(t, d.Env, ctx, func(r *domain.Request) { r.Status = domain.RequestStatusExecuting })
		_, err := uc.Execute(ctx, in(r, "main"))
		requireLocked(t, err, "execution")
		got, _ := d.Requests.Get(ctx, r.ID)
		if got.BaseRef != "" || got.Version != r.Version {
			t.Fatalf("a refused write must change nothing: %+v", got)
		}
	})

	t.Run("a finished Request is locked", func(t *testing.T) {
		r := createRequest(t, d.Env, ctx, func(r *domain.Request) { r.Status = domain.RequestStatusCompleted })
		_, err := uc.Execute(ctx, in(r, "main"))
		requireLocked(t, err, "terminal")
	})

	t.Run("stale version and unknown request", func(t *testing.T) {
		r := createRequest(t, d.Env, ctx, nil)
		stale := in(r, "a")
		stale.ExpectedVersion = r.Version + 5
		_, err := uc.Execute(ctx, stale)
		requireCode(t, err, "REQUEST_VERSION_CONFLICT")
		_, err = uc.Execute(ctx, usecase.SetRequestBaseRefInput{RequestID: uuid.NewString(), BaseRef: "a", ActorID: "u", ActorKind: domain.ActorKindUser})
		requireCode(t, err, "REQUEST_NOT_FOUND")
	})
}

func requireLocked(t *testing.T, err error, reason string) {
	t.Helper()
	requireCode(t, err, "REQUEST_BASE_REF_LOCKED")
	if details, ok := domain.BaseRefDetails(err); !ok || details["reason"] != reason {
		t.Fatalf("lock reason = %v, want %s", details, reason)
	}
}

// setBaseRefRacesWorktree races SetRequestBaseRef against a transaction that does what worktree creation does: lock the
// Request, read its base ref, record the worktree. Whatever order wins, the worktree must have started from the ref that ends
// up stored, and the change must succeed only when it came first.
func setBaseRefRacesWorktree(t *testing.T, d BaseRefLockDeps) {
	uc := d.setUseCase(t)
	ctx := userCtx(newTenant())
	var changeFirst, worktreeFirst int
	for i := 0; i < 40; i++ {
		r := createRequest(t, d.Env, ctx, nil)
		var seen string
		var setErr, worktreeErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			time.Sleep(time.Duration(rand.Intn(3000)) * time.Microsecond)
			worktreeErr = d.Tx.InTx(ctx, func(txCtx context.Context) error {
				locked, err := d.Locker.LockRequest(txCtx, r.ID)
				if err != nil {
					return err
				}
				seen = locked.BaseRef
				c := fullChange(r.ID)
				c.Status = domain.ChangeStatusPreparing
				_, err = d.Changes.Upsert(txCtx, c)
				return err
			})
		}()
		go func() {
			defer wg.Done()
			<-start
			time.Sleep(time.Duration(rand.Intn(3000)) * time.Microsecond)
			_, setErr = uc.Execute(ctx, usecase.SetRequestBaseRefInput{RequestID: r.ID, BaseRef: "release/race", ActorID: uuid.NewString(), ActorKind: domain.ActorKindUser})
		}()
		close(start)
		wg.Wait()
		if worktreeErr != nil {
			t.Fatalf("iteration %d: worktree transaction: %v", i, worktreeErr)
		}
		final, err := d.Requests.Get(ctx, r.ID)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case setErr == nil:
			changeFirst++
			if final.BaseRef != "release/race" || seen != "release/race" {
				t.Fatalf("iteration %d: the change won but the worktree saw %q and %q is stored", i, seen, final.BaseRef)
			}
		case isLocked(setErr):
			worktreeFirst++
			if final.BaseRef != "" || seen != "" {
				t.Fatalf("iteration %d: the worktree won but the ref moved: saw %q, stored %q", i, seen, final.BaseRef)
			}
		default:
			t.Fatalf("iteration %d: unexpected error %v", i, setErr)
		}
	}
	t.Logf("change first: %d, worktree first: %d", changeFirst, worktreeFirst)
}

func isLocked(err error) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == "REQUEST_BASE_REF_LOCKED"
}
