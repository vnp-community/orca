package devserveragent

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeFrameRoundTrip(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":7,"method":"ports.scan"}`)
	frame := EncodeFrame(MessageTypeRegular, 7, 3, payload)

	if len(frame) != HeaderLength+len(payload) {
		t.Fatalf("frame length = %d, want %d", len(frame), HeaderLength+len(payload))
	}

	decoded, err := DecodeFrame(frame)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	if decoded.Type != MessageTypeRegular {
		t.Errorf("Type = %d, want %d", decoded.Type, MessageTypeRegular)
	}
	if decoded.ID != 7 {
		t.Errorf("ID = %d, want 7", decoded.ID)
	}
	if decoded.Ack != 3 {
		t.Errorf("Ack = %d, want 3", decoded.Ack)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Errorf("Payload = %q, want %q", decoded.Payload, payload)
	}
}

func TestEncodeKeepAliveFrame(t *testing.T) {
	frame := EncodeKeepAliveFrame(5, 4)
	if len(frame) != HeaderLength {
		t.Fatalf("keepalive frame length = %d, want %d (empty payload)", len(frame), HeaderLength)
	}
	decoded, err := DecodeFrame(frame)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	if decoded.Type != MessageTypeKeepAlive {
		t.Errorf("Type = %d, want %d", decoded.Type, MessageTypeKeepAlive)
	}
	if len(decoded.Payload) != 0 {
		t.Errorf("Payload = %q, want empty", decoded.Payload)
	}
}

func TestDecodeFrameRejectsShortBuffer(t *testing.T) {
	if _, err := DecodeFrame([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected error for buffer shorter than header")
	}
}

func TestDecodeFrameRejectsTruncatedPayload(t *testing.T) {
	frame := EncodeFrame(MessageTypeRegular, 1, 0, []byte("hello"))
	truncated := frame[:len(frame)-2] // declared length says 5 bytes, only 3 present
	if _, err := DecodeFrame(truncated); err == nil {
		t.Fatal("expected error for truncated payload")
	}
}

func TestDecodeFrameRejectsOversizedDeclaredLength(t *testing.T) {
	header := make([]byte, HeaderLength)
	header[0] = MessageTypeRegular
	// Declare a length far beyond MaxMessageSize in the LENGTH field.
	header[9], header[10], header[11], header[12] = 0xFF, 0xFF, 0xFF, 0xFF
	if _, err := DecodeFrame(header); err == nil {
		t.Fatal("expected error for oversized declared length")
	}
}
