package domain

import (
	"strconv"
	"strings"
)

// oscPrefix / maxOscCarryLength port
// agent/src/shared/terminal-osc133-command-finished.ts's OSC_133_PREFIX /
// MAX_OSC_CARRY_LENGTH constants exactly.
const (
	oscPrefix         = "\x1b]133;"
	maxOscCarryLength = 4096
)

// Osc133Marker is one complete OSC 133;C or ;D sequence found by Feed.
type Osc133Marker struct {
	// Kind is "C" (command started) or "D" (command finished) — the only
	// two sequences the ported TS scanner recognizes; there is no "A"
	// marker in the real implementation.
	Kind string
	// ExitCode is D's best-effort exit code, nil if absent/unparseable.
	// Meaningless for Kind == "C".
	ExitCode *int
}

// Osc133Scanner is stateful ACROSS calls (carries a partial-prefix tail
// between chunks) — one instance per AgentSession, matching the TS
// original's per-pane scanner instance (createOsc133CommandFinishedScanner).
type Osc133Scanner struct {
	carry string
}

// Feed processes one raw output chunk, returning every complete marker
// found — direct port of the TS scanner's scan() closure.
func (s *Osc133Scanner) Feed(data string) []Osc133Marker {
	combined := s.carry + data
	s.carry = ""
	var markers []Osc133Marker

	for len(combined) > 0 {
		start := strings.Index(combined, oscPrefix)
		if start == -1 {
			s.carry = findPrefixCarry(combined)
			return markers
		}

		payloadStart := start + len(oscPrefix)
		term := findOscTerminator(combined, payloadStart)
		if term == nil {
			s.carry = combined[start:]
			if len(s.carry) > maxOscCarryLength {
				s.carry = s.carry[len(s.carry)-maxOscCarryLength:]
			}
			return markers
		}

		if m, ok := parseOsc133Payload(combined[payloadStart:term.index]); ok {
			markers = append(markers, m)
		}
		combined = combined[term.index+term.length:]
	}
	return markers
}

// Reset drops the cross-chunk carry — mirrors the TS scanner's reset(),
// used on transport teardown.
func (s *Osc133Scanner) Reset() {
	s.carry = ""
}

func parseOsc133Payload(payload string) (Osc133Marker, bool) {
	parts := strings.SplitN(payload, ";", 2)
	switch parts[0] {
	case "C":
		return Osc133Marker{Kind: "C"}, true
	case "D":
		var exitCode *int
		if len(parts) > 1 {
			if v, err := strconv.Atoi(parts[1]); err == nil {
				exitCode = &v
			}
		}
		return Osc133Marker{Kind: "D", ExitCode: exitCode}, true
	default:
		return Osc133Marker{}, false
	}
}

type oscTerminator struct {
	index  int
	length int
}

// findOscTerminator ports findOscTerminator — a BEL (\x07, length 1) or an
// ST (\x1b\\, length 2) terminates an OSC sequence; BEL wins on a tie.
func findOscTerminator(data string, startIndex int) *oscTerminator {
	bel := strings.Index(data[startIndex:], "\x07")
	st := strings.Index(data[startIndex:], "\x1b\\")

	if bel == -1 && st == -1 {
		return nil
	}
	if bel != -1 && (st == -1 || bel < st) {
		return &oscTerminator{index: startIndex + bel, length: 1}
	}
	return &oscTerminator{index: startIndex + st, length: 2}
}

// findPrefixCarry ports findPrefixCarry — the longest suffix of data that
// is itself a (possibly-incomplete) prefix of oscPrefix, so a prefix split
// across two chunks is recognized on the next Feed call.
func findPrefixCarry(data string) string {
	maxCarryLength := len(oscPrefix) - 1
	if len(data) < maxCarryLength {
		maxCarryLength = len(data)
	}
	for length := maxCarryLength; length > 0; length-- {
		suffix := data[len(data)-length:]
		if strings.HasPrefix(oscPrefix, suffix) {
			return suffix
		}
	}
	return ""
}
