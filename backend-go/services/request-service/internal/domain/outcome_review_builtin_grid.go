package domain

import (
	"fmt"
	"math"
	"strings"
)

// stateMachine lays the states out like an architecture diagram: left to right in the direction of the transitions.
func (c *canvas) stateMachine() { c.layered(stateLayout, true) }

// stateBox is a rounded box with the name of the state.
func (c *canvas) stateBox(n DiagramNode, idx int, x, y, w, h float64) {
	st := stateOf(n.State)
	c.statesIn[st] = true
	fmt.Fprintf(&c.body, `<g id="%s"><rect x="%s" y="%s" width="%s" height="%s" rx="18" class="rv-b s-%s"/>`, c.id(fmt.Sprintf("n%d", idx)), f(x), f(y), f(w), f(h), st)
	lines := wrap(statePrefix(st)+n.Label, 20)
	if len(lines) > 2 {
		lines = append(lines[:1], strings.Join(lines[1:], " "))
	}
	for i, l := range lines {
		c.text(&c.body, x+w/2, y+h/2+4-float64(len(lines)-1)*8+float64(i)*16, "rv-t", "middle", l)
	}
	c.body.WriteString(`</g>`)
}

// sidePoints picks the side of each box that faces the other.
func sidePoints(a, b [2]float64, w, h float64) ([2]float64, [2]float64) {
	dx, dy := b[0]-a[0], b[1]-a[1]
	if math.Abs(dx) >= math.Abs(dy) {
		if dx >= 0 {
			return [2]float64{a[0] + w, a[1] + h/2}, [2]float64{b[0], b[1] + h/2}
		}
		return [2]float64{a[0], a[1] + h/2}, [2]float64{b[0] + w, b[1] + h/2}
	}
	if dy >= 0 {
		return [2]float64{a[0] + w/2, a[1] + h}, [2]float64{b[0] + w/2, b[1]}
	}
	return [2]float64{a[0] + w/2, a[1]}, [2]float64{b[0] + w/2, b[1] + h}
}

// schema draws each table as a header and one row per column, three tables to a row, with the relations between the tables.
func (c *canvas) schema() {
	const w, head, rowH, gapX, gapY, margin, perRow = 300.0, 30.0, 22.0, 80.0, 40.0, 24.0, 3
	m := c.m
	at := map[string]int{}
	pos := make([][2]float64, len(m.Nodes))
	heights := make([]float64, len(m.Nodes))
	rowTop, rowMax := margin, 0.0
	for i, n := range m.Nodes {
		at[n.ID] = i
		col := i % perRow
		if col == 0 && i > 0 {
			rowTop += rowMax + gapY
			rowMax = 0
		}
		heights[i] = head + float64(maxInt(len(n.Columns), 1))*rowH + 6
		if heights[i] > rowMax {
			rowMax = heights[i]
		}
		pos[i] = [2]float64{margin + float64(col)*(w+gapX), rowTop}
	}
	for i, n := range m.Nodes {
		st := stateOf(n.State)
		c.statesIn[st] = true
		x, y := pos[i][0], pos[i][1]
		fmt.Fprintf(&c.body, `<g id="%s"><rect x="%s" y="%s" width="%s" height="%s" rx="8" class="rv-b s-%s"/>`, c.id(fmt.Sprintf("n%d", i)), f(x), f(y), f(w), f(heights[i]), st)
		c.text(&c.body, x+12, y+20, "rv-t", "start", statePrefix(st)+n.Label)
		fmt.Fprintf(&c.body, `<line x1="%s" y1="%s" x2="%s" y2="%s" class="rv-l"/>`, f(x), f(y+head), f(x+w), f(y+head))
		for r, col := range n.Columns {
			cs := stateOf(col.State)
			c.statesIn[cs] = true
			c.text(&c.body, x+12, y+head+float64(r)*rowH+16, "rv-m", "start", statePrefix(cs)+col.Name)
			c.text(&c.body, x+w-12, y+head+float64(r)*rowH+16, "rv-m", "end", col.Type)
		}
		c.body.WriteString(`</g>`)
	}
	for k, e := range m.Edges {
		a, okA := at[e.From]
		b, okB := at[e.To]
		if !okA || !okB || a == b {
			continue
		}
		p1, p2 := sidePoints(pos[a], pos[b], w, heights[a])
		p2[1] = pos[b][1] + heights[b]/2
		if math.Abs(pos[b][0]-pos[a][0]) < 1 { // same column: join the facing edges
			p1[1], p2[1] = pos[a][1]+heights[a]/2, pos[b][1]+heights[b]/2
		}
		mid := [2]float64{(p1[0] + p2[0]) / 2, (p1[1] + p2[1]) / 2}
		x, y := c.curve(p1, mid, mid, p2, e, k)
		c.edgeCaptionInline(x, y-6, e)
	}
	c.w = margin*2 + math.Min(float64(len(m.Nodes)), perRow)*w + (math.Min(float64(len(m.Nodes)), perRow)-1)*gapX
	c.h = rowTop + rowMax + 56
	c.legend(c.h - 20)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
