//go:build integration

package eventbus_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/testutil"
)

func TestBoundedLog_FollowFirstPurgeAndBound(t *testing.T) {
	url := testutil.StartNATS(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	l, err := eventbus.NewBoundedLog(ctx, url, eventbus.BoundedLogConfig{
		Stream: "TESTSSE", Subjects: []string{"orca.sse.test.>"}, MaxMsgsPerSubject: 5, MaxAge: time.Minute, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	subj := "orca.sse.test.s1.a"
	if _, err := l.First(ctx, subj); !errors.Is(err, eventbus.ErrLogEmpty) {
		t.Fatalf("empty subject: %v", err)
	}
	for i := 0; i < 8; i++ { // 3 oldest fall off: bounded, no growth
		if _, err := l.Append(ctx, subj, map[string]string{"Idx": fmt.Sprint(i)}, []byte(fmt.Sprint("m", i))); err != nil {
			t.Fatal(err)
		}
	}
	first, err := l.First(ctx, subj)
	if err != nil || first.Header["Idx"] != "3" {
		t.Fatalf("oldest retained must be idx 3: %+v %v", first, err)
	}
	// Follow = replay of the retained 5 then a live one.
	got := make(chan string, 16)
	done := make(chan error, 1)
	fctx, fcancel := context.WithCancel(ctx)
	go func() {
		done <- l.Follow(fctx, subj, func(e eventbus.LogEntry) (bool, error) { got <- e.Header["Idx"]; return e.Header["Idx"] == "8", nil })
	}()
	for _, want := range []string{"3", "4", "5", "6", "7"} {
		select {
		case g := <-got:
			if g != want {
				t.Fatalf("replay order: got %s want %s", g, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("timeout replaying")
		}
	}
	if _, err := l.Append(ctx, subj, map[string]string{"Idx": "8"}, []byte("m8")); err != nil {
		t.Fatal(err)
	}
	if g := <-got; g != "8" {
		t.Fatalf("live: %s", g)
	}
	if err := <-done; err != nil {
		t.Fatalf("follow stop: %v", err)
	}
	fcancel()
	if err := l.Purge(ctx, "orca.sse.test.s1.>"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.First(ctx, subj); !errors.Is(err, eventbus.ErrLogEmpty) {
		t.Fatalf("after purge: %v", err)
	}
}

func TestEphemeral_PubSubAcrossConnections(t *testing.T) {
	url := testutil.StartNATS(t)
	a, closeA, err := eventbus.NewEphemeral(url)
	if err != nil {
		t.Fatal(err)
	}
	defer closeA()
	b, closeB, err := eventbus.NewEphemeral(url)
	if err != nil {
		t.Fatal(err)
	}
	defer closeB()
	got := make(chan string, 1)
	unsub, err := b.Subscribe("orca.ephemeral.t.>", func(s string, d []byte) { got <- s + "=" + string(d) })
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()
	time.Sleep(200 * time.Millisecond)
	if err := a.Publish("orca.ephemeral.t.x", []byte("hi")); err != nil {
		t.Fatal(err)
	}
	select {
	case g := <-got:
		if g != "orca.ephemeral.t.x=hi" {
			t.Fatal(g)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no delivery")
	}
}
