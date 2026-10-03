package tools

import (
	"encoding/json"
	"path"
)

// Files tools must never reveal credential files (.env, keys, .git...), even
// under an innocent request, so the sensitive-path rules run on the input path
// and on every path in the result.
type pathFilterKind int

const (
	filterNone       pathFilterKind = iota
	filterDirEntries                // [{name,...}] relative to the requested directory
	filterMatches                   // [{path,...}]
	filterPaths                     // ["a/b", ...]
)

var errFilesNotFound = &ToolError{"MCP_NOT_FOUND", "not found or not permitted"}

// guardInputPath rejects unsafe or sensitive paths with the same answer as a
// missing file so the rule is not an oracle.
func (s *ToolSpec) guardInputPath(input json.RawMessage, extra []string) error {
	if s.PathArg == "" {
		return nil
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(input, &m)
	var p string
	_ = json.Unmarshal(m[s.PathArg], &p)
	if p == "" && s.PathOptional {
		return nil
	}
	clean, err := CleanWorktreePath(p)
	if err != nil || IsSensitivePath(clean, extra) {
		return errFilesNotFound
	}
	return nil
}

// filterSensitiveItems drops sensitive entries from a normalized list result.
func (s *ToolSpec) filterSensitiveItems(obj map[string]any, input json.RawMessage, extra []string) {
	if s.PathFilter == filterNone {
		return
	}
	items, ok := obj["items"].([]any)
	if !ok {
		return
	}
	var dir string
	if s.PathFilter == filterDirEntries {
		var m map[string]any
		_ = json.Unmarshal(input, &m)
		dir, _ = m[s.PathArg].(string)
	}
	kept := items[:0]
	for _, it := range items {
		var p string
		switch s.PathFilter {
		case filterDirEntries:
			if e, ok := it.(map[string]any); ok {
				n, _ := e["name"].(string)
				p = path.Join(dir, n)
			}
		case filterMatches:
			if e, ok := it.(map[string]any); ok {
				p, _ = e["path"].(string)
			}
		case filterPaths:
			p, _ = it.(string)
		}
		if p == "" || IsSensitivePath(p, extra) {
			continue
		}
		kept = append(kept, it)
	}
	obj["items"] = kept
}
