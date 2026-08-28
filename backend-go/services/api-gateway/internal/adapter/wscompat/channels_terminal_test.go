package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeTerminalInfraFleetClient is channels_terminal_test.go's own
// InfraFleetServiceClient test double — kept separate from
// channels_test.go's fakeInfraFleetClient rather than extending that one,
// so this file never needs to touch channels_test.go (out of scope for
// this pass, same as channels.go itself). Embeds the nil interface and
// overrides only the terminal.* methods this file's channel handlers call.
type fakeTerminalInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	spawnFunc              func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error)
	resizeFunc             func(*infrafleetv1.ResizeTerminalSessionRequest) error
	killFunc               func(*infrafleetv1.KillTerminalSessionRequest) error
	stopFunc               func(*infrafleetv1.StopTerminalProcessRequest) error
	listFunc               func(*infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error)
	waitFunc               func(*infrafleetv1.WaitTerminalSessionRequest) (*infrafleetv1.WaitTerminalSessionResponse, error)
	focusFunc              func(*infrafleetv1.FocusTerminalSessionRequest) error
	getTerminalStatusFunc  func(*infrafleetv1.GetTerminalAgentStatusRequest) (*infrafleetv1.GetTerminalAgentStatusResponse, error)
	inspectProcessFunc     func(*infrafleetv1.InspectTerminalProcessRequest) (*infrafleetv1.InspectTerminalProcessResponse, error)
	attachPtyErr           error
	getTerminalStatusCalls int

	// lastStreamMu guards lastStream — every pre-existing test in this file
	// calls AttachPty and reads lastStream back on the SAME goroutine (via
	// r.Dispatch), so this was never a real race before
	// channels_terminal_multiplex_test.go's end-to-end test started
	// reading it from a separate test goroutine while handler.go's binary
	// frame dispatch (its own goroutine — see handler.go's ServeHTTP doc
	// comment on why) writes it concurrently via AttachPty.
	lastStreamMu sync.Mutex
	lastStream   *fakePtyStream
}

// getLastStream is the race-safe way to read lastStream from a goroutine
// other than the one that called AttachPty — see lastStreamMu's doc comment.
func (f *fakeTerminalInfraFleetClient) getLastStream() *fakePtyStream {
	f.lastStreamMu.Lock()
	defer f.lastStreamMu.Unlock()
	return f.lastStream
}

func (f *fakeTerminalInfraFleetClient) SpawnTerminalSession(_ context.Context, in *infrafleetv1.SpawnTerminalSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	return f.spawnFunc(in)
}

func (f *fakeTerminalInfraFleetClient) ResizeTerminalSession(_ context.Context, in *infrafleetv1.ResizeTerminalSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.resizeFunc(in)
}

func (f *fakeTerminalInfraFleetClient) KillTerminalSession(_ context.Context, in *infrafleetv1.KillTerminalSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.killFunc(in)
}

func (f *fakeTerminalInfraFleetClient) StopTerminalProcess(_ context.Context, in *infrafleetv1.StopTerminalProcessRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.stopFunc(in)
}

func (f *fakeTerminalInfraFleetClient) ListTerminalSessions(_ context.Context, in *infrafleetv1.ListTerminalSessionsRequest, _ ...grpc.CallOption) (*infrafleetv1.ListTerminalSessionsResponse, error) {
	return f.listFunc(in)
}

func (f *fakeTerminalInfraFleetClient) WaitTerminalSession(_ context.Context, in *infrafleetv1.WaitTerminalSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.WaitTerminalSessionResponse, error) {
	return f.waitFunc(in)
}

func (f *fakeTerminalInfraFleetClient) FocusTerminalSession(_ context.Context, in *infrafleetv1.FocusTerminalSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.focusFunc(in)
}

func (f *fakeTerminalInfraFleetClient) GetTerminalAgentStatus(_ context.Context, in *infrafleetv1.GetTerminalAgentStatusRequest, _ ...grpc.CallOption) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
	f.getTerminalStatusCalls++
	return f.getTerminalStatusFunc(in)
}

func (f *fakeTerminalInfraFleetClient) InspectTerminalProcess(_ context.Context, in *infrafleetv1.InspectTerminalProcessRequest, _ ...grpc.CallOption) (*infrafleetv1.InspectTerminalProcessResponse, error) {
	return f.inspectProcessFunc(in)
}

func (f *fakeTerminalInfraFleetClient) AttachPty(_ context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[infrafleetv1.PtyClientFrame, infrafleetv1.PtyServerFrame], error) {
	if f.attachPtyErr != nil {
		return nil, f.attachPtyErr
	}
	stream := newFakePtyStream()
	f.lastStreamMu.Lock()
	f.lastStream = stream
	f.lastStreamMu.Unlock()
	return stream, nil
}

// fakePtyStream implements grpc.BidiStreamingClient[PtyClientFrame,
// PtyServerFrame] — enough to drive registerTerminalCreateChannel's
// stream.Send(attach) and drainAttachPtyOutput's stream.Recv() loop without
// a real gRPC transport.
type fakePtyStream struct {
	sent chan *infrafleetv1.PtyClientFrame
	recv chan *infrafleetv1.PtyServerFrame
	err  chan error
}

func newFakePtyStream() *fakePtyStream {
	return &fakePtyStream{
		sent: make(chan *infrafleetv1.PtyClientFrame, 16),
		recv: make(chan *infrafleetv1.PtyServerFrame, 16),
		err:  make(chan error, 1),
	}
}

func (s *fakePtyStream) Send(f *infrafleetv1.PtyClientFrame) error {
	s.sent <- f
	return nil
}

func (s *fakePtyStream) Recv() (*infrafleetv1.PtyServerFrame, error) {
	select {
	case f := <-s.recv:
		return f, nil
	case err := <-s.err:
		return nil, err
	}
}

func (s *fakePtyStream) Header() (metadata.MD, error) { return nil, nil }
func (s *fakePtyStream) Trailer() metadata.MD         { return nil }
func (s *fakePtyStream) CloseSend() error             { s.err <- errCloseSend; return nil }
func (s *fakePtyStream) Context() context.Context     { return context.Background() }
func (s *fakePtyStream) SendMsg(any) error            { return nil }
func (s *fakePtyStream) RecvMsg(any) error            { return nil }

var errCloseSend = errors.New("fakePtyStream: closed")

func awaitSentFrame(t *testing.T, s *fakePtyStream) *infrafleetv1.PtyClientFrame {
	t.Helper()
	select {
	case f := <-s.sent:
		return f
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a frame to be sent on the fake AttachPty stream")
		return nil
	}
}

// awaitPushEvent reads one PushEvent off events, failing the test if none
// arrives in time — the standard timeout guard for every push-delivery
// assertion in this file.
func awaitPushEvent(t *testing.T, events <-chan PushEvent) PushEvent {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("events channel closed before delivering the expected push event")
		}
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a push event")
		return PushEvent{}
	}
}

// newTerminalTestCtx returns a context wrapped exactly like ServeHTTP wraps
// one real WebSocket connection's ctx (handler.go) — a fresh, empty
// terminalStreamRegistry attached via terminalStreamsContext. Two calls to
// this helper simulate two DIFFERENT WS connections: each gets its own
// registry, so a pty_id created on one is invisible from the other, the same
// isolation guarantee production gets from ServeHTTP constructing a fresh
// registry per accepted connection.
func newTerminalTestCtx() context.Context {
	return terminalStreamsContext(context.Background(), newTerminalStreamRegistry())
}

func TestTerminalCreateChannel_SpawnsAndOpensAttachPtyStream(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		spawnFunc: func(in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			if in.GetConnectionId() != "conn-1" || in.GetCwd() != "/repo" {
				t.Errorf("unexpected SpawnTerminalSessionRequest: %+v", in)
			}
			return &infrafleetv1.SpawnTerminalSessionResponse{
				Session: &infrafleetv1.TerminalSession{PtyId: "pty-1", ConnectionId: "conn-1", Cwd: "/repo", CreatedAtUnixMs: 1000, LastActiveAtUnixMs: 1000},
			}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	ack, events, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "terminal.create",
		argsJSON(t, terminalCreateArgs{ConnectionID: "conn-1", Cwd: "/repo", Cols: 80, Rows: 24}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isStream {
		t.Fatal("expected terminal.create to be registered as a StreamChannelHandler")
	}
	if events == nil {
		t.Fatal("expected a non-nil push events channel")
	}
	view, ok := ack.(terminalSessionView)
	if !ok || view.PtyID != "pty-1" || view.ConnectionID != "conn-1" {
		t.Fatalf("unexpected ack: %+v", ack)
	}

	if fake.lastStream == nil {
		t.Fatal("expected AttachPty to have been called")
	}
	frame := awaitSentFrame(t, fake.lastStream)
	attach := frame.GetAttach()
	if attach == nil || attach.GetPtyId() != "pty-1" {
		t.Errorf("expected the stream's first frame to be an attach frame for pty-1, got %+v", frame)
	}
}

// TestTerminalCreateChannel_ShellIntegrationPassesThrough guards BR-TM-13's
// pass-through chain: shellIntegration in terminal.create's args must ride
// straight through to SpawnTerminalSessionRequest.ShellIntegration.
func TestTerminalCreateChannel_ShellIntegrationPassesThrough(t *testing.T) {
	var gotShellIntegration bool
	fake := &fakeTerminalInfraFleetClient{
		spawnFunc: func(in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			gotShellIntegration = in.GetShellIntegration()
			return &infrafleetv1.SpawnTerminalSessionResponse{
				Session: &infrafleetv1.TerminalSession{PtyId: "pty-1", ConnectionId: "conn-1", Cwd: "/repo"},
			}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, _, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "terminal.create",
		argsJSON(t, terminalCreateArgs{ConnectionID: "conn-1", Cwd: "/repo", ShellIntegration: true}))
	if !isStream {
		t.Fatal("expected terminal.create to be registered as a StreamChannelHandler")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotShellIntegration {
		t.Error("expected ShellIntegration: true to pass through to SpawnTerminalSessionRequest")
	}
}

// TestTerminalCreateChannel_ShellIntegrationOmitted_DefaultsFalse is the
// regression guard: an existing caller that never sets shellIntegration
// must keep getting ShellIntegration: false, unchanged.
func TestTerminalCreateChannel_ShellIntegrationOmitted_DefaultsFalse(t *testing.T) {
	var gotShellIntegration bool
	fake := &fakeTerminalInfraFleetClient{
		spawnFunc: func(in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			gotShellIntegration = in.GetShellIntegration()
			return &infrafleetv1.SpawnTerminalSessionResponse{
				Session: &infrafleetv1.TerminalSession{PtyId: "pty-1", ConnectionId: "conn-1", Cwd: "/repo"},
			}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, _, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "terminal.create",
		argsJSON(t, terminalCreateArgs{ConnectionID: "conn-1", Cwd: "/repo"}))
	if !isStream {
		t.Fatal("expected terminal.create to be registered as a StreamChannelHandler")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotShellIntegration {
		t.Error("expected ShellIntegration: false when omitted from args")
	}
}

func TestTerminalCreateChannel_SpawnFailurePropagates(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		spawnFunc: func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			return nil, errors.New("infra-fleet-service: no dev server owns this connectionId")
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, events, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "unknown"}))
	if !isStream {
		t.Fatal("expected terminal.create to be registered as a StreamChannelHandler")
	}
	if err == nil {
		t.Fatal("expected the spawn error to propagate")
	}
	if events != nil {
		t.Error("expected a nil events channel on spawn failure")
	}
	if fake.lastStream != nil {
		t.Error("expected no AttachPty call when SpawnTerminalSession fails")
	}
}

// TestTerminalCreateChannel_MissingStreamRegistryOnContext_ReturnsError
// guards the defensive check in registerTerminalCreateChannel: if ctx was
// never wrapped by terminalStreamsContext (a wiring bug — every real
// connection gets this from ServeHTTP, see handler.go), terminal.create
// must fail cleanly instead of nil-pointer-panicking on streams.put.
func TestTerminalCreateChannel_MissingStreamRegistryOnContext_ReturnsError(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		spawnFunc: func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			t.Fatal("SpawnTerminalSession should not be called before the stream-registry check")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, events, isStream, err := r.DispatchStreamChannel(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "conn-1"}))
	if !isStream {
		t.Fatal("expected terminal.create to be registered as a StreamChannelHandler")
	}
	if err == nil {
		t.Fatal("expected an error when ctx has no per-connection terminal stream registry")
	}
	if events != nil {
		t.Error("expected a nil events channel")
	}
}

// createSession is the shared setup for tests that need a live
// terminal.create'd session before exercising send/resize/close/etc. — ctx
// must be the SAME context used for every follow-up r.Dispatch call in the
// test, since it carries the per-connection terminalStreamRegistry
// terminal.send/close look up.
func createSession(t *testing.T, ctx context.Context, r *Registry, fake *fakeTerminalInfraFleetClient, ptyID string) <-chan PushEvent {
	t.Helper()
	if fake.spawnFunc == nil {
		fake.spawnFunc = func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: ptyID, ConnectionId: "conn-1"}}, nil
		}
	}
	_, events, isStream, err := r.DispatchStreamChannel(ctx, Identity{TenantID: "tenant-1"}, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "conn-1"}))
	if err != nil {
		t.Fatalf("terminal.create setup failed: %v", err)
	}
	if !isStream {
		t.Fatal("expected terminal.create to be registered as a StreamChannelHandler")
	}
	return events
}

// TestTerminalCreateChannel_ForwardsOutputAndExitPushEvents is the core
// regression guard for TASK-186: a subscriber (the events channel
// terminal.create's StreamChannelHandler returns) must actually receive
// terminal.output for each PtyServerFrame the agent sends, and a final
// terminal.exited carrying the real exit code — not have every frame
// silently discarded.
func TestTerminalCreateChannel_ForwardsOutputAndExitPushEvents(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	events := createSession(t, newTerminalTestCtx(), r, fake, "pty-1")
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	fake.lastStream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Out{Out: &infrafleetv1.PtyOutput{Data: []byte("hello\n")}}}

	outEv := awaitPushEvent(t, events)
	if outEv.Channel != "terminal.output" {
		t.Fatalf("expected channel=terminal.output, got %+v", outEv)
	}
	payload, ok := outEv.Args[0].(map[string]any)
	if !ok || payload["ptyId"] != "pty-1" {
		t.Fatalf("unexpected terminal.output payload: %+v", outEv.Args)
	}
	if data, ok := payload["data"].([]byte); !ok || string(data) != "hello\n" {
		t.Fatalf("unexpected terminal.output data: %+v", payload["data"])
	}

	fake.lastStream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Exited{Exited: &infrafleetv1.PtyExited{ExitCode: 7}}}

	exitEv := awaitPushEvent(t, events)
	if exitEv.Channel != "terminal.exited" {
		t.Fatalf("expected channel=terminal.exited, got %+v", exitEv)
	}
	exitPayload, ok := exitEv.Args[0].(map[string]any)
	if !ok || exitPayload["ptyId"] != "pty-1" || exitPayload["exitCode"] != int32(7) {
		t.Fatalf("unexpected terminal.exited payload: %+v", exitEv.Args)
	}

	// drainAttachPtyOutput must close events once it returns after the exit
	// frame — pipePush relies on the closed channel to end its own loop.
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected the events channel to be closed after terminal.exited")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the events channel to close after terminal.exited")
	}
}

// TestTerminalCreateChannel_StreamErrorClosesEventsWithoutHanging verifies
// drainAttachPtyOutput's other exit path: the underlying AttachPty stream
// erroring out (transport failure, unexpected close) must close events too,
// instead of leaving pipePush's read loop parked forever.
func TestTerminalCreateChannel_StreamErrorClosesEventsWithoutHanging(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	events := createSession(t, newTerminalTestCtx(), r, fake, "pty-1")
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	fake.lastStream.err <- errors.New("transport: connection reset")

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected the events channel to be closed after a stream error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the events channel to close after a stream error")
	}
}

func TestTerminalSendChannel_RelaysInputOnTheStream(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	ctx := newTerminalTestCtx()
	createSession(t, ctx, r, fake, "pty-1")
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	_, err := r.Dispatch(ctx, Identity{TenantID: "tenant-1"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-1", Data: "ls\n"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	frame := awaitSentFrame(t, fake.lastStream)
	input := frame.GetInput()
	if input == nil || string(input.GetData()) != "ls\n" {
		t.Errorf("expected an input frame carrying %q, got %+v", "ls\n", frame)
	}
}

func TestTerminalSendChannel_UnknownPtyID_ReturnsError(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, err := r.Dispatch(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-unknown", Data: "x"}))
	if err == nil {
		t.Fatal("expected an error for a pty_id with no live stream")
	}
}

func TestTerminalResizeChannel_CallsResizeRPC(t *testing.T) {
	var got *infrafleetv1.ResizeTerminalSessionRequest
	fake := &fakeTerminalInfraFleetClient{
		resizeFunc: func(in *infrafleetv1.ResizeTerminalSessionRequest) error { got = in; return nil },
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.resize", argsJSON(t, terminalResizeArgs{PtyID: "pty-1", Cols: 100, Rows: 40}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.GetPtyId() != "pty-1" || got.GetCols() != 100 || got.GetRows() != 40 {
		t.Errorf("unexpected ResizeTerminalSessionRequest: %+v", got)
	}
}

func TestTerminalCloseChannel_KillsAndRemovesStream(t *testing.T) {
	var killed *infrafleetv1.KillTerminalSessionRequest
	fake := &fakeTerminalInfraFleetClient{
		killFunc: func(in *infrafleetv1.KillTerminalSessionRequest) error { killed = in; return nil },
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	ctx := newTerminalTestCtx()
	createSession(t, ctx, r, fake, "pty-1")
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	_, err := r.Dispatch(ctx, Identity{TenantID: "tenant-1"}, "terminal.close", argsJSON(t, terminalPtyIDArg{PtyID: "pty-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if killed == nil || killed.GetPtyId() != "pty-1" {
		t.Errorf("expected KillTerminalSession to be called with pty-1, got %+v", killed)
	}

	// The stream entry must be gone — a subsequent terminal.send must fail.
	_, err = r.Dispatch(ctx, Identity{TenantID: "tenant-1"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-1", Data: "x"}))
	if err == nil {
		t.Error("expected terminal.send to fail after terminal.close removed the stream")
	}
}

func TestTerminalStopChannel_CallsStopRPC(t *testing.T) {
	var got *infrafleetv1.StopTerminalProcessRequest
	fake := &fakeTerminalInfraFleetClient{
		stopFunc: func(in *infrafleetv1.StopTerminalProcessRequest) error { got = in; return nil },
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.stop", argsJSON(t, terminalPtyIDArg{PtyID: "pty-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.GetPtyId() != "pty-1" {
		t.Errorf("unexpected StopTerminalProcessRequest: %+v", got)
	}
}

func TestTerminalListChannel_ReturnsSessions(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		listFunc: func(*infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
			return &infrafleetv1.ListTerminalSessionsResponse{Sessions: []*infrafleetv1.TerminalSession{
				{PtyId: "pty-1", ConnectionId: "conn-1"},
				{PtyId: "pty-2", ConnectionId: "conn-1"},
			}}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.list", argsJSON(t, terminalListArgs{ConnectionID: "conn-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	views, ok := result.([]terminalSessionView)
	if !ok || len(views) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTerminalWaitChannel_ReturnsExitOutcome(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		waitFunc: func(in *infrafleetv1.WaitTerminalSessionRequest) (*infrafleetv1.WaitTerminalSessionResponse, error) {
			if in.GetPtyId() != "pty-1" || in.GetTimeoutMs() != 500 {
				t.Errorf("unexpected WaitTerminalSessionRequest: %+v", in)
			}
			return &infrafleetv1.WaitTerminalSessionResponse{Exited: true, ExitCode: 2}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.wait", argsJSON(t, terminalWaitArgs{PtyID: "pty-1", TimeoutMs: 500}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(terminalWaitView)
	if !ok || !view.Exited || view.ExitCode != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTerminalFocusChannel_CallsFocusRPC(t *testing.T) {
	var got *infrafleetv1.FocusTerminalSessionRequest
	fake := &fakeTerminalInfraFleetClient{
		focusFunc: func(in *infrafleetv1.FocusTerminalSessionRequest) error { got = in; return nil },
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.focus", argsJSON(t, terminalPtyIDArg{PtyID: "pty-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.GetPtyId() != "pty-1" {
		t.Errorf("unexpected FocusTerminalSessionRequest: %+v", got)
	}
}

func TestTerminalAgentStatusAndIsRunningAgentChannels_ShareTheSameRPC(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		getTerminalStatusFunc: func(*infrafleetv1.GetTerminalAgentStatusRequest) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
			return &infrafleetv1.GetTerminalAgentStatusResponse{AgentRunning: true, AgentKind: "claude", ReadyForInput: true}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.agentStatus", argsJSON(t, terminalPtyIDArg{PtyID: "pty-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	statusView, ok := result.(terminalAgentStatusView)
	if !ok || !statusView.AgentRunning || statusView.AgentKind != "claude" {
		t.Fatalf("unexpected result: %+v", result)
	}

	result2, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.isRunningAgent", argsJSON(t, terminalPtyIDArg{PtyID: "pty-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	running, ok := result2.(bool)
	if !ok || !running {
		t.Fatalf("unexpected result: %+v", result2)
	}

	if fake.getTerminalStatusCalls != 2 {
		t.Errorf("expected GetTerminalAgentStatus to be called twice (once per channel), got %d", fake.getTerminalStatusCalls)
	}
}

func TestTerminalInspectProcessChannel_ReturnsProcessInfo(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		inspectProcessFunc: func(*infrafleetv1.InspectTerminalProcessRequest) (*infrafleetv1.InspectTerminalProcessResponse, error) {
			return &infrafleetv1.InspectTerminalProcessResponse{Known: true, Command: "claude", Cwd: "/work"}, nil
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.inspectProcess", argsJSON(t, terminalPtyIDArg{PtyID: "pty-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(terminalInspectProcessView)
	if !ok || !view.Known || view.Command != "claude" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// TestTerminalSendChannel_ConcurrentSendsAreSerializedBySendMu guards
// terminalStreamEntry.send's sendMu: grpc.ClientStream forbids concurrent
// Send calls from multiple goroutines, and terminal.send's handler runs on a
// fresh goroutine per invoke (handler.go's handleInvoke), so two panes typing
// into the SAME pty concurrently is a realistic race, not a hypothetical
// one. Run with -race — without sendMu this reliably trips the race
// detector on fakePtyStream.sent (a buffered channel is still a data race
// to send on concurrently without synchronization at the gRPC-stream layer
// in the real implementation, and the fake mirrors that contract).
func TestTerminalSendChannel_ConcurrentSendsAreSerializedBySendMu(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	ctx := newTerminalTestCtx()
	createSession(t, ctx, r, fake, "pty-1")
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	// concurrentSends deliberately exceeds fakePtyStream.sent's buffer
	// (16, see newFakePtyStream) — a concurrent drainer goroutine below
	// keeps it from filling up, since a full, unread channel would otherwise
	// block a Send call while it still holds sendMu, wedging every other
	// concurrent terminal.send behind it.
	const concurrentSends = 32
	got := make(chan *infrafleetv1.PtyClientFrame, concurrentSends)
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for i := 0; i < concurrentSends; i++ {
			got <- <-fake.lastStream.sent
		}
	}()

	var wg sync.WaitGroup
	wg.Add(concurrentSends)
	for i := 0; i < concurrentSends; i++ {
		go func() {
			defer wg.Done()
			_, err := r.Dispatch(ctx, Identity{TenantID: "tenant-1"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-1", Data: "x"}))
			if err != nil {
				t.Errorf("unexpected error from concurrent terminal.send: %v", err)
			}
		}()
	}
	wg.Wait()

	select {
	case <-drainDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out draining the stream — a send may have deadlocked while holding sendMu")
	}
	close(got)
	count := 0
	for frame := range got {
		if frame.GetInput() == nil {
			t.Fatalf("expected only input frames, got %+v", frame)
		}
		count++
	}
	if count != concurrentSends {
		t.Fatalf("expected %d input frames on the stream, got %d", concurrentSends, count)
	}
}

// TestTerminalStreamRegistry_IsolatesConnectionsWithTheSamePtyID replaces the
// old TestTerminalStreamRegistry_IsSharedAcrossAllConnections characterization
// test: now that terminalStreamRegistry is resolved per-connection from ctx
// (terminalStreamsContext/terminalStreamsFromContext, wired once per
// WebSocket connection in Handler.ServeHTTP), two different simulated
// connections that happen to create/attach the SAME pty_id must NOT collide
// — a terminal.send on one connection's ctx must only ever reach that same
// connection's AttachPty stream.
func TestTerminalStreamRegistry_IsolatesConnectionsWithTheSamePtyID(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake) // one process-wide Registry, exactly as main.go wires it

	// "Connection A" creates a terminal and gets ptyId "pty-shared".
	ctxA := newTerminalTestCtx()
	fake.spawnFunc = func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
		return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: "pty-shared", ConnectionId: "conn-A"}}, nil
	}
	if _, _, _, err := r.DispatchStreamChannel(ctxA, Identity{TenantID: "tenant-A"}, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "conn-A"})); err != nil {
		t.Fatalf("connection A's terminal.create failed: %v", err)
	}
	streamA := fake.lastStream
	awaitSentFrame(t, streamA) // drain A's attach frame

	// "Connection B" (a different simulated WS connection — its own
	// terminalStreamsContext, per this test's name) creates a terminal that
	// happens to land on the SAME ptyId.
	ctxB := newTerminalTestCtx()
	if _, _, _, err := r.DispatchStreamChannel(ctxB, Identity{TenantID: "tenant-B"}, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "conn-B"})); err != nil {
		t.Fatalf("connection B's terminal.create failed: %v", err)
	}
	streamB := fake.lastStream // AttachPty was called again, replacing lastStream
	awaitSentFrame(t, streamB) // drain B's attach frame
	if streamA == streamB {
		t.Fatal("test setup bug: expected two distinct AttachPty streams")
	}

	// terminal.send for "pty-shared" on connection B's ctx must reach ONLY
	// B's stream.
	if _, err := r.Dispatch(ctxB, Identity{TenantID: "tenant-B"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-shared", Data: "from-B"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case frame := <-streamB.sent:
		if string(frame.GetInput().GetData()) != "from-B" {
			t.Errorf("expected B's input on B's stream, got %+v", frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for B's stream to receive the send")
	}
	select {
	case frame := <-streamA.sent:
		t.Errorf("isolation regression: connection A's stream must never receive traffic sent on connection B's ctx, but got %+v", frame)
	case <-time.After(100 * time.Millisecond):
		// expected: A's stream stays silent.
	}

	// And the reverse: terminal.send on connection A's ctx must reach ONLY
	// A's stream, even though both registries hold an entry keyed
	// "pty-shared".
	if _, err := r.Dispatch(ctxA, Identity{TenantID: "tenant-A"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-shared", Data: "from-A"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case frame := <-streamA.sent:
		if string(frame.GetInput().GetData()) != "from-A" {
			t.Errorf("expected A's input on A's stream, got %+v", frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for A's stream to receive the send")
	}
	select {
	case frame := <-streamB.sent:
		t.Errorf("isolation regression: connection B's stream must never receive traffic sent on connection A's ctx, but got %+v", frame)
	case <-time.After(100 * time.Millisecond):
		// expected: B's stream stays silent.
	}

	// terminal.close on A's ctx must remove only A's registry entry — B's
	// "pty-shared" entry (a completely separate registry instance) must
	// still be reachable afterward.
	fake.killFunc = func(*infrafleetv1.KillTerminalSessionRequest) error { return nil }
	if _, err := r.Dispatch(ctxA, Identity{TenantID: "tenant-A"}, "terminal.close", argsJSON(t, terminalPtyIDArg{PtyID: "pty-shared"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := r.Dispatch(ctxA, Identity{TenantID: "tenant-A"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-shared", Data: "after-close"})); err == nil {
		t.Error("expected terminal.send on A's ctx to fail after A's terminal.close")
	}
	if _, err := r.Dispatch(ctxB, Identity{TenantID: "tenant-B"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-shared", Data: "still-alive"})); err != nil {
		t.Fatalf("expected B's stream to be unaffected by A's terminal.close, got error: %v", err)
	}
}

// TestTerminalCreateChannel_EndToEndPushInterleavesWithConcurrentSend drives
// the real Handler.ServeHTTP/pipePush/handleInvoke path (handler_test.go's
// dialTestClient/newTestHandlerServer harness, same as
// channels_push_test.go's notifications.subscribe integration test) to
// prove a concurrent terminal.send invoke on the SAME connection completes
// cleanly while terminal.create's push stream is actively delivering
// terminal.output events — both writes go through the same writeMu
// (handler.go), so neither frame corrupts the other.
func TestTerminalCreateChannel_EndToEndPushInterleavesWithConcurrentSend(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		spawnFunc: func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: "pty-1", ConnectionId: "conn-1"}}, nil
		},
	}
	registry := NewRegistry()
	registerTerminalChannels(registry, fake)

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "create-1", Type: "invoke", Channel: "terminal.create", Args: argsJSON(t, terminalCreateArgs{ConnectionID: "conn-1"})}); err != nil {
		t.Fatalf("writing terminal.create invoke: %v", err)
	}

	ack := readWireMessage(t, ctx, client)
	if ack.Type != "result" || ack.ID != "create-1" {
		t.Fatalf("first frame = %+v, want the terminal.create ack", ack)
	}
	var session terminalSessionView
	if err := json.Unmarshal(ack.Result, &session); err != nil || session.PtyID != "pty-1" {
		t.Fatalf("unexpected terminal.create ack result: %s (err=%v)", ack.Result, err)
	}

	stream := fake.lastStream
	if stream == nil {
		t.Fatal("expected AttachPty to have been called")
	}
	awaitSentFrame(t, stream) // drain the attach frame

	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "send-1", Type: "invoke", Channel: "terminal.send", Args: argsJSON(t, terminalSendArgs{PtyID: "pty-1", Data: "ls\n"})}); err != nil {
		t.Fatalf("writing terminal.send invoke: %v", err)
	}
	stream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Out{Out: &infrafleetv1.PtyOutput{Data: []byte("hello\n")}}}

	seenSendResult, seenPush := false, false
	for i := 0; i < 2; i++ {
		got := readWireMessage(t, ctx, client)
		switch {
		case got.Type == "result" && got.ID == "send-1":
			seenSendResult = true
		case got.Type == "push" && got.Channel == "terminal.output":
			seenPush = true
			payload, ok := got.Args[0].(map[string]any)
			if !ok || payload["ptyId"] != "pty-1" {
				t.Fatalf("unexpected terminal.output push payload: %+v", got.Args)
			}
		default:
			t.Fatalf("unexpected/corrupted frame %d: %+v", i, got)
		}
	}
	if !seenSendResult || !seenPush {
		t.Fatalf("missing frame(s): sendResult=%v push=%v", seenSendResult, seenPush)
	}

	// Drive the session to exit and confirm terminal.exited also arrives
	// over the wire, end to end.
	stream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Exited{Exited: &infrafleetv1.PtyExited{ExitCode: 3}}}
	exited := readWireMessage(t, ctx, client)
	if exited.Type != "push" || exited.Channel != "terminal.exited" {
		t.Fatalf("expected a terminal.exited push frame, got %+v", exited)
	}
	exitPayload, ok := exited.Args[0].(map[string]any)
	if !ok || exitPayload["ptyId"] != "pty-1" || exitPayload["exitCode"] != float64(3) {
		t.Fatalf("unexpected terminal.exited payload: %+v", exited.Args)
	}
}
