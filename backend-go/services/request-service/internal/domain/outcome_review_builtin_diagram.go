package domain

import (
	"fmt"
	"html"
	"math"
	"strings"
	"unicode/utf8"
)

// BuiltinDrawSkillVersion keys the drawings Orca makes itself. They need no agent, no dev server and no skill, cost nothing and say
// exactly what the checked model says; an agent drawing of the same model (skill installed) is a separate revision.
const BuiltinDrawSkillVersion = "orca-builtin/1"

// RenderBuiltinDiagram draws a checked model as one self-contained page with one <svg>. It is a layout of the model, not an
// interpretation: every box, arrow and word comes from the model or from the legend, and the colours are role variables that the
// viewer fills with Orca's tokens, so the drawing follows the theme.
func RenderBuiltinDiagram(m DiagramModel) ([]byte, error) {
	if len(m.Nodes) == 0 {
		return nil, ErrReviewArtifactUnsafe("the model has no node to draw")
	}
	c := newCanvas(m)
	switch m.Type {
	case DiagramArchitecture:
		c.architecture()
	case DiagramSequence:
		c.sequence()
	case DiagramStateMachine:
		c.stateMachine()
	case DiagramDatabase:
		c.schema()
	default:
		return nil, ErrReviewArtifactUnsafe(fmt.Sprintf("the builtin drawing has no layout for %q", m.Type))
	}
	return c.page(), nil
}

type canvas struct {
	m        DiagramModel
	prefix   string
	body     strings.Builder
	labels   strings.Builder // edge labels go last so a box or a line never hides one
	w, h     float64
	statesIn map[EdgeState]bool
}

func newCanvas(m DiagramModel) *canvas {
	return &canvas{m: m, prefix: fmt.Sprintf("rv-%s-%s-", safeID(string(m.Section)), safeID(m.Variant)), statesIn: map[EdgeState]bool{}}
}

func safeID(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

func esc(s string) string {
	clean := strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' {
			return ' '
		}
		return r
	}, s)
	return html.EscapeString(clean)
}

func f(v float64) string { return fmt.Sprintf("%.1f", v) }

func stateOf(s EdgeState) EdgeState {
	switch s {
	case StateAdded, StateRemoved, StateDeviation, StateMissing, StateUnmapped:
		return s
	}
	return StateSame
}

// statePrefix is the mark that says the state without colour: a colour alone is not read by everyone.
func statePrefix(s EdgeState) string {
	switch s {
	case StateAdded:
		return "+ "
	case StateRemoved:
		return "− "
	case StateDeviation:
		return "! "
	case StateMissing:
		return "? "
	}
	return ""
}

// wrap splits a label at spaces into lines of at most n runes; a word longer than n is cut.
func wrap(s string, n int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		for utf8.RuneCountInString(w) > n {
			r := []rune(w)
			if cur != "" {
				lines, cur = append(lines, cur), ""
			}
			lines, w = append(lines, string(r[:n])), string(r[n:])
		}
		switch {
		case cur == "":
			cur = w
		case utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= n:
			cur += " " + w
		default:
			lines, cur = append(lines, cur), w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

func (c *canvas) id(name string) string { return c.prefix + name }

// text writes one <text> line; class is rv-t (label), rv-m (caption) or one of them with the halo.
func (c *canvas) text(b *strings.Builder, x, y float64, class, anchor, s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	fmt.Fprintf(b, `<text x="%s" y="%s" class="%s" text-anchor="%s">%s</text>`, f(x), f(y), class, anchor, esc(s))
}

func (c *canvas) edgeClass(s EdgeState, advisory bool) string {
	s = stateOf(s)
	c.statesIn[s] = true
	cls := "e s-" + string(s)
	if advisory {
		cls += " adv"
	}
	return cls
}

func (c *canvas) markerRef(s EdgeState) string {
	return fmt.Sprintf("url(#%s)", c.id("ar-"+string(stateOf(s))))
}

// edgeCaptionInline writes the label and the protocol of an edge on one line, for diagrams where arrows run side by side.
func (c *canvas) edgeCaptionInline(x, y float64, e DiagramEdge) {
	parts := []string{}
	if e.Label != "" {
		parts = append(parts, e.Label)
	}
	if e.Protocol != "" && e.Protocol != e.Label {
		parts = append(parts, e.Protocol)
	}
	joined := strings.Join(parts, " \u00b7 ")
	if utf8.RuneCountInString(joined) <= 24 || len(parts) < 2 {
		c.text(&c.labels, x, y, "rv-m rv-h", "middle", joined) // never cut: a half word is a word the model does not have
		return
	}
	for i, p := range parts {
		c.text(&c.labels, x, y-float64(len(parts)-1-i)*13, "rv-m rv-h", "middle", p)
	}
}

// edgeCaption writes the label and the protocol of an edge, centred on a halo.
func (c *canvas) edgeCaption(x, y float64, e DiagramEdge) {
	parts := []string{}
	if e.Label != "" {
		parts = append(parts, e.Label)
	}
	if e.Protocol != "" && e.Protocol != e.Label {
		parts = append(parts, e.Protocol)
	}
	for i, p := range parts {
		c.text(&c.labels, x, y+float64(i)*13, "rv-m rv-h", "middle", p)
	}
}

func (c *canvas) legend(y float64) {
	order := []EdgeState{StateAdded, StateRemoved, StateDeviation, StateMissing, StateUnmapped, StateSame}
	var b strings.Builder
	fmt.Fprintf(&b, `<g id="rv-legend-%s-%s">`, safeID(string(c.m.Section)), safeID(c.m.Variant))
	x := 24.0
	for _, s := range order {
		if !c.statesIn[s] {
			continue
		}
		label := map[EdgeState]string{
			StateAdded: "+ added", StateRemoved: "− removed", StateDeviation: "! deviation",
			StateMissing: "? missing", StateUnmapped: "not in design", StateSame: "same",
		}[s]
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" class="e s-%s"/>`, f(x), f(y), f(x+28), f(y), s)
		c.text(&b, x+34, y+4, "rv-m", "start", label)
		x += 52 + float64(utf8.RuneCountInString(label))*6.4
	}
	b.WriteString(`</g>`)
	c.body.WriteString(b.String())
	if x+24 > c.w {
		c.w = x + 24
	}
}

func (c *canvas) markers() string {
	var b strings.Builder
	b.WriteString("<defs>")
	for _, s := range []EdgeState{StateSame, StateAdded, StateRemoved, StateDeviation, StateMissing, StateUnmapped} {
		if !c.statesIn[s] {
			continue
		}
		fmt.Fprintf(&b, `<marker id="%s" viewBox="0 0 10 10" refX="9" refY="5" markerUnits="userSpaceOnUse" markerWidth="10" markerHeight="10" orient="auto"><path d="M0 0L10 5L0 10z" class="m-%s"/></marker>`, c.id("ar-"+string(s)), s)
	}
	b.WriteString("</defs>")
	return b.String()
}

const builtinCSS = `.rv-t{font:600 13px ui-sans-serif,system-ui,sans-serif;fill:var(--rv-fg,#1f2328)}` +
	`.rv-m{font:11px ui-monospace,SFMono-Regular,monospace;fill:var(--rv-muted,#6b7280)}` +
	`.rv-h{paint-order:stroke;stroke:var(--rv-bg,#ffffff);stroke-width:4px;stroke-linejoin:round}` +
	`.rv-b{fill:var(--rv-bg,#ffffff);stroke-width:1.5}.rv-g{fill:var(--rv-bg,#ffffff);stroke:var(--rv-border,#d0d7de);stroke-width:1.2}` +
	`.rv-l{stroke:var(--rv-border,#d0d7de);stroke-dasharray:4 4;stroke-width:1}` +
	`.e{fill:none;stroke-width:1.6}.adv{opacity:.65;stroke-width:1.1}` +
	`.s-same{stroke:var(--rv-muted,#6b7280)}.s-added{stroke:var(--rv-diff-add,#2e7d32)}` +
	`.s-removed{stroke:var(--rv-diff-del,#c62828);stroke-dasharray:6 4}` +
	`.s-deviation{stroke:var(--rv-accent,#d97757);stroke-width:2.4}.s-missing{stroke:var(--rv-diff-del,#c62828);stroke-dasharray:2 4}` +
	`.s-unmapped{stroke:var(--rv-accent,#d97757);stroke-dasharray:2 4}` +
	`.m-same{fill:var(--rv-muted,#6b7280)}.m-added{fill:var(--rv-diff-add,#2e7d32)}.m-removed{fill:var(--rv-diff-del,#c62828)}` +
	`.m-deviation{fill:var(--rv-accent,#d97757)}.m-missing{fill:var(--rv-diff-del,#c62828)}.m-unmapped{fill:var(--rv-accent,#d97757)}` +
	`.bd{fill:var(--rv-bg,#ffffff);stroke:var(--rv-accent,#d97757);stroke-width:1.5}.bn{font:600 11px ui-sans-serif,system-ui,sans-serif;fill:var(--rv-accent,#d97757)}`

func (c *canvas) titleAndDesc() (string, string) {
	kind := map[DiagramType]string{
		DiagramArchitecture: "Architecture diagram", DiagramSequence: "Sequence diagram", DiagramStateMachine: "State diagram", DiagramDatabase: "Schema diagram",
	}[c.m.Type]
	name := c.m.Name
	if name == "" {
		name = string(c.m.Section) + " " + c.m.Variant
	}
	return fmt.Sprintf("%s: %s", kind, name), fmt.Sprintf("%d boxes and %d arrows, drawn by Orca from the checked model of the review.", len(c.m.Nodes), len(c.m.Edges))
}

func (c *canvas) page() []byte {
	title, desc := c.titleAndDesc()
	w, h := math.Ceil(c.w), math.Ceil(c.h)
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>` + esc(title) + `</title></head><body style="margin:0">`)
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" role="img" viewBox="0 0 %s %s" width="100%%" font-family="ui-sans-serif, system-ui, sans-serif">`, f(w), f(h))
	b.WriteString(`<title>` + esc(title) + `</title><desc>` + esc(desc) + `</desc>`)
	b.WriteString(`<style>` + builtinCSS + `</style>`)
	b.WriteString(c.markers())
	b.WriteString(c.body.String())
	b.WriteString(c.labels.String())
	b.WriteString(`</svg></body></html>`)
	return []byte(b.String())
}

// nodeBox draws one box and returns nothing: the caller knows where it put it.
func (c *canvas) nodeBox(n DiagramNode, idx int, x, y, w, h float64) {
	st := stateOf(n.State)
	c.statesIn[st] = true
	fmt.Fprintf(&c.body, `<g id="%s"><rect x="%s" y="%s" width="%s" height="%s" rx="8" class="rv-b s-%s"/>`, c.id(fmt.Sprintf("n%d", idx)), f(x), f(y), f(w), f(h), st)
	lines := wrap(statePrefix(st)+n.Label, 21)
	if len(lines) > 2 {
		lines = append(lines[:1], strings.Join(lines[1:], " "))
	}
	for i, l := range lines {
		c.text(&c.body, x+14, y+24+float64(i)*16, "rv-t", "start", l)
	}
	// Captions flow under the label, never cut: the box is tall enough for a label, a kind and tech line, and "not in design".
	cy := y + 24 + float64(len(lines)-1)*16 + 18
	for _, l := range wrap(strings.Join(nonEmpty(n.Kind, n.Tech), " \u00b7 "), 27) {
		if l != "" {
			c.text(&c.body, x+14, cy, "rv-m", "start", l)
			cy += 13
		}
	}
	if st == StateUnmapped {
		c.text(&c.body, x+14, cy, "rv-m", "start", "not in design")
	}
	if n.ComponentsChanged > 0 {
		fmt.Fprintf(&c.body, `<circle cx="%s" cy="%s" r="11" class="bd"/>`, f(x+w-6), f(y+6))
		c.text(&c.body, x+w-6, y+10, "bn", "middle", fmt.Sprint(n.ComponentsChanged))
	}
	c.body.WriteString(`</g>`)
}

func nonEmpty(in ...string) []string {
	var out []string
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// cubicMid is the point halfway along a cubic curve.
func cubicMid(p1, c1, c2, p2 [2]float64) (float64, float64) {
	return (p1[0] + 3*c1[0] + 3*c2[0] + p2[0]) / 8, (p1[1] + 3*c1[1] + 3*c2[1] + p2[1]) / 8
}

func (c *canvas) curve(p1, c1, c2, p2 [2]float64, e DiagramEdge, idx int) (float64, float64) {
	fmt.Fprintf(&c.body, `<path id="%s" d="M%s %s C%s %s %s %s %s %s" class="%s" marker-end="%s"/>`,
		c.id(fmt.Sprintf("e%d", idx)), f(p1[0]), f(p1[1]), f(c1[0]), f(c1[1]), f(c2[0]), f(c2[1]), f(p2[0]), f(p2[1]), c.edgeClass(e.State, e.Advisory || e.Unverified), c.markerRef(e.State))
	return cubicMid(p1, c1, c2, p2)
}
