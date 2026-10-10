package grpcclient

import (
	"context"
	"errors"
	"io"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// fakeExecOutputStream implements grpc.ServerStreamingClient[AgentExecOutputEvent]
// by hand — the ClientStream methods below are never exercised by
// AgentExecOutputRelay.StreamExecOutput (it only calls Recv), they exist
// only to satisfy the generated interface, mirroring this package's
// existing fakeInfraFleetServiceClient's "fake the port, not the
// transport" convention.
type fakeExecOutputStream struct {
	events []*infrafleetv1.AgentExecOutputEvent
	pos    int
	// finalErr is returned once events is exhausted — nil means io.EOF
	// (the normal "stream ended cleanly" case).
	finalErr error
}

func (f *fakeExecOutputStream) Recv() (*infrafleetv1.AgentExecOutputEvent, error) {
	if f.pos < len(f.events) {
		ev := f.events[f.pos]
		f.pos++
		return ev, nil
	}
	if f.finalErr != nil {
		return nil, f.finalErr
	}
	return nil, io.EOF
}

func (f *fakeExecOutputStream) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeExecOutputStream) Trailer() metadata.MD         { return nil }
func (f *fakeExecOutputStream) CloseSend() error             { return nil }
func (f *fakeExecOutputStream) Context() context.Context     { return context.Background() }
func (f *fakeExecOutputStream) SendMsg(m any) error          { return nil }
func (f *fakeExecOutputStream) RecvMsg(m any) error          { return nil }

// fakeStreamExecOutputClient extends fakeInfraFleetServiceClient with a
// StreamExecOutput implementation — kept in this file (not
// fakeInfraFleetServiceClient itself) since AgentExecOutputRelay is the
// only caller of this method; simple_executor_test.go's tests never touch
// it (they depend on the narrower usecase.AgentExecOutputStreamer port
// instead — see AgentExecOutputRelay's own doc comment for why).
type fakeStreamExecOutputClient struct {
	fakeInfraFleetServiceClient

	stream       *fakeExecOutputStream
	streamErr    error
	gotStreamReq *infrafleetv1.StreamExecOutputRequest
}

func (f *fakeStreamExecOutputClient) StreamExecOutput(ctx context.Context, in *infrafleetv1.StreamExecOutputRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[infrafleetv1.AgentExecOutputEvent], error) {
	f.gotStreamReq = in
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return f.stream, nil
}

// streamFailures records onFailure calls.
type streamFailures struct{ errs []error }

func (f *streamFailures) record(err error) { f.errs = append(f.errs, err) }

func TestAgentExecOutputRelay_StreamExecOutput_PumpsEventsIntoChannel(t *testing.T) {
	client := &fakeStreamExecOutputClient{
		stream: &fakeExecOutputStream{events: []*infrafleetv1.AgentExecOutputEvent{
			{StepId: "step-1", Stream: "stdout", Data: "hello "},
			{StepId: "step-1", Stream: "stdout", Data: "world"},
		}},
	}
	relay := NewAgentExecOutputRelay(client)
	failures := &streamFailures{}

	out := relay.StreamExecOutput(context.Background(), usecase.AgentExecOutputTarget{ConnectionID: "conn-1", StepID: "step-1"}, failures.record)

	var got []usecase.AgentExecOutputChunk
	for chunk := range out {
		got = append(got, chunk)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %+v", len(got), got)
	}
	if got[0].Data != "hello " || got[1].Data != "world" {
		t.Errorf("unexpected chunk contents: %+v", got)
	}
	if client.gotStreamReq.GetConnectionId() != "conn-1" || client.gotStreamReq.GetStepId() != "step-1" {
		t.Errorf("expected connectionId/stepId to be forwarded, got %+v", client.gotStreamReq)
	}
}

func TestAgentExecOutputRelay_StreamExecOutput_DialFailure_ReturnsClosedEmptyChannel(t *testing.T) {
	client := &fakeStreamExecOutputClient{streamErr: errors.New("boom")}
	relay := NewAgentExecOutputRelay(client)
	failures := &streamFailures{}

	out := relay.StreamExecOutput(context.Background(), usecase.AgentExecOutputTarget{ConnectionID: "conn-1", StepID: "step-1"}, failures.record)

	select {
	case chunk, ok := <-out:
		if ok {
			t.Fatalf("expected an already-closed empty channel, got a chunk: %+v", chunk)
		}
	default:
		t.Fatal("expected the channel to be immediately closed (non-blocking receive)")
	}
	if len(failures.errs) != 1 {
		t.Errorf("expected the dial failure to be reported once, got %v", failures.errs)
	}
}

func TestAgentExecOutputRelay_StreamExecOutput_RecvErrorClosesChannelWithoutPanicking(t *testing.T) {
	client := &fakeStreamExecOutputClient{
		stream: &fakeExecOutputStream{
			events:   []*infrafleetv1.AgentExecOutputEvent{{StepId: "step-1", Stream: "stdout", Data: "partial"}},
			finalErr: errors.New("transport error"),
		},
	}
	relay := NewAgentExecOutputRelay(client)
	failures := &streamFailures{}

	out := relay.StreamExecOutput(context.Background(), usecase.AgentExecOutputTarget{ConnectionID: "conn-1", StepID: "step-1"}, failures.record)

	var got []usecase.AgentExecOutputChunk
	for chunk := range out {
		got = append(got, chunk)
	}
	if len(got) != 1 || got[0].Data != "partial" {
		t.Errorf("expected the one chunk delivered before the Recv error, got %+v", got)
	}
	if len(failures.errs) != 1 {
		t.Errorf("expected the broken stream to be reported once, got %v", failures.errs)
	}
}

// The server's own refusal (infra-fleet rejects an unknown target) arrives on the first Recv, not at dial time.
func TestAgentExecOutputRelay_StreamExecOutput_ServerRefusalIsReported(t *testing.T) {
	refusal := status.Error(codes.InvalidArgument, "connectionId or devServerId is required")
	client := &fakeStreamExecOutputClient{stream: &fakeExecOutputStream{finalErr: refusal}}
	failures := &streamFailures{}

	for range NewAgentExecOutputRelay(client).StreamExecOutput(context.Background(), usecase.AgentExecOutputTarget{StepID: "step-1"}, failures.record) {
	}
	if len(failures.errs) != 1 || status.Code(failures.errs[0]) != codes.InvalidArgument {
		t.Errorf("expected the refusal to be reported, got %v", failures.errs)
	}
}

// A clean end (EOF) and the caller's own cancellation are not failures.
func TestAgentExecOutputRelay_StreamExecOutput_CleanEndAndCancelAreNotFailures(t *testing.T) {
	failures := &streamFailures{}
	client := &fakeStreamExecOutputClient{stream: &fakeExecOutputStream{events: []*infrafleetv1.AgentExecOutputEvent{{Data: "x"}}}}
	for range NewAgentExecOutputRelay(client).StreamExecOutput(context.Background(), usecase.AgentExecOutputTarget{ConnectionID: "c", StepID: "s"}, failures.record) {
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := &fakeStreamExecOutputClient{stream: &fakeExecOutputStream{finalErr: status.Error(codes.Canceled, "context canceled")}}
	for range NewAgentExecOutputRelay(cancelled).StreamExecOutput(ctx, usecase.AgentExecOutputTarget{ConnectionID: "c", StepID: "s"}, failures.record) {
	}
	if len(failures.errs) != 0 {
		t.Errorf("expected no failure reports, got %v", failures.errs)
	}
}

// The dev server id travels to infra-fleet next to (not instead of) the connection id.
func TestAgentExecOutputRelay_StreamExecOutput_ForwardsDevServerID(t *testing.T) {
	client := &fakeStreamExecOutputClient{stream: &fakeExecOutputStream{}}
	for range NewAgentExecOutputRelay(client).StreamExecOutput(context.Background(), usecase.AgentExecOutputTarget{DevServerID: "ds-1", StepID: "step-1"}, nil) {
	}
	if client.gotStreamReq.GetDevServerId() != "ds-1" || client.gotStreamReq.GetConnectionId() != "" || client.gotStreamReq.GetStepId() != "step-1" {
		t.Errorf("request = %+v", client.gotStreamReq)
	}
}
