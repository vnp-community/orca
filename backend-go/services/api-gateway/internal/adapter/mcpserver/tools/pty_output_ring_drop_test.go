package tools

import (
	"testing"
	"time"
)

// The drop hook is a metrics tap: it sees exactly the bytes the ring reports
// as dropped and never changes what the ring holds.
func TestPtyOutputRing_OnDropMatchesDroppedAndLeavesContentsAlone(t *testing.T) {
	now := time.Unix(0, 0)
	withHook, plain := newPtyOutputRing(32, now), newPtyOutputRing(32, now)
	var seen uint64
	withHook.onDrop = func(n uint64) { seen += n }

	writes := [][]byte{[]byte("0123456789"), []byte("abcdefghijklmnopqrstuvwxyz"), make([]byte, 100), []byte("tail")}
	for i, w := range writes {
		withHook.Append(w, now)
		plain.Append(w, now)
		if seen != withHook.dropped {
			t.Fatalf("write %d: hook saw %d bytes, ring dropped %d", i, seen, withHook.dropped)
		}
	}
	if seen == 0 {
		t.Fatal("test must overflow the ring")
	}
	a, err1 := withHook.Read(nil, 1024)
	b, err2 := plain.Read(nil, 1024)
	if err1 != nil || err2 != nil || string(a.Data) != string(b.Data) || a.From != b.From || a.Tail != b.Tail {
		t.Fatalf("hook changed ring behaviour: %+v vs %+v", a, b)
	}
}

func TestPtyOutputRing_NoDropNoHookCall(t *testing.T) {
	r := newPtyOutputRing(64, time.Unix(0, 0))
	called := false
	r.onDrop = func(uint64) { called = true }
	r.Append([]byte("short"), time.Unix(0, 0))
	if called {
		t.Fatal("hook must not fire without overwriting")
	}
}
