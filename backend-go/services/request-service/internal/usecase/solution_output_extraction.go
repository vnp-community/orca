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
		if end, ok := balancedObjectEnd(text, start); ok {
			if json.Valid([]byte(text[start:end])) {
				return []byte(text[start:end]), nil
			}
			// Long documents often lose a `]` before the closing braces; without this the scan below would
			// settle on an inner object and report a misleading schema error.
			if fixed, ok := insertMissingClosers(text[start:end]); ok {
				return []byte(fixed), nil
			}
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

// insertMissingClosers repairs one kind of damage: a `}` or `]` that arrives while the other kind is still open means the
// closer of the inner container was left out, so it is put back. It never guesses at truncated text and never removes anything,
// and the caller only takes the result when it is valid JSON.
func insertMissingClosers(doc string) (string, bool) {
	var out strings.Builder
	var open []byte // the closers still expected, innermost last
	inString, escape, repaired := false, false, false
	for i := 0; i < len(doc); i++ {
		c := doc[i]
		if inString {
			switch {
			case escape:
				escape = false
			case c == '\\':
				escape = true
			case c == '"':
				inString = false
			}
			out.WriteByte(c)
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			open = append(open, '}')
		case '[':
			open = append(open, ']')
		case '}', ']':
			for len(open) > 0 && open[len(open)-1] != c {
				out.WriteByte(open[len(open)-1])
				open = open[:len(open)-1]
				repaired = true
			}
			if len(open) == 0 {
				return "", false
			}
			open = open[:len(open)-1]
		}
		out.WriteByte(c)
	}
	if !repaired || len(open) != 0 || !json.Valid([]byte(out.String())) {
		return "", false
	}
	return out.String(), true
}
