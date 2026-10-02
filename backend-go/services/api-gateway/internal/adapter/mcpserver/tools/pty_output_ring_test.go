package tools

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var t0 = time.Unix(1700000000, 0)

func u64(v uint64) *uint64 { return &v }

func TestPtyOutputRing_SeqIsMonotonicAndCursorReadsContinue(t *testing.T) {
	r := newPtyOutputRing(64, t0)
	r.Append([]byte("hello "), t0)
	r.Append([]byte("world"), t0)
	a, err := r.Read(nil, 100)
	if err != nil || string(a.Data) != "hello world" || a.From != 0 || a.Next != 11 || a.Dropped != 0 {
		t.Fatalf("first read %+v err=%v", a, err)
	}
	r.Append([]byte("!!"), t0)
	b, _ := r.Read(u64(a.Next), 100)
	if string(b.Data) != "!!" || b.From != 11 || b.Next != 13 {
		t.Fatalf("second read %+v", b)
	}
	c, _ := r.Read(u64(b.Next), 100)
	if len(c.Data) != 0 || c.Next != 13 {
		t.Fatalf("empty read %+v", c)
	}
}

func TestPtyOutputRing_WrapKeepsNewestAndAdvancesBase(t *testing.T) {
	r := newPtyOutputRing(16, t0)
	for i := 0; i < 10; i++ {
		r.Append([]byte("0123456789"), t0) // 100 bytes through a 16-byte ring
	}
	got, err := r.Read(nil, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 16 || got.Next != 100 || got.From != 84 {
		t.Fatalf("wrap read from=%d next=%d len=%d", got.From, got.Next, len(got.Data))
	}
	if string(got.Data) != "4567890123456789" {
		t.Fatalf("ring content %q", got.Data)
	}
	_, _, _, dropped := r.State()
	if dropped != 84 {
		t.Fatalf("dropped_total=%d want 84", dropped)
	}
}

func TestPtyOutputRing_OldCursorReportsGapAndResumesAtBase(t *testing.T) {
	r := newPtyOutputRing(16, t0)
	r.Append([]byte(strings.Repeat("a", 40)), t0)
	got, err := r.Read(u64(5), 100)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Behind || got.Dropped != 24-5 || got.From != 24 || len(got.Data) != 16 {
		t.Fatalf("gap read %+v", got)
	}
}

func TestPtyOutputRing_CursorBeyondTailIsInvalid(t *testing.T) {
	r := newPtyOutputRing(16, t0)
	r.Append([]byte("abc"), t0)
	if _, err := r.Read(u64(4), 10); err == nil || !strings.HasPrefix(err.Error(), "INVALID_CURSOR") {
		t.Fatalf("err=%v", err)
	}
	if _, err := r.Read(u64(3), 10); err != nil {
		t.Fatalf("cursor at the tail is valid: %v", err)
	}
}

func TestPtyOutputRing_ReadCapAndHugeInputStayBounded(t *testing.T) {
	r := newPtyOutputRing(1024, t0)
	chunk := []byte(strings.Repeat("y\n", 4096)) // "yes" flood: 8 KiB per chunk
	for i := 0; i < 6400; i++ {                  // ~50 MB
		r.Append(chunk, t0)
	}
	got, _ := r.Read(nil, 300)
	if len(got.Data) > 300 {
		t.Fatalf("read exceeded the cap: %d", len(got.Data))
	}
	if r.Tail() != uint64(len(chunk))*6400 {
		t.Fatalf("tail %d", r.Tail())
	}
	if len(r.buf) != 1024 {
		t.Fatal("ring grew")
	}
}

func TestPtyOutputRing_NeverSplitsAUTF8RuneAcrossReads(t *testing.T) {
	r := newPtyOutputRing(256, t0)
	src := strings.Repeat("héllo wörld ✓ 日本語 ", 6)
	r.Append([]byte(src), t0)
	var sb strings.Builder
	var cur *uint64
	for i := 0; i < 1000; i++ {
		got, err := r.Read(cur, 7)
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.Valid(got.Data) {
			t.Fatalf("read %d split a rune: %q", i, got.Data)
		}
		if len(got.Data) == 0 {
			break
		}
		sb.Write(got.Data)
		cur = u64(got.Next)
	}
	if sb.String() != src {
		t.Fatalf("reassembled text differs")
	}
}

func TestPtyOutputRing_HoldsBackIncompleteRuneUntilCompletedOrExit(t *testing.T) {
	r := newPtyOutputRing(64, t0)
	snow := []byte("☃") // 3 bytes
	r.Append(append([]byte("ab"), snow[:2]...), t0)
	got, _ := r.Read(nil, 64)
	if string(got.Data) != "ab" || got.Next != 2 {
		t.Fatalf("incomplete tail must be held: %q next=%d", got.Data, got.Next)
	}
	r.Append(snow[2:], t0)
	got, _ = r.Read(u64(got.Next), 64)
	if string(got.Data) != "☃" {
		t.Fatalf("completed rune: %q", got.Data)
	}
	r.Append(snow[:1], t0)
	r.MarkExited(0, t0)
	got, _ = r.Read(u64(got.Next), 64)
	if len(got.Data) != 1 || !got.Exited {
		t.Fatalf("after exit the partial byte is returned: %+v", got)
	}
}

func TestPtyOutputRing_WrapSkipsLeadingContinuationBytes(t *testing.T) {
	r := newPtyOutputRing(16, t0)
	r.Append([]byte(strings.Repeat("日", 10)), t0) // 30 bytes into 16
	got, _ := r.Read(nil, 64)
	if !utf8.Valid(got.Data) {
		t.Fatalf("read after wrap starts mid-rune: %q", got.Data)
	}
}

func TestPtyOutputRing_ExitAndReadAfterExit(t *testing.T) {
	r := newPtyOutputRing(64, t0)
	r.Append([]byte("bye"), t0)
	r.MarkExited(3, t0)
	got, _ := r.Read(nil, 10)
	if !got.Exited || got.ExitCode != 3 || string(got.Data) != "bye" {
		t.Fatalf("%+v", got)
	}
	again, _ := r.Read(u64(got.Next), 10)
	if !again.Exited || again.ExitCode != 3 || len(again.Data) != 0 {
		t.Fatalf("read after exit must stay stable: %+v", again)
	}
}

func TestPtyOutputRing_WaitWakesOnAppendAndTimesOut(t *testing.T) {
	r := newPtyOutputRing(64, t0)
	done := make(chan struct{})
	if r.Wait(done, 0, 20*time.Millisecond) {
		t.Fatal("must time out without data")
	}
	go func() { time.Sleep(10 * time.Millisecond); r.Append([]byte("x"), time.Now()) }()
	if !r.Wait(done, 0, 2*time.Second) {
		t.Fatal("must wake on append")
	}
}
