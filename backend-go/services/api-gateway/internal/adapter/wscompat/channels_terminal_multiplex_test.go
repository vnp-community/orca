package wscompat

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeBinaryStreamIO is a unit-test double for BinaryStreamIO — reused
// across every test in this file so registerTerminalMultiplexChannel's
// handler can be exercised directly, without a real *websocket.Conn (the
// same "test against fakes" convention channels_terminal_test.go's
// newTerminalTestCtx already establishes for the JSON side).
type fakeBinaryStreamIO struct {
	mu       sync.Mutex
	handlers map[uint32]BinaryFrameHandler
	sent     chan []byte
}

func newFakeBinaryStreamIO() *fakeBinaryStreamIO {
	return &fakeBinaryStreamIO{handlers: make(map[uint32]BinaryFrameHandler), sent: make(chan []byte, 32)}
}

func (f *fakeBinaryStreamIO) io() BinaryStreamIO {
	return BinaryStreamIO{
		SendBinary: func(frame []byte) bool {
			f.sent <- frame
			return true
		},
		RegisterFrameHandler: func(streamID uint32, h BinaryFrameHandler) func() {
			f.mu.Lock()
			f.handlers[streamID] = h
			f.mu.Unlock()
			return func() {
				f.mu.Lock()
				delete(f.handlers, streamID)
				f.mu.Unlock()
			}
		},
	}
}

// deliver simulates an inbound WS binary frame reaching streamID's
// registered handler, if any — mirrors what handler.go's ServeHTTP read
// loop does for a real connection (decode, then binaryStreamRouter.dispatch).
func (f *fakeBinaryStreamIO) deliver(frame TerminalStreamFrame) {
	f.mu.Lock()
	h := f.handlers[frame.StreamID]
	f.mu.Unlock()
	if h != nil {
		h(frame)
	}
}

func (f *fakeBinaryStreamIO) hasHandler(streamID uint32) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.handlers[streamID]
	return ok
}

func awaitSentBinaryFrame(t *testing.T, sent <-chan []byte) TerminalStreamFrame {
	t.Helper()
	select {
	case raw := <-sent:
		frame, err := DecodeTerminalStreamFrame(raw)
		if err != nil {
			t.Fatalf("decoding sent binary frame: %v", err)
		}
		return frame
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a binary frame to be sent")
		return TerminalStreamFrame{}
	}
}

func subscribeFramePayload(t *testing.T, ptyID string, streamID uint32) []byte {
	t.Helper()
	b, err := json.Marshal(terminalMultiplexSubscribePayload{PtyID: ptyID, StreamID: streamID})
	if err != nil {
		t.Fatalf("marshaling subscribe payload: %v", err)
	}
	return b
}

// startMultiplex registers terminal.multiplex on a fresh Registry, invokes
// it once (mirrors one WS connection's single terminal.multiplex call), and
// returns the fake binary IO plus a cancel func that simulates the WS
// connection closing.
func startMultiplex(t *testing.T, fake *fakeTerminalInfraFleetClient) (*fakeBinaryStreamIO, context.CancelFunc) {
	t.Helper()
	registry := NewRegistry()
	registerTerminalMultiplexChannel(registry, fake)
	h, ok := registry.BinaryStreamHandlerFor("terminal.multiplex")
	if !ok {
		t.Fatal("terminal.multiplex was not registered as a BinaryStreamChannelHandler")
	}
	ctx, cancel := context.WithCancel(context.Background())
	io := newFakeBinaryStreamIO()
	ack, err := h(ctx, Identity{TenantID: "tenant-1"}, nil, io.io())
	if err != nil {
		t.Fatalf("terminal.multiplex invoke: %v", err)
	}
	if okMap, ok := ack.(map[string]bool); !ok || !okMap["ok"] {
		t.Fatalf("unexpected terminal.multiplex ack: %+v", ack)
	}
	return io, cancel
}

func TestTerminalMultiplexChannel_SubscribeOpensAttachPtyAndForwardsOutput(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	io, cancel := startMultiplex(t, fake)
	defer cancel()

	io.deliver(TerminalStreamFrame{
		Opcode:   TerminalStreamOpcodeSubscribe,
		StreamID: terminalMultiplexControlStreamID,
		Payload:  subscribeFramePayload(t, "pty-1", 7),
	})

	stream := fake.lastStream
	if stream == nil {
		t.Fatal("expected AttachPty to have been called on Subscribe")
	}
	attachFrame := awaitSentFrame(t, stream)
	attach := attachFrame.GetAttach()
	if attach == nil || attach.GetPtyId() != "pty-1" {
		t.Fatalf("expected an Attach frame for pty-1, got %+v", attachFrame)
	}

	stream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Out{Out: &infrafleetv1.PtyOutput{Data: []byte("hello\n")}}}

	got := awaitSentBinaryFrame(t, io.sent)
	if got.Opcode != TerminalStreamOpcodeOutput || got.StreamID != 7 || string(got.Payload) != "hello\n" {
		t.Fatalf("unexpected Output frame: %+v", got)
	}
}

func TestTerminalMultiplexChannel_ExitedSendsErrorFrame(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	io, cancel := startMultiplex(t, fake)
	defer cancel()

	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeSubscribe, StreamID: 0, Payload: subscribeFramePayload(t, "pty-1", 3)})
	stream := fake.lastStream
	awaitSentFrame(t, stream) // drain the attach frame

	stream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Exited{Exited: &infrafleetv1.PtyExited{ExitCode: 9}}}

	got := awaitSentBinaryFrame(t, io.sent)
	if got.Opcode != TerminalStreamOpcodeError || got.StreamID != 3 {
		t.Fatalf("expected an Error frame on streamId 3 for process exit, got %+v", got)
	}
}

func TestTerminalMultiplexChannel_InputForwardsRawBytesToPtyStream(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	io, cancel := startMultiplex(t, fake)
	defer cancel()

	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeSubscribe, StreamID: 0, Payload: subscribeFramePayload(t, "pty-1", 1)})
	stream := fake.lastStream
	awaitSentFrame(t, stream) // drain the attach frame

	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeInput, StreamID: 1, Payload: []byte("ls\n")})

	sent := awaitSentFrame(t, stream)
	input := sent.GetInput()
	if input == nil || string(input.GetData()) != "ls\n" {
		t.Fatalf("expected an Input frame carrying \"ls\\n\", got %+v", sent)
	}
}

func TestTerminalMultiplexChannel_ResizeCallsResizeRPC(t *testing.T) {
	var resizeCalls []*infrafleetv1.ResizeTerminalSessionRequest
	fake := &fakeTerminalInfraFleetClient{
		resizeFunc: func(req *infrafleetv1.ResizeTerminalSessionRequest) error {
			resizeCalls = append(resizeCalls, req)
			return nil
		},
	}
	io, cancel := startMultiplex(t, fake)
	defer cancel()

	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeSubscribe, StreamID: 0, Payload: subscribeFramePayload(t, "pty-1", 5)})
	stream := fake.lastStream
	awaitSentFrame(t, stream) // drain the attach frame

	resizePayload, err := json.Marshal(terminalMultiplexResizePayload{Cols: 120, Rows: 40})
	if err != nil {
		t.Fatalf("marshaling resize payload: %v", err)
	}
	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeResize, StreamID: 5, Payload: resizePayload})

	deadline := time.After(2 * time.Second)
	for len(resizeCalls) == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for ResizeTerminalSession to be called")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if resizeCalls[0].GetPtyId() != "pty-1" || resizeCalls[0].GetCols() != 120 || resizeCalls[0].GetRows() != 40 {
		t.Fatalf("unexpected ResizeTerminalSessionRequest: %+v", resizeCalls[0])
	}
}

func TestTerminalMultiplexChannel_UnsubscribeStopsForwardingFurtherInput(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	io, cancel := startMultiplex(t, fake)
	defer cancel()

	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeSubscribe, StreamID: 0, Payload: subscribeFramePayload(t, "pty-1", 2)})
	stream := fake.lastStream
	awaitSentFrame(t, stream) // drain the attach frame

	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeUnsubscribe, StreamID: 2})

	// Give detach's synchronous teardown a moment, then confirm the slot's
	// frame handler was unregistered (a real connection would simply have
	// nothing to route a stray frame to afterward).
	deadline := time.After(2 * time.Second)
	for io.hasHandler(2) {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for streamId 2's frame handler to be unregistered after Unsubscribe")
		case <-time.After(10 * time.Millisecond):
		}
	}

	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeInput, StreamID: 2, Payload: []byte("should-not-forward")})
	select {
	case f := <-stream.sent:
		t.Fatalf("expected no further frames sent on the pty stream after Unsubscribe, got %+v", f)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTerminalMultiplexChannel_ConnectionCloseTearsDownEverySlot(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	io, cancel := startMultiplex(t, fake)

	io.deliver(TerminalStreamFrame{Opcode: TerminalStreamOpcodeSubscribe, StreamID: 0, Payload: subscribeFramePayload(t, "pty-1", 4)})
	stream := fake.lastStream
	awaitSentFrame(t, stream) // drain the attach frame

	cancel() // simulates the WS connection closing

	deadline := time.After(2 * time.Second)
	for io.hasHandler(4) {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for the slot to be torn down after the connection context was cancelled")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// readBinaryFrame reads exactly one WS binary message and decodes it as a
// TerminalStreamFrame — the binary-wire counterpart of readWireMessage
// (handler_test.go), which only handles text/JSON frames (wsjson.Read would
// fail to JSON-decode a binary multiplex frame — see handler.go's read loop
// for why the two paths are split).
func readBinaryFrame(t *testing.T, ctx context.Context, conn *websocket.Conn) TerminalStreamFrame {
	t.Helper()
	typ, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reading binary frame: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("expected a binary WS message, got message type %v", typ)
	}
	frame, err := DecodeTerminalStreamFrame(data)
	if err != nil {
		t.Fatalf("decoding binary frame: %v", err)
	}
	return frame
}

func writeBinaryFrame(ctx context.Context, conn *websocket.Conn, frame TerminalStreamFrame) error {
	return conn.Write(ctx, websocket.MessageBinary, EncodeTerminalStreamFrame(frame))
}

// awaitLastStream polls for fake.getLastStream() — AttachPty happens on a
// goroutine handler.go's read loop spawns for every binary frame (so a slow
// Subscribe never blocks reading the next WS frame on the connection, see
// that loop's own doc comment), so it is not necessarily set by the time
// writeBinaryFrame's call returns. Uses the mutex-guarded getter (not the
// raw field every synchronous unit test above reads directly) since this is
// the one test in this file that reads it from a different goroutine than
// the one that calls AttachPty.
func awaitLastStream(t *testing.T, fake *fakeTerminalInfraFleetClient) *fakePtyStream {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if s := fake.getLastStream(); s != nil {
			return s
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for AttachPty to be called")
			return nil
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// TestTerminalMultiplexChannel_EndToEndSubscribeOutputAndInput drives the
// real Handler.ServeHTTP path (handler_test.go's dialTestClient/
// newTestHandlerServer harness, same as
// TestTerminalCreateChannel_EndToEndPushInterleavesWithConcurrentSend) with
// ACTUAL binary WS frames on the wire — proving terminal.multiplex's
// Subscribe/Output/Input round trip works end to end, not just against the
// in-process fakeBinaryStreamIO the unit tests above use.
func TestTerminalMultiplexChannel_EndToEndSubscribeOutputAndInput(t *testing.T) {
	fake := &fakeTerminalInfraFleetClient{}
	registry := NewRegistry()
	registerTerminalChannels(registry, fake) // registers terminal.multiplex alongside every other terminal.* channel

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "mx-1", Type: "invoke", Channel: "terminal.multiplex"}); err != nil {
		t.Fatalf("writing terminal.multiplex invoke: %v", err)
	}
	ack := readWireMessage(t, ctx, client)
	if ack.Type != "result" || ack.ID != "mx-1" {
		t.Fatalf("first frame = %+v, want the terminal.multiplex ack", ack)
	}

	if err := writeBinaryFrame(ctx, client, TerminalStreamFrame{
		Opcode:   TerminalStreamOpcodeSubscribe,
		StreamID: terminalMultiplexControlStreamID,
		Payload:  subscribeFramePayload(t, "pty-1", 1),
	}); err != nil {
		t.Fatalf("writing Subscribe frame: %v", err)
	}

	stream := awaitLastStream(t, fake)
	awaitSentFrame(t, stream) // drain the attach frame

	stream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Out{Out: &infrafleetv1.PtyOutput{Data: []byte("hi\n")}}}

	gotOutput := readBinaryFrame(t, ctx, client)
	if gotOutput.Opcode != TerminalStreamOpcodeOutput || gotOutput.StreamID != 1 || string(gotOutput.Payload) != "hi\n" {
		t.Fatalf("unexpected Output frame over the wire: %+v", gotOutput)
	}

	if err := writeBinaryFrame(ctx, client, TerminalStreamFrame{Opcode: TerminalStreamOpcodeInput, StreamID: 1, Payload: []byte("ls\n")}); err != nil {
		t.Fatalf("writing Input frame: %v", err)
	}
	sentToPty := awaitSentFrame(t, stream)
	if input := sentToPty.GetInput(); input == nil || string(input.GetData()) != "ls\n" {
		t.Fatalf("expected the pty stream to receive Input \"ls\\n\", got %+v", sentToPty)
	}

	// Drive the session to exit and confirm it arrives as an Error frame —
	// see channels_terminal_multiplex.go's package doc comment for why
	// there is no dedicated "exited" opcode.
	stream.recv <- &infrafleetv1.PtyServerFrame{Frame: &infrafleetv1.PtyServerFrame_Exited{Exited: &infrafleetv1.PtyExited{ExitCode: 1}}}
	gotError := readBinaryFrame(t, ctx, client)
	if gotError.Opcode != TerminalStreamOpcodeError || gotError.StreamID != 1 {
		t.Fatalf("expected an Error frame for process exit over the wire, got %+v", gotError)
	}
}
