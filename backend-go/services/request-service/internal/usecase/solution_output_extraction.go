package usecase

import (
	"encoding/json"
	"errors"
	"strings"
)

var ErrNoJSONObject = errors.New("no JSON object found")

// ExtractJSONObject returns the first well-formed top-level JSON object in model output. Code fences and prose
// around it are ignored, and a stray `{` in the prose before the object is skipped rather than trusted.
func ExtractJSONObject(text string) ([]byte, error) {
	for from := 0; ; {
		rel := strings.IndexByte(text[from:], '{')
		if rel < 0 {
			return nil, ErrNoJSONObject
		}
		start := from + rel
		if end, ok := balancedObjectEnd(text, start); ok && json.Valid([]byte(text[start:end])) {
			return []byte(text[start:end]), nil
		}
		from = start + 1
	}
}

// balancedObjectEnd finds the brace that closes the object opened at start, honouring strings and escapes.
func balancedObjectEnd(text string, start int) (int, bool) {
	inString, escape := false, false
	depth := 0
	for i := start; i < len(text); i++ {
		c := text[i]
		if inString {
			switch {
			case escape:
				escape = false
			case c == '\\':
				escape = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}
