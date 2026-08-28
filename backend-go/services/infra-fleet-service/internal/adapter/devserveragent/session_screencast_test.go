package devserveragent

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

// screencastNotificationFor builds a JSONRPCNotification with the
// {worktreeId, subscriptionId, browserPageId, format, dataBase64, message}
// params shape session.go's routeScreencastNotification decodes — mirrors
// notificationFor's (session_test.go) precedent, calling
// session.routeScreencastNotification directly (no network, no fake agent).
func screencastNotificationFor(t *testing.T, method, worktreeID string, extra map[string]any) JSONRPCNotification {
	t.Helper()
	params := map[string]any{"worktreeId": worktreeID}
	for k, v := range extra {
		params[k] = v
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshaling notification params: %v", err)
	}
	return JSONRPCNotification{JSONRPC: "2.0", Method: method, Params: raw}
}

// TestSession_RouteScreencastNotification_RoutesOnlyToMatchingWorktreeID
// mirrors TestSession_RouteNotification_RoutesOnlyToMatchingPtyID — the same
// demux guarantee, keyed by worktree_id instead of pty_id.
func TestSession_RouteScreencastNotification_RoutesOnlyToMatchingWorktreeID(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())

	chA := sess.subscribeScreencast("wt-a")
	t.Cleanup(func() { sess.unsubscribeScreencast("wt-a", chA) })
	chB := sess.subscribeScreencast("wt-b")
	t.Cleanup(func() { sess.unsubscribeScreencast("wt-b", chB) })

	sess.routeScreencastNotification(screencastNotificationFor(t, "browser.screencastReady", "wt-a", map[string]any{
		"subscriptionId": "sub-1", "browserPageId": "page-1", "format": "jpeg",
	}))

	select {
	case n := <-chA:
		if !n.Ready || n.SubscriptionID != "sub-1" || n.BrowserPageID != "page-1" || n.Format != "jpeg" {
			t.Errorf("unexpected notification on chA: %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for chA's notification")
	}

	select {
	case n := <-chB:
		t.Fatalf("chB should not have received wt-a's notification, got %+v", n)
	case <-time.After(50 * time.Millisecond):
		// expected: chB is a different worktree, must see nothing
	}
}

func TestSession_RouteScreencastNotification_DecodesFrameBase64(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())
	ch := sess.subscribeScreencast("wt-1")
	t.Cleanup(func() { sess.unsubscribeScreencast("wt-1", ch) })

	payload := []byte("jpeg-frame-bytes")
	sess.routeScreencastNotification(screencastNotificationFor(t, "browser.screencastFrame", "wt-1", map[string]any{
		"dataBase64": base64.StdEncoding.EncodeToString(payload),
	}))

	select {
	case n := <-ch:
		if string(n.Frame) != string(payload) {
			t.Errorf("expected decoded frame %q, got %q", payload, n.Frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for frame notification")
	}
}

func TestSession_RouteScreencastNotification_EndedAndError(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())
	ch := sess.subscribeScreencast("wt-1")
	t.Cleanup(func() { sess.unsubscribeScreencast("wt-1", ch) })

	sess.routeScreencastNotification(screencastNotificationFor(t, "browser.screencastEnded", "wt-1", nil))
	select {
	case n := <-ch:
		if !n.Ended {
			t.Errorf("expected Ended=true, got %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ended notification")
	}

	sess.routeScreencastNotification(screencastNotificationFor(t, "browser.screencastError", "wt-1", map[string]any{"message": "boom"}))
	select {
	case n := <-ch:
		if n.ErrorMsg != "boom" {
			t.Errorf("expected ErrorMsg=boom, got %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error notification")
	}
}

// TestSession_UnsubscribeScreencast_ClosesChannelAndStopsRouting mirrors
// TestSession_UnsubscribePty_ClosesChannelAndStopsRouting.
func TestSession_UnsubscribeScreencast_ClosesChannelAndStopsRouting(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())

	ch := sess.subscribeScreencast("wt-1")
	sess.unsubscribeScreencast("wt-1", ch)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected ch to be closed after unsubscribeScreencast")
		}
	default:
		t.Fatal("expected ch to be immediately closed (non-blocking receive)")
	}

	// A notification after unsubscribe must not panic or reach anyone —
	// there's no subscriber left to route to.
	sess.routeScreencastNotification(screencastNotificationFor(t, "browser.screencastEnded", "wt-1", nil))
}
