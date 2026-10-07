package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestHandler_ReadLimit(t *testing.T) {
	r := NewRegistry()
	r.Register("test.echo", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]string{"status": "ok"}, nil
	})

	ts := newTestHandlerServer(t, r)

	t.Run("frame under 320 KiB succeeds", func(t *testing.T) {
		client := dialTestClient(t, ts)
		defer client.CloseNow()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 300 KiB payload: ~307200 bytes
		data300KiB := strings.Repeat("x", 300<<10)
		msg := InboundMessage{
			ID:      "req-1",
			Type:    "invoke",
			Channel: "test.echo",
			Args:    []json.RawMessage{json.RawMessage(`{"data":"` + data300KiB + `"}`)},
		}

		if err := writeJSONFrame(ctx, client, msg); err != nil {
			t.Fatalf("write frame under limit failed: %v", err)
		}

		resp := readWireMessage(t, ctx, client)
		if resp.ID != "req-1" || resp.Type != "result" {
			t.Fatalf("expected result for req-1, got %+v", resp)
		}
	})

	t.Run("frame over 320 KiB closes connection", func(t *testing.T) {
		client := dialTestClient(t, ts)
		defer client.CloseNow()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 400 KiB payload: ~409600 bytes
		data400KiB := strings.Repeat("x", 400<<10)
		msg := InboundMessage{
			ID:      "req-2",
			Type:    "invoke",
			Channel: "test.echo",
			Args:    []json.RawMessage{json.RawMessage(`{"data":"` + data400KiB + `"}`)},
		}

		_ = writeJSONFrame(ctx, client, msg)

		var raw map[string]any
		err := wsjson.Read(ctx, client, &raw)
		if err == nil {
			t.Fatalf("expected read to fail due to closed connection, but succeeded with %+v", raw)
		}
		t.Logf("connection closed as expected on 400 KiB frame: %v (close status: %v)", err, websocket.CloseStatus(err))
	})
}
