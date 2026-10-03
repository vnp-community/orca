//go:build integration

package mcpsession_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpsession"
)

// Two replicas = two connections and two stores over the SAME real MCPSSE
// stream. Their appends to one session's standalone stream must never reuse an
// ordinal, and a resume on either replica sees the other's events.
func TestTwoReplicasAppendingToTheSameStreamThenResuming(t *testing.T) {
	url := testutil.StartNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mk := func() *mcpsession.JetStreamEventStore {
		l, err := mcpsession.NewLog(ctx, url, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(l.Close)
		return mcpsession.NewJetStreamEventStore(l, 0)
	}
	a, b := mk(), mk()
	const sess = "session-secret-xyz"
	if err := a.Open(ctx, sess, ""); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	const perWriter = 20
	for _, w := range []*mcpsession.JetStreamEventStore{a, b, a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := w.Append(ctx, sess, "", []byte(`{"jsonrpc":"2.0","method":"notifications/message"}`)); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	total := 4 * perWriter
	var ords []int
	fctx, fcancel := context.WithTimeout(ctx, 20*time.Second)
	defer fcancel()
	err := b.Follow(fctx, sess, "", -1, func(i int, _ []byte) (bool, error) {
		ords = append(ords, i)
		return len(ords) == total, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sort.IntsAreSorted(ords) {
		t.Fatalf("not increasing: %v", ords)
	}
	for i, o := range ords {
		if o != i {
			t.Fatalf("duplicate or missing ordinal at %d: %v", i, ords)
		}
	}

	// Resume on the OTHER replica from the middle: exactly the rest, in order.
	var rest []int
	if err := a.Follow(fctx, sess, "", total-4, func(i int, _ []byte) (bool, error) {
		rest = append(rest, i)
		return i == total-1, nil
	}); err != nil || fmt.Sprint(rest) != fmt.Sprint([]int{total - 3, total - 2, total - 1}) {
		t.Fatalf("resume: %v %v", rest, err)
	}

	// The buffer is bounded: overflow it from both replicas and the old start is a 404-gap.
	for i := 0; i < mcpsession.DefaultMaxMsgsPerStream+10; i++ {
		w := a
		if i%2 == 1 {
			w = b
		}
		if err := w.Append(ctx, sess, "", []byte(`{"jsonrpc":"2.0","method":"notifications/message"}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Check(ctx, sess, "", 3); !errors.Is(err, mcpserver.ErrResumeGap) {
		t.Fatalf("resume from a dropped ordinal must stay a gap, got %v", err)
	}
	if err := a.Purge(ctx, sess); err != nil {
		t.Fatal(err)
	}
}
