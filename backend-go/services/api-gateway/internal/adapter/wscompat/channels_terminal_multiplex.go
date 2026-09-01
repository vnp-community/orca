// channels_terminal_multiplex.go implements terminal.multiplex — the real
// binary, opcode-tagged streaming protocol the UNMODIFIED frontend actually
// speaks for terminal I/O (confirmed by reading
// backend/src/main/runtime/rpc/methods/terminal.ts's 'terminal.multiplex'
// defineStreamingMethod handler, ~line 1510, and
// backend/src/shared/terminal-stream-protocol.ts for the exact frame
// encoding — both read-only reference; backend/ is not modified by this
// pass). This SUPPLEMENTS channels_terminal.go's terminal.output/
// terminal.exited JSON push channels rather than replacing them: those
// remain in place (their own tests still pass) for any caller still using
// the plain-JSON path, while terminal.multiplex below is what a real,
// unmodified frontend pane actually opens.
//
// # Scope: what this reproduces vs. deliberately does not
//
// The real terminal.ts handler is ~1000 lines covering ACK-gated flow
// control (TerminalStreamOpcode.Ack), remote-desktop viewport claims
// (ClaimViewport), on-demand scrollback snapshots (SnapshotRequest/Start/
// Chunk/End), and mobile-specific auto-fit. Per this task's explicit scope
// ("reuse AttachPty's existing PTY-output-forwarding logic — you're
// changing the OUTPUT ENCODING, not re-plumbing the PTY relay itself"),
// this file implements exactly the opcode surface a basic multiplexed
// terminal pane needs — Subscribe/Output/Input/Resize/Unsubscribe/Error —
// on top of the SAME AttachPty gRPC stream channels_terminal.go's
// terminal.create already opens. Ack/ClaimViewport/SnapshotRequest frames
// are accepted (never corrupt the connection) but are documented no-ops —
// a real client falls back to full redraws instead of flow control/
// snapshot recovery, which is safe, just less optimal.
//
// # Subscribe's "terminal" field: the wire key stays "terminal"; the value is a ptyId
//
// terminal-stream-protocol.ts's real Subscribe frame carries `{terminal:
// <opaque handle>, streamId, client?, viewport?, capabilities?}`, resolved
// server-side (in the real backend) via runtime.resolveLiveLeafForHandle.
// backend-go has no such handle-resolution layer — channels_terminal.go's
// terminal.create already established (and tests) the convention that the
// frontend treats the agent-assigned ptyId itself as the terminal handle
// for every other terminal.* channel (terminal.send/resize/close all key by
// ptyId, not an opaque handle) — but that convention only changes what
// VALUE the frontend puts in the field, never the field's KEY. The
// unmodified frontend still encodes the JSON key as "terminal"; decoding it
// under a "ptyId" key (an earlier version of this struct did exactly that)
// silently drops every Subscribe frame — found live 2026-08-30.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// terminalMultiplexControlStreamID is the reserved streamId Subscribe
// control frames arrive on — mirrors terminal.ts's
// `registerBinaryStreamHandler(0, ...)`.
const terminalMultiplexControlStreamID uint32 = 0

// terminalMultiplexSubscribePayload is the JSON payload of a
// TerminalStreamOpcodeSubscribe control frame. The wire field is genuinely
// named "terminal" — the real, unmodified frontend
// (remote-runtime-terminal-multiplexer.ts) encodes `{streamId, terminal:
// args.terminal, ...}` verbatim, unaware backend-go treats the VALUE it
// carries as a ptyId (see this file's package doc comment on that
// convention). This file's earlier `json:"ptyId"` tag assumed the KEY name
// would also change to match the new semantics, but the frontend is
// read-only reference here — it was never going to send a "ptyId" key.
// Every Subscribe frame silently dropped (payload.PtyID == "" below) until
// this was found live 2026-08-30, immediately after the connectionId/
// terminal.create fixes finally got a real client subscribing at all.
type terminalMultiplexSubscribePayload struct {
	PtyID    string `json:"terminal"`
	StreamID uint32 `json:"streamId"`
}

// terminalMultiplexResizePayload is the JSON payload of a
// TerminalStreamOpcodeResize slot frame — field names match
// terminal.ts's own `{cols, rows}` decode.
type terminalMultiplexResizePayload struct {
	Cols int32 `json:"cols"`
	Rows int32 `json:"rows"`
}

// multiplexSlot is one subscribed pane's live AttachPty stream, keyed by its
// client-assigned streamId (NOT ptyId — see this file's package doc comment;
// unlike channels_terminal.go's terminalStreamRegistry, which is keyed by
// ptyId for the JSON terminal.create/send/resize channels, multiplex slots
// are keyed by streamId because the wire protocol demuxes by streamId and,
// in principle, the same ptyId could be attached under more than one
// streamId).
type multiplexSlot struct {
	streamID uint32
	ptyID    string
	stream   infrafleetv1.InfraFleetService_AttachPtyClient
	cancel   context.CancelFunc
	// unregisterFrame removes this slot's BinaryFrameHandler registration —
	// called by detach synchronously (NOT deferred to drainOutput noticing
	// the stream ended), since cancel()ing a fake/test double's stream
	// doesn't necessarily unblock a blocked Recv() the way a real gRPC
	// stream's context cancellation does. A real AttachPty stream's Recv()
	// still unblocks on its own once cancel() fires; detach no longer waits
	// for that to also happen before the slot looks torn down to callers.
	unregisterFrame func()

	sendMu sync.Mutex
	// cursor mirrors terminal.ts's local `cursor++` — the seq value non-
	// Output control frames (Error, in a future pass Resized/Metadata) carry
	// when the underlying event has no real seq of its own. Output frames
	// always carry seq=0 (TS's own "no seq" sentinel — see sendFrame's doc
	// comment in terminal.ts), since AttachPty's PtyOutput has no seq field.
	cursor uint64
}

func (s *multiplexSlot) send(frame []byte, sendBinary func([]byte) bool) bool {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return sendBinary(frame)
}

func (s *multiplexSlot) nextCursor() uint64 {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.cursor++
	return s.cursor
}

// terminalMultiplexSession is ONE terminal.multiplex invoke's worth of
// state: every slot (pane) subscribed on this connection, keyed by
// streamId. Constructed fresh per invoke — see registerTerminalMultiplexChannel.
type terminalMultiplexSession struct {
	client infrafleetv1.InfraFleetServiceClient
	id     Identity
	io     BinaryStreamIO

	mu    sync.Mutex
	slots map[uint32]*multiplexSlot
}

func registerTerminalMultiplexChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterBinaryStreamHandler("terminal.multiplex", func(ctx context.Context, id Identity, _ []json.RawMessage, io BinaryStreamIO) (any, error) {
		session := &terminalMultiplexSession{client: client, id: id, io: io, slots: make(map[uint32]*multiplexSlot)}
		unregisterControl := io.RegisterFrameHandler(terminalMultiplexControlStreamID, session.handleControlFrame)

		// ctx here is the connection-lifetime context Handler.handleInvoke
		// passes into BinaryStreamChannelHandler (see binary_stream_registry.go's
		// doc comment) — when the WS connection closes, ctx is cancelled and
		// every slot this session opened is torn down with it.
		go func() {
			<-ctx.Done()
			unregisterControl()
			session.closeAll()
		}()

		return map[string]bool{"ok": true}, nil
	})
}

// handleControlFrame processes a frame on the reserved control streamId —
// only Subscribe is meaningful there (mirrors terminal.ts's control handler,
// which only branches on TerminalStreamOpcode.Subscribe).
func (s *terminalMultiplexSession) handleControlFrame(frame TerminalStreamFrame) {
	if frame.Opcode != TerminalStreamOpcodeSubscribe {
		return
	}
	var payload terminalMultiplexSubscribePayload
	if err := json.Unmarshal(frame.Payload, &payload); err != nil || payload.PtyID == "" {
		return
	}
	s.subscribe(payload.PtyID, payload.StreamID)
}

// subscribe opens a new AttachPty stream for ptyID and binds it to streamID,
// detaching any prior slot already using that streamID first — mirrors
// terminal.ts's handleSubscribeFrame calling detachStream(request.streamId)
// before installing the new stream.
func (s *terminalMultiplexSession) subscribe(ptyID string, streamID uint32) {
	s.detach(streamID)

	// See channels_terminal.go's attachContext doc comment for why this is
	// context.Background()-derived (identity-stamped) rather than inheriting
	// the invoke's own dispatchCtx: the stream must outlive one invoke's
	// 25s deadline.
	streamCtx, cancel := attachContext(s.id)
	stream, err := s.client.AttachPty(streamCtx)
	if err != nil {
		cancel()
		s.sendError(streamID, fmt.Sprintf("failed to open pty stream: %v", err))
		return
	}
	if err := stream.Send(&infrafleetv1.PtyClientFrame{
		Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: ptyID}},
	}); err != nil {
		cancel()
		s.sendError(streamID, fmt.Sprintf("failed to attach pty stream: %v", err))
		return
	}

	slot := &multiplexSlot{streamID: streamID, ptyID: ptyID, stream: stream, cancel: cancel}
	slot.unregisterFrame = s.io.RegisterFrameHandler(streamID, slot.handleFrame(s))

	s.mu.Lock()
	s.slots[streamID] = slot
	s.mu.Unlock()

	go s.drainOutput(slot)
}

// drainOutput reads PtyServerFrame off slot's AttachPty stream until it ends
// (Unsubscribe's cancel(), the pty process exiting, or a transport error),
// forwarding each as a binary TerminalStreamOpcodeOutput frame — the actual
// "change the OUTPUT ENCODING" this task asks for, reusing the exact same
// AttachPty relay channels_terminal.go's drainAttachPtyOutput already
// established, just encoding to the opcode-framed wire format instead of a
// terminal.output PushEvent.
//
// Deliberately does NOT unregister/remove the slot on exit — detach already
// does that synchronously (see its doc comment for why: a fake/test
// double's Recv() may never unblock even after cancel(), and a real
// connection must not look like it's still subscribed while its Recv loop
// is merely slow to notice cancellation). detachIfCurrent below only
// handles the "stream ended on its own" cases (pty exited, transport
// error) that detach was never called for.
func (s *terminalMultiplexSession) drainOutput(slot *multiplexSlot) {
	defer s.detachIfCurrent(slot)
	for {
		frame, err := slot.stream.Recv()
		if err != nil {
			return // stream ended — io.EOF on a clean close, or a transport error either way
		}
		switch f := frame.GetFrame().(type) {
		case *infrafleetv1.PtyServerFrame_Out:
			// seq=0: AttachPty's PtyOutput carries no seq of its own — see
			// multiplexSlot.cursor's doc comment for why Output specifically
			// uses the TS "no seq" sentinel rather than s.nextCursor().
			out := EncodeTerminalStreamFrame(TerminalStreamFrame{Opcode: TerminalStreamOpcodeOutput, StreamID: slot.streamID, Seq: 0, Payload: f.Out.GetData()})
			if !slot.send(out, s.io.SendBinary) {
				return
			}
		case *infrafleetv1.PtyServerFrame_Exited:
			// No TerminalStreamOpcode exists for "process exited" (see this
			// file's package doc comment) — surfaced as an Error frame,
			// which every client already knows how to render, rather than
			// inventing a new opcode the real frontend wouldn't recognize.
			s.sendError(slot.streamID, fmt.Sprintf("process exited (code %d)", f.Exited.GetExitCode()))
			return
		}
	}
}

func (s *terminalMultiplexSession) sendError(streamID uint32, message string) {
	s.mu.Lock()
	slot := s.slots[streamID]
	s.mu.Unlock()
	seq := uint64(0)
	if slot != nil {
		seq = slot.nextCursor()
	}
	frame := EncodeTerminalStreamFrame(TerminalStreamFrame{Opcode: TerminalStreamOpcodeError, StreamID: streamID, Seq: seq, Payload: []byte(message)})
	s.io.SendBinary(frame)
}

// handleFrame returns slot's per-frame callback for its own streamId —
// Unsubscribe/Input/Resize are the only opcodes this scoped-down pass acts
// on (see package doc comment); anything else (Ack/ClaimViewport/
// SnapshotRequest) is accepted and silently ignored, never an error.
func (slot *multiplexSlot) handleFrame(s *terminalMultiplexSession) BinaryFrameHandler {
	return func(frame TerminalStreamFrame) {
		switch frame.Opcode {
		case TerminalStreamOpcodeUnsubscribe:
			s.detach(slot.streamID)
		case TerminalStreamOpcodeInput:
			// Forwarded as raw bytes, not round-tripped through UTF-8 text
			// decode/re-encode the way terminal.ts's decodeTerminalStreamText
			// does — Go's PtyInput.Data is already []byte, so there is no
			// text-vs-bytes boundary to cross here; behavior is equivalent
			// for the terminal-input case (interactive keystrokes/paste are
			// what this opcode carries) without the redundant conversion.
			if len(frame.Payload) == 0 {
				return
			}
			if err := slot.stream.Send(&infrafleetv1.PtyClientFrame{
				Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: frame.Payload}},
			}); err != nil {
				s.sendError(slot.streamID, fmt.Sprintf("failed to send input: %v", err))
			}
		case TerminalStreamOpcodeResize:
			var payload terminalMultiplexResizePayload
			if err := json.Unmarshal(frame.Payload, &payload); err != nil || payload.Cols <= 0 || payload.Rows <= 0 {
				return
			}
			// Unary RPC, not the in-stream PtyResize frame — same choice
			// channels_terminal.go's terminal.resize channel already made
			// and documented (the proto's own doc comment calls the
			// in-stream frame "the low-latency ALTERNATIVE to the unary
			// RPC", i.e. the unary path is primary).
			ctx := gatewaygrpc.AttachIdentity(context.Background(), usecase.Identity{TenantID: s.id.TenantID, UserID: s.id.UserID})
			if _, err := s.client.ResizeTerminalSession(ctx, &infrafleetv1.ResizeTerminalSessionRequest{PtyId: slot.ptyID, Cols: payload.Cols, Rows: payload.Rows}); err != nil {
				s.sendError(slot.streamID, fmt.Sprintf("failed to resize: %v", err))
			}
		}
	}
}

// detach tears down streamID's slot, if any, SYNCHRONOUSLY — used by
// explicit Unsubscribe frames, subscribe's own "replace any prior slot"
// step, and closeAll. Unregisters the frame handler and removes the map
// entry immediately rather than waiting for drainOutput's goroutine to
// notice cancel() and exit on its own — see multiplexSlot.unregisterFrame's
// doc comment for why that distinction matters.
func (s *terminalMultiplexSession) detach(streamID uint32) {
	s.mu.Lock()
	slot, ok := s.slots[streamID]
	delete(s.slots, streamID)
	s.mu.Unlock()
	if ok {
		slot.unregisterFrame()
		slot.cancel() // unblocks a real AttachPty stream's Recv() with context.Canceled
	}
}

// detachIfCurrent removes slot from the session ONLY if it is still the
// slot registered under its own streamID — a newer subscribe() may have
// already replaced it (calling detach itself) while this stream's Recv loop
// was mid-read; in that case detachIfCurrent must be a no-op so it doesn't
// tear down the successor's live registration. Called from drainOutput when
// the stream ends on its own (pty exit, transport error) — the one case
// nothing else already called detach for.
func (s *terminalMultiplexSession) detachIfCurrent(slot *multiplexSlot) {
	s.mu.Lock()
	current, ok := s.slots[slot.streamID]
	if !ok || current != slot {
		s.mu.Unlock()
		return
	}
	delete(s.slots, slot.streamID)
	s.mu.Unlock()
	slot.unregisterFrame()
}

// closeAll tears down every slot this session opened — called when the WS
// connection closes (see registerTerminalMultiplexChannel's ctx.Done goroutine).
func (s *terminalMultiplexSession) closeAll() {
	s.mu.Lock()
	streamIDs := make([]uint32, 0, len(s.slots))
	for id := range s.slots {
		streamIDs = append(streamIDs, id)
	}
	s.mu.Unlock()
	for _, id := range streamIDs {
		s.detach(id)
	}
}
