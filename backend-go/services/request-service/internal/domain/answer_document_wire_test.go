package domain

import (
	"encoding/json"
	"testing"
)

// api-gateway hands this JSON to the UI untouched (solution.list -> options),
// so these key names are a client contract, not an implementation detail.
func TestAnswerDocument_MarshalUsesTheKeysTheUIReads(t *testing.T) {
	d := AnswerDocument{SchemaVersion: 1, Kind: "answer", AnswerMarkdown: "# hi", Confidence: 0.5,
		Citations: []Citation{{Ref: "a.go:1", Excerpt: "x"}}, Limitations: []string{"l"}}
	raw, err := d.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"schema_version", "kind", "answer_markdown", "confidence", "citations", "limitations"} {
		if _, ok := m[k]; !ok {
			t.Errorf("key %q missing in %s", k, raw)
		}
	}
	cite := m["citations"].([]any)[0].(map[string]any)
	if cite["ref"] != "a.go:1" || cite["excerpt"] != "x" {
		t.Errorf("citation = %v", cite)
	}
}
