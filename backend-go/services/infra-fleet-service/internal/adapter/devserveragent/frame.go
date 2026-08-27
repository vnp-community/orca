// Frame codec for the Dev Server Agent's 13-byte wire header — a precise
// Go port of Stack B ("Relay protocol", backend/src/main/ssh/relay-protocol.ts).
// Epic A implements relay-websocket mode only (see client.go's doc comment);
// that mode's handshake (backend/src/main/dev-server/ws-handshake.ts's
// runOrcaInitiatorHandshake) uses Stack B's encodeJsonRpcFrame/FrameDecoder,
// NOT Stack A's agent-wire.ts, despite both stacks sharing this exact byte
// layout — see specs/agent/api/connection-modes.md §0's "Important
// convergence" note. Porting Stack B (not A) here is deliberate and matches
// what actually runs on the wire for this mode.
package devserveragent

import (
	"encoding/binary"
	"fmt"
)

// HeaderLength is the fixed frame header size in bytes:
// [TYPE u8][ID u32BE][ACK u32BE][LENGTH u32BE].
const HeaderLength = 13

// MaxMessageSize mirrors relay-protocol.ts's MAX_MESSAGE_SIZE (16 MiB) —
// the cap both sides use to reject/discard an oversized frame rather than
// attempt to buffer it.
const MaxMessageSize = 16 * 1024 * 1024

// MessageType values — byte-identical to relay-protocol.ts's MessageType
// (Regular/KeepAlive; Stack B's Handshake=2 is intentionally omitted since
// relay-websocket's handshake is a plain Regular-framed JSON-RPC exchange,
// not a dedicated handshake frame type — that's relay-ssh's variant only).
const (
	MessageTypeRegular   byte = 1
	MessageTypeKeepAlive byte = 9
)

// EncodeFrame builds one wire frame: 13-byte header + payload. id plays the
// SEQ role in relay-protocol.ts's naming; ack is the highest peer id/seq
// observed so far.
func EncodeFrame(msgType byte, id, ack uint32, payload []byte) []byte {
	frame := make([]byte, HeaderLength+len(payload))
	frame[0] = msgType
	binary.BigEndian.PutUint32(frame[1:5], id)
	binary.BigEndian.PutUint32(frame[5:9], ack)
	binary.BigEndian.PutUint32(frame[9:13], uint32(len(payload)))
	copy(frame[HeaderLength:], payload)
	return frame
}

// EncodeKeepAliveFrame builds an empty-payload KeepAlive frame.
func EncodeKeepAliveFrame(id, ack uint32) []byte {
	return EncodeFrame(MessageTypeKeepAlive, id, ack, nil)
}

// DecodedFrame is one parsed wire frame.
type DecodedFrame struct {
	Type    byte
	ID      uint32
	Ack     uint32
	Payload []byte
}

// DecodeFrame parses exactly one frame from buf. Unlike relay-protocol.ts's
// FrameDecoder (an incremental byte-stream parser needed because SSH
// exec-channel stdio delivers arbitrary-sized chunks with no message
// boundary), a WebSocket transport already delivers whole messages — one WS
// binary message is always exactly one frame here, so a single-shot decode
// is the faithful equivalent for this transport, not a simplification that
// changes behavior.
func DecodeFrame(buf []byte) (DecodedFrame, error) {
	if len(buf) < HeaderLength {
		return DecodedFrame{}, fmt.Errorf("devserveragent: frame shorter than header (%d bytes)", len(buf))
	}
	length := binary.BigEndian.Uint32(buf[9:13])
	if length > MaxMessageSize {
		return DecodedFrame{}, fmt.Errorf("devserveragent: frame payload too large: %d bytes (max %d)", length, MaxMessageSize)
	}
	if uint32(len(buf)-HeaderLength) < length {
		return DecodedFrame{}, fmt.Errorf("devserveragent: frame payload truncated: declared %d bytes, got %d", length, len(buf)-HeaderLength)
	}
	return DecodedFrame{
		Type:    buf[0],
		ID:      binary.BigEndian.Uint32(buf[1:5]),
		Ack:     binary.BigEndian.Uint32(buf[5:9]),
		Payload: buf[HeaderLength : HeaderLength+length],
	}, nil
}
