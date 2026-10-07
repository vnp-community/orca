package domain

import (
	"encoding/json"
)

// AgentRPCError represents a structured JSON-RPC error returned by a dev server agent
// for codeintel.* and quality.* methods.
type AgentRPCError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

func (e *AgentRPCError) Error() string {
	return e.Message
}

func (e *AgentRPCError) Unwrap() error {
	if e.Code == -32601 {
		return ErrAgentMethodNotFound
	}
	return nil
}
