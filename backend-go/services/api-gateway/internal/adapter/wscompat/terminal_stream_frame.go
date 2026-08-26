// terminal_stream_frame.go implements the binary, opcode-tagged frame
// encoding terminal.multiplex speaks — byte-for-byte identical to the OLD TS
// backend's backend/src/shared/terminal-stream-protocol.ts
// (encodeTerminalStreamFrame/decodeTerminalStreamFrame), which is what the
// real frontend's client-side demuxer still decodes. That file is NOT
// modified by this pass (backend/ is being replaced) — it is read-only
// reference for the wire format backend-go must reproduce so the unmodified
// frontend can talk to it.
//
// Header layout (16 bytes, all multi-byte fields little-endian, matching
// DataView's `true` little-endian flag in the TS encoder):
//
//	byte 0:    kind    = 0x74 ('t')
//	byte 1:    version = 1
//	byte 2:    opcode  (TerminalStreamOpcode, uint8)
//	byte 3:    reserved (always 0)
//	bytes 4-7:   streamId  (uint32 LE) — client-assigned, demuxes panes on one connection
//	bytes 8-11:  seq high 32 bits (uint32 LE)
//	bytes 12-15: seq low 32 bits (uint32 LE) — seq is a uint64 split into two
//	             uint32 LE words because the TS source builds it via
//	             Math.floor(seq / 0x100000000) / (seq >>> 0), not a native
//	             64-bit write.
//	bytes 16+: payload
package wscompat

import (
	"encoding/binary"
	"fmt"
)

const (
	terminalStreamKind        byte = 0x74
	terminalStreamVersion     byte = 1
	terminalStreamHeaderBytes      = 16
)

// TerminalStreamOpcode mirrors terminal-stream-protocol.ts's
// TerminalStreamOpcode enum exactly — the numeric values are the wire
// format, not just internal labels, so they must never be renumbered.
type TerminalStreamOpcode uint8

const (
	TerminalStreamOpcodeOutput          TerminalStreamOpcode = 1
	TerminalStreamOpcodeSnapshotStart   TerminalStreamOpcode = 2
	TerminalStreamOpcodeSnapshotChunk   TerminalStreamOpcode = 3
	TerminalStreamOpcodeSnapshotEnd     TerminalStreamOpcode = 4
	TerminalStreamOpcodeResized         TerminalStreamOpcode = 5
	TerminalStreamOpcodeError           TerminalStreamOpcode = 6
	TerminalStreamOpcodeInput           TerminalStreamOpcode = 7
	TerminalStreamOpcodeResize          TerminalStreamOpcode = 8
	TerminalStreamOpcodeSubscribe       TerminalStreamOpcode = 9
	TerminalStreamOpcodeUnsubscribe     TerminalStreamOpcode = 10
	TerminalStreamOpcodeSnapshotRequest TerminalStreamOpcode = 11
	TerminalStreamOpcodeMetadata        TerminalStreamOpcode = 12
	TerminalStreamOpcodeAck             TerminalStreamOpcode = 13
	TerminalStreamOpcodeClaimViewport   TerminalStreamOpcode = 14
)

func (op TerminalStreamOpcode) valid() bool {
	return op >= TerminalStreamOpcodeOutput && op <= TerminalStreamOpcodeClaimViewport
}

// TerminalStreamFrame is the decoded form of one binary multiplex frame —
// field-for-field what terminal-stream-protocol.ts's TerminalStreamFrame
// type carries.
type TerminalStreamFrame struct {
	Opcode   TerminalStreamOpcode
	StreamID uint32
	Seq      uint64
	Payload  []byte
}

// EncodeTerminalStreamFrame builds the wire bytes for frame — the Go
// counterpart of encodeTerminalStreamFrame in terminal-stream-protocol.ts.
func EncodeTerminalStreamFrame(frame TerminalStreamFrame) []byte {
	out := make([]byte, terminalStreamHeaderBytes+len(frame.Payload))
	out[0] = terminalStreamKind
	out[1] = terminalStreamVersion
	out[2] = byte(frame.Opcode)
	out[3] = 0
	binary.LittleEndian.PutUint32(out[4:8], frame.StreamID)
	binary.LittleEndian.PutUint32(out[8:12], uint32(frame.Seq>>32))
	binary.LittleEndian.PutUint32(out[12:16], uint32(frame.Seq))
	copy(out[terminalStreamHeaderBytes:], frame.Payload)
	return out
}

// DecodeTerminalStreamFrame parses raw bytes off the wire — the Go
// counterpart of decodeTerminalStreamFrame in terminal-stream-protocol.ts.
// Returns an error (never a panic) for anything too short, wrong
// kind/version, or an unrecognized opcode — mirrors the TS decoder
// returning null for the same cases, translated to this codebase's
// error-returning convention.
func DecodeTerminalStreamFrame(raw []byte) (TerminalStreamFrame, error) {
	if len(raw) < terminalStreamHeaderBytes {
		return TerminalStreamFrame{}, fmt.Errorf("wscompat: terminal stream frame too short (%d bytes, want at least %d)", len(raw), terminalStreamHeaderBytes)
	}
	if raw[0] != terminalStreamKind || raw[1] != terminalStreamVersion {
		return TerminalStreamFrame{}, fmt.Errorf("wscompat: terminal stream frame has unexpected kind/version (0x%02x/%d)", raw[0], raw[1])
	}
	opcode := TerminalStreamOpcode(raw[2])
	if !opcode.valid() {
		return TerminalStreamFrame{}, fmt.Errorf("wscompat: terminal stream frame has unknown opcode %d", raw[2])
	}
	streamID := binary.LittleEndian.Uint32(raw[4:8])
	high := binary.LittleEndian.Uint32(raw[8:12])
	low := binary.LittleEndian.Uint32(raw[12:16])
	seq := uint64(high)<<32 | uint64(low)
	payload := raw[terminalStreamHeaderBytes:]
	return TerminalStreamFrame{Opcode: opcode, StreamID: streamID, Seq: seq, Payload: payload}, nil
}
