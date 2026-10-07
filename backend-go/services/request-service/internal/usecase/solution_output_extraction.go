package usecase

import (
	"errors"
	"strings"
)

var ErrNoJSONObject = errors.New("no JSON object found")

// ExtractJSONObject extracts the outermost JSON object `{...}` from a string, 
// ignoring surrounding text and code fences. It respects string literals and escapes.
func ExtractJSONObject(text string) ([]byte, error) {
	startIdx := strings.Index(text, "{")
	if startIdx == -1 {
		return nil, ErrNoJSONObject
	}

	// Simple parser to find matching closing brace
	inString := false
	escape := false
	depth := 0

	for i := startIdx; i < len(text); i++ {
		c := text[i]

		if inString {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inString = false
			}
		} else {
			if c == '"' {
				inString = true
			} else if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					// found end
					return []byte(text[startIdx : i+1]), nil
				}
			}
		}
	}

	return nil, ErrNoJSONObject
}
