package apiclient

import (
	"encoding/json"
	"fmt"
)

// Document is a JSON object the gateway returned. The Request flow has many shapes and the CLI reads only a few
// fields of each (status, version, ids); the rest is printed as received, so a new field shows up without a CLI change.
type Document map[string]any

// Str reads a nested string ("request", "status"); anything missing or not a string is "".
func (d Document) Str(path ...string) string {
	switch v := d.at(path).(type) {
	case string:
		return v
	case float64, bool:
		return fmt.Sprint(v)
	}
	return ""
}

// Int reads a nested JSON number; anything missing or not a number is 0.
func (d Document) Int(path ...string) int64 {
	if v, ok := d.at(path).(float64); ok {
		return int64(v)
	}
	return 0
}

// Docs reads a nested array of objects.
func (d Document) Docs(path ...string) []Document {
	list, _ := d.at(path).([]any)
	out := make([]Document, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, Document(m))
		}
	}
	return out
}

// Doc reads a nested object (nil when absent).
func (d Document) Doc(path ...string) Document {
	if m, ok := d.at(path).(map[string]any); ok {
		return Document(m)
	}
	return nil
}

func (d Document) at(path []string) any {
	var cur any = map[string]any(d)
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

// ParseDocument reads one JSON object, e.g. a body given on the command line or in a file.
func ParseDocument(raw []byte) (Document, error) {
	var d Document
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	return d, nil
}

// Bool reads a nested JSON boolean; anything missing or not a boolean is false.
func (d Document) Bool(path ...string) bool {
	v, _ := d.at(path).(bool)
	return v
}

// Strs reads a nested array of strings; anything that is not a string is skipped.
func (d Document) Strs(path ...string) []string {
	list, _ := d.at(path).([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		if v, ok := item.(string); ok {
			out = append(out, v)
		}
	}
	return out
}
