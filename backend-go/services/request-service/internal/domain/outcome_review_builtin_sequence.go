package domain

import (
	"fmt"
	"sort"
	"strings"
)

const (
	seqLaneW   = 200.0
	seqLaneGap = 44.0
	seqHeadH   = 44.0
	seqStep    = 54.0
	seqMargin  = 24.0
)

// sequence draws one lane per node, left to right in the model's order, and one arrow per message, top to bottom in its order.
func (c *canvas) sequence() {
	m := c.m
	msgs := make([]int, 0, len(m.Edges))
	for i := range m.Edges {
		msgs = append(msgs, i)
	}
	sort.SliceStable(msgs, func(a, b int) bool { return m.Edges[msgs[a]].Order < m.Edges[msgs[b]].Order })
	lanes := laneOrder(m, msgs)
	cx := make([]float64, len(m.Nodes))
	at := map[string]int{}
	for i, n := range m.Nodes {
		at[n.ID] = i
	}
	for pos, i := range lanes {
		cx[i] = seqMargin + float64(pos)*(seqLaneW+seqLaneGap) + seqLaneW/2
	}
	bottom := seqMargin + seqHeadH + 40 + float64(len(msgs))*seqStep
	for _, i := range lanes {
		n := m.Nodes[i]
		fmt.Fprintf(&c.body, `<line x1="%s" y1="%s" x2="%s" y2="%s" class="rv-l"/>`, f(cx[i]), f(seqMargin+seqHeadH), f(cx[i]), f(bottom))
		c.nodeLane(n, i, cx[i]-seqLaneW/2, seqMargin)
	}
	for k, ei := range msgs {
		e := m.Edges[ei]
		a, okA := at[e.From]
		b, okB := at[e.To]
		if !okA || !okB {
			continue
		}
		y := seqMargin + seqHeadH + 40 + float64(k)*seqStep
		cls, mk := c.edgeClass(e.State, e.Advisory || e.Unverified), c.markerRef(e.State)
		if a == b {
			x := cx[a]
			fmt.Fprintf(&c.body, `<path id="%s" d="M%s %s h44 v24 h-44" class="%s" marker-end="%s"/>`, c.id(fmt.Sprintf("e%d", ei)), f(x), f(y), cls, mk)
			c.edgeCaption(x+22+seqLaneGap, y+6, e)
			continue
		}
		x1, x2 := cx[a], cx[b]
		fmt.Fprintf(&c.body, `<line id="%s" x1="%s" y1="%s" x2="%s" y2="%s" class="%s" marker-end="%s"/>`, c.id(fmt.Sprintf("e%d", ei)), f(x1), f(y), f(x2), f(y), cls, mk)
		c.edgeCaption((x1+x2)/2, y-18, e)
	}
	c.w = seqMargin*2 + float64(len(m.Nodes))*seqLaneW + float64(len(m.Nodes)-1)*seqLaneGap
	c.h = bottom + 48
	c.legend(c.h - 20)
}

// nodeLane is the head box of a lane.
func (c *canvas) nodeLane(n DiagramNode, idx int, x, y float64) {
	st := stateOf(n.State)
	c.statesIn[st] = true
	fmt.Fprintf(&c.body, `<g id="%s"><rect x="%s" y="%s" width="%s" height="%s" rx="8" class="rv-b s-%s"/>`, c.id(fmt.Sprintf("n%d", idx)), f(x), f(y), f(seqLaneW), f(seqHeadH), st)
	lines := wrap(statePrefix(st)+n.Label, 24)
	if len(lines) > 2 {
		lines = append(lines[:1], strings.Join(lines[1:], " "))
	}
	for i, l := range lines {
		c.text(&c.body, x+seqLaneW/2, y+float64(20+(2-len(lines))*8)+float64(i)*15, "rv-t", "middle", l)
	}
	c.body.WriteString(`</g>`)
}

// laneOrder puts the lanes in the order they first speak or are spoken to, so the caller of the first message is on the left; lanes
// that take no part in a message follow in the model's order.
func laneOrder(m DiagramModel, msgs []int) []int {
	at := map[string]int{}
	for i, n := range m.Nodes {
		at[n.ID] = i
	}
	seen := make([]bool, len(m.Nodes))
	var out []int
	add := func(id string) {
		if i, ok := at[id]; ok && !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
	}
	for _, k := range msgs {
		add(m.Edges[k].From)
		add(m.Edges[k].To)
	}
	for _, n := range m.Nodes {
		add(n.ID)
	}
	return out
}
