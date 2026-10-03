//go:build integration

package eventbus_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/testutil"
)

func TestBoundedLog_AppendAfterIsACompareAndSetOnTheSubjectTail(t *testing.T) {
	url := testutil.StartNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	l, err := eventbus.NewBoundedLog(ctx, url, eventbus.BoundedLogConfig{
		Stream: "TESTCAS", Subjects: []string{"orca.sse.cas.>"}, MaxMsgsPerSubject: 100, MaxAge: time.Minute, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	subj := "orca.sse.cas.k.s"

	if _, err := l.Last(ctx, subj); !errors.Is(err, eventbus.ErrLogEmpty) {
		t.Fatalf("empty subject: %v", err)
	}
	// 0 means "the subject must still be empty": only one of two racers wins.
	seq1, err := l.AppendAfter(ctx, subj, 0, map[string]string{"Idx": "0"}, []byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.AppendAfter(ctx, subj, 0, map[string]string{"Idx": "0"}, []byte("b")); !errors.Is(err, eventbus.ErrLogConflict) {
		t.Fatalf("second writer on an empty-expecting append must conflict, got %v", err)
	}
	last, err := l.Last(ctx, subj)
	if err != nil || last.Seq != seq1 || string(last.Data) != "a" || last.Header["Idx"] != "0" {
		t.Fatalf("last = %+v %v", last, err)
	}
	seq2, err := l.AppendAfter(ctx, subj, seq1, map[string]string{"Idx": "1"}, []byte("c"))
	if err != nil || seq2 <= seq1 {
		t.Fatalf("append after the tail: %d %v", seq2, err)
	}
	if _, err := l.AppendAfter(ctx, subj, seq1, nil, []byte("stale")); !errors.Is(err, eventbus.ErrLogConflict) {
		t.Fatalf("a stale tail must conflict, got %v", err)
	}

	// Many concurrent writers: every append lands exactly once, tails never fork.
	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				cur, err := l.Last(ctx, subj)
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := l.AppendAfter(ctx, subj, cur.Seq, nil, []byte("x")); err == nil {
					mu.Lock()
					won++
					mu.Unlock()
					return
				} else if !errors.Is(err, eventbus.ErrLogConflict) {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	snap, err := l.Snapshot(ctx, subj)
	if err != nil || won != 16 || len(snap) != 2+16 {
		t.Fatalf("won=%d stored=%d err=%v", won, len(snap), err)
	}
}
