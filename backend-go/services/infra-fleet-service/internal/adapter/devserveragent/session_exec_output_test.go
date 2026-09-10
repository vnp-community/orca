package devserveragent

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

// execOutputNotificationFor builds a JSONRPCNotification with the
// {stepId, stream, data} params shape session.go's
// routeExecOutputNotification decodes — mirrors notificationFor
// (session_test.go) / screencastNotificationFor (session_screencast_test.go)'s
// precedent, calling session.routeExecOutputNotification directly (no
// network, no fake agent).
func execOutputNotificationFor(t *testing.T, stepID, stream, data string) JSONRPCNotification {
	t.Helper()
	params, err := json.Marshal(map[string]any{"stepId": stepID, "stream": stream, "data": data})
	if err != nil {
		t.Fatalf("marshaling notification params: %v", err)
	}
	return JSONRPCNotification{JSONRPC: "2.0", Method: "agent.execPrompt.output", Params: params}
}

// TestSession_RouteExecOutputNotification_RoutesOnlyToMatchingStepID mirrors
// TestSession_RouteNotification_RoutesOnlyToMatchingPtyID /
// TestSession_RouteScreencastNotification_RoutesOnlyToMatchingWorktreeID —
// the same demux guarantee, keyed by stepId instead of pty_id/worktree_id.
func TestSession_RouteExecOutputNotification_RoutesOnlyToMatchingStepID(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())
	chA := sess.subscribeExecOutput("step-a")
	t.Cleanup(func() { sess.unsubscribeExecOutput("step-a", chA) })
	chB := sess.subscribeExecOutput("step-b")
	t.Cleanup(func() { sess.unsubscribeExecOutput("step-b", chB) })

	sess.routeExecOutputNotification(execOutputNotificationFor(t, "step-a", "stdout", "hello-a"))

	select {
	case n := <-chA:
		if n.StepID != "step-a" || n.Stream != "stdout" || n.Data != "hello-a" {
			t.Errorf("unexpected notification for step-a: %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for step-a's notification")
	}

	select {
	case n := <-chB:
		t.Fatalf("step-b's channel should not have received a step-a notification, got %+v", n)
	case <-time.After(100 * time.Millisecond):
		// expected: no cross-talk between subscribers keyed by different stepIds
	}
}

// TestSession_RouteExecOutputNotification_StdoutAndStderr confirms both
// stream values pass through untouched — this notification has no
// exit/ended variant (see rawExecOutputNotification's doc comment), unlike
// pty.exit/browser.screencastEnded.
func TestSession_RouteExecOutputNotification_StdoutAndStderr(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())
	ch := sess.subscribeExecOutput("step-1")
	t.Cleanup(func() { sess.unsubscribeExecOutput("step-1", ch) })

	sess.routeExecOutputNotification(execOutputNotificationFor(t, "step-1", "stdout", "out-chunk"))
	sess.routeExecOutputNotification(execOutputNotificationFor(t, "step-1", "stderr", "err-chunk"))

	for _, want := range []rawExecOutputNotification{
		{StepID: "step-1", Stream: "stdout", Data: "out-chunk"},
		{StepID: "step-1", Stream: "stderr", Data: "err-chunk"},
	} {
		select {
		case n := <-ch:
			if n != want {
				t.Errorf("expected %+v, got %+v", want, n)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %+v", want)
		}
	}
}

// TestSession_UnsubscribeExecOutput_ClosesChannelAndStopsRouting mirrors
// TestSession_UnsubscribePty_ClosesChannelAndStopsRouting /
// TestSession_UnsubscribeScreencast_ClosesChannelAndStopsRouting.
func TestSession_UnsubscribeExecOutput_ClosesChannelAndStopsRouting(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())

	ch := sess.subscribeExecOutput("step-1")
	sess.unsubscribeExecOutput("step-1", ch)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected ch to be closed after unsubscribeExecOutput")
		}
	default:
		t.Fatal("expected ch to be immediately closed (non-blocking receive)")
	}

	// A notification after unsubscribe must not panic or reach anyone —
	// there's no subscriber left to route to.
	sess.routeExecOutputNotification(execOutputNotificationFor(t, "step-1", "stdout", "too-late"))
}

// TestSession_RouteNotification_DispatchesExecOutputWithoutAffectingPtyOrScreencast
// is TASK-AG-FLOWTASK-002's required regression guard: adding the third
// demux branch must not disturb the two pre-existing ones.
func TestSession_RouteNotification_DispatchesExecOutputWithoutAffectingPtyOrScreencast(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())
	execCh := sess.subscribeExecOutput("step-1")
	t.Cleanup(func() { sess.unsubscribeExecOutput("step-1", execCh) })
	ptyCh := sess.subscribePty("pty-1")
	t.Cleanup(func() { sess.unsubscribePty("pty-1", ptyCh) })
	screencastCh := sess.subscribeScreencast("wt-1")
	t.Cleanup(func() { sess.unsubscribeScreencast("wt-1", screencastCh) })

	sess.routeNotification(execOutputNotificationFor(t, "step-1", "stdout", "hi"))

	select {
	case n := <-execCh:
		if n.Data != "hi" {
			t.Errorf("expected data=hi, got %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exec-output notification via routeNotification")
	}

	select {
	case n := <-ptyCh:
		t.Fatalf("pty subscriber should not have received an exec-output notification, got %+v", n)
	case <-time.After(100 * time.Millisecond):
	}
	select {
	case n := <-screencastCh:
		t.Fatalf("screencast subscriber should not have received an exec-output notification, got %+v", n)
	case <-time.After(100 * time.Millisecond):
	}

	// The two pre-existing branches must still dispatch correctly too.
	sess.routeNotification(notificationFor(t, "pty.data", "pty-1", "pty-hello", 0))
	select {
	case n := <-ptyCh:
		if string(n.Data) != "pty-hello" {
			t.Errorf("expected pty data=pty-hello, got %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pty notification via routeNotification")
	}

	sess.routeNotification(screencastNotificationFor(t, "browser.screencastEnded", "wt-1", nil))
	select {
	case n := <-screencastCh:
		if !n.Ended {
			t.Errorf("expected Ended=true, got %+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for screencast notification via routeNotification")
	}
}

// TestSession_RouteNotification_UnrelatedMethodStillFallsThroughToDefault
// guards the silent-drop behavior for genuinely unhandled methods (e.g.
// shell.exec.output, which this task deliberately does NOT demux — see
// routeNotification's case comment) — must not panic and must not reach any
// subscriber.
func TestSession_RouteNotification_UnrelatedMethodStillFallsThroughToDefault(t *testing.T) {
	sess := newSession("example.invalid", DefaultConfig(), slog.Default())
	execCh := sess.subscribeExecOutput("trace-1")
	t.Cleanup(func() { sess.unsubscribeExecOutput("trace-1", execCh) })

	params, err := json.Marshal(map[string]any{"traceId": "trace-1", "stream": "stdout", "data": "x"})
	if err != nil {
		t.Fatalf("marshaling params: %v", err)
	}
	sess.routeNotification(JSONRPCNotification{JSONRPC: "2.0", Method: "shell.exec.output", Params: params})

	select {
	case n := <-execCh:
		t.Fatalf("shell.exec.output must not be demuxed into execOutputSubs, got %+v", n)
	case <-time.After(100 * time.Millisecond):
		// expected: unrecognized method silently dropped, matching every
		// other unhandled notification family.
	}
}
