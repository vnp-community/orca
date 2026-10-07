package secretscan

import (
	"sort"
	"strings"
)

const PatternsVersion = "ss/1"

const (
	redactedMarker = "[REDACTED]"
	maxScanBytes   = 1024 * 1024 // 1 MiB limit
)

// Scan returns a list of findings in the text. Overlapping findings are resolved
// by picking the one that starts first or is more specific (since patterns are ordered).
func Scan(text string) []Finding {
	if text == "" {
		return nil
	}

	limit := len(text)
	if limit > maxScanBytes {
		limit = maxScanBytes
	}
	scanText := text[:limit]

	var allFindings []Finding
	for _, p := range secretPatterns {
		matches := p.re.FindAllStringSubmatchIndex(scanText, -1)
		for _, m := range matches {
			start, end := m[0], m[1]
			if p.valueGroup > 0 && len(m) > p.valueGroup*2+1 {
				start = m[p.valueGroup*2]
				end = m[p.valueGroup*2+1]
			}
			if start == -1 || end == -1 {
				continue // Optional group didn't match
			}
			allFindings = append(allFindings, Finding{
				Kind:       p.kind,
				Confidence: p.confidence,
				Start:      start,
				End:        end,
			})
		}
	}

	if len(allFindings) == 0 {
		return nil
	}

	// Sort findings by Start, then by length (longest first)
	sort.Slice(allFindings, func(i, j int) bool {
		if allFindings[i].Start == allFindings[j].Start {
			lenI := allFindings[i].End - allFindings[i].Start
			lenJ := allFindings[j].End - allFindings[j].Start
			return lenI > lenJ
		}
		return allFindings[i].Start < allFindings[j].Start
	})

	// Remove overlaps
	var final []Finding
	lastEnd := -1
	for _, f := range allFindings {
		if f.Start >= lastEnd {
			final = append(final, f)
			lastEnd = f.End
		}
	}

	return final
}

// Redact replaces any detected secret value with [REDACTED].
// Returns the redacted string and a boolean indicating if changes were made.
func Redact(text string) (string, bool) {
	findings := Scan(text)
	if len(findings) == 0 {
		return text, false
	}

	var sb strings.Builder
	lastEnd := 0
	for _, f := range findings {
		sb.WriteString(text[lastEnd:f.Start])
		sb.WriteString(redactedMarker)
		lastEnd = f.End
	}
	sb.WriteString(text[lastEnd:])
	return sb.String(), true
}

type Result struct {
	Text      string
	Kinds     []Kind
	Truncated bool
}

// RedactKinds replaces detected secrets meeting minConfidence with [REDACTED:<kind>].
func RedactKinds(text string, minConfidence Confidence) Result {
	findings := Scan(text)
	var filtered []Finding
	var kinds []Kind

	for _, f := range findings {
		if minConfidence == ConfidenceHigh && f.Confidence != ConfidenceHigh {
			continue
		}
		filtered = append(filtered, f)
		kinds = append(kinds, f.Kind)
	}

	truncated := len(text) > maxScanBytes

	if len(filtered) == 0 {
		return Result{Text: text, Kinds: kinds, Truncated: truncated}
	}

	var sb strings.Builder
	lastEnd := 0
	for _, f := range filtered {
		sb.WriteString(text[lastEnd:f.Start])
		sb.WriteString("[REDACTED:" + string(f.Kind) + "]")
		lastEnd = f.End
	}
	
	if truncated {
		sb.WriteString(text[lastEnd:maxScanBytes])
		sb.WriteString(text[maxScanBytes:])
	} else {
		sb.WriteString(text[lastEnd:])
	}

	return Result{Text: sb.String(), Kinds: kinds, Truncated: truncated}
}
