package grpc

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

func TestGraphProtoMapping_SymbolRef_RoundTrip(t *testing.T) {
	orig := domain.SymbolRef{
		Key:           "method:src/calc.ts:Calc.add",
		Kind:          domain.SymbolKindMethod,
		NativeKind:    "Method",
		Name:          "add",
		QualifiedName: "Calc.add",
		FilePath:      "src/calc.ts",
		StartLine:     15,
		EndLine:       25,
		GitNexusID:    "gn-123",
		CodeGraphID:   "cg-456",
		Language:      "typescript",
	}

	p := ToProtoSymbolRef(orig)
	back := FromProtoSymbolRef(p)
	p2 := ToProtoSymbolRef(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for SymbolRef:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_SymbolEdge_RoundTrip(t *testing.T) {
	orig := domain.SymbolEdge{
		FromKey:    "function:src/a.ts:foo",
		ToKey:      "function:src/b.ts:bar",
		Kind:       domain.EdgeKindCalls,
		Sources:    []string{"gitnexus", "codegraph"},
		Confidence: 0.95,
	}

	p := ToProtoSymbolEdge(orig)
	back := FromProtoSymbolEdge(p)
	p2 := ToProtoSymbolEdge(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for SymbolEdge:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_ResultMeta_RoundTrip(t *testing.T) {
	orig := domain.ResultMeta{
		Repo:        "orca",
		WorktreeRef: "wt-main",
		Commit:      "c0ffee123",
		View:        domain.ViewKindArchitecture,
		Sources: []domain.SourceInfo{
			{
				Tool:      "gitnexus",
				Version:   "1.2.0",
				IndexedAt: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
				Commit:    "c0ffee123",
				LineBase:  1,
			},
		},
		Cached:      true,
		ETag:        "\"etag-123\"",
		GeneratedAt: time.Date(2026, 10, 5, 12, 5, 0, 0, time.UTC),
		Truncated:   true,
		TotalBefore: 5000,
		NotModified: false,
	}

	p := ToProtoResultMeta(orig)
	back := FromProtoResultMeta(p)
	p2 := ToProtoResultMeta(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for ResultMeta:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_ArchitectureGraph_RoundTrip(t *testing.T) {
	orig := domain.ArchitectureGraph{
		Nodes: []domain.ClusterNode{
			{
				ID:               "c1",
				Label:            "Auth",
				SymbolCount:      42,
				Cohesion:         0.88,
				Keywords:         []string{"login", "token"},
				TopFiles:         []string{"src/auth.ts"},
				DominantLanguage: "typescript",
				Area:             "core",
			},
		},
		Edges: []domain.ClusterEdge{
			{
				From:       "c1",
				To:         "c2",
				Weight:     10,
				KindCounts: map[string]int32{"CALLS": 5},
			},
		},
	}

	p := ToProtoArchitectureGraph(orig)
	back := FromProtoArchitectureGraph(p)
	p2 := ToProtoArchitectureGraph(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for ArchitectureGraph:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_ModuleGraph_RoundTrip(t *testing.T) {
	orig := domain.ModuleGraph{
		Nodes: []domain.ModuleNode{
			{
				ID:          "m1",
				Language:    "go",
				SymbolCount: 15,
				Loc:         300,
				Cluster:     "core",
				Area:        "backend",
			},
		},
		Edges: []domain.ModuleEdge{
			{
				From:  "m1",
				To:    "m2",
				Count: 3,
			},
		},
	}

	p := ToProtoModuleGraph(orig)
	back := FromProtoModuleGraph(p)
	p2 := ToProtoModuleGraph(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for ModuleGraph:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_SymbolGraph_RoundTrip(t *testing.T) {
	orig := domain.SymbolGraph{
		Center: domain.SymbolRef{Key: "fn:main", Name: "main"},
		Nodes: []domain.SymbolNode{
			{
				Ref:        domain.SymbolRef{Key: "fn:main", Name: "main"},
				IsExported: true,
				Signature:  "() => void",
				Cluster:    "core",
			},
		},
		Edges: []domain.SymbolEdge{
			{
				FromKey:    "fn:main",
				ToKey:      "fn:sub",
				Kind:       domain.EdgeKindCalls,
				Confidence: 1.0,
			},
		},
		Depth: 1,
	}

	p := ToProtoSymbolGraph(orig)
	back := FromProtoSymbolGraph(p)
	p2 := ToProtoSymbolGraph(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for SymbolGraph:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_FlowGraph_RoundTrip(t *testing.T) {
	orig := domain.FlowGraph{
		Flow: domain.FlowSummary{
			ID:          "flow-1",
			Label:       "LoginFlow",
			ProcessType: "auth",
			StepCount:   2,
			Communities: []string{"comm-1"},
			Entry:       "fn:entry",
			Terminal:    "fn:terminal",
		},
		Steps: []domain.FlowStep{
			{
				Step:      1,
				Symbol:    domain.SymbolRef{Key: "fn:entry"},
				Cluster:   "auth",
				FilePath:  "src/auth.ts",
				StartLine: 10,
			},
		},
		Edges: []domain.SymbolEdge{},
	}

	p := ToProtoFlowGraph(orig)
	back := FromProtoFlowGraph(p)
	p2 := ToProtoFlowGraph(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for FlowGraph:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_ImpactGraph_RoundTrip(t *testing.T) {
	step := int32(2)
	orig := domain.ImpactGraph{
		Target:    domain.SymbolRef{Key: "fn:target"},
		Direction: "upstream",
		Risk:      domain.RiskHigh,
		Levels: []domain.ImpactLevel{
			{
				Depth: 1,
				Symbols: []domain.ImpactSymbol{
					{
						Symbol:     domain.SymbolRef{Key: "fn:caller"},
						Via:        domain.EdgeKindCalls,
						Direct:     true,
						Confidence: 0.9,
					},
				},
			},
		},
		AffectedFlows: []domain.AffectedFlow{
			{
				FlowID:      "fl-1",
				Label:       "Flow1",
				StepCount:   5,
				ChangedStep: &step,
			},
		},
		AffectedClusters: []domain.AffectedCluster{
			{
				ID:     "cl-1",
				Label:  "Cluster1",
				Hits:   2,
				Impact: "0.75",
			},
		},
		TestsCovering: []string{"test_target.ts"},
		ImpactedCount: 1,
	}

	p := ToProtoImpactGraph(orig)
	back := FromProtoImpactGraph(p)
	p2 := ToProtoImpactGraph(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for ImpactGraph:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_RouteMap_RoundTrip(t *testing.T) {
	orig := domain.RouteMap{
		Routes: []domain.RouteNode{
			{
				ID:           "r-1",
				Path:         "/api/v1/auth",
				Method:       "POST",
				FilePath:     "server/auth.go",
				Middleware:   []string{"logging", "cors"},
				ResponseKeys: []string{"AuthResponse"},
				ErrorKeys:    []string{"AuthError"},
				Side:         "server",
			},
		},
		Edges: []domain.RouteEdge{
			{
				RouteID: "r-1",
				Handler: "fn:AuthHandler",
				Kind:    "HANDLES",
			},
		},
	}

	p := ToProtoRouteMap(orig)
	back := FromProtoRouteMap(p)
	p2 := ToProtoRouteMap(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for RouteMap:\np1: %+v\np2: %+v", p, p2)
	}
}

func TestGraphProtoMapping_SymbolDetail_RoundTrip(t *testing.T) {
	orig := domain.SymbolDetail{
		Symbol: domain.SymbolRef{Key: "fn:calc"},
		Incoming: map[string]domain.RelatedSymbolList{
			"CALLS": {
				Symbols: []domain.RelatedSymbol{
					{
						Ref:  domain.SymbolRef{Key: "fn:main"},
						Kind: domain.EdgeKindCalls,
					},
				},
			},
		},
		Outgoing: map[string]domain.RelatedSymbolList{},
		Flows: []domain.SymbolFlowRef{
			{
				FlowID:    "flow-1",
				FlowLabel: "Flow1",
				Step:      1,
			},
		},
		Source: domain.SymbolSource{
			Text:      "func calc() int { return 42 }",
			StartLine: 10,
			EndLine:   12,
			Truncated: false,
		},
		SourceOmitted: "",
	}

	p := ToProtoSymbolDetail(orig)
	back := FromProtoSymbolDetail(p)
	p2 := ToProtoSymbolDetail(back)

	if !proto.Equal(p, p2) {
		t.Errorf("round trip failed for SymbolDetail:\np1: %+v\np2: %+v", p, p2)
	}
}
