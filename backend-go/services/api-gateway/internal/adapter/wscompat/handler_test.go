package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var errSubscribeFailed = errors.New("subscribe failed")

func writeJSONFrame(ctx context.Context, conn *websocket.Conn, v any) error {
	return wsjson.Write(ctx, conn, v)
}

func readWireMessage(t *testing.T, ctx context.Context, conn *websocket.Conn) wireMessage {
	t.Helper()
	var msg wireMessage
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		t.Fatalf("reading frame: %v", err)
	}
	return msg
}

// fakeSessionValidator implements SessionValidator with a canned identity —
// every test connection in this file authenticates successfully.
type fakeSessionValidator struct {
	identity Identity
}

func (f fakeSessionValidator) ValidateCookie(_ context.Context, _ *http.Request) (Identity, error) {
	return f.identity, nil
}

func newTestHandlerServer(t *testing.T, registry *Registry) *httptest.Server {
	t.Helper()
	h := New(slog.Default(), fakeSessionValidator{identity: Identity{TenantID: "tenant-1", UserID: "user-1"}}, registry)
	ts := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
	t.Cleanup(ts.Close)
	return ts
}

// wireMessage is a superset envelope covering every server->client frame
// shape (ResultMessage, ErrorMessage, PushMessage) so a test can read
// whatever comes next off the connection and branch on Type.
type wireMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Message string          `json:"message"`
	Channel string          `json:"channel"`
	Args    []any           `json:"args"`
}

func TestHandleSubscribe_AcksThenStreams(t *testing.T) {
	registry := NewRegistry()
	events := make(chan PushEvent, 2)
	registry.RegisterStream("test.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		return events, nil
	})

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "sub-1", Type: "invoke", Channel: "test.subscribe"}); err != nil {
		t.Fatalf("writing subscribe invoke: %v", err)
	}

	// 1. the subscribe ack — an ordinary ResultMessage with a nil result.
	ack := readWireMessage(t, ctx, client)
	if ack.Type != "result" || ack.ID != "sub-1" {
		t.Fatalf("first frame = %+v, want the subscribe ack (type=result, id=sub-1)", ack)
	}

	// 2. two push frames, in order.
	events <- PushEvent{Channel: "test.event", Args: []any{"first"}}
	events <- PushEvent{Channel: "test.event", Args: []any{"second"}}

	for i, want := range []string{"first", "second"} {
		got := readWireMessage(t, ctx, client)
		if got.Type != "push" || got.Channel != "test.event" {
			t.Fatalf("push frame %d = %+v, want type=push channel=test.event", i, got)
		}
		if len(got.Args) != 1 || got.Args[0] != want {
			t.Fatalf("push frame %d args = %v, want [%q]", i, got.Args, want)
		}
	}
}

func TestHandleSubscribe_SubscriberErrorSurfacesAsErrorMessage(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterStream("test.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		return nil, errSubscribeFailed
	})

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "sub-1", Type: "invoke", Channel: "test.subscribe"}); err != nil {
		t.Fatalf("writing subscribe invoke: %v", err)
	}

	got := readWireMessage(t, ctx, client)
	if got.Type != "error" || got.ID != "sub-1" {
		t.Fatalf("got %+v, want an ErrorMessage for id=sub-1", got)
	}
}

// TestHandleSubscribe_InterleavesWithConcurrentInvoke fires one subscribe
// and one concurrent ordinary invoke on the SAME connection, and asserts
// every frame decodes cleanly with the right id/channel — a regression
// guard on writeMu being shared correctly between handleInvoke,
// handleSubscribe, and pipePush (handler.go/push_bridge.go). Frame
// corruption from an unsynchronized concurrent write would show up here as
// a JSON decode failure or a frame with a wrong/garbled id.
func TestHandleSubscribe_InterleavesWithConcurrentInvoke(t *testing.T) {
	registry := NewRegistry()
	events := make(chan PushEvent, 1)
	registry.RegisterStream("test.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		return events, nil
	})
	registry.Register("test.invoke", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return "invoke-result", nil
	})

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "sub-1", Type: "invoke", Channel: "test.subscribe"}); err != nil {
		t.Fatalf("writing subscribe invoke: %v", err)
	}
	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "call-1", Type: "invoke", Channel: "test.invoke"}); err != nil {
		t.Fatalf("writing ordinary invoke: %v", err)
	}
	events <- PushEvent{Channel: "test.event", Args: []any{"payload"}}

	seenSubAck, seenInvokeResult, seenPush := false, false, false
	for i := 0; i < 3; i++ {
		got := readWireMessage(t, ctx, client)
		switch {
		case got.Type == "result" && got.ID == "sub-1":
			seenSubAck = true
		case got.Type == "result" && got.ID == "call-1":
			seenInvokeResult = true
			var result string
			if err := json.Unmarshal(got.Result, &result); err != nil || result != "invoke-result" {
				t.Fatalf("call-1 result = %s (err=%v), want %q", got.Result, err, "invoke-result")
			}
		case got.Type == "push" && got.Channel == "test.event":
			seenPush = true
		default:
			t.Fatalf("unexpected/corrupted frame %d: %+v", i, got)
		}
	}
	if !seenSubAck || !seenInvokeResult || !seenPush {
		t.Fatalf("missing frame(s): subAck=%v invokeResult=%v push=%v", seenSubAck, seenInvokeResult, seenPush)
	}
}
