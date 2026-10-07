package domain

import (
	"fmt"
	"reflect"
	"testing"
)

func TestMergeSymbolSets_PrimaryMatch(t *testing.T) {
	a := []SymbolRef{
		{
			Key:        "function:src/util.ts:formatDate",
			Kind:       SymbolKindFunction,
			NativeKind: "Function",
			Name:       "formatDate",
			FilePath:   "src/util.ts",
			StartLine:  10,
			EndLine:    20,
			GitNexusID: "gn-1",
		},
	}
	b := []SymbolRef{
		{
			Key:         "function:src/util.ts:formatDate",
			Kind:        SymbolKindFunction,
			NativeKind:  "function",
			Name:        "formatDate",
			FilePath:    "src/util.ts",
			StartLine:   11,
			EndLine:     21,
			CodeGraphID: "cg-1",
			Signature:   "(d: Date) => string",
			Language:    "typescript",
		},
	}

	merged, report := MergeSymbolSets(a, b)
	if report.PrimaryMatches != 1 {
		t.Fatalf("expected 1 primary match, got %d", report.PrimaryMatches)
	}
	if report.SecondaryMatches != 0 {
		t.Fatalf("expected 0 secondary matches, got %d", report.SecondaryMatches)
	}
	if report.Unmatched != 0 {
		t.Fatalf("expected 0 unmatched, got %d", report.Unmatched)
	}
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged node, got %d", len(merged))
	}

	m := merged[0]
	if m.GitNexusID != "gn-1" || m.CodeGraphID != "cg-1" {
		t.Errorf("expected both IDs preserved, got gn=%s, cg=%s", m.GitNexusID, m.CodeGraphID)
	}
	if m.StartLine != 10 { // GitNexus takes precedence for startLine
		t.Errorf("expected startLine 10, got %d", m.StartLine)
	}
	if m.Signature != "(d: Date) => string" { // CodeGraph takes precedence for signature
		t.Errorf("expected CodeGraph signature, got %s", m.Signature)
	}
	if m.NativeKind != "Function|function" {
		t.Errorf("expected combined nativeKind Function|function, got %s", m.NativeKind)
	}
}

func TestMergeSymbolSets_SecondaryMatch_PrefetchCandidate(t *testing.T) {
	// Vector 14: GitNexus line 1041, CodeGraph line 1042, different initial keys
	a := []SymbolRef{
		{
			Key:        "method:src/commands.ts:RuntimeWorktreeCreationCommands.prefetchManagedWorktreeCreateBase#1",
			Kind:       SymbolKindMethod,
			NativeKind: "Method",
			Name:       "prefetchManagedWorktreeCreateBase",
			FilePath:   "src/commands.ts",
			StartLine:  1041,
			EndLine:    1050,
			GitNexusID: "gn-prefetch",
		},
	}
	b := []SymbolRef{
		{
			Key:         "function:src/commands.ts:RuntimeWorktreeCreationCommands.prefetchManagedWorktreeCreateBase",
			Kind:        SymbolKindFunction,
			NativeKind:  "function",
			Name:        "prefetchManagedWorktreeCreateBase",
			FilePath:    "src/commands.ts",
			StartLine:   1042,
			EndLine:     1052,
			CodeGraphID: "cg-prefetch",
			Signature:   "() => Promise<void>",
		},
	}

	merged, report := MergeSymbolSets(a, b)
	if report.SecondaryMatches != 1 {
		t.Fatalf("expected 1 secondary match, got %d", report.SecondaryMatches)
	}
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged node, got %d", len(merged))
	}
	if merged[0].GitNexusID != "gn-prefetch" || merged[0].CodeGraphID != "cg-prefetch" {
		t.Errorf("expected both IDs, got gn=%s, cg=%s", merged[0].GitNexusID, merged[0].CodeGraphID)
	}
	if merged[0].StartLine != 1041 {
		t.Errorf("expected startLine 1041, got %d", merged[0].StartLine)
	}
}

func TestMergeSymbolSets_NoSecondaryMatchWhenMultipleCandidates(t *testing.T) {
	// Two candidates with same name in same file on side A -> no secondary match!
	a := []SymbolRef{
		{
			Key:        "method:src/calc.ts:Calc.add#1",
			Kind:       SymbolKindMethod,
			Name:       "add",
			FilePath:   "src/calc.ts",
			StartLine:  10,
			GitNexusID: "gn-add-1",
		},
		{
			Key:        "method:src/calc.ts:Calc.add#2",
			Kind:       SymbolKindMethod,
			Name:       "add",
			FilePath:   "src/calc.ts",
			StartLine:  12,
			GitNexusID: "gn-add-2",
		},
	}
	b := []SymbolRef{
		{
			Key:         "function:src/calc.ts:add",
			Kind:        SymbolKindFunction,
			Name:        "add",
			FilePath:    "src/calc.ts",
			StartLine:   11,
			CodeGraphID: "cg-add",
		},
	}

	merged, report := MergeSymbolSets(a, b)
	if report.SecondaryMatches != 0 {
		t.Fatalf("expected 0 secondary matches when side A has 2 candidates, got %d", report.SecondaryMatches)
	}
	if report.Unmatched != 3 {
		t.Fatalf("expected 3 unmatched nodes, got %d", report.Unmatched)
	}
	if len(merged) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(merged))
	}
}

func TestMergeSymbolSets_DisagreementOnLineDiffGreaterThan2(t *testing.T) {
	a := []SymbolRef{
		{
			Key:        "function:src/util.ts:formatDate",
			Kind:       SymbolKindFunction,
			Name:       "formatDate",
			FilePath:   "src/util.ts",
			StartLine:  10,
			GitNexusID: "gn-1",
		},
	}
	b := []SymbolRef{
		{
			Key:         "function:src/util.ts:formatDate",
			Kind:        SymbolKindFunction,
			Name:        "formatDate",
			FilePath:    "src/util.ts",
			StartLine:   14, // diff = 4 > 2
			CodeGraphID: "cg-1",
		},
	}

	_, report := MergeSymbolSets(a, b)
	if len(report.Disagreements) != 1 {
		t.Fatalf("expected 1 disagreement for line diff > 2, got %d", len(report.Disagreements))
	}
}

func TestMergeSymbolSets_Commutative(t *testing.T) {
	a := []SymbolRef{
		{
			Key:        "function:src/a.ts:foo",
			Kind:       SymbolKindFunction,
			NativeKind: "Function",
			Name:       "foo",
			FilePath:   "src/a.ts",
			StartLine:  10,
			GitNexusID: "gn-foo",
		},
		{
			Key:        "function:src/b.ts:bar",
			Kind:       SymbolKindFunction,
			Name:       "bar",
			FilePath:   "src/b.ts",
			StartLine:  50,
			GitNexusID: "gn-bar",
		},
	}
	b := []SymbolRef{
		{
			Key:         "function:src/a.ts:foo",
			Kind:        SymbolKindFunction,
			NativeKind:  "function",
			Name:        "foo",
			FilePath:    "src/a.ts",
			StartLine:   11,
			CodeGraphID: "cg-foo",
			Signature:   "() => void",
		},
		{
			Key:         "function:src/c.ts:baz",
			Kind:        SymbolKindFunction,
			Name:        "baz",
			FilePath:    "src/c.ts",
			StartLine:   100,
			CodeGraphID: "cg-baz",
		},
	}

	mergedAB, reportAB := MergeSymbolSets(a, b)
	mergedBA, reportBA := MergeSymbolSets(b, a)

	if !reflect.DeepEqual(mergedAB, mergedBA) {
		t.Errorf("merging is not commutative:\nAB: %+v\nBA: %+v", mergedAB, mergedBA)
	}
	if reportAB.PrimaryMatches != reportBA.PrimaryMatches ||
		reportAB.SecondaryMatches != reportBA.SecondaryMatches ||
		reportAB.Unmatched != reportBA.Unmatched {
		t.Errorf("reports differ: AB=%+v, BA=%+v", reportAB, reportBA)
	}
}

func TestMergeRoutes(t *testing.T) {
	a := []SymbolRef{
		{
			Key:        "route:server/routes.go:/api/v1/health",
			Kind:       SymbolKindRoute,
			GitNexusID: "gn-route-1",
			StartLine:  10,
		},
	}
	b := []SymbolRef{
		{
			Key:         "route:server/routes.go:/api/v1/health",
			Kind:        SymbolKindRoute,
			CodeGraphID: "cg-route-1",
			StartLine:   10,
		},
		{
			Key:         "route:server/routes.go:POST:/api/v1/login",
			Kind:        SymbolKindRoute,
			CodeGraphID: "cg-route-2",
			StartLine:   25,
		},
	}

	routes := MergeRoutes(a, b)
	if len(routes) != 3 {
		t.Fatalf("expected 3 separate routes (routes do not merge across sources), got %d", len(routes))
	}
}

func TestMergeEdges(t *testing.T) {
	a := []SymbolEdge{
		{
			FromKey:    "function:src/a.ts:foo",
			ToKey:      "function:src/b.ts:bar",
			Kind:       EdgeKindCalls,
			Sources:    []string{"gitnexus"},
			Confidence: 0.8,
		},
	}
	b := []SymbolEdge{
		{
			FromKey:    "function:src/a.ts:foo",
			ToKey:      "function:src/b.ts:bar",
			Kind:       EdgeKindCalls,
			Sources:    []string{"codegraph"},
			Confidence: 0.95,
		},
		{
			FromKey:    "function:src/b.ts:bar",
			ToKey:      "function:src/c.ts:baz",
			Kind:       EdgeKindReferences,
			Sources:    []string{"codegraph"},
			Confidence: 0.7,
		},
	}

	edges := MergeEdges(a, b)
	if len(edges) != 2 {
		t.Fatalf("expected 2 merged edges, got %d", len(edges))
	}

	callEdge := edges[0]
	if callEdge.Confidence != 0.95 {
		t.Errorf("expected max confidence 0.95, got %f", callEdge.Confidence)
	}
	if len(callEdge.Sources) != 2 || callEdge.Sources[0] != "codegraph" || callEdge.Sources[1] != "gitnexus" {
		t.Errorf("expected unioned sources [codegraph, gitnexus], got %v", callEdge.Sources)
	}
}

func BenchmarkMergeSymbolSets_1500Nodes(b *testing.B) {
	const n = 1500
	listA := make([]SymbolRef, n)
	listB := make([]SymbolRef, n)

	for i := 0; i < n; i++ {
		listA[i] = SymbolRef{
			Key:        fmt.Sprintf("function:src/mod%d.ts:fn%d", i/10, i),
			Kind:       SymbolKindFunction,
			Name:       fmt.Sprintf("fn%d", i),
			FilePath:   fmt.Sprintf("src/mod%d.ts", i/10),
			StartLine:  int32(i * 10),
			GitNexusID: fmt.Sprintf("gn-%d", i),
		}
		listB[i] = SymbolRef{
			Key:         fmt.Sprintf("function:src/mod%d.ts:fn%d", i/10, i),
			Kind:        SymbolKindFunction,
			Name:        fmt.Sprintf("fn%d", i),
			FilePath:    fmt.Sprintf("src/mod%d.ts", i/10),
			StartLine:   int32(i*10 + 1),
			CodeGraphID: fmt.Sprintf("cg-%d", i),
			Signature:   "() => void",
			Language:    "typescript",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = MergeSymbolSets(listA, listB)
	}
}
