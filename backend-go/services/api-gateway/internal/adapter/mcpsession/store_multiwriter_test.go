package mcpsession

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

func ordinalsOf(t *testing.T, s *JetStreamEventStore, sess, stream string, after int) []int {
	t.Helper()
	var got []int
	entries, err := s.log.Snapshot(context.Background(), subjectFor(sess, stream))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if i, ok := entryIdx(e); ok && i > after {
			got = append(got, i)
		}
	}
	return got
}

// Two replicas that both hold a session append to its standalone stream. Their
// local counters would both start at 0 and collide; the store must hand out
// unique, increasing ordinals instead.
func TestTwoReplicasAppendingToOneStreamGetUniqueIncreasingOrdinals(t *testing.T) {
	log := newFakeLog(100)
	a, b := NewJetStreamEventStore(log, 0), NewJetStreamEventStore(log, 0)
	ctx := context.Background()
	if err := a.Open(ctx, "sess", ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ { // strictly alternating writers
		w := a
		if i%2 == 1 {
			w = b
		}
		if err := w.Append(ctx, "sess", "", []byte(fmt.Sprintf(`{"n":%d}`, i))); err != nil {
			t.Fatal(err)
		}
	}
	got := ordinalsOf(t, a, "sess", "", -1)
	for i, o := range got {
		if o != i {
			t.Fatalf("ordinals = %v, want 0..5 without duplicates", got)
		}
	}
	if len(got) != 6 {
		t.Fatalf("ordinals = %v", got)
	}

	// Resume after ordinal 2 on the other replica gets exactly 3,4,5 in order.
	var seen []int
	err := b.Follow(ctx, "sess", "", 2, func(i int, d []byte) (bool, error) {
		seen = append(seen, i)
		return i == 5, nil
	})
	if err != nil || fmt.Sprint(seen) != "[3 4 5]" {
		t.Fatalf("resume on the other replica: %v %v", seen, err)
	}
}

func TestConcurrentWritersOnTwoReplicasNeverDuplicateAnOrdinal(t *testing.T) {
	log := newFakeLog(1000)
	a, b := NewJetStreamEventStore(log, 0), NewJetStreamEventStore(log, 0)
	ctx := context.Background()
	var wg sync.WaitGroup
	const perWriter = 25
	for _, w := range []*JetStreamEventStore{a, b, a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := w.Append(ctx, "sess", "", []byte(`{"jsonrpc":"2.0","method":"notifications/x"}`)); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	got := ordinalsOf(t, a, "sess", "", -1)
	if len(got) != 4*perWriter || !sort.IntsAreSorted(got) {
		t.Fatalf("stored %d ordinals (want %d), sorted=%v", len(got), 4*perWriter, sort.IntsAreSorted(got))
	}
	for i, o := range got {
		if o != i {
			t.Fatalf("ordinal %d at position %d: duplicates or holes: %v", o, i, got)
		}
	}
}

// A purge by one replica (new standalone stream generation) must not make the
// other replica reuse a stale tail or an old ordinal.
func TestOpenByOneReplicaResetsTheStreamForTheOther(t *testing.T) {
	log := newFakeLog(100)
	a, b := NewJetStreamEventStore(log, 0), NewJetStreamEventStore(log, 0)
	ctx := context.Background()
	_ = a.Open(ctx, "sess", "")
	for i := 0; i < 3; i++ {
		_ = b.Append(ctx, "sess", "", []byte(`{"n":1}`))
	}
	if err := a.Open(ctx, "sess", ""); err != nil { // client reconnected its GET stream on replica A
		t.Fatal(err)
	}
	if err := b.Append(ctx, "sess", "", []byte(`{"n":2}`)); err != nil { // B still caches the old tail
		t.Fatal(err)
	}
	if got := ordinalsOf(t, a, "sess", "", -1); fmt.Sprint(got) != "[0]" {
		t.Fatalf("new generation must restart at 0, got %v", got)
	}
}

// The bound still produces a gap (404), also when two replicas wrote the stream.
func TestGapIsStillReportedWhenTwoReplicasOverflowTheBoundedBuffer(t *testing.T) {
	log := newFakeLog(3)
	a, b := NewJetStreamEventStore(log, 0), NewJetStreamEventStore(log, 0)
	ctx := context.Background()
	for i := 0; i < 8; i++ {
		w := a
		if i%2 == 1 {
			w = b
		}
		_ = w.Append(ctx, "sess", "", []byte(`{"n":1}`))
	}
	// Retained ordinals: 5,6,7.
	if err := b.Check(ctx, "sess", "", 4); err != nil {
		t.Errorf("resume from 4 needs 5..7 which are retained: %v", err)
	}
	if err := a.Check(ctx, "sess", "", 2); !errors.Is(err, mcpserver.ErrResumeGap) {
		t.Errorf("resume from 2 needs dropped events: %v", err)
	}
}

type failingAppendLog struct{ *fakeLog }

func (failingAppendLog) AppendAfter(context.Context, string, uint64, map[string]string, []byte) (uint64, error) {
	return 0, errors.New("nats down")
}

func TestAppendSurfacesNonConflictErrors(t *testing.T) {
	s := NewJetStreamEventStore(failingAppendLog{newFakeLog(3)}, 0)
	if err := s.Append(context.Background(), "x", "S", []byte(`{}`)); err == nil {
		t.Fatal("a broken log must fail the append, not loop")
	}
}
