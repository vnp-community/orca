package usecase

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// A worktree being created takes the Request row lock, reads the base ref and records itself. Whichever transaction gets the
// lock first, the worktree must have started from the ref that ends up stored.
func TestSetRequestBaseRef_ConcurrentWithWorktreeCreation_NoLostLock(t *testing.T) {
	var changeWon, worktreeWon int
	for i := 0; i < 200; i++ {
		f := newSetBaseRefFixture(t)
		r := f.seed(nil)
		var seen string
		var setErr, wtErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			wtErr = f.store.InTx(tctx(), func(ctx context.Context) error {
				locked, err := f.store.LockRequest(ctx, r.ID)
				if err != nil {
					return err
				}
				seen = locked.BaseRef
				f.presence.set(r.ID, domain.BaseRefLockedProposal)
				return nil
			})
		}()
		go func() {
			defer wg.Done()
			<-start
			_, setErr = f.uc.Execute(tctx(), setIn(r, "release/race"))
		}()
		close(start)
		wg.Wait()
		final, _ := f.store.read(r.ID)
		switch {
		case wtErr != nil:
			t.Fatalf("worktree transaction: %v", wtErr)
		case setErr == nil:
			changeWon++
			if seen != "release/race" || final.BaseRef != "release/race" {
				t.Fatalf("run %d: change won but the worktree saw %q (stored %q)", i, seen, final.BaseRef)
			}
		case codeOf(setErr) == "REQUEST_BASE_REF_LOCKED":
			worktreeWon++
			if seen != "" || final.BaseRef != "" {
				t.Fatalf("run %d: worktree won but the ref moved: saw %q, stored %q", i, seen, final.BaseRef)
			}
		default:
			t.Fatalf("run %d: %v", i, setErr)
		}
	}
	t.Logf("change first %d, worktree first %d", changeWon, worktreeWon)
}

func TestSetRequestBaseRef_TwoWritersSameVersion_OneWins(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = f.uc.Execute(tctx(), setIn(r, fmt.Sprintf("branch-%d", i)))
		}()
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		switch {
		case err == nil:
			wins++
		case codeOf(err) != "REQUEST_VERSION_CONFLICT":
			t.Fatalf("unexpected: %v", err)
		}
	}
	if final, _ := f.store.read(r.ID); wins != 1 || final.Version != r.Version+1 {
		t.Fatalf("wins=%d version=%d", wins, final.Version)
	}
}
