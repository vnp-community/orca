package devserveragent

import (
	"encoding/json"
	"fmt"
)

// JSONRPCRequest mirrors relay-protocol.ts's JsonRpcRequest.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint32          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse mirrors relay-protocol.ts's JsonRpcResponse. Result/Error
// are both raw — this package only needs to route by ID and hand the raw
// bytes back to the caller (or fail on Error), not interpret every possible
// agent-side result shape.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint32          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return e.Message
}

// EncodeJSONRPCFrame marshals msg and wraps it in a Regular frame — the Go
// equivalent of relay-protocol.ts's encodeJsonRpcFrame.
func EncodeJSONRPCFrame(msg any, id, ack uint32) ([]byte, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	if len(payload) > MaxMessageSize {
		return nil, jsonrpcErrorTooLarge(len(payload))
	}
	return EncodeFrame(MessageTypeRegular, id, ack, payload), nil
}

func jsonrpcErrorTooLarge(n int) error {
	return fmt.Errorf("devserveragent: message too large to encode: %d bytes (max %d)", n, MaxMessageSize)
}

// ParseJSONRPCResponse parses a frame payload as a JsonRpcResponse. It
// tolerates (does not reject) a payload that also happens to carry a
// "method" field — the caller (session.go's read loop) is responsible for
// only routing payloads with an "id" and no "method" to pending calls, per
// runOrcaInitiatorHandshake's own filtering rule.
func ParseJSONRPCResponse(payload []byte) (JSONRPCResponse, bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return JSONRPCResponse{}, false, err
	}
	_, hasID := raw["id"]
	_, hasMethod := raw["method"]
	if !hasID || hasMethod {
		return JSONRPCResponse{}, false, nil
	}
	var resp JSONRPCResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return JSONRPCResponse{}, false, err
	}
	return resp, true, nil
}

// JSONRPCNotification mirrors relay-protocol.ts's notification shape (a
// method + params, no id) — the mirror image of ParseJSONRPCResponse's own
// hasID/hasMethod filter. Used by session.go's read loop to demux
// pty.data/pty.exit/pty.replay pushes (see routeNotification) — the only
// notification methods this adapter currently understands; TASK-183's
// "Two RPC surfaces" package doc comment note applies here too: every other
// notification the agent might send is simply not routed anywhere yet.
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// ParseJSONRPCNotification parses payload as a notification: an object with
// "method" and no "id".
func ParseJSONRPCNotification(payload []byte) (JSONRPCNotification, bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return JSONRPCNotification{}, false, err
	}
	_, hasID := raw["id"]
	_, hasMethod := raw["method"]
	if hasID || !hasMethod {
		return JSONRPCNotification{}, false, nil
	}
	var notif JSONRPCNotification
	if err := json.Unmarshal(payload, &notif); err != nil {
		return JSONRPCNotification{}, false, err
	}
	return notif, true, nil
}
