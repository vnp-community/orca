package wscompat

import (
	"bytes"
	"testing"
)

// TestEncodeTerminalStreamFrame_MatchesTSByteLayout locks in the exact byte
// layout terminal-stream-protocol.ts's encodeTerminalStreamFrame produces —
// computed by hand against that file's DataView writes (kind=0x74,
// version=1, all multi-byte fields little-endian, seq split into two LE
// uint32 words at offsets 8/12) so a future refactor can't silently drift
// from the wire format the real frontend decodes.
func TestEncodeTerminalStreamFrame_MatchesTSByteLayout(t *testing.T) {
	frame := TerminalStreamFrame{
		Opcode:   TerminalStreamOpcodeOutput,
		StreamID: 0x00000007,
		Seq:      0x0000000100000002, // high=1, low=2 — exercises the 64-bit split
		Payload:  []byte("hi"),
	}
	got := EncodeTerminalStreamFrame(frame)
	want := []byte{
		0x74, 0x01, 0x01, 0x00, // kind, version, opcode, reserved
		0x07, 0x00, 0x00, 0x00, // streamId = 7 LE
		0x01, 0x00, 0x00, 0x00, // seq high = 1 LE
		0x02, 0x00, 0x00, 0x00, // seq low = 2 LE
		'h', 'i', // payload
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeTerminalStreamFrame() = %x, want %x", got, want)
	}
}

func TestTerminalStreamFrame_RoundTrip(t *testing.T) {
	cases := []TerminalStreamFrame{
		{Opcode: TerminalStreamOpcodeOutput, StreamID: 1, Seq: 0, Payload: []byte("hello")},
		{Opcode: TerminalStreamOpcodeResized, StreamID: 42, Seq: 9999, Payload: []byte(`{"cols":80,"rows":24}`)},
		{Opcode: TerminalStreamOpcodeUnsubscribe, StreamID: 3, Seq: 1, Payload: nil},
		{Opcode: TerminalStreamOpcodeSubscribe, StreamID: 0, Seq: 5, Payload: []byte(`{"ptyId":"pty-1","streamId":1}`)},
		// A seq large enough to require both 32-bit words, proving the split
		// reassembles correctly (not just the zero/high-bit-unset common case).
		{Opcode: TerminalStreamOpcodeAck, StreamID: 2, Seq: 0x1_0000_0005, Payload: []byte(`{"bytes":5}`)},
	}
	for _, c := range cases {
		encoded := EncodeTerminalStreamFrame(c)
		decoded, err := DecodeTerminalStreamFrame(encoded)
		if err != nil {
			t.Fatalf("DecodeTerminalStreamFrame: %v", err)
		}
		if decoded.Opcode != c.Opcode || decoded.StreamID != c.StreamID || decoded.Seq != c.Seq {
			t.Errorf("round trip mismatch: got %+v, want opcode/streamId/seq = %v/%v/%v", decoded, c.Opcode, c.StreamID, c.Seq)
		}
		if !bytes.Equal(decoded.Payload, c.Payload) {
			t.Errorf("round trip payload mismatch: got %q, want %q", decoded.Payload, c.Payload)
		}
	}
}

func TestDecodeTerminalStreamFrame_RejectsTooShort(t *testing.T) {
	if _, err := DecodeTerminalStreamFrame([]byte{0x74, 0x01, 0x01}); err == nil {
		t.Fatal("expected an error for a frame shorter than the 16-byte header")
	}
}

func TestDecodeTerminalStreamFrame_RejectsWrongKindOrVersion(t *testing.T) {
	frame := EncodeTerminalStreamFrame(TerminalStreamFrame{Opcode: TerminalStreamOpcodeOutput, StreamID: 1})
	badKind := append([]byte(nil), frame...)
	badKind[0] = 0x99
	if _, err := DecodeTerminalStreamFrame(badKind); err == nil {
		t.Fatal("expected an error for the wrong kind byte")
	}

	badVersion := append([]byte(nil), frame...)
	badVersion[1] = 0x02
	if _, err := DecodeTerminalStreamFrame(badVersion); err == nil {
		t.Fatal("expected an error for an unsupported version byte")
	}
}

func TestDecodeTerminalStreamFrame_RejectsUnknownOpcode(t *testing.T) {
	frame := EncodeTerminalStreamFrame(TerminalStreamFrame{Opcode: TerminalStreamOpcodeOutput, StreamID: 1})
	bad := append([]byte(nil), frame...)
	bad[2] = 99
	if _, err := DecodeTerminalStreamFrame(bad); err == nil {
		t.Fatal("expected an error for an opcode outside the known enum range")
	}
}
