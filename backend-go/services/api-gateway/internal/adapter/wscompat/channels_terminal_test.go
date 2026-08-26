package wscompat

import (
	"context"
	"errors"
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
	lastStream             *fakePtyStream
	getTerminalStatusCalls int
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
	f.lastStream = newFakePtyStream()
	return f.lastStream, nil
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

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "terminal.create",
		argsJSON(t, terminalCreateArgs{ConnectionID: "conn-1", Cwd: "/repo", Cols: 80, Rows: 24}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(terminalSessionView)
	if !ok || view.PtyID != "pty-1" || view.ConnectionID != "conn-1" {
		t.Fatalf("unexpected result: %+v", result)
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

func TestTerminalCreateChannel_SpawnFailurePropagates(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{
		spawnFunc: func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			return nil, errors.New("infra-fleet-service: no dev server owns this connectionId")
		},
	}
	r := NewRegistry()
	registerTerminalChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "unknown"}))
	if err == nil {
		t.Fatal("expected the spawn error to propagate")
	}
	if fake.lastStream != nil {
		t.Error("expected no AttachPty call when SpawnTerminalSession fails")
	}
}

// createSession is the shared setup for tests that need a live
// terminal.create'd session before exercising send/resize/close/etc.
func createSession(t *testing.T, r *Registry, fake *fakeTerminalInfraFleetClient, ptyID string) {
	t.Helper()
	if fake.spawnFunc == nil {
		fake.spawnFunc = func(*infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: ptyID, ConnectionId: "conn-1"}}, nil
		}
	}
	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.create", argsJSON(t, terminalCreateArgs{ConnectionID: "conn-1"})); err != nil {
		t.Fatalf("terminal.create setup failed: %v", err)
	}
}

func TestTerminalSendChannel_RelaysInputOnTheStream(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	r := NewRegistry()
	registerTerminalChannels(r, fake)
	createSession(t, r, fake, "pty-1")
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-1", Data: "ls\n"}))
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

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-unknown", Data: "x"}))
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
	createSession(t, r, fake, "pty-1")
	awaitSentFrame(t, fake.lastStream) // drain the attach frame

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.close", argsJSON(t, terminalPtyIDArg{PtyID: "pty-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if killed == nil || killed.GetPtyId() != "pty-1" {
		t.Errorf("expected KillTerminalSession to be called with pty-1, got %+v", killed)
	}

	// The stream entry must be gone — a subsequent terminal.send must fail.
	_, err = r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.send", argsJSON(t, terminalSendArgs{PtyID: "pty-1", Data: "x"}))
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
