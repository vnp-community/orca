package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// The design in a SAD is often a mermaid block. An agent copies the block as text and Orca reads it with this parser, so the
// edges of the design do not depend on how well a model reads a picture. Only flowchart and sequenceDiagram are read; any
// other block is reported as unsupported and ignored.

// ErrUnsupportedMermaid is returned for a block that is not a flowchart or a sequenceDiagram.
var ErrUnsupportedMermaid = errors.New("unsupported mermaid diagram type")

const (
	MermaidFlowchart = "flowchart"
	MermaidSequence  = "sequence"

	maxMermaidBytes = 16 * 1024
	maxMermaidLines = 400
)

type MermaidNode struct {
	ID    string
	Label string
	// Shape is box, datastore, queue, round or decision.
	Shape string
	// Group is the label of the subgraph the node was first seen in.
	Group string
}

type MermaidEdge struct {
	From, To, Label string
	// Both is true for a double-headed arrow, which is two edges.
	Both bool
}

type MermaidMessage struct {
	From, To, Label string
	Order           int
	Reply           bool
}

// MermaidDiagram is the result of reading one block. Warnings name what was skipped; the parser never panics on odd input.
type MermaidDiagram struct {
	Kind     string
	Nodes    []MermaidNode
	Edges    []MermaidEdge
	Messages []MermaidMessage
	Warnings []string
}

// ParseMermaid reads a flowchart or sequenceDiagram block.
func ParseMermaid(text string) (MermaidDiagram, error) {
	if len(text) > maxMermaidBytes {
		return MermaidDiagram{}, fmt.Errorf("mermaid block is %d bytes; the limit is %d", len(text), maxMermaidBytes)
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) > maxMermaidLines {
		return MermaidDiagram{}, fmt.Errorf("mermaid block has %d lines; the limit is %d", len(lines), maxMermaidLines)
	}
	header := ""
	start := 0
	for i, l := range lines {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "%%") {
			header, start = t, i+1
			break
		}
	}
	switch {
	case strings.HasPrefix(header, "flowchart"), strings.HasPrefix(header, "graph"):
		return parseFlowchart(lines[start:], start), nil
	case strings.HasPrefix(header, "sequenceDiagram"):
		return parseSequence(lines[start:], start), nil
	}
	return MermaidDiagram{}, ErrUnsupportedMermaid
}

var (
	brTag        = regexp.MustCompile(`(?i)<br\s*/?>`)
	arrowTextRe  = regexp.MustCompile(`^--\s+([^<>=]+?)\s+-->`)
	arrowRe      = regexp.MustCompile(`^(<-->|<==>|-->|==>|---|===|-\.->|-\.-|--[ox])`)
	nodeIDRe     = regexp.MustCompile(`^[A-Za-z0-9_]+`)
	flowIgnoreRe = regexp.MustCompile(`^(direction|classDef|class|style|linkStyle|click|accTitle|accDescr)\b`)
)

func cleanMermaidLabel(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)
	s = brTag.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

type flowParser struct {
	nodes   []MermaidNode
	index   map[string]int
	edges   []MermaidEdge
	group   []string
	skipped []string
}

func parseFlowchart(lines []string, offset int) MermaidDiagram {
	p := &flowParser{index: map[string]int{}}
	for n, raw := range lines {
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, "%%"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		switch {
		case line == "", flowIgnoreRe.MatchString(line):
		case strings.HasPrefix(line, "subgraph"):
			p.group = append(p.group, p.subgraphLabel(strings.TrimSpace(strings.TrimPrefix(line, "subgraph"))))
		case line == "end":
			if len(p.group) > 0 {
				p.group = p.group[:len(p.group)-1]
			}
		default:
			if !p.statement(line) {
				p.skipped = append(p.skipped, fmt.Sprintf("mermaid_unparsed_line:%d", offset+n+1))
			}
		}
	}
	return MermaidDiagram{Kind: MermaidFlowchart, Nodes: p.nodes, Edges: p.edges, Warnings: p.skipped}
}

func (p *flowParser) subgraphLabel(rest string) string {
	if id := nodeIDRe.FindString(rest); id != "" {
		if tail := strings.TrimSpace(rest[len(id):]); strings.HasPrefix(tail, "[") {
			if label, _, ok := readShape(tail); ok {
				return cleanMermaidLabel(label)
			}
		}
		return id
	}
	return cleanMermaidLabel(rest)
}

// statement reads `group (arrow [|label|] group)*`.
func (p *flowParser) statement(line string) bool {
	rest := line
	prev, rest, ok := p.nodeGroup(rest)
	if !ok {
		return false
	}
	any := false
	for {
		rest = strings.TrimSpace(rest)
		if rest == "" || rest == ";" {
			return true
		}
		label := ""
		if m := arrowTextRe.FindStringSubmatch(rest); m != nil {
			next, tail, ok := p.nodeGroup(strings.TrimSpace(rest[len(m[0]):]))
			if !ok {
				return any
			}
			p.link(prev, next, cleanMermaidLabel(m[1]), false)
			prev, rest, any = next, tail, true
			continue
		}
		arrow := arrowRe.FindString(rest)
		if arrow == "" {
			return any
		}
		rest = strings.TrimSpace(rest[len(arrow):])
		if strings.HasPrefix(rest, "|") {
			end := strings.Index(rest[1:], "|")
			if end < 0 {
				return any
			}
			label, rest = cleanMermaidLabel(rest[1:1+end]), strings.TrimSpace(rest[end+2:])
		}
		next, tail, ok := p.nodeGroup(rest)
		if !ok {
			return any
		}
		p.link(prev, next, label, arrow == "<-->" || arrow == "<==>")
		prev, rest, any = next, tail, true
	}
}

func (p *flowParser) link(from, to []string, label string, both bool) {
	for _, f := range from {
		for _, t := range to {
			p.edges = append(p.edges, MermaidEdge{From: f, To: t, Label: label, Both: both})
		}
	}
}

// nodeGroup reads `node (& node)*` and returns the ids.
func (p *flowParser) nodeGroup(s string) (ids []string, rest string, ok bool) {
	rest = strings.TrimSpace(s)
	for {
		id, tail, nodeOK := p.node(rest)
		if !nodeOK {
			return ids, rest, len(ids) > 0
		}
		ids, rest = append(ids, id), strings.TrimSpace(tail)
		if !strings.HasPrefix(rest, "&") {
			return ids, rest, true
		}
		rest = strings.TrimSpace(rest[1:])
	}
}

func (p *flowParser) node(s string) (id, rest string, ok bool) {
	id = nodeIDRe.FindString(s)
	if id == "" {
		return "", s, false
	}
	rest = s[len(id):]
	label, shape := "", ""
	if strings.HasPrefix(rest, "[") || strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "{") {
		l, sh, tail, shapeOK := readShapeWithTail(rest)
		if shapeOK {
			label, shape, rest = cleanMermaidLabel(l), sh, tail
		}
	}
	p.touch(id, label, shape)
	return id, rest, true
}

func (p *flowParser) touch(id, label, shape string) {
	if i, ok := p.index[id]; ok {
		if label != "" && p.nodes[i].Label == p.nodes[i].ID {
			p.nodes[i].Label, p.nodes[i].Shape = label, shape
		}
		return
	}
	if label == "" {
		label, shape = id, "box"
	}
	group := ""
	if len(p.group) > 0 {
		group = p.group[len(p.group)-1]
	}
	p.index[id] = len(p.nodes)
	p.nodes = append(p.nodes, MermaidNode{ID: id, Label: label, Shape: shape, Group: group})
}

var shapeDelims = []struct{ open, close, shape string }{
	{"[(", ")]", "datastore"},
	{"[[", "]]", "queue"},
	{"((", "))", "round"},
	{"[", "]", "box"},
	{"(", ")", "round"},
	{"{", "}", "decision"},
}

func readShape(s string) (label, shape string, ok bool) {
	label, shape, _, ok = readShapeWithTail(s)
	return label, shape, ok
}

// readShapeWithTail reads a shape such as [(MySQL)] or ["a<br/>b"], honouring a quoted label that may contain the closer.
func readShapeWithTail(s string) (label, shape, tail string, ok bool) {
	for _, d := range shapeDelims {
		if !strings.HasPrefix(s, d.open) {
			continue
		}
		body := s[len(d.open):]
		if strings.HasPrefix(body, `"`) {
			if q := strings.Index(body[1:], `"`); q >= 0 {
				after := body[q+2:]
				if strings.HasPrefix(after, d.close) {
					return body[1 : q+1], d.shape, after[len(d.close):], true
				}
			}
		}
		if end := strings.Index(body, d.close); end >= 0 {
			return body[:end], d.shape, body[end+len(d.close):], true
		}
	}
	return "", "", s, false
}

var (
	seqParticipantRe = regexp.MustCompile(`^(?:participant|actor)\s+([A-Za-z0-9_]+)(?:\s+as\s+(.+))?$`)
	seqMessageRe     = regexp.MustCompile(`^([A-Za-z0-9_]+)\s*(-->>|->>|--x|-x|--\)|-\)|-->|->)\s*[+-]?\s*([A-Za-z0-9_]+)\s*:\s*(.*)$`)
	seqStructureRe   = regexp.MustCompile(`^(note|loop|alt|else|opt|par|and|end|rect|critical|break|activate|deactivate|autonumber|title|box|link|links)\b`)
)

func parseSequence(lines []string, offset int) MermaidDiagram {
	d := MermaidDiagram{Kind: MermaidSequence}
	index := map[string]int{}
	declare := func(id, label string) {
		if i, ok := index[id]; ok {
			if label != "" {
				d.Nodes[i].Label = label
			}
			return
		}
		if label == "" {
			label = id
		}
		index[id] = len(d.Nodes)
		d.Nodes = append(d.Nodes, MermaidNode{ID: id, Label: label, Shape: "box"})
	}
	for n, raw := range lines {
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, "%%"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if m := seqParticipantRe.FindStringSubmatch(line); m != nil {
			declare(m[1], cleanMermaidLabel(m[2]))
			continue
		}
		if m := seqMessageRe.FindStringSubmatch(line); m != nil {
			declare(m[1], "")
			declare(m[3], "")
			if m[1] == m[3] {
				d.Warnings = append(d.Warnings, fmt.Sprintf("mermaid_self_message_skipped:%d", offset+n+1))
				continue
			}
			d.Messages = append(d.Messages, MermaidMessage{From: m[1], To: m[3], Label: cleanMermaidLabel(m[4]), Order: len(d.Messages) + 1, Reply: strings.HasPrefix(m[2], "--")})
			continue
		}
		if seqStructureRe.MatchString(strings.ToLower(line)) {
			continue
		}
		d.Warnings = append(d.Warnings, fmt.Sprintf("mermaid_unparsed_line:%d", offset+n+1))
	}
	return d
}
