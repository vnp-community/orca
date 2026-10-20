package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// DrawResult is what the agent returns from a draw run: one HTML page for each diagram it was asked for.
type DrawResult struct {
	Schema    string          `json:"schema"`
	Artifacts []DrawnArtifact `json:"artifacts"`
	Warnings  []string        `json:"warnings"`
}

type DrawnArtifact struct {
	Section     ReviewSection `json:"section"`
	Variant     string        `json:"variant"`
	Name        string        `json:"name"`
	InputDigest string        `json:"inputDigest"`
	HTML        string        `json:"html"`
}

// MaxDrawResultBytes is the result block cap of the agent.
const MaxDrawResultBytes = 256 * 1024

// ParseDrawResult decodes the block. A block that is not one JSON object of the contract is a violation the agent is told about.
func ParseDrawResult(block []byte) (DrawResult, error) {
	if len(block) == 0 {
		return DrawResult{}, ContextViolations{"the reply has no result block between ORCA_RESULT_BEGIN and ORCA_RESULT_END with the nonce"}
	}
	if len(block) > MaxDrawResultBytes {
		return DrawResult{}, ContextViolations{fmt.Sprintf("the result is %d bytes; the limit is %d", len(block), MaxDrawResultBytes)}
	}
	dec := json.NewDecoder(bytes.NewReader(block))
	var r DrawResult
	if err := dec.Decode(&r); err != nil {
		return DrawResult{}, ContextViolations{"the result is not valid JSON of the contract: " + err.Error()}
	}
	if dec.More() {
		return DrawResult{}, ContextViolations{"the result has more than one JSON value"}
	}
	if r.Schema != ReviewArtifactSchemaName {
		return DrawResult{}, ContextViolations{fmt.Sprintf("schema must be %q, not %q", ReviewArtifactSchemaName, r.Schema)}
	}
	return r, nil
}
