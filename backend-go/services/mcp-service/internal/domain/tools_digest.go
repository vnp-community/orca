package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// ToolInfo is what a probe learned about one tool of an external server. The
// text is untrusted: stored and shown to admins, never put in prompts or INFO logs.
type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ComputeToolsDigest pins name, description and inputSchema of every tool:
// sha256 over canonical JSON (sorted by name, sorted object keys; text is hashed as-is, so even a Unicode-normalization-only change is flagged, the safe direction), so
// reordering is stable and any wording change is a rug-pull signal.
func ComputeToolsDigest(tools []ToolInfo) string {
	sorted := make([]ToolInfo, len(tools))
	copy(sorted, tools)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	items := make([]any, 0, len(sorted))
	for _, t := range sorted {
		items = append(items, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": canonicalRaw(t.InputSchema),
		})
	}
	return digestOf(items)
}

// canonicalRaw decodes then re-encodes so key order in the server's JSON
// cannot change the digest (encoding/json sorts map keys).
func canonicalRaw(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

func digestOf(v any) string {
	b, _ := json.Marshal(v) // maps with string keys and plain values cannot fail
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
