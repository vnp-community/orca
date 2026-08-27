package devserveragent

import (
	"encoding/json"
	"testing"
)

func TestEncodeParseJSONRPCRoundTrip(t *testing.T) {
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 9, Method: "preflight.check"}
	frame, err := EncodeJSONRPCFrame(req, 9, 0)
	if err != nil {
		t.Fatalf("EncodeJSONRPCFrame: %v", err)
	}
	decoded, err := DecodeFrame(frame)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}

	var roundTripped JSONRPCRequest
	if err := json.Unmarshal(decoded.Payload, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if roundTripped.Method != "preflight.check" || roundTripped.ID != 9 {
		t.Errorf("roundTripped = %+v, want method=preflight.check id=9", roundTripped)
	}
}

func TestParseJSONRPCResponseResult(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":3,"result":{"ok":true}}`)
	resp, ok, err := ParseJSONRPCResponse(payload)
	if err != nil {
		t.Fatalf("ParseJSONRPCResponse: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for a response with id and no method")
	}
	if resp.ID != 3 {
		t.Errorf("ID = %d, want 3", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("Error = %+v, want nil", resp.Error)
	}
}

func TestParseJSONRPCResponseError(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":3,"error":{"code":-32601,"message":"Method not found: bogus"}}`)
	resp, ok, err := ParseJSONRPCResponse(payload)
	if err != nil {
		t.Fatalf("ParseJSONRPCResponse: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Errorf("Error = %+v, want code -32601", resp.Error)
	}
}

// TestParseJSONRPCResponseIgnoresRequests mirrors
// runOrcaInitiatorHandshake's own filter ("Only process response to our
// handshake (must have id field, no method)") — a request/notification
// frame from the peer must not be misrouted as a response.
func TestParseJSONRPCResponseIgnoresRequests(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"agent.hook","params":{}}`)
	_, ok, err := ParseJSONRPCResponse(payload)
	if err != nil {
		t.Fatalf("ParseJSONRPCResponse: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for a payload carrying a method field")
	}
}
