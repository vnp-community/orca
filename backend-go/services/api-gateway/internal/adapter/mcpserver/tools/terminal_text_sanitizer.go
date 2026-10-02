package tools

import (
	"strings"
	"unicode/utf8"
)

const (
	maxSanitizedLine = 4096
	lineTruncMarker  = "… [line truncated]"
	// An unterminated escape sequence longer than this is garbage, not a
	// sequence still being delivered: drop it instead of holding output back.
	maxPendingEscape = 8192
)

// sanitizeTerminal turns raw PTY bytes into plain text for an LLM: escape
// sequences (CSI, OSC incl. clipboard/hyperlink/prompt marks, DCS/APC/PM/SOS,
// two-byte escapes) are removed with a state machine, "\r" and backspace move a
// column cursor so progress bars collapse to their last state, other control
// characters are dropped and lines are capped. It returns how many input bytes
// were consumed: when final is false a trailing incomplete escape sequence is
// left unconsumed so the next read can complete it.
func sanitizeTerminal(raw []byte, final bool) (text string, consumed int) {
	var out strings.Builder
	var line []rune
	col := 0
	flush := func(nl bool) {
		s := line
		if len(s) > maxSanitizedLine {
			out.WriteString(string(s[:maxSanitizedLine]))
			out.WriteString(lineTruncMarker)
		} else {
			out.WriteString(string(s))
		}
		if nl {
			out.WriteByte('\n')
		}
		line, col = line[:0], 0
	}
	put := func(r rune) {
		switch {
		case col < len(line):
			line[col] = r
		case len(line) <= maxSanitizedLine+1: // beyond the cap nothing is kept
			line = append(line, r)
		}
		col++
	}
	i := 0
	for i < len(raw) {
		b := raw[i]
		// Escape introducers: ESC or an 8-bit C1 (U+009B etc. in UTF-8).
		kind := byte(0)
		size := 1
		if b == 0x1b {
			if i+1 >= len(raw) {
				if final {
					i++
					continue
				}
				break
			}
			kind = raw[i+1]
			size = 2
		} else if b >= 0x80 {
			r, n := utf8.DecodeRune(raw[i:])
			if r >= 0x80 && r <= 0x9f && n > 0 {
				switch r {
				case 0x9b:
					kind = '['
				case 0x9d:
					kind = ']'
				case 0x90:
					kind = 'P'
				case 0x98, 0x9e, 0x9f:
					kind = '_'
				default:
					i += n // other C1 controls carry nothing
					continue
				}
				size = n
			}
		}
		if kind != 0 {
			end, complete := skipEscape(raw, i, size, kind)
			if !complete {
				if !final && len(raw)-i <= maxPendingEscape {
					break
				}
				end = len(raw)
			}
			i = end
			continue
		}
		switch {
		case b == '\n':
			flush(true)
			i++
		case b == '\r':
			if i+1 < len(raw) && raw[i+1] == '\n' {
				flush(true)
				i += 2
				continue
			}
			col = 0
			i++
		case b == '\b':
			if col > 0 {
				col--
			}
			i++
		case b == '\t':
			put('\t')
			i++
		case b < 0x20 || b == 0x7f:
			i++
		default:
			r, n := utf8.DecodeRune(raw[i:])
			if r == utf8.RuneError && n <= 1 {
				if !final && !utf8.FullRune(raw[i:]) {
					goto done // incomplete rune at the end
				}
				put(utf8.RuneError)
				i++
				continue
			}
			put(r)
			i += n
		}
	}
done:
	flush(false)
	return out.String(), i
}

// skipEscape returns the index after the escape sequence starting at i (whose
// introducer is `size` bytes and whose type byte is kind), and whether the
// sequence is complete within raw.
func skipEscape(raw []byte, i, size int, kind byte) (end int, complete bool) {
	j := i + size
	switch kind {
	case '[': // CSI: params 0x30-0x3f, intermediates 0x20-0x2f, final 0x40-0x7e
		for j < len(raw) {
			c := raw[j]
			j++
			if c >= 0x40 && c <= 0x7e {
				return j, true
			}
			if c < 0x20 { // aborted by a control byte, which is then handled normally
				return j - 1, true
			}
		}
		return len(raw), false
	case ']', 'P', '_', '^', 'X': // OSC and string sequences end at BEL (OSC) or ST
		for j < len(raw) {
			c := raw[j]
			if c == 0x07 && kind == ']' {
				return j + 1, true
			}
			if c == 0x1b {
				if j+1 >= len(raw) {
					return len(raw), false
				}
				if raw[j+1] == '\\' {
					return j + 2, true
				}
				return j, true // a new escape aborts the string
			}
			if c == 0xc2 && j+1 < len(raw) && raw[j+1] == 0x9c { // U+009C as UTF-8
				return j + 2, true
			}
			j++
		}
		return len(raw), false
	default: // ESC intermediates* final (charset selection, save cursor, ...)
		if size == 2 && kind >= 0x20 && kind <= 0x2f {
			for j < len(raw) {
				c := raw[j]
				j++
				if c >= 0x30 && c <= 0x7e {
					return j, true
				}
				if c < 0x20 || c > 0x2f {
					return j - 1, true
				}
			}
			return len(raw), false
		}
		return j, true
	}
}
