package wscompat

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeAgentInfraFleetClient is channels_agent_test.go's own
// InfraFleetServiceClient test double — kept separate from
// channels_terminal_test.go's fakeTerminalInfraFleetClient (this file's own
// scope, same convention that file's doc comment already establishes).
// Embeds the nil interface and overrides only the agent.* + AttachPty
// methods this file's channel handlers call.
type fakeAgentInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	startFunc         func(*infrafleetv1.StartAgentSessionRequest) (*infrafleetv1.AgentSession, error)
	stopFunc          func(*infrafleetv1.StopAgentSessionRequest) error
	killFunc          func(*infrafleetv1.KillAgentSessionRequest) error
	resumeFunc        func(*infrafleetv1.ResumeAgentSessionRequest) (*infrafleetv1.AgentSession, error)
	switchAccountFunc func(*infrafleetv1.SwitchAgentAccountRequest) (*infrafleetv1.AgentSession, error)
	attachPtyErr      error
	lastStream        *fakePtyStream
}

func (f *fakeAgentInfraFleetClient) StartAgentSession(_ context.Context, in *infrafleetv1.StartAgentSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.AgentSession, error) {
	return f.startFunc(in)
}

func (f *fakeAgentInfraFleetClient) StopAgentSession(_ context.Context, in *infrafleetv1.StopAgentSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.stopFunc(in)
}

func (f *fakeAgentInfraFleetClient) KillAgentSession(_ context.Context, in *infrafleetv1.KillAgentSessionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.killFunc(in)
}

func (f *fakeAgentInfraFleetClient) ResumeAgentSession(_ context.Context, in *infrafleetv1.ResumeAgentSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.AgentSession, error) {
	return f.resumeFunc(in)
}

func (f *fakeAgentInfraFleetClient) SwitchAgentAccount(_ context.Context, in *infrafleetv1.SwitchAgentAccountRequest, _ ...grpc.CallOption) (*infrafleetv1.AgentSession, error) {
	return f.switchAccountFunc(in)
}

func (f *fakeAgentInfraFleetClient) AttachPty(_ context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[infrafleetv1.PtyClientFrame, infrafleetv1.PtyServerFrame], error) {
	if f.attachPtyErr != nil {
		return nil, f.attachPtyErr
	}
	stream := newFakePtyStream()
	f.lastStream = stream
	return stream, nil
}

func TestAgentStartChannel_SpawnsAndOpensAttachPtyStream(t *testing.T) {
	fake := &fakeAgentInfraFleetClient{
		startFunc: func(in *infrafleetv1.StartAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
			if in.GetConnectionId() != "conn-1" || in.GetModelId() != "claude" {
				t.Errorf("unexpected StartAgentSessionRequest: %+v", in)
			}
			return &infrafleetv1.AgentSession{
				Id: "sess-1", PtyId: "agent-pty-1", WorktreeId: "wt-1", UserId: "user-1",
				ModelId: "claude", Status: "spawning", StartedAtUnixMs: 1000, LastActiveAtUnixMs: 1000,
			}, nil
		},
	}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)

	ack, events, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "agent.start",
		argsJSON(t, agentStartArgs{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1", ModelID: "claude"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isStream {
		t.Fatal("expected agent.start to be registered as a StreamChannelHandler")
	}
	if events == nil {
		t.Fatal("expected a non-nil push events channel")
	}
	view, ok := ack.(agentSessionView)
	if !ok || view.PtyID != "agent-pty-1" || view.ID != "sess-1" {
		t.Fatalf("unexpected ack: %+v", ack)
	}

	if fake.lastStream == nil {
		t.Fatal("expected AttachPty to have been called")
	}
	frame := awaitSentFrame(t, fake.lastStream)
	attach := frame.GetAttach()
	if attach == nil || attach.GetPtyId() != "agent-pty-1" {
		t.Errorf("expected the stream's first frame to be an attach frame for agent-pty-1, got %+v", frame)
	}
}

func TestAgentStartChannel_SpawnFailurePropagates(t *testing.T) {
	fake := &fakeAgentInfraFleetClient{
		startFunc: func(*infrafleetv1.StartAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
			return nil, errors.New("infra-fleet-service: an agent is already running")
		},
	}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)

	_, events, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "agent.start", argsJSON(t, agentStartArgs{ConnectionID: "conn-1"}))
	if !isStream {
		t.Fatal("expected agent.start to be registered as a StreamChannelHandler")
	}
	if err == nil {
		t.Fatal("expected the spawn error to propagate")
	}
	if events != nil {
		t.Error("expected a nil events channel on spawn failure")
	}
	if fake.lastStream != nil {
		t.Error("expected no AttachPty call when StartAgentSession fails")
	}
}

func TestAgentStartChannel_MissingStreamRegistryOnContext_ReturnsError(t *testing.T) {
	fake := &fakeAgentInfraFleetClient{
		startFunc: func(*infrafleetv1.StartAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
			t.Fatal("StartAgentSession should not be called before the stream-registry check")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)

	_, events, isStream, err := r.DispatchStreamChannel(context.Background(), Identity{TenantID: "tenant-1"}, "agent.start", argsJSON(t, agentStartArgs{ConnectionID: "conn-1"}))
	if !isStream {
		t.Fatal("expected agent.start to be registered as a StreamChannelHandler")
	}
	if err == nil {
		t.Fatal("expected an error when ctx has no per-connection terminal stream registry")
	}
	if events != nil {
		t.Error("expected a nil events channel")
	}
}

func TestAgentStopChannel_CallsStopRPC(t *testing.T) {
	var got *infrafleetv1.StopAgentSessionRequest
	fake := &fakeAgentInfraFleetClient{
		stopFunc: func(in *infrafleetv1.StopAgentSessionRequest) error { got = in; return nil },
	}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "agent.stop", argsJSON(t, agentStopArgs{SessionID: "sess-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.GetSessionId() != "sess-1" {
		t.Errorf("unexpected StopAgentSessionRequest: %+v", got)
	}
	m, ok := result.(map[string]any)
	if !ok || m["ok"] != true {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestAgentKillChannel_CallsKillRPC(t *testing.T) {
	var got *infrafleetv1.KillAgentSessionRequest
	fake := &fakeAgentInfraFleetClient{
		killFunc: func(in *infrafleetv1.KillAgentSessionRequest) error { got = in; return nil },
	}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "agent.kill", argsJSON(t, agentKillArgs{SessionID: "sess-1", Signal: "SIGTERM"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.GetSessionId() != "sess-1" || got.GetSignal() != "SIGTERM" {
		t.Errorf("unexpected KillAgentSessionRequest: %+v", got)
	}
}

func TestAgentResumeChannel_ResumesAndOpensAttachPtyStream(t *testing.T) {
	fake := &fakeAgentInfraFleetClient{
		resumeFunc: func(in *infrafleetv1.ResumeAgentSessionRequest) (*infrafleetv1.AgentSession, error) {
			if in.GetWorktreeId() != "wt-1" {
				t.Errorf("unexpected ResumeAgentSessionRequest: %+v", in)
			}
			return &infrafleetv1.AgentSession{Id: "sess-2", PtyId: "agent-pty-2", WorktreeId: "wt-1"}, nil
		},
	}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)

	ack, events, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "agent.resume",
		argsJSON(t, agentResumeArgs{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isStream || events == nil {
		t.Fatal("expected agent.resume to be a StreamChannelHandler with a non-nil events channel")
	}
	view, ok := ack.(agentSessionView)
	if !ok || view.PtyID != "agent-pty-2" {
		t.Fatalf("unexpected ack: %+v", ack)
	}
	if fake.lastStream == nil {
		t.Fatal("expected AttachPty to have been called for the resumed session")
	}
}

func TestAgentSwitchAccountChannel_SwitchesAndOpensAttachPtyStream(t *testing.T) {
	fake := &fakeAgentInfraFleetClient{
		switchAccountFunc: func(in *infrafleetv1.SwitchAgentAccountRequest) (*infrafleetv1.AgentSession, error) {
			if in.GetProjectId() != "proj-1" {
				t.Errorf("unexpected SwitchAgentAccountRequest: %+v", in)
			}
			return &infrafleetv1.AgentSession{Id: "sess-3", PtyId: "agent-pty-3", AccountId: "acc-new"}, nil
		},
	}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)

	ack, events, isStream, err := r.DispatchStreamChannel(newTerminalTestCtx(), Identity{TenantID: "tenant-1"}, "agent.switchAccount",
		argsJSON(t, agentSwitchAccountArgs{ConnectionID: "conn-1", WorktreeID: "wt-1", UserID: "user-1", ProjectID: "proj-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isStream || events == nil {
		t.Fatal("expected agent.switchAccount to be a StreamChannelHandler with a non-nil events channel")
	}
	view, ok := ack.(agentSessionView)
	if !ok || view.AccountID != "acc-new" {
		t.Fatalf("unexpected ack: %+v", ack)
	}
	if fake.lastStream == nil {
		t.Fatal("expected AttachPty to have been called for the switched session")
	}
}

// TestAgentSubscribeStatusChannel_NilBus_ReturnsClosedChannel guards the
// nil-bus degrade path (api-gateway couldn't reach NATS at startup) —
// agent.subscribeStatus must return an already-closed channel rather than
// erroring or panicking, per registerAgentStatusSubscribeChannel's doc
// comment.
func TestAgentSubscribeStatusChannel_NilBus_ReturnsClosedChannel(t *testing.T) {
	fake := &fakeAgentInfraFleetClient{}
	r := NewRegistry()
	registerAgentChannels(r, fake, nil)

	h, ok := r.StreamHandlerFor("agent.subscribeStatus")
	if !ok {
		t.Fatal("expected agent.subscribeStatus to be registered as a StreamHandler")
	}
	events, err := h(context.Background(), Identity{TenantID: "tenant-1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected the events channel to be closed immediately when bus is nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the nil-bus events channel to close")
	}
}
