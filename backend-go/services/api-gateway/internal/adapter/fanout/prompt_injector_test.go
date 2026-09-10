package fanout

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeAttachPtyStream implements grpc.BidiStreamingClient[PtyClientFrame,
// PtyServerFrame] just enough to drive InjectPrompt's Send/CloseSend
// sequence without a real gRPC transport — mirrors
// wscompat/channels_terminal_test.go's fakePtyStream, kept as this
// package's own minimal copy since GRPCPromptInjector only ever sends,
// never reads.
type fakeAttachPtyStream struct {
	sent            []*infrafleetv1.PtyClientFrame
	closeSendCalled bool
	sendErr         error
}

func (s *fakeAttachPtyStream) Send(f *infrafleetv1.PtyClientFrame) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, f)
	return nil
}
func (s *fakeAttachPtyStream) Recv() (*infrafleetv1.PtyServerFrame, error) {
	return nil, errors.New("fakeAttachPtyStream: Recv not supported")
}
func (s *fakeAttachPtyStream) Header() (metadata.MD, error) { return nil, nil }
func (s *fakeAttachPtyStream) Trailer() metadata.MD         { return nil }
func (s *fakeAttachPtyStream) CloseSend() error             { s.closeSendCalled = true; return nil }
func (s *fakeAttachPtyStream) Context() context.Context     { return context.Background() }
func (s *fakeAttachPtyStream) SendMsg(any) error            { return nil }
func (s *fakeAttachPtyStream) RecvMsg(any) error            { return nil }

type fakeAttachPtyInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	stream       *fakeAttachPtyStream
	attachPtyErr error
}

func (f *fakeAttachPtyInfraFleetClient) AttachPty(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[infrafleetv1.PtyClientFrame, infrafleetv1.PtyServerFrame], error) {
	if f.attachPtyErr != nil {
		return nil, f.attachPtyErr
	}
	return f.stream, nil
}

// TestGRPCPromptInjector_SendsAttachFrameThenInputFrame asserts InjectPrompt's
// frame sequence: an Attach frame sent before an Input frame.
func TestGRPCPromptInjector_SendsAttachFrameThenInputFrame(t *testing.T) {
	stream := &fakeAttachPtyStream{}
	client := &fakeAttachPtyInfraFleetClient{stream: stream}
	injector := NewGRPCPromptInjector(client)

	if err := injector.InjectPrompt(context.Background(), "conn-1", "pty-1", "hello agent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stream.sent) != 2 {
		t.Fatalf("expected exactly 2 frames sent, got %d", len(stream.sent))
	}
	attach, ok := stream.sent[0].GetFrame().(*infrafleetv1.PtyClientFrame_Attach)
	if !ok {
		t.Fatalf("expected first frame to be an Attach frame, got %T", stream.sent[0].GetFrame())
	}
	if attach.Attach.GetPtyId() != "pty-1" {
		t.Errorf("expected Attach.PtyId=pty-1, got %q", attach.Attach.GetPtyId())
	}
	input, ok := stream.sent[1].GetFrame().(*infrafleetv1.PtyClientFrame_Input)
	if !ok {
		t.Fatalf("expected second frame to be an Input frame, got %T", stream.sent[1].GetFrame())
	}
	if string(input.Input.GetData()) != "hello agent\n" {
		t.Errorf("expected Input.Data=%q, got %q", "hello agent\n", string(input.Input.GetData()))
	}
	if !stream.closeSendCalled {
		t.Error("expected CloseSend to have been called (via defer)")
	}
}

func TestGRPCPromptInjector_AttachPtyFails_NoFramesSent(t *testing.T) {
	client := &fakeAttachPtyInfraFleetClient{attachPtyErr: errors.New("infra-fleet-service unreachable")}
	injector := NewGRPCPromptInjector(client)

	if err := injector.InjectPrompt(context.Background(), "conn-1", "pty-1", "hello"); err == nil {
		t.Fatal("expected an error")
	}
}
