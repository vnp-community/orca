package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	regionBeginPattern = regexp.MustCompile(`^<!-- orca:begin ([a-z0-9-]+)((?: [a-z]+=[^ >]+)*) -->$`)
	regionEndPattern   = regexp.MustCompile(`^<!-- orca:end ([a-z0-9-]+) -->$`)
	regionDigestAttr   = regexp.MustCompile(`digest=(sha256:[0-9a-f]{64})`)
)

const orcaJSONFence = "```orca-json"

// region is one Orca-owned span: begin and end are 0-based line indexes of the markers.
type region struct {
	section    string
	begin, end int
	digest     string
}

func trimCR(l string) string { return strings.TrimSuffix(l, "\r") }

// scanRegions finds every begin/end pair. Markers inside a fenced block do not count, so prose
// that quotes a marker cannot open or close a region. Nested or unbalanced markers are violations.
func scanRegions(lines []string, firstLine int) ([]region, []Violation) {
	var regions []region
	var out []Violation
	open := -1
	var cur region
	fence := false
	for i, raw := range lines {
		l := trimCR(raw)
		if strings.HasPrefix(l, "```") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		if m := regionBeginPattern.FindStringSubmatch(l); m != nil {
			if open >= 0 {
				out = append(out, Violation{Line: firstLine + i, Code: CodeProjectionRegionMalformed, Message: "regions cannot be nested: " + m[1] + " opens inside " + cur.section})
				continue
			}
			open, cur = i, region{section: m[1], begin: i}
			if d := regionDigestAttr.FindStringSubmatch(m[2]); d != nil {
				cur.digest = d[1]
			}
			continue
		}
		if m := regionEndPattern.FindStringSubmatch(l); m != nil {
			if open < 0 || m[1] != cur.section {
				out = append(out, Violation{Line: firstLine + i, Code: CodeProjectionRegionMalformed, Message: "unexpected end marker for " + m[1]})
				continue
			}
			cur.end = i
			regions = append(regions, cur)
			open = -1
		}
	}
	if open >= 0 {
		out = append(out, Violation{Line: firstLine + open, Code: CodeProjectionRegionMalformed, Message: "region " + cur.section + " is never closed"})
	}
	return regions, out
}

// ExtractRegion returns the text between the markers of section (markers excluded) and their 1-based line numbers.
func ExtractRegion(md, section string) (body string, startLine, endLine int, ok bool) {
	lines := strings.Split(md, "\n")
	regions, vs := scanRegions(lines, 1)
	if len(vs) > 0 {
		return "", 0, 0, false
	}
	for _, r := range regions {
		if r.section == section {
			return strings.Join(lines[r.begin+1:r.end], "\n"), r.begin + 1, r.end + 1, true
		}
	}
	return "", 0, 0, false
}

// ExtractOrcaJSON wants exactly one ```orca-json block in a region body; lines are relative to body.
func ExtractOrcaJSON(body string) ([]byte, []Violation) {
	raw, _, vs := extractOrcaJSON(strings.Split(body, "\n"), 1)
	return raw, vs
}

// extractOrcaJSON also returns the absolute line of the JSON text itself, for violations about its content.
func extractOrcaJSON(lines []string, firstLine int) ([]byte, int, []Violation) {
	type block struct {
		start int
		text  []string
	}
	var blocks []block
	inOrca, inOther := false, false
	for i, raw := range lines {
		l := trimCR(raw)
		switch {
		case inOrca:
			if l == "```" {
				inOrca = false
				continue
			}
			blocks[len(blocks)-1].text = append(blocks[len(blocks)-1].text, l)
		case inOther:
			if strings.HasPrefix(l, "```") {
				inOther = false
			}
		case l == orcaJSONFence:
			inOrca = true
			blocks = append(blocks, block{start: firstLine + i})
		case strings.HasPrefix(l, "```"):
			inOther = true
		}
	}
	if inOrca {
		return nil, 0, []Violation{{Line: blocks[len(blocks)-1].start, Code: CodeProjectionJSONBlock, Message: "the orca-json block is never closed"}}
	}
	switch len(blocks) {
	case 0:
		return nil, 0, []Violation{{Line: firstLine, Code: CodeProjectionJSONBlock, Message: "the region has no orca-json block"}}
	case 1:
	default:
		return nil, 0, []Violation{{Line: blocks[1].start, Code: CodeProjectionJSONBlock, Message: "the region has more than one orca-json block"}}
	}
	raw := []byte(strings.Join(blocks[0].text, "\n"))
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, 0, []Violation{{Line: blocks[0].start + 1, Code: CodeProjectionJSONBlock, Message: "the orca-json block is not a JSON object: " + err.Error()}}
	}
	return raw, blocks[0].start + 1, nil
}

// ReplaceRegion swaps the lines between the markers of section and keeps every other byte, so text
// a person wrote outside the region survives (CR-REQ-027 section 2.7 rule 4). A missing region is an error, never a guess.
func ReplaceRegion(md, section, newBody string) (string, error) {
	lines := strings.SplitAfter(md, "\n")
	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = strings.TrimSuffix(l, "\n")
	}
	regions, vs := scanRegions(plain, 1)
	if len(vs) > 0 {
		return "", fmt.Errorf("%s: %s (line %d)", vs[0].Code, vs[0].Message, vs[0].Line)
	}
	for _, r := range regions {
		if r.section != section {
			continue
		}
		var b bytes.Buffer
		for _, l := range lines[:r.begin+1] {
			b.WriteString(l)
		}
		if newBody != "" {
			b.WriteString(strings.TrimRight(newBody, "\n"))
			b.WriteString("\n")
		}
		for _, l := range lines[r.end:] {
			b.WriteString(l)
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("%s: no region named %q", CodeProjectionRegionMissing, section)
}
