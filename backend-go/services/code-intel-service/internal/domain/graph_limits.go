package domain

import (
	"errors"
	"math"
	"sort"
)

const (
	ClusterNodesMax        = 500
	ClusterEdgesMax        = 5000
	FlowPageMax            = 100
	FlowStepsMax           = 200
	SymbolGraphNodesMax    = 1500
	SymbolGraphEdgesMax    = 4000
	ImpactSymbolsMax       = 300
	ChangedSymbolsMax      = 2000
	SymbolSourceMaxBytes   = 200 << 10 // 200 KiB
	ResponseMaxBytes       = 2 << 20   // 2 MiB
	SymbolResponseMaxBytes = 320 << 10 // 320 KiB
)

var ErrResponseTooLarge = errors.New("CODEINTEL_RESPONSE_TOO_LARGE")

// SizeFunc calculates the serialized byte size of a response payload.
type SizeFunc func(v any) int

// LimitArchitecture deterministic truncation for ArchitectureGraph.
func LimitArchitecture(g ArchitectureGraph, sizeFn SizeFunc) (ArchitectureGraph, bool, int64, error) {
	totalBefore := int64(len(g.Nodes))
	truncated := false

	// Sort nodes: SymbolCount desc, then ID asc
	sort.Slice(g.Nodes, func(i, j int) bool {
		if g.Nodes[i].SymbolCount != g.Nodes[j].SymbolCount {
			return g.Nodes[i].SymbolCount > g.Nodes[j].SymbolCount
		}
		return g.Nodes[i].ID < g.Nodes[j].ID
	})

	if len(g.Nodes) > ClusterNodesMax {
		g.Nodes = g.Nodes[:ClusterNodesMax]
		truncated = true
	}

	filterArchitectureEdges(&g)

	// Secondary size-based truncation (10% step)
	for sizeFn != nil && sizeFn(g) > ResponseMaxBytes {
		if len(g.Nodes) <= 1 {
			return g, truncated, totalBefore, ErrResponseTooLarge
		}
		drop := int(math.Ceil(float64(len(g.Nodes)) * 0.10))
		newLen := len(g.Nodes) - drop
		if newLen < 1 {
			newLen = 1
		}
		g.Nodes = g.Nodes[:newLen]
		filterArchitectureEdges(&g)
		truncated = true
	}

	return g, truncated, totalBefore, nil
}

func filterArchitectureEdges(g *ArchitectureGraph) {
	nodeIDs := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeIDs[n.ID] = true
	}

	var validEdges []ClusterEdge
	for _, e := range g.Edges {
		if nodeIDs[e.From] && nodeIDs[e.To] {
			validEdges = append(validEdges, e)
		}
	}

	sort.Slice(validEdges, func(i, j int) bool {
		if validEdges[i].Weight != validEdges[j].Weight {
			return validEdges[i].Weight > validEdges[j].Weight
		}
		if validEdges[i].From != validEdges[j].From {
			return validEdges[i].From < validEdges[j].From
		}
		return validEdges[i].To < validEdges[j].To
	})

	if len(validEdges) > ClusterEdgesMax {
		validEdges = validEdges[:ClusterEdgesMax]
	}
	g.Edges = validEdges
}

// LimitModuleGraph deterministic truncation for ModuleGraph.
func LimitModuleGraph(g ModuleGraph, sizeFn SizeFunc) (ModuleGraph, bool, int64, error) {
	totalBefore := int64(len(g.Nodes))
	truncated := false

	sort.Slice(g.Nodes, func(i, j int) bool {
		if g.Nodes[i].SymbolCount != g.Nodes[j].SymbolCount {
			return g.Nodes[i].SymbolCount > g.Nodes[j].SymbolCount
		}
		if g.Nodes[i].Loc != g.Nodes[j].Loc {
			return g.Nodes[i].Loc > g.Nodes[j].Loc
		}
		return g.Nodes[i].ID < g.Nodes[j].ID
	})

	if len(g.Nodes) > ClusterNodesMax {
		g.Nodes = g.Nodes[:ClusterNodesMax]
		truncated = true
	}

	filterModuleEdges(&g)

	for sizeFn != nil && sizeFn(g) > ResponseMaxBytes {
		if len(g.Nodes) <= 1 {
			return g, truncated, totalBefore, ErrResponseTooLarge
		}
		drop := int(math.Ceil(float64(len(g.Nodes)) * 0.10))
		newLen := len(g.Nodes) - drop
		if newLen < 1 {
			newLen = 1
		}
		g.Nodes = g.Nodes[:newLen]
		filterModuleEdges(&g)
		truncated = true
	}

	return g, truncated, totalBefore, nil
}

func filterModuleEdges(g *ModuleGraph) {
	nodeIDs := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeIDs[n.ID] = true
	}

	var validEdges []ModuleEdge
	for _, e := range g.Edges {
		if nodeIDs[e.From] && nodeIDs[e.To] {
			validEdges = append(validEdges, e)
		}
	}

	sort.Slice(validEdges, func(i, j int) bool {
		if validEdges[i].Count != validEdges[j].Count {
			return validEdges[i].Count > validEdges[j].Count
		}
		if validEdges[i].From != validEdges[j].From {
			return validEdges[i].From < validEdges[j].From
		}
		return validEdges[i].To < validEdges[j].To
	})

	if len(validEdges) > ClusterEdgesMax {
		validEdges = validEdges[:ClusterEdgesMax]
	}
	g.Edges = validEdges
}

// LimitSymbolGraph deterministic truncation for SymbolGraph.
// Distance to Center is preserved, center is never dropped, and orphan edges are eliminated.
func LimitSymbolGraph(g SymbolGraph, sizeFn SizeFunc) (SymbolGraph, bool, int64, error) {
	totalBefore := int64(len(g.Nodes))
	truncated := false

	// Build adjacency list for distance and degree
	adj := make(map[string][]string)
	degree := make(map[string]int)
	for _, e := range g.Edges {
		adj[e.FromKey] = append(adj[e.FromKey], e.ToKey)
		adj[e.ToKey] = append(adj[e.ToKey], e.FromKey)
		degree[e.FromKey]++
		degree[e.ToKey]++
	}

	centerKey := g.Center.Key
	dist := make(map[string]int)
	for _, n := range g.Nodes {
		dist[n.Ref.Key] = 999999
	}
	if centerKey != "" {
		dist[centerKey] = 0
		queue := []string{centerKey}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			currDist := dist[curr]
			for _, neighbor := range adj[curr] {
				if d, ok := dist[neighbor]; ok && d > currDist+1 {
					dist[neighbor] = currDist + 1
					queue = append(queue, neighbor)
				}
			}
		}
	}

	// Sort nodes: dist asc, degree desc, key asc
	sort.Slice(g.Nodes, func(i, j int) bool {
		ki := g.Nodes[i].Ref.Key
		kj := g.Nodes[j].Ref.Key
		if ki == centerKey {
			return true
		}
		if kj == centerKey {
			return false
		}
		if dist[ki] != dist[kj] {
			return dist[ki] < dist[kj]
		}
		if degree[ki] != degree[kj] {
			return degree[ki] > degree[kj]
		}
		return ki < kj
	})

	if len(g.Nodes) > SymbolGraphNodesMax {
		g.Nodes = g.Nodes[:SymbolGraphNodesMax]
		truncated = true
	}

	filterSymbolEdges(&g)

	for sizeFn != nil && sizeFn(g) > ResponseMaxBytes {
		if len(g.Nodes) <= 1 {
			return g, truncated, totalBefore, ErrResponseTooLarge
		}
		drop := int(math.Ceil(float64(len(g.Nodes)) * 0.10))
		newLen := len(g.Nodes) - drop
		if newLen < 1 {
			newLen = 1
		}
		g.Nodes = g.Nodes[:newLen]
		filterSymbolEdges(&g)
		truncated = true
	}

	return g, truncated, totalBefore, nil
}

func filterSymbolEdges(g *SymbolGraph) {
	keys := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		keys[n.Ref.Key] = true
	}

	var validEdges []SymbolEdge
	for _, e := range g.Edges {
		if keys[e.FromKey] && keys[e.ToKey] {
			validEdges = append(validEdges, e)
		}
	}

	sort.Slice(validEdges, func(i, j int) bool {
		if validEdges[i].Confidence != validEdges[j].Confidence {
			return validEdges[i].Confidence > validEdges[j].Confidence
		}
		if validEdges[i].FromKey != validEdges[j].FromKey {
			return validEdges[i].FromKey < validEdges[j].FromKey
		}
		if validEdges[i].ToKey != validEdges[j].ToKey {
			return validEdges[i].ToKey < validEdges[j].ToKey
		}
		return validEdges[i].Kind < validEdges[j].Kind
	})

	if len(validEdges) > SymbolGraphEdgesMax {
		validEdges = validEdges[:SymbolGraphEdgesMax]
	}
	g.Edges = validEdges
}

// LimitFlowGraph deterministic truncation for FlowGraph.
func LimitFlowGraph(g FlowGraph, sizeFn SizeFunc) (FlowGraph, bool, int64, error) {
	totalBefore := int64(len(g.Steps))
	truncated := false

	if len(g.Steps) > FlowStepsMax {
		g.Steps = g.Steps[:FlowStepsMax]
		truncated = true
	}

	// Filter edges to only steps present
	stepKeys := make(map[string]bool, len(g.Steps))
	for _, s := range g.Steps {
		stepKeys[s.Symbol.Key] = true
	}
	var validEdges []SymbolEdge
	for _, e := range g.Edges {
		if stepKeys[e.FromKey] && stepKeys[e.ToKey] {
			validEdges = append(validEdges, e)
		}
	}
	g.Edges = validEdges

	for sizeFn != nil && sizeFn(g) > ResponseMaxBytes {
		if len(g.Steps) <= 1 {
			return g, truncated, totalBefore, ErrResponseTooLarge
		}
		drop := int(math.Ceil(float64(len(g.Steps)) * 0.10))
		newLen := len(g.Steps) - drop
		if newLen < 1 {
			newLen = 1
		}
		g.Steps = g.Steps[:newLen]
		truncated = true
	}

	return g, truncated, totalBefore, nil
}

// LimitImpactGraph deterministic truncation for ImpactGraph.
func LimitImpactGraph(g ImpactGraph, sizeFn SizeFunc) (ImpactGraph, bool, int64, error) {
	totalBefore := int64(0)
	for _, l := range g.Levels {
		totalBefore += int64(len(l.Symbols))
	}
	truncated := false

	sort.Slice(g.Levels, func(i, j int) bool {
		return g.Levels[i].Depth < g.Levels[j].Depth
	})

	for idx := range g.Levels {
		sort.Slice(g.Levels[idx].Symbols, func(i, j int) bool {
			si := g.Levels[idx].Symbols[i]
			sj := g.Levels[idx].Symbols[j]
			if si.Direct != sj.Direct {
				return si.Direct // true comes first
			}
			return si.Symbol.Key < sj.Symbol.Key
		})
	}

	// Truncate to ImpactSymbolsMax across all levels
	remaining := ImpactSymbolsMax
	var trimmedLevels []ImpactLevel
	for _, l := range g.Levels {
		if remaining <= 0 {
			truncated = true
			break
		}
		if len(l.Symbols) > remaining {
			l.Symbols = l.Symbols[:remaining]
			remaining = 0
			truncated = true
		} else {
			remaining -= len(l.Symbols)
		}
		trimmedLevels = append(trimmedLevels, l)
	}
	g.Levels = trimmedLevels

	totalAfter := int32(0)
	for _, l := range g.Levels {
		totalAfter += int32(len(l.Symbols))
	}
	g.ImpactedCount = totalAfter

	for sizeFn != nil && sizeFn(g) > ResponseMaxBytes {
		return g, truncated, totalBefore, ErrResponseTooLarge
	}

	return g, truncated, totalBefore, nil
}

// LimitRouteMap deterministic truncation for RouteMap.
func LimitRouteMap(g RouteMap, sizeFn SizeFunc) (RouteMap, bool, int64, error) {
	totalBefore := int64(len(g.Routes))
	truncated := false

	sort.Slice(g.Routes, func(i, j int) bool {
		if g.Routes[i].Path != g.Routes[j].Path {
			return g.Routes[i].Path < g.Routes[j].Path
		}
		return g.Routes[i].Method < g.Routes[j].Method
	})

	routeIDs := make(map[string]bool, len(g.Routes))
	for _, r := range g.Routes {
		routeIDs[r.ID] = true
	}

	var validEdges []RouteEdge
	for _, e := range g.Edges {
		if routeIDs[e.RouteID] {
			validEdges = append(validEdges, e)
		}
	}
	g.Edges = validEdges

	for sizeFn != nil && sizeFn(g) > ResponseMaxBytes {
		if len(g.Routes) <= 1 {
			return g, truncated, totalBefore, ErrResponseTooLarge
		}
		drop := int(math.Ceil(float64(len(g.Routes)) * 0.10))
		newLen := len(g.Routes) - drop
		if newLen < 1 {
			newLen = 1
		}
		g.Routes = g.Routes[:newLen]
		truncated = true
	}

	return g, truncated, totalBefore, nil
}

// LimitSymbolDetail limits source text to SymbolSourceMaxBytes and response to SymbolResponseMaxBytes.
func LimitSymbolDetail(d SymbolDetail, sizeFn SizeFunc) (SymbolDetail, bool, int64, error) {
	truncated := false
	if len(d.Source.Text) > SymbolSourceMaxBytes {
		d.Source.Text = d.Source.Text[:SymbolSourceMaxBytes]
		d.Source.Truncated = true
		truncated = true
	}

	if sizeFn != nil && sizeFn(d) > SymbolResponseMaxBytes {
		return d, truncated, 1, ErrResponseTooLarge
	}

	return d, truncated, 1, nil
}
