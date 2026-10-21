package domain

import "strings"

const (
	archBoxW   = 190.0
	archBoxH   = 100.0
	archColGap = 170.0
	archRowGap = 52.0
	archMargin = 24.0
	archMaxCol = 4
)

// layout is the size of a box and the space around it; a state diagram uses smaller boxes than a container diagram.
type layout struct{ boxW, boxH, colGap, rowGap float64 }

var (
	containerLayout = layout{archBoxW, archBoxH, archColGap, archRowGap}
	stateLayout     = layout{176, 56, 150, 44}
)

func isStoreKind(kind string) bool {
	k := strings.ToLower(kind)
	for _, w := range []string{"database", "datastore", "store", "queue", "cache", "bucket", "db"} {
		if strings.Contains(k, w) {
			return true
		}
	}
	return false
}

// forwardEdges are the edges that do not close a cycle (a depth-first walk in the model's order marks the ones that do), so the
// columns can be worked out as longest paths.
func forwardEdges(m DiagramModel, at map[string]int) [][2]int {
	adj := make([][]int, len(m.Nodes))
	for _, e := range m.Edges {
		a, okA := at[e.From]
		b, okB := at[e.To]
		if okA && okB && a != b {
			adj[a] = append(adj[a], b)
		}
	}
	state := make([]int, len(m.Nodes)) // 0 new, 1 on the walk, 2 done
	var out [][2]int
	var walk func(int)
	walk = func(u int) {
		state[u] = 1
		for _, v := range adj[u] {
			switch state[v] {
			case 1: // closes a cycle: left out of the layering
			case 0:
				out = append(out, [2]int{u, v})
				walk(v)
			default:
				out = append(out, [2]int{u, v})
			}
		}
		state[u] = 2
	}
	for i := range m.Nodes {
		if state[i] == 0 {
			walk(i)
		}
	}
	return out
}

// architectureColumns puts each box one column right of the left-most box that points at it (longest path, bounded), and the data
// stores in the last column.
func architectureColumns(m DiagramModel) []int {
	at := map[string]int{}
	for i, n := range m.Nodes {
		at[n.ID] = i
	}
	edges := forwardEdges(m, at)
	col := make([]int, len(m.Nodes))
	for pass := 0; pass < len(m.Nodes); pass++ {
		moved := false
		for _, e := range edges {
			if col[e[1]] < col[e[0]]+1 && col[e[0]]+1 <= archMaxCol {
				col[e[1]] = col[e[0]] + 1
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	maxCol := 0
	for i, n := range m.Nodes {
		if !isStoreKind(n.Kind) && col[i] > maxCol {
			maxCol = col[i]
		}
	}
	for i, n := range m.Nodes {
		if isStoreKind(n.Kind) && maxCol > 0 {
			col[i] = maxCol + 1
		}
	}
	return col
}

func (c *canvas) architecture() { c.layered(containerLayout, false) }

// layered puts the boxes in columns by the direction of their arrows and draws the arrows between them: the layout of an
// architecture diagram and of a state diagram.
func (c *canvas) layered(l layout, states bool) {
	m := c.m
	col := architectureColumns(m)
	byCol := map[int][]int{}
	cols, rows := 0, 0
	for i := range m.Nodes {
		byCol[col[i]] = append(byCol[col[i]], i)
		if col[i]+1 > cols {
			cols = col[i] + 1
		}
	}
	for _, list := range byCol {
		if len(list) > rows {
			rows = len(list)
		}
	}
	pitch := l.boxH + l.rowGap
	pos := make([][2]float64, len(m.Nodes))
	for cIdx, list := range byCol {
		top := archMargin + float64(rows-len(list))*pitch/2
		for r, i := range list {
			pos[i] = [2]float64{archMargin + float64(cIdx)*(l.boxW+l.colGap), top + float64(r)*pitch}
		}
	}
	at := map[string]int{}
	for i, n := range m.Nodes {
		at[n.ID] = i
	}
	seen := map[[2]int]int{}
	for k, e := range m.Edges {
		a, okA := at[e.From]
		b, okB := at[e.To]
		if !okA || !okB {
			continue
		}
		key := [2]int{a, b}
		if a > b {
			key = [2]int{b, a}
		}
		off := float64(seen[key]) * 26
		seen[key]++
		mx, my := c.archEdge(l, k, e, pos[a], pos[b], col[a], col[b], off, a == b)
		c.edgeCaptionInline(mx, my-5, e)
	}
	for i, n := range m.Nodes {
		if states {
			c.stateBox(n, i, pos[i][0], pos[i][1], l.boxW, l.boxH)
		} else {
			c.nodeBox(n, i, pos[i][0], pos[i][1], l.boxW, l.boxH)
		}
	}
	c.w = archMargin*2 + float64(cols)*l.boxW + float64(cols-1)*l.colGap
	c.h = archMargin + float64(rows)*pitch - l.rowGap + 56
	c.legend(c.h - 20)
}

func (c *canvas) archEdge(l layout, idx int, e DiagramEdge, a, b [2]float64, ca, cb int, off float64, self bool) (float64, float64) {
	w, h := l.boxW, l.boxH
	switch {
	case self: // a loop over the top of the box
		p1 := [2]float64{a[0] + w*0.3, a[1]}
		p2 := [2]float64{a[0] + w*0.7, a[1]}
		return c.curve(p1, [2]float64{p1[0] - 10, p1[1] - 46}, [2]float64{p2[0] + 10, p2[1] - 46}, p2, e, idx)
	case ca < cb:
		p1 := [2]float64{a[0] + w, a[1] + h/2 + off}
		p2 := [2]float64{b[0], b[1] + h/2 + off}
		d, lift := maxF(30, (p2[0]-p1[0])/2), 0.0
		if cb-ca > 1 {
			lift = -(h/2 + 34)
		}
		return c.curve(p1, [2]float64{p1[0] + d, p1[1] + lift}, [2]float64{p2[0] - d, p2[1] + lift}, p2, e, idx)
	case ca > cb:
		p1 := [2]float64{a[0], a[1] + h/2 + off}
		p2 := [2]float64{b[0] + w, b[1] + h/2 + off}
		d, lift := maxF(30, (p1[0]-p2[0])/2), 0.0
		if ca-cb > 1 {
			lift = h/2 + 34
		}
		return c.curve(p1, [2]float64{p1[0] - d, p1[1] + lift}, [2]float64{p2[0] + d, p2[1] + lift}, p2, e, idx)
	}
	down := b[1] >= a[1]
	p1 := [2]float64{a[0] + w/2 + off, a[1] + h}
	p2 := [2]float64{b[0] + w/2 + off, b[1]}
	dy := 22.0
	if !down {
		p1[1], p2[1], dy = a[1], b[1]+h, -22
	}
	return c.curve(p1, [2]float64{p1[0] + 70, p1[1] + dy}, [2]float64{p2[0] + 70, p2[1] - dy}, p2, e, idx)
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
