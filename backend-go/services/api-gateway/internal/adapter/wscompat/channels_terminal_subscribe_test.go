package wscompat

import (
	"encoding/json"
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// TestTerminalSubscribeChannel_AcksSubscribedAndStreamsData is the core
// end-to-end regression for the live bug: WebSessionClient-backed ('session-
// auth') web sessions cannot carry the binary terminal.multiplex protocol at
// all, so this plain-JSON fallback is the ONLY way such a session gets
// terminal output. Hand-writes the raw args exactly as
// remote-runtime-terminal-json-subscribe.ts's real, unmodified encode
// produces them (key "terminal", not "ptyId") — the same "circular test"
// pitfall this investigation already hit once for terminal.multiplex's
// Subscribe frame.
func TestTerminalSubscribeChannel_AcksSubscribedAndStreamsData(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	rawArgs := []json.RawMessage{[]byte(`{"terminal":"pty-1","client":{"id":"c1","type":"desktop"}}`)}
	ack, events, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "terminal.subscribe", rawArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isStream {
		t.Fatal("expected terminal.subscribe to be registered as a StreamChannelHandler")
	}
	view, ok := ack.(terminalJsonSubscribeEvent)
	if !ok || view.Type != "subscribed" {
		t.Fatalf("unexpected ack: %+v", ack)
	}

	if fake.lastStream == nil {
		t.Fatal("expected AttachPty to have been called for pty-1")
	}
	attachFrame := awaitSentFrame(t, fake.lastStream)
	attach := attachFrame.GetAttach()
	if attach == nil || attach.GetPtyId() != "pty-1" {
		t.Fatalf("expected an Attach frame for pty-1, got %+v", attachFrame)
	}

	fake.lastStream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Out{Out: &infrafleetv1.PtyOutput{Data: []byte("hello\n")}}}

	ev := awaitPushEvent(t, events)
	data, ok := ev.Args[0].(terminalJsonSubscribeEvent)
	if !ok || data.Type != "data" || data.Chunk != "hello\n" {
		t.Fatalf("unexpected data event: %+v", ev)
	}
}

func TestTerminalSubscribeChannel_ClosesEventsOnExit(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	rawArgs := []json.RawMessage{[]byte(`{"terminal":"pty-1","client":{"id":"c1","type":"desktop"}}`)}
	_, events, _, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "terminal.subscribe", rawArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	fake.lastStream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Exited{Exited: &infrafleetv1.PtyExited{ExitCode: 0}}}

	if _, ok := <-events; ok {
		t.Fatal("expected the events channel to close once the pty exits")
	}
}

func TestTerminalSubscribeChannel_RequiresTerminal(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, _, _, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "terminal.subscribe", argsJSON(t, terminalJsonSubscribeArgs{}))
	if err == nil {
		t.Fatal("expected an error when terminal is omitted")
	}
}

// TestTerminalUnsubscribeChannel_CancelsTheMatchingSubscription checks the
// same synchronous state channels_terminal_multiplex_test.go's own
// Unsubscribe test checks (io.hasHandler after Unsubscribe) rather than
// waiting for the subscribe goroutine's events channel to close: a fake
// PtyStream's Recv() has no context awareness (see fakePtyStream's doc
// comment), so cancel() alone never unblocks it here the way a real gRPC
// stream's Recv() does in production — waiting on the channel would hang
// this test forever, not exercise a real bug.
func TestTerminalUnsubscribeChannel_CancelsTheMatchingSubscription(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	ctx := newTerminalTestCtx()

	rawArgs := []json.RawMessage{[]byte(`{"terminal":"pty-1","client":{"id":"c1","type":"desktop"}}`)}
	if _, _, _, err := r.DispatchStreamChannel(ctx, Identity{TenantID: "tenant-1"}, "terminal.subscribe", rawArgs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	reg := terminalJSONSubscribeFromContext(ctx)
	reg.mu.Lock()
	_, hadEntry := reg.cancels["pty-1:c1"]
	reg.mu.Unlock()
	if !hadEntry {
		t.Fatal("expected terminal.subscribe to register a cancel func under \"pty-1:c1\"")
	}

	unsubArgs := []json.RawMessage{[]byte(`{"subscriptionId":"pty-1:c1"}`)}
	if _, err := r.Dispatch(ctx, Identity{TenantID: "tenant-1"}, "terminal.unsubscribe", unsubArgs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reg.mu.Lock()
	_, stillPresent := reg.cancels["pty-1:c1"]
	reg.mu.Unlock()
	if stillPresent {
		t.Fatal("expected terminal.unsubscribe to remove the subscription's cancel func")
	}
}

func TestTerminalUpdateViewportChannel_CallsResizeRPC(t *testing.T) {
	var gotReq *infrafleetv1.ResizeTerminalSessionRequest
	fake := &fakeTerminalInfraFleetClient{
		resizeFunc: func(in *infrafleetv1.ResizeTerminalSessionRequest) error {
			gotReq = in
			return nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	rawArgs := []json.RawMessage{[]byte(`{"terminal":"pty-1","client":{"id":"c1","type":"desktop"},"viewport":{"cols":83,"rows":11}}`)}
	if _, err := r.Dispatch(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "terminal.updateViewport", rawArgs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetPtyId() != "pty-1" || gotReq.GetCols() != 83 || gotReq.GetRows() != 11 {
		t.Fatalf("unexpected ResizeTerminalSessionRequest: %+v", gotReq)
	}
}

func TestTerminalUpdateViewportChannel_IgnoresZeroViewport(t *testing.T) {
	called := false
	fake := &fakeTerminalInfraFleetClient{
		resizeFunc: func(*infrafleetv1.ResizeTerminalSessionRequest) error {
			called = true
			return nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	rawArgs := []json.RawMessage{[]byte(`{"terminal":"pty-1","client":{"id":"c1","type":"desktop"}}`)}
	if _, err := r.Dispatch(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "terminal.updateViewport", rawArgs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected ResizeTerminalSession not to be called for a zero-value viewport")
	}
}
