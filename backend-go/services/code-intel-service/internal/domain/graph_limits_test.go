package domain

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestLimitSymbolGraph_5000Nodes(t *testing.T) {
	const total = 5000
	centerKey := "function:src/core.ts:main"

	nodes := make([]SymbolNode, total)
	edges := make([]SymbolEdge, 0, total*2)

	nodes[0] = SymbolNode{
		Ref: SymbolRef{Key: centerKey, Name: "main"},
	}

	for i := 1; i < total; i++ {
		key := fmt.Sprintf("function:src/file%d.ts:fn%d", i, i)
		nodes[i] = SymbolNode{
			Ref: SymbolRef{Key: key, Name: fmt.Sprintf("fn%d", i)},
		}
		// Create an edge from center to first 1000 nodes, and chain edges
		if i <= 1000 {
			edges = append(edges, SymbolEdge{
				FromKey:    centerKey,
				ToKey:      key,
				Confidence: 1.0,
			})
		} else {
			parentKey := fmt.Sprintf("function:src/file%d.ts:fn%d", i-1000, i-1000)
			edges = append(edges, SymbolEdge{
				FromKey:    parentKey,
				ToKey:      key,
				Confidence: 0.9,
			})
		}
	}

	sg := SymbolGraph{
		Center: SymbolRef{Key: centerKey, Name: "main"},
		Nodes:  nodes,
		Edges:  edges,
		Depth:  2,
	}

	// Run twice to ensure determinism
	res1, trunc1, before1, err1 := LimitSymbolGraph(sg, nil)
	res2, trunc2, before2, err2 := LimitSymbolGraph(sg, nil)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected error: %v, %v", err1, err2)
	}
	if before1 != total || before2 != total {
		t.Errorf("expected totalBefore = %d, got %d, %d", total, before1, before2)
	}
	if !trunc1 || !trunc2 {
		t.Errorf("expected truncated = true, got %v, %v", trunc1, trunc2)
	}
	if len(res1.Nodes) != SymbolGraphNodesMax {
		t.Fatalf("expected %d nodes, got %d", SymbolGraphNodesMax, len(res1.Nodes))
	}
	if !reflect.DeepEqual(res1, res2) {
		t.Error("truncation is not deterministic across runs")
	}

	// Verify center is present
	foundCenter := false
	for _, n := range res1.Nodes {
		if n.Ref.Key == centerKey {
			foundCenter = true
			break
		}
	}
	if !foundCenter {
		t.Fatal("center was pruned from symbol graph")
	}

	// Property: every retained edge must have both endpoints in retained nodes
	retainedKeys := make(map[string]bool, len(res1.Nodes))
	for _, n := range res1.Nodes {
		retainedKeys[n.Ref.Key] = true
	}
	for _, e := range res1.Edges {
		if !retainedKeys[e.FromKey] || !retainedKeys[e.ToKey] {
			t.Fatalf("orphan edge found after limit: from=%s, to=%s", e.FromKey, e.ToKey)
		}
	}
}

func TestLimitArchitecture_9039Clusters(t *testing.T) {
	const total = 9039
	nodes := make([]ClusterNode, total)
	for i := 0; i < total; i++ {
		nodes[i] = ClusterNode{
			ID:          fmt.Sprintf("comm_%05d", i),
			SymbolCount: int32(i % 100),
		}
	}

	g := ArchitectureGraph{Nodes: nodes}
	res, truncated, before, err := LimitArchitecture(g, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if before != total {
		t.Errorf("expected totalBefore %d, got %d", total, before)
	}
	if !truncated {
		t.Error("expected truncated = true")
	}
	if len(res.Nodes) != ClusterNodesMax {
		t.Fatalf("expected %d nodes, got %d", ClusterNodesMax, len(res.Nodes))
	}
}

func TestLimitArchitecture_SizeFuncSecondaryTruncation(t *testing.T) {
	nodes := make([]ClusterNode, 100)
	for i := range nodes {
		nodes[i] = ClusterNode{ID: fmt.Sprintf("c%d", i), SymbolCount: 10}
	}
	g := ArchitectureGraph{Nodes: nodes}

	// Fake size function: each node is 50,000 bytes.
	// 100 nodes = 5,000,000 bytes > 2 MiB (2,097,152 bytes).
	fakeSizeFn := func(v any) int {
		ag := v.(ArchitectureGraph)
		return len(ag.Nodes) * 50000
	}

	res, truncated, _, err := LimitArchitecture(g, fakeSizeFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !truncated {
		t.Error("expected truncated true")
	}
	finalSize := fakeSizeFn(res)
	if finalSize > ResponseMaxBytes {
		t.Errorf("expected final size <= %d, got %d", ResponseMaxBytes, finalSize)
	}
}

func TestLimitSymbolDetail_SourceTruncationAndSizeLimit(t *testing.T) {
	hugeSource := strings.Repeat("A", (200<<10)+1000) // 200 KiB + 1000 bytes
	d := SymbolDetail{
		Symbol: SymbolRef{Key: "function:src/huge.ts:render"},
		Source: SymbolSource{
			Text: hugeSource,
		},
	}

	res, truncated, _, err := LimitSymbolDetail(d, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !truncated || !res.Source.Truncated {
		t.Errorf("expected source to be truncated")
	}
	if len(res.Source.Text) != SymbolSourceMaxBytes {
		t.Errorf("expected source length %d, got %d", SymbolSourceMaxBytes, len(res.Source.Text))
	}

	// Test exceeding 320 KiB returns ErrResponseTooLarge
	fakeHugeSizeFn := func(v any) int {
		return 350 << 10 // 350 KiB > 320 KiB
	}
	_, _, _, err2 := LimitSymbolDetail(d, fakeHugeSizeFn)
	if err2 != ErrResponseTooLarge {
		t.Fatalf("expected ErrResponseTooLarge, got %v", err2)
	}
}
