package mcpserver

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCursorSignRoundTripAndTamper(t *testing.T) {
	c, _ := NewCursorCodec([]byte("key-one"))
	cur := c.Encode("tools", "s1", 7)
	if off, err := c.Decode("tools", "s1", cur); err != nil || off != 7 {
		t.Fatalf("round trip: %d %v", off, err)
	}
	if off, err := c.Decode("tools", "s1", ""); err != nil || off != 0 {
		t.Fatalf("empty cursor = first page: %d %v", off, err)
	}
	mut := []byte(cur)
	mut[2] ^= 1 // flip one byte of the payload
	for name, bad := range map[string]string{
		"flipped byte": string(mut), "wrong kind": cur, "wrong session": cur, "no dot": "abc", "empty sig": strings.Split(cur, ".")[0] + ".",
	} {
		kind, sid := "tools", "s1"
		switch name {
		case "wrong kind":
			kind = "prompts"
		case "wrong session":
			sid = "s2"
		}
		if _, err := c.Decode(kind, sid, bad); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("%s: want ErrInvalidCursor, got %v", name, err)
		}
	}
}

func TestCursorExpiryAndKeyRotation(t *testing.T) {
	now := time.Now()
	oldC, _ := NewCursorCodec([]byte("old"))
	oldC.now = func() time.Time { return now }
	cur := oldC.Encode("tools", "s", 3)

	rotated, _ := NewCursorCodec([]byte("new"), []byte("old")) // signs with new, still verifies old
	rotated.now = func() time.Time { return now }
	if off, err := rotated.Decode("tools", "s", cur); err != nil || off != 3 {
		t.Fatalf("previous key must verify: %d %v", off, err)
	}
	if _, err := mustCodec("new").Decode("tools", "s", cur); err == nil {
		t.Fatal("cursor signed by a dropped key must be rejected")
	}
	rotated.now = func() time.Time { return now.Add(cursorTTL + time.Second) }
	if _, err := rotated.Decode("tools", "s", cur); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expired cursor accepted: %v", err)
	}
	if _, err := NewCursorCodec(nil, []byte{}); err == nil {
		t.Fatal("codec without keys must fail")
	}
}

func mustCodec(k string) *CursorCodec { c, _ := NewCursorCodec([]byte(k)); return c }

func TestPaginateInvalidParamsCode(t *testing.T) {
	c := mustCodec("k")
	_, _, err := Paginate(c, "tools", "s", "bogus", []int{1, 2, 3}, 2)
	var je *jsonrpc.Error
	if !errors.As(err, &je) || je.Code != jsonrpc.CodeInvalidParams {
		t.Fatalf("got %v", err)
	}
	page, next, err := Paginate(c, "tools", "s", "", []int{}, 2)
	if err != nil || page == nil || len(page) != 0 || next != "" {
		t.Fatalf("empty list must be a non-nil empty page: %v %v %q", page, err, next)
	}
}

func TestMapToolError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{status.Error(codes.NotFound, "MCP_NOT_FOUND: task not found"), "MCP_NOT_FOUND: task not found"},
		{errors.New("TASK_INVALID: title is required"), "TASK_INVALID: title is required"},
		{status.Error(codes.Internal, "pq: password authentication failed"), genericToolFailure},
		{errors.New("rpc error: code = Unavailable desc = task-service connection refused"), genericToolFailure},
	}
	for _, c := range cases {
		got := MapToolError(c.err)
		if got != c.want {
			t.Errorf("MapToolError(%v) = %q, want %q", c.err, got, c.want)
		}
		for _, bad := range []string{"rpc error", "pq:", "task-service", "goroutine"} {
			if strings.Contains(got, bad) {
				t.Errorf("leaked %q in %q", bad, got)
			}
		}
	}
	if MapToolError(errors.New("ERROR: relation does not exist")) != genericToolFailure {
		t.Error("a bare 'ERROR:' prefix must not pass through")
	}
}

func TestNegotiateAndSDKOrder(t *testing.T) {
	if Negotiate("2025-06-18") != "2025-06-18" || Negotiate("nope") != LatestProtocolVersion() {
		t.Fatal("negotiate")
	}
	sdk := sdkProtocolVersions()
	if sdk[0] != LatestProtocolVersion() || len(sdk) != len(SupportedProtocolVersions) {
		t.Fatalf("sdk order %v", sdk)
	}
}

func TestStreamLimiter(t *testing.T) {
	l := newStreamLimiter(2, 3)
	a, b := Principal{TenantID: "t", UserID: "a"}, Principal{TenantID: "t", UserID: "b"}
	if !l.acquire(a) || !l.acquire(a) || l.acquire(a) {
		t.Fatal("per-user cap of 2")
	}
	if !l.acquire(b) || l.acquire(Principal{TenantID: "t", UserID: "c"}) {
		t.Fatal("per-tenant cap of 3")
	}
	l.release(a)
	if !l.acquire(Principal{TenantID: "t", UserID: "c"}) {
		t.Fatal("release must free capacity")
	}
}

func TestReadySessionsPrune(t *testing.T) {
	r := newReadySessions(time.Minute)
	now := time.Now()
	r.clock = func() time.Time { return now }
	r.mark("old")
	r.clock = func() time.Time { return now.Add(10 * time.Minute) }
	for i := 0; i < 256; i++ {
		r.mark("fresh")
	}
	if r.has("old") || !r.has("fresh") {
		t.Fatal("stale entry must be pruned, fresh kept")
	}
}
