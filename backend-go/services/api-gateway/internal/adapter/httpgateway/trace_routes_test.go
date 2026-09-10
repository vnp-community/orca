package httpgateway

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func newTraceStreamTestServer(t *testing.T, broadcast *TraceBroadcast) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	mountTraceRoutes(r, broadcast)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// connectSSE opens a streaming GET to /api/trace-stream and reads the
// first line, which must be the ": connected" comment — every test below
// uses this to establish a subscribed client before doing anything else.
func connectSSE(t *testing.T, ctx context.Context, url string) (*http.Response, *bufio.Reader) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/api/trace-stream", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading connected line: %v", err)
	}
	if !strings.Contains(line, "connected") {
		t.Fatalf("first line = %q, want a connected comment", line)
	}
	// Every SSE message here is written as "<content>\n\n" in one Write —
	// ReadString('\n') above only consumed the content line, so the
	// blank-line terminator is still buffered. Swallow it now, or the
	// caller's next read sees a stray "\n" instead of real content.
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("reading connected block's terminating blank line: %v", err)
	}
	return resp, reader
}

func TestMountTraceRoutes_ConnectedCommentStillSentImmediately(t *testing.T) {
	broadcast := NewTraceBroadcast()
	srv := newTraceStreamTestServer(t, broadcast)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, _ := connectSSE(t, ctx, srv.URL)
	defer resp.Body.Close()
	// connectSSE already asserted the connected line — nothing further to check.
}

func TestMountTraceRoutes_ForwardsRealEventAsSSEData(t *testing.T) {
	broadcast := NewTraceBroadcast()
	srv := newTraceStreamTestServer(t, broadcast)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, reader := connectSSE(t, ctx, srv.URL)
	defer resp.Body.Close()

	// Give Subscribe() time to register before publishing — the client
	// goroutine that calls broadcast.Subscribe() races with this test
	// goroutine's Publish below; polling the registry avoids a fixed sleep.
	deadline := time.Now().Add(2 * time.Second)
	for {
		broadcast.mu.Lock()
		n := len(broadcast.subs)
		broadcast.mu.Unlock()
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for SSE client to subscribe")
		}
		time.Sleep(5 * time.Millisecond)
	}

	want := `{"id":"abc","flow":"svc:span","level":"ok","fields":{},"ts":1}`
	broadcast.Publish([]byte(want))

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading data line: %v", err)
	}
	if !strings.HasPrefix(line, "data: "+want) {
		t.Fatalf("data line = %q, want prefix %q", line, "data: "+want)
	}
	blank, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(blank) != "" {
		t.Fatalf("expected a blank line terminating the SSE event, got %q (err=%v)", blank, err)
	}
}

func TestMountTraceRoutes_HeartbeatStillFiresWithNoEvents(t *testing.T) {
	// traceStreamHeartbeatInterval is process-global state read by the
	// handler goroutine. srv.Close() (httptest.Server) blocks until that
	// goroutine has returned, so restoring the var only AFTER Close()
	// (not via a defer racing the still-running handler) is what actually
	// makes this race-free — a naive defer here would race the ticker's
	// read of the var against this restore under -race.
	original := traceStreamHeartbeatInterval
	traceStreamHeartbeatInterval = 20 * time.Millisecond

	broadcast := NewTraceBroadcast()
	r := chi.NewRouter()
	mountTraceRoutes(r, broadcast)
	srv := httptest.NewServer(r)

	func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, reader := connectSSE(t, ctx, srv.URL)
		defer resp.Body.Close()

		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading heartbeat line: %v", err)
		}
		if !strings.Contains(line, "heartbeat") {
			t.Fatalf("line = %q, want a heartbeat comment", line)
		}
	}()

	srv.Close()
	traceStreamHeartbeatInterval = original
}

func TestMountTraceRoutes_ClientDisconnectCallsUnsubscribe(t *testing.T) {
	broadcast := NewTraceBroadcast()
	srv := newTraceStreamTestServer(t, broadcast)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, _ := connectSSE(t, ctx, srv.URL)

	broadcast.mu.Lock()
	n := len(broadcast.subs)
	broadcast.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 subscriber while connected, got %d", n)
	}

	resp.Body.Close()
	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for {
		broadcast.mu.Lock()
		n := len(broadcast.subs)
		broadcast.mu.Unlock()
		if n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for unsubscribe after client disconnect, subs=%d", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestMountTraceRoutes_MultipleClientsReceiveSameEvent(t *testing.T) {
	broadcast := NewTraceBroadcast()
	srv := newTraceStreamTestServer(t, broadcast)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp1, reader1 := connectSSE(t, ctx, srv.URL)
	defer resp1.Body.Close()
	resp2, reader2 := connectSSE(t, ctx, srv.URL)
	defer resp2.Body.Close()

	deadline := time.Now().Add(2 * time.Second)
	for {
		broadcast.mu.Lock()
		n := len(broadcast.subs)
		broadcast.mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for both SSE clients to subscribe")
		}
		time.Sleep(5 * time.Millisecond)
	}

	want := `{"id":"multi","flow":"svc:span","level":"start","fields":{},"ts":2}`
	broadcast.Publish([]byte(want))

	for i, reader := range []*bufio.Reader{reader1, reader2} {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("client %d: reading data line: %v", i, err)
		}
		if !strings.HasPrefix(line, "data: "+want) {
			t.Fatalf("client %d: data line = %q, want prefix %q", i, line, "data: "+want)
		}
	}
}
