package grpcclient

import (
	"context"
	"errors"
	"io"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

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

func TestAgentExecOutputRelay_StreamExecOutput_PumpsEventsIntoChannel(t *testing.T) {
	client := &fakeStreamExecOutputClient{
		stream: &fakeExecOutputStream{events: []*infrafleetv1.AgentExecOutputEvent{
			{StepId: "step-1", Stream: "stdout", Data: "hello "},
			{StepId: "step-1", Stream: "stdout", Data: "world"},
		}},
	}
	relay := NewAgentExecOutputRelay(client)

	out := relay.StreamExecOutput(context.Background(), "conn-1", "step-1")

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

	out := relay.StreamExecOutput(context.Background(), "conn-1", "step-1")

	select {
	case chunk, ok := <-out:
		if ok {
			t.Fatalf("expected an already-closed empty channel, got a chunk: %+v", chunk)
		}
	default:
		t.Fatal("expected the channel to be immediately closed (non-blocking receive)")
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

	out := relay.StreamExecOutput(context.Background(), "conn-1", "step-1")

	var got []usecase.AgentExecOutputChunk
	for chunk := range out {
		got = append(got, chunk)
	}
	if len(got) != 1 || got[0].Data != "partial" {
		t.Errorf("expected the one chunk delivered before the Recv error, got %+v", got)
	}
}
