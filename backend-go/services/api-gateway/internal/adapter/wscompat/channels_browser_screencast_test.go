package wscompat

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeScreencastInfraFleetClient/fakeScreencastStream mirror
// fakeTerminalInfraFleetClient/fakePtyStream (channels_terminal_test.go)
// exactly, for AttachScreencast instead of AttachPty.
type fakeScreencastInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	attachErr  error
	lastStream *fakeScreencastStream
}

func (f *fakeScreencastInfraFleetClient) AttachScreencast(_ context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[infrafleetv1.ScreencastClientFrame, infrafleetv1.ScreencastServerFrame], error) {
	if f.attachErr != nil {
		return nil, f.attachErr
	}
	stream := newFakeScreencastStream()
	f.lastStream = stream
	return stream, nil
}

type fakeScreencastStream struct {
	sent chan *infrafleetv1.ScreencastClientFrame
	recv chan *infrafleetv1.ScreencastServerFrame
	err  chan error
}

func newFakeScreencastStream() *fakeScreencastStream {
	return &fakeScreencastStream{
		sent: make(chan *infrafleetv1.ScreencastClientFrame, 16),
		recv: make(chan *infrafleetv1.ScreencastServerFrame, 16),
		err:  make(chan error, 1),
	}
}

func (s *fakeScreencastStream) Send(f *infrafleetv1.ScreencastClientFrame) error {
	s.sent <- f
	return nil
}

func (s *fakeScreencastStream) Recv() (*infrafleetv1.ScreencastServerFrame, error) {
	select {
	case f := <-s.recv:
		return f, nil
	case err := <-s.err:
		return nil, err
	}
}

func (s *fakeScreencastStream) Header() (metadata.MD, error) { return nil, nil }
func (s *fakeScreencastStream) Trailer() metadata.MD         { return nil }
func (s *fakeScreencastStream) CloseSend() error             { s.err <- errCloseSend; return nil }
func (s *fakeScreencastStream) Context() context.Context     { return context.Background() }
func (s *fakeScreencastStream) SendMsg(any) error            { return nil }
func (s *fakeScreencastStream) RecvMsg(any) error            { return nil }

func TestBrowserScreencastChannel_RequiresWorktree(t *testing.T) {
	fake := &fakeScreencastInfraFleetClient{}
	r := NewRegistry()
	registerBrowserScreencastChannel(r, fake)
	h, ok := r.BinaryStreamHandlerFor("browser.screencast")
	if !ok {
		t.Fatal("expected browser.screencast to be registered as a binary stream handler")
	}

	io := newFakeBinaryStreamIO()
	_, err := h(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, argsJSON(t, map[string]any{}), io.io())
	if err == nil {
		t.Fatal("expected an error when worktree is missing")
	}
	if fake.lastStream != nil {
		t.Error("expected AttachScreencast to never be called without a worktree")
	}
}

func TestBrowserScreencastChannel_SendsClampedStartAndReturnsReadyAck(t *testing.T) {
	fake := &fakeScreencastInfraFleetClient{}
	r := NewRegistry()
	registerBrowserScreencastChannel(r, fake)
	h, _ := r.BinaryStreamHandlerFor("browser.screencast")

	io := newFakeBinaryStreamIO()
	args := argsJSON(t, map[string]any{
		"worktree": "wt-1", "page": "page-1", "format": "png",
		"quality": 500, "maxWidth": 1, "everyNthFrame": 0,
	})

	resultCh := make(chan any, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := h(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, args, io.io())
		resultCh <- result
		errCh <- err
	}()

	sent := awaitSentScreencastFrame(t, fake)
	start := sent.GetStart()
	if start.GetWorktreeId() != "wt-1" || start.GetPage() != "page-1" {
		t.Errorf("unexpected start frame: %+v", start)
	}
	if start.GetFormat() != "png" {
		t.Errorf("expected format=png to pass through, got %q", start.GetFormat())
	}
	if start.GetQuality() != 100 {
		t.Errorf("expected quality clamped to 100, got %d", start.GetQuality())
	}
	if start.GetMaxWidth() != 320 {
		t.Errorf("expected maxWidth clamped to 320, got %d", start.GetMaxWidth())
	}
	if start.GetEveryNthFrame() != 1 {
		t.Errorf("expected everyNthFrame=0 clamped up to the floor (1), got %d", start.GetEveryNthFrame())
	}
	if start.GetMaxHeight() != 1200 {
		t.Errorf("expected an OMITTED maxHeight to fall back to its default 1200, got %d", start.GetMaxHeight())
	}

	fake.lastStream.recv <- &infrafleetv1.ScreencastServerFrame{Frame: &infrafleetv1.ScreencastServerFrame_Ready{
		Ready: &infrafleetv1.ScreencastReady{SubscriptionId: "sub-1", BrowserPageId: "page-1", Format: "png"},
	}}

	select {
	case result := <-resultCh:
		ack, ok := result.(map[string]any)
		if !ok || ack["subscriptionId"] != "sub-1" {
			t.Errorf("expected ack with subscriptionId sub-1, got %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the invoke ack")
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBrowserScreencastChannel_FramesForwardedViaSendBinary(t *testing.T) {
	fake := &fakeScreencastInfraFleetClient{}
	r := NewRegistry()
	registerBrowserScreencastChannel(r, fake)
	h, _ := r.BinaryStreamHandlerFor("browser.screencast")

	io := newFakeBinaryStreamIO()
	args := argsJSON(t, map[string]any{"worktree": "wt-1"})

	go func() { _, _ = h(context.Background(), Identity{TenantID: "t1"}, args, io.io()) }()

	awaitSentScreencastFrame(t, fake)
	fake.lastStream.recv <- &infrafleetv1.ScreencastServerFrame{Frame: &infrafleetv1.ScreencastServerFrame_Ready{
		Ready: &infrafleetv1.ScreencastReady{SubscriptionId: "sub-1"},
	}}
	// Give the handler a moment to return its ack and spawn the pump
	// goroutine before pushing a frame.
	time.Sleep(50 * time.Millisecond)

	fake.lastStream.recv <- &infrafleetv1.ScreencastServerFrame{Frame: &infrafleetv1.ScreencastServerFrame_FrameData{
		FrameData: &infrafleetv1.ScreencastFrame{Data: []byte("opaque-jpeg-frame-bytes")},
	}}

	select {
	case got := <-io.sent:
		if string(got) != "opaque-jpeg-frame-bytes" {
			t.Errorf("expected the frame's raw bytes to pass through io.SendBinary opaquely, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SendBinary to be called")
	}
}

func TestBrowserScreencastChannel_ErrorFirstFrame_FailsSynchronously(t *testing.T) {
	fake := &fakeScreencastInfraFleetClient{}
	r := NewRegistry()
	registerBrowserScreencastChannel(r, fake)
	h, _ := r.BinaryStreamHandlerFor("browser.screencast")

	io := newFakeBinaryStreamIO()
	args := argsJSON(t, map[string]any{"worktree": "wt-1"})

	resultCh := make(chan any, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := h(context.Background(), Identity{TenantID: "t1"}, args, io.io())
		resultCh <- result
		errCh <- err
	}()

	awaitSentScreencastFrame(t, fake)
	fake.lastStream.recv <- &infrafleetv1.ScreencastServerFrame{Frame: &infrafleetv1.ScreencastServerFrame_Error{
		Error: &infrafleetv1.ScreencastError{Message: "no chrome on host"},
	}}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected an error when the first frame is Error, not Ready")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the invoke to fail")
	}
}

func TestBrowserScreencastChannel_AttachScreencastError_PropagatesSynchronously(t *testing.T) {
	fake := &fakeScreencastInfraFleetClient{attachErr: errors.New("dial failed")}
	r := NewRegistry()
	registerBrowserScreencastChannel(r, fake)
	h, _ := r.BinaryStreamHandlerFor("browser.screencast")

	io := newFakeBinaryStreamIO()
	args := argsJSON(t, map[string]any{"worktree": "wt-1"})
	_, err := h(context.Background(), Identity{TenantID: "t1"}, args, io.io())
	if err == nil {
		t.Fatal("expected AttachScreencast's error to propagate")
	}
}

func awaitSentScreencastFrame(t *testing.T, fake *fakeScreencastInfraFleetClient) *infrafleetv1.ScreencastClientFrame {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if fake.lastStream != nil {
			select {
			case f := <-fake.lastStream.sent:
				return f
			case <-deadline:
				t.Fatal("timed out waiting for a frame to be sent on the fake AttachScreencast stream")
				return nil
			}
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for AttachScreencast to be called")
			return nil
		case <-time.After(5 * time.Millisecond):
		}
	}
}
