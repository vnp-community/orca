package grpc

import (
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

// --- Enum Mappings ---

func ToProtoSymbolKind(k domain.SymbolKind) codeintelv1.SymbolKind {
	switch k {
	case domain.SymbolKindFunction:
		return codeintelv1.SymbolKind_SYMBOL_KIND_FUNCTION
	case domain.SymbolKindMethod:
		return codeintelv1.SymbolKind_SYMBOL_KIND_METHOD
	case domain.SymbolKindType:
		return codeintelv1.SymbolKind_SYMBOL_KIND_TYPE
	case domain.SymbolKindValue:
		return codeintelv1.SymbolKind_SYMBOL_KIND_VALUE
	case domain.SymbolKindFile:
		return codeintelv1.SymbolKind_SYMBOL_KIND_FILE
	case domain.SymbolKindFolder:
		return codeintelv1.SymbolKind_SYMBOL_KIND_FOLDER
	case domain.SymbolKindRoute:
		return codeintelv1.SymbolKind_SYMBOL_KIND_ROUTE
	case domain.SymbolKindComponent:
		return codeintelv1.SymbolKind_SYMBOL_KIND_COMPONENT
	case domain.SymbolKindNamespace:
		return codeintelv1.SymbolKind_SYMBOL_KIND_NAMESPACE
	case domain.SymbolKindImport:
		return codeintelv1.SymbolKind_SYMBOL_KIND_IMPORT
	case domain.SymbolKindCluster:
		return codeintelv1.SymbolKind_SYMBOL_KIND_CLUSTER
	case domain.SymbolKindFlow:
		return codeintelv1.SymbolKind_SYMBOL_KIND_FLOW
	case domain.SymbolKindDoc:
		return codeintelv1.SymbolKind_SYMBOL_KIND_DOC
	default:
		return codeintelv1.SymbolKind_SYMBOL_KIND_UNSPECIFIED
	}
}

func FromProtoSymbolKind(k codeintelv1.SymbolKind) domain.SymbolKind {
	switch k {
	case codeintelv1.SymbolKind_SYMBOL_KIND_FUNCTION:
		return domain.SymbolKindFunction
	case codeintelv1.SymbolKind_SYMBOL_KIND_METHOD:
		return domain.SymbolKindMethod
	case codeintelv1.SymbolKind_SYMBOL_KIND_TYPE:
		return domain.SymbolKindType
	case codeintelv1.SymbolKind_SYMBOL_KIND_VALUE:
		return domain.SymbolKindValue
	case codeintelv1.SymbolKind_SYMBOL_KIND_FILE:
		return domain.SymbolKindFile
	case codeintelv1.SymbolKind_SYMBOL_KIND_FOLDER:
		return domain.SymbolKindFolder
	case codeintelv1.SymbolKind_SYMBOL_KIND_ROUTE:
		return domain.SymbolKindRoute
	case codeintelv1.SymbolKind_SYMBOL_KIND_COMPONENT:
		return domain.SymbolKindComponent
	case codeintelv1.SymbolKind_SYMBOL_KIND_NAMESPACE:
		return domain.SymbolKindNamespace
	case codeintelv1.SymbolKind_SYMBOL_KIND_IMPORT:
		return domain.SymbolKindImport
	case codeintelv1.SymbolKind_SYMBOL_KIND_CLUSTER:
		return domain.SymbolKindCluster
	case codeintelv1.SymbolKind_SYMBOL_KIND_FLOW:
		return domain.SymbolKindFlow
	case codeintelv1.SymbolKind_SYMBOL_KIND_DOC:
		return domain.SymbolKindDoc
	default:
		return domain.SymbolKindUnspecified
	}
}

func ToProtoEdgeKind(k domain.EdgeKind) codeintelv1.EdgeKind {
	switch k {
	case domain.EdgeKindCalls:
		return codeintelv1.EdgeKind_EDGE_KIND_CALLS
	case domain.EdgeKindImports:
		return codeintelv1.EdgeKind_EDGE_KIND_IMPORTS
	case domain.EdgeKindAccesses:
		return codeintelv1.EdgeKind_EDGE_KIND_ACCESSES
	case domain.EdgeKindExtends:
		return codeintelv1.EdgeKind_EDGE_KIND_EXTENDS
	case domain.EdgeKindImplements:
		return codeintelv1.EdgeKind_EDGE_KIND_IMPLEMENTS
	case domain.EdgeKindMethodOverrides:
		return codeintelv1.EdgeKind_EDGE_KIND_METHOD_OVERRIDES
	case domain.EdgeKindMethodImplements:
		return codeintelv1.EdgeKind_EDGE_KIND_METHOD_IMPLEMENTS
	case domain.EdgeKindHasMethod:
		return codeintelv1.EdgeKind_EDGE_KIND_HAS_METHOD
	case domain.EdgeKindHasProperty:
		return codeintelv1.EdgeKind_EDGE_KIND_HAS_PROPERTY
	case domain.EdgeKindReferences:
		return codeintelv1.EdgeKind_EDGE_KIND_REFERENCES
	case domain.EdgeKindInstantiates:
		return codeintelv1.EdgeKind_EDGE_KIND_INSTANTIATES
	case domain.EdgeKindContains:
		return codeintelv1.EdgeKind_EDGE_KIND_CONTAINS
	case domain.EdgeKindHandlesRoute:
		return codeintelv1.EdgeKind_EDGE_KIND_HANDLES_ROUTE
	case domain.EdgeKindFetches:
		return codeintelv1.EdgeKind_EDGE_KIND_FETCHES
	default:
		return codeintelv1.EdgeKind_EDGE_KIND_UNSPECIFIED
	}
}

func FromProtoEdgeKind(k codeintelv1.EdgeKind) domain.EdgeKind {
	switch k {
	case codeintelv1.EdgeKind_EDGE_KIND_CALLS:
		return domain.EdgeKindCalls
	case codeintelv1.EdgeKind_EDGE_KIND_IMPORTS:
		return domain.EdgeKindImports
	case codeintelv1.EdgeKind_EDGE_KIND_ACCESSES:
		return domain.EdgeKindAccesses
	case codeintelv1.EdgeKind_EDGE_KIND_EXTENDS:
		return domain.EdgeKindExtends
	case codeintelv1.EdgeKind_EDGE_KIND_IMPLEMENTS:
		return domain.EdgeKindImplements
	case codeintelv1.EdgeKind_EDGE_KIND_METHOD_OVERRIDES:
		return domain.EdgeKindMethodOverrides
	case codeintelv1.EdgeKind_EDGE_KIND_METHOD_IMPLEMENTS:
		return domain.EdgeKindMethodImplements
	case codeintelv1.EdgeKind_EDGE_KIND_HAS_METHOD:
		return domain.EdgeKindHasMethod
	case codeintelv1.EdgeKind_EDGE_KIND_HAS_PROPERTY:
		return domain.EdgeKindHasProperty
	case codeintelv1.EdgeKind_EDGE_KIND_REFERENCES:
		return domain.EdgeKindReferences
	case codeintelv1.EdgeKind_EDGE_KIND_INSTANTIATES:
		return domain.EdgeKindInstantiates
	case codeintelv1.EdgeKind_EDGE_KIND_CONTAINS:
		return domain.EdgeKindContains
	case codeintelv1.EdgeKind_EDGE_KIND_HANDLES_ROUTE:
		return domain.EdgeKindHandlesRoute
	case codeintelv1.EdgeKind_EDGE_KIND_FETCHES:
		return domain.EdgeKindFetches
	default:
		return domain.EdgeKindUnspecified
	}
}

func ToProtoViewKind(v domain.ViewKind) codeintelv1.ViewKind {
	return codeintelv1.ViewKind(v)
}

func FromProtoViewKind(v codeintelv1.ViewKind) domain.ViewKind {
	return domain.ViewKind(v)
}

func ToProtoRisk(r domain.Risk) codeintelv1.Risk {
	switch r {
	case domain.RiskLow:
		return codeintelv1.Risk_RISK_LOW
	case domain.RiskMedium:
		return codeintelv1.Risk_RISK_MEDIUM
	case domain.RiskHigh:
		return codeintelv1.Risk_RISK_HIGH
	case domain.RiskCritical:
		return codeintelv1.Risk_RISK_CRITICAL
	case domain.RiskUnknown:
		return codeintelv1.Risk_RISK_UNKNOWN
	default:
		return codeintelv1.Risk_RISK_UNSPECIFIED
	}
}

func FromProtoRisk(r codeintelv1.Risk) domain.Risk {
	switch r {
	case codeintelv1.Risk_RISK_LOW:
		return domain.RiskLow
	case codeintelv1.Risk_RISK_MEDIUM:
		return domain.RiskMedium
	case codeintelv1.Risk_RISK_HIGH:
		return domain.RiskHigh
	case codeintelv1.Risk_RISK_CRITICAL:
		return domain.RiskCritical
	case codeintelv1.Risk_RISK_UNKNOWN:
		return domain.RiskUnknown
	default:
		return domain.RiskUnspecified
	}
}

func toProtoToolName(s string) codeintelv1.ToolName {
	switch strings.ToLower(s) {
	case "gitnexus":
		return codeintelv1.ToolName_TOOL_NAME_GITNEXUS
	case "codegraph":
		return codeintelv1.ToolName_TOOL_NAME_CODEGRAPH
	default:
		return codeintelv1.ToolName_TOOL_NAME_UNSPECIFIED
	}
}

func fromProtoToolName(t codeintelv1.ToolName) string {
	switch t {
	case codeintelv1.ToolName_TOOL_NAME_GITNEXUS:
		return "gitnexus"
	case codeintelv1.ToolName_TOOL_NAME_CODEGRAPH:
		return "codegraph"
	default:
		return ""
	}
}

// --- SymbolRef & SymbolEdge ---

func ToProtoSymbolRef(s domain.SymbolRef) *codeintelv1.SymbolRef {
	return &codeintelv1.SymbolRef{
		Key:           s.Key,
		Kind:          ToProtoSymbolKind(s.Kind),
		NativeKind:    s.NativeKind,
		Name:          s.Name,
		QualifiedName: s.QualifiedName,
		FilePath:      s.FilePath,
		StartLine:     s.StartLine,
		EndLine:       s.EndLine,
		GitnexusId:    s.GitNexusID,
		CodegraphId:   s.CodeGraphID,
		Language:      s.Language,
	}
}

func FromProtoSymbolRef(p *codeintelv1.SymbolRef) domain.SymbolRef {
	if p == nil {
		return domain.SymbolRef{}
	}
	return domain.SymbolRef{
		Key:           p.Key,
		Kind:          FromProtoSymbolKind(p.Kind),
		NativeKind:    p.NativeKind,
		Name:          p.Name,
		QualifiedName: p.QualifiedName,
		FilePath:      p.FilePath,
		StartLine:     p.StartLine,
		EndLine:       p.EndLine,
		GitNexusID:    p.GitnexusId,
		CodeGraphID:   p.CodegraphId,
		Language:      p.Language,
	}
}

func ToProtoSymbolEdge(e domain.SymbolEdge) *codeintelv1.SymbolEdge {
	sources := make([]codeintelv1.ToolName, 0, len(e.Sources))
	for _, s := range e.Sources {
		sources = append(sources, toProtoToolName(s))
	}
	return &codeintelv1.SymbolEdge{
		FromKey:    e.FromKey,
		ToKey:      e.ToKey,
		Kind:       ToProtoEdgeKind(e.Kind),
		Confidence: e.Confidence,
		Sources:    sources,
	}
}

func FromProtoSymbolEdge(p *codeintelv1.SymbolEdge) domain.SymbolEdge {
	if p == nil {
		return domain.SymbolEdge{}
	}
	sources := make([]string, 0, len(p.Sources))
	for _, t := range p.Sources {
		s := fromProtoToolName(t)
		if s != "" {
			sources = append(sources, s)
		}
	}
	return domain.SymbolEdge{
		FromKey:    p.FromKey,
		ToKey:      p.ToKey,
		Kind:       FromProtoEdgeKind(p.Kind),
		Confidence: p.Confidence,
		Sources:    sources,
	}
}

// --- ResultMeta & SourceInfo ---

func ToProtoSourceInfo(s domain.SourceInfo) *codeintelv1.SourceInfo {
	return &codeintelv1.SourceInfo{
		Tool:      toProtoToolName(s.Tool),
		Version:   s.Version,
		IndexedAt: s.IndexedAt.Format(time.RFC3339),
		Commit:    s.Commit,
		LineBase:  int32(s.LineBase),
	}
}

func FromProtoSourceInfo(p *codeintelv1.SourceInfo) domain.SourceInfo {
	if p == nil {
		return domain.SourceInfo{}
	}
	t, _ := time.Parse(time.RFC3339, p.IndexedAt)
	return domain.SourceInfo{
		Tool:      fromProtoToolName(p.Tool),
		Version:   p.Version,
		IndexedAt: t,
		Commit:    p.Commit,
		LineBase:  int(p.LineBase),
	}
}

func ToProtoResultMeta(m domain.ResultMeta) *codeintelv1.ResultMeta {
	sources := make([]*codeintelv1.SourceInfo, 0, len(m.Sources))
	for _, s := range m.Sources {
		sources = append(sources, ToProtoSourceInfo(s))
	}

	var genAt *timestamppb.Timestamp
	if !m.GeneratedAt.IsZero() {
		genAt = timestamppb.New(m.GeneratedAt)
	}

	tc := m.TotalCount
	if tc == 0 && m.TotalBefore > 0 {
		tc = m.TotalBefore
	}

	return &codeintelv1.ResultMeta{
		Repo:        m.Repo,
		WorktreeId:  m.WorktreeRef,
		DevServerId: m.DevServerID,
		View:        ToProtoViewKind(m.View),
		Sources:     sources,
		HeadCommit:  m.Commit,
		Stale:       m.Stale,
		Truncated:   m.Truncated,
		TotalCount:  tc,
		Etag:        m.ETag,
		GeneratedAt: genAt,
		FromCache:   m.Cached,
		NotModified: m.NotModified,
	}
}

func FromProtoResultMeta(p *codeintelv1.ResultMeta) domain.ResultMeta {
	if p == nil {
		return domain.ResultMeta{}
	}
	sources := make([]domain.SourceInfo, 0, len(p.Sources))
	for _, s := range p.Sources {
		sources = append(sources, FromProtoSourceInfo(s))
	}

	var genAt time.Time
	if p.GeneratedAt != nil {
		genAt = p.GeneratedAt.AsTime()
	}

	return domain.ResultMeta{
		Repo:        p.Repo,
		WorktreeRef: p.WorktreeId,
		DevServerID: p.DevServerId,
		View:        FromProtoViewKind(p.View),
		Sources:     sources,
		Commit:      p.HeadCommit,
		Stale:       p.Stale,
		Truncated:   p.Truncated,
		TotalCount:  p.TotalCount,
		TotalBefore: p.TotalCount,
		ETag:        p.Etag,
		GeneratedAt: genAt,
		Cached:      p.FromCache,
		NotModified: p.NotModified,
	}
}


// --- ArchitectureGraph ---

func ToProtoArchitectureGraph(g domain.ArchitectureGraph) *codeintelv1.ArchitectureGraph {
	nodes := make([]*codeintelv1.ClusterNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, &codeintelv1.ClusterNode{
			Id:               n.ID,
			Label:            n.Label,
			SymbolCount:      n.SymbolCount,
			Cohesion:         n.Cohesion,
			Keywords:         n.Keywords,
			TopFiles:         n.TopFiles,
			DominantLanguage: n.DominantLanguage,
			Area:             n.Area,
		})
	}

	edges := make([]*codeintelv1.ClusterEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		edges = append(edges, &codeintelv1.ClusterEdge{
			From:       e.From,
			To:         e.To,
			Weight:     int32(e.Weight),
			KindCounts: e.KindCounts,
		})
	}

	return &codeintelv1.ArchitectureGraph{
		Nodes: nodes,
		Edges: edges,
	}
}

func FromProtoArchitectureGraph(p *codeintelv1.ArchitectureGraph) domain.ArchitectureGraph {
	if p == nil {
		return domain.ArchitectureGraph{}
	}
	nodes := make([]domain.ClusterNode, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		nodes = append(nodes, domain.ClusterNode{
			ID:               n.Id,
			Label:            n.Label,
			SymbolCount:      n.SymbolCount,
			Cohesion:         n.Cohesion,
			Keywords:         n.Keywords,
			TopFiles:         n.TopFiles,
			DominantLanguage: n.DominantLanguage,
			Area:             n.Area,
		})
	}
	edges := make([]domain.ClusterEdge, 0, len(p.Edges))
	for _, e := range p.Edges {
		edges = append(edges, domain.ClusterEdge{
			From:       e.From,
			To:         e.To,
			Weight:     float64(e.Weight),
			KindCounts: e.KindCounts,
		})
	}
	return domain.ArchitectureGraph{Nodes: nodes, Edges: edges}
}

// --- ModuleGraph ---

func ToProtoModuleGraph(g domain.ModuleGraph) *codeintelv1.ModuleGraph {
	nodes := make([]*codeintelv1.ModuleNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, &codeintelv1.ModuleNode{
			Id:          n.ID,
			Language:    n.Language,
			SymbolCount: n.SymbolCount,
			Loc:         n.Loc,
			Cluster:     n.Cluster,
			Area:        n.Area,
		})
	}
	edges := make([]*codeintelv1.ModuleEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		edges = append(edges, &codeintelv1.ModuleEdge{
			From:  e.From,
			To:    e.To,
			Count: e.Count,
		})
	}
	return &codeintelv1.ModuleGraph{Nodes: nodes, Edges: edges}
}

func FromProtoModuleGraph(p *codeintelv1.ModuleGraph) domain.ModuleGraph {
	if p == nil {
		return domain.ModuleGraph{}
	}
	nodes := make([]domain.ModuleNode, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		nodes = append(nodes, domain.ModuleNode{
			ID:          n.Id,
			Language:    n.Language,
			SymbolCount: n.SymbolCount,
			Loc:         n.Loc,
			Cluster:     n.Cluster,
			Area:        n.Area,
		})
	}
	edges := make([]domain.ModuleEdge, 0, len(p.Edges))
	for _, e := range p.Edges {
		edges = append(edges, domain.ModuleEdge{
			From:  e.From,
			To:    e.To,
			Count: e.Count,
		})
	}
	return domain.ModuleGraph{Nodes: nodes, Edges: edges}
}

// --- SymbolGraph ---

func ToProtoSymbolGraph(g domain.SymbolGraph) *codeintelv1.SymbolGraph {
	nodes := make([]*codeintelv1.SymbolNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, &codeintelv1.SymbolNode{
			Ref:        ToProtoSymbolRef(n.Ref),
			IsExported: n.IsExported,
			Signature:  n.Signature,
			Cluster:    n.Cluster,
		})
	}
	edges := make([]*codeintelv1.SymbolEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		edges = append(edges, ToProtoSymbolEdge(e))
	}
	return &codeintelv1.SymbolGraph{
		Center: ToProtoSymbolRef(g.Center),
		Nodes:  nodes,
		Edges:  edges,
		Depth:  g.Depth,
	}
}

func FromProtoSymbolGraph(p *codeintelv1.SymbolGraph) domain.SymbolGraph {
	if p == nil {
		return domain.SymbolGraph{}
	}
	nodes := make([]domain.SymbolNode, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		nodes = append(nodes, domain.SymbolNode{
			Ref:        FromProtoSymbolRef(n.Ref),
			IsExported: n.IsExported,
			Signature:  n.Signature,
			Cluster:    n.Cluster,
		})
	}
	edges := make([]domain.SymbolEdge, 0, len(p.Edges))
	for _, e := range p.Edges {
		edges = append(edges, FromProtoSymbolEdge(e))
	}
	return domain.SymbolGraph{
		Center: FromProtoSymbolRef(p.Center),
		Nodes:  nodes,
		Edges:  edges,
		Depth:  p.Depth,
	}
}

// --- FlowGraph ---

func ToProtoFlowGraph(g domain.FlowGraph) *codeintelv1.FlowGraph {
	steps := make([]*codeintelv1.FlowStep, 0, len(g.Steps))
	for _, s := range g.Steps {
		steps = append(steps, &codeintelv1.FlowStep{
			Step:      s.Step,
			Symbol:    ToProtoSymbolRef(s.Symbol),
			Cluster:   s.Cluster,
			FilePath:  s.FilePath,
			StartLine: s.StartLine,
		})
	}
	edges := make([]*codeintelv1.SymbolEdge, 0, len(g.Edges))
	for _, e := range g.Edges {
		edges = append(edges, ToProtoSymbolEdge(e))
	}
	return &codeintelv1.FlowGraph{
		Flow: &codeintelv1.FlowSummary{
			Id:          g.Flow.ID,
			Label:       g.Flow.Label,
			ProcessType: g.Flow.ProcessType,
			StepCount:   g.Flow.StepCount,
			Communities: g.Flow.Communities,
			Entry:       ToProtoSymbolRef(domain.SymbolRef{Key: g.Flow.Entry}),
			Terminal:    ToProtoSymbolRef(domain.SymbolRef{Key: g.Flow.Terminal}),
		},
		Steps: steps,
		Edges: edges,
	}
}

func FromProtoFlowGraph(p *codeintelv1.FlowGraph) domain.FlowGraph {
	if p == nil {
		return domain.FlowGraph{}
	}
	steps := make([]domain.FlowStep, 0, len(p.Steps))
	for _, s := range p.Steps {
		steps = append(steps, domain.FlowStep{
			Step:      s.Step,
			Symbol:    FromProtoSymbolRef(s.Symbol),
			Cluster:   s.Cluster,
			FilePath:  s.FilePath,
			StartLine: s.StartLine,
		})
	}
	edges := make([]domain.SymbolEdge, 0, len(p.Edges))
	for _, e := range p.Edges {
		edges = append(edges, FromProtoSymbolEdge(e))
	}
	var flowSummary domain.FlowSummary
	if p.Flow != nil {
		flowSummary = domain.FlowSummary{
			ID:          p.Flow.Id,
			Label:       p.Flow.Label,
			ProcessType: p.Flow.ProcessType,
			StepCount:   p.Flow.StepCount,
			Communities: p.Flow.Communities,
		}
		if p.Flow.Entry != nil {
			flowSummary.Entry = p.Flow.Entry.Key
		}
		if p.Flow.Terminal != nil {
			flowSummary.Terminal = p.Flow.Terminal.Key
		}
	}
	return domain.FlowGraph{
		Flow:  flowSummary,
		Steps: steps,
		Edges: edges,
	}
}

// --- ImpactGraph ---

func ToProtoImpactGraph(g domain.ImpactGraph) *codeintelv1.ImpactGraph {
	levels := make([]*codeintelv1.ImpactLevel, 0, len(g.Levels))
	for _, l := range g.Levels {
		syms := make([]*codeintelv1.ImpactSymbol, 0, len(l.Symbols))
		for _, s := range l.Symbols {
			syms = append(syms, &codeintelv1.ImpactSymbol{
				Symbol:     ToProtoSymbolRef(s.Symbol),
				Via:        ToProtoEdgeKind(s.Via),
				Direct:     s.Direct,
				Confidence: s.Confidence,
			})
		}
		levels = append(levels, &codeintelv1.ImpactLevel{
			Depth:   l.Depth,
			Symbols: syms,
		})
	}

	flows := make([]*codeintelv1.AffectedFlow, 0, len(g.AffectedFlows))
	for _, f := range g.AffectedFlows {
		flows = append(flows, &codeintelv1.AffectedFlow{
			FlowId:      f.FlowID,
			Label:       f.Label,
			StepCount:   f.StepCount,
			ChangedStep: f.ChangedStep,
		})
	}

	clusters := make([]*codeintelv1.AffectedCluster, 0, len(g.AffectedClusters))
	for _, c := range g.AffectedClusters {
		var idPtr *string
		if c.ID != "" {
			idCopy := c.ID
			idPtr = &idCopy
		}
		clusters = append(clusters, &codeintelv1.AffectedCluster{
			Id:     idPtr,
			Label:  c.Label,
			Hits:   c.Hits,
			Impact: c.Impact,
		})
	}

	return &codeintelv1.ImpactGraph{
		Target:           ToProtoSymbolRef(g.Target),
		Direction:        g.Direction,
		Risk:             ToProtoRisk(g.Risk),
		Levels:           levels,
		AffectedFlows:    flows,
		AffectedClusters: clusters,
		TestsCovering:    g.TestsCovering,
		ImpactedCount:    g.ImpactedCount,
	}
}

func FromProtoImpactGraph(p *codeintelv1.ImpactGraph) domain.ImpactGraph {
	if p == nil {
		return domain.ImpactGraph{}
	}
	levels := make([]domain.ImpactLevel, 0, len(p.Levels))
	for _, l := range p.Levels {
		syms := make([]domain.ImpactSymbol, 0, len(l.Symbols))
		for _, s := range l.Symbols {
			syms = append(syms, domain.ImpactSymbol{
				Symbol:     FromProtoSymbolRef(s.Symbol),
				Via:        FromProtoEdgeKind(s.Via),
				Direct:     s.Direct,
				Confidence: s.Confidence,
			})
		}
		levels = append(levels, domain.ImpactLevel{
			Depth:   l.Depth,
			Symbols: syms,
		})
	}

	flows := make([]domain.AffectedFlow, 0, len(p.AffectedFlows))
	for _, f := range p.AffectedFlows {
		flows = append(flows, domain.AffectedFlow{
			FlowID:      f.FlowId,
			Label:       f.Label,
			StepCount:   f.StepCount,
			ChangedStep: f.ChangedStep,
		})
	}

	clusters := make([]domain.AffectedCluster, 0, len(p.AffectedClusters))
	for _, c := range p.AffectedClusters {
		var id string
		if c.Id != nil {
			id = *c.Id
		}
		clusters = append(clusters, domain.AffectedCluster{
			ID:     id,
			Label:  c.Label,
			Hits:   c.Hits,
			Impact: c.Impact,
		})
	}

	return domain.ImpactGraph{
		Target:           FromProtoSymbolRef(p.Target),
		Direction:        p.Direction,
		Risk:             FromProtoRisk(p.Risk),
		Levels:           levels,
		AffectedFlows:    flows,
		AffectedClusters: clusters,
		TestsCovering:    p.TestsCovering,
		ImpactedCount:    p.ImpactedCount,
	}
}

// --- RouteMap ---

func ToProtoRouteMap(m domain.RouteMap) *codeintelv1.RouteMap {
	routes := make([]*codeintelv1.RouteNode, 0, len(m.Routes))
	for _, r := range m.Routes {
		routes = append(routes, &codeintelv1.RouteNode{
			Id:           r.ID,
			Path:         r.Path,
			Method:       r.Method,
			FilePath:     r.FilePath,
			Middleware:   r.Middleware,
			ResponseKeys: r.ResponseKeys,
			ErrorKeys:    r.ErrorKeys,
			Side:         r.Side,
		})
	}
	edges := make([]*codeintelv1.RouteEdge, 0, len(m.Edges))
	for _, e := range m.Edges {
		edges = append(edges, &codeintelv1.RouteEdge{
			RouteId: e.RouteID,
			Handler: e.Handler,
			Kind:    e.Kind,
		})
	}
	return &codeintelv1.RouteMap{Routes: routes, Edges: edges}
}

func FromProtoRouteMap(p *codeintelv1.RouteMap) domain.RouteMap {
	if p == nil {
		return domain.RouteMap{}
	}
	routes := make([]domain.RouteNode, 0, len(p.Routes))
	for _, r := range p.Routes {
		routes = append(routes, domain.RouteNode{
			ID:           r.Id,
			Path:         r.Path,
			Method:       r.Method,
			FilePath:     r.FilePath,
			Middleware:   r.Middleware,
			ResponseKeys: r.ResponseKeys,
			ErrorKeys:    r.ErrorKeys,
			Side:         r.Side,
		})
	}
	edges := make([]domain.RouteEdge, 0, len(p.Edges))
	for _, e := range p.Edges {
		edges = append(edges, domain.RouteEdge{
			RouteID: e.RouteId,
			Handler: e.Handler,
			Kind:    e.Kind,
		})
	}
	return domain.RouteMap{Routes: routes, Edges: edges}
}

// --- SymbolDetail ---

func ToProtoSymbolDetail(d domain.SymbolDetail) *codeintelv1.SymbolDetail {
	incoming := make(map[string]*codeintelv1.RelatedSymbolList, len(d.Incoming))
	for k, v := range d.Incoming {
		syms := make([]*codeintelv1.RelatedSymbol, 0, len(v.Symbols))
		for _, s := range v.Symbols {
			syms = append(syms, &codeintelv1.RelatedSymbol{
				Ref:  ToProtoSymbolRef(s.Ref),
				Kind: ToProtoEdgeKind(s.Kind),
			})
		}
		incoming[k] = &codeintelv1.RelatedSymbolList{Symbols: syms}
	}

	outgoing := make(map[string]*codeintelv1.RelatedSymbolList, len(d.Outgoing))
	for k, v := range d.Outgoing {
		syms := make([]*codeintelv1.RelatedSymbol, 0, len(v.Symbols))
		for _, s := range v.Symbols {
			syms = append(syms, &codeintelv1.RelatedSymbol{
				Ref:  ToProtoSymbolRef(s.Ref),
				Kind: ToProtoEdgeKind(s.Kind),
			})
		}
		outgoing[k] = &codeintelv1.RelatedSymbolList{Symbols: syms}
	}

	flows := make([]*codeintelv1.SymbolFlowRef, 0, len(d.Flows))
	for _, f := range d.Flows {
		flows = append(flows, &codeintelv1.SymbolFlowRef{
			FlowId:    f.FlowID,
			FlowLabel: f.FlowLabel,
			Step:      f.Step,
		})
	}

	return &codeintelv1.SymbolDetail{
		Symbol:   ToProtoSymbolRef(d.Symbol),
		Incoming: incoming,
		Outgoing: outgoing,
		Flows:    flows,
		Source: &codeintelv1.SymbolSource{
			Text:      d.Source.Text,
			StartLine: d.Source.StartLine,
			EndLine:   d.Source.EndLine,
			Truncated: d.Source.Truncated,
		},
		SourceOmitted: d.SourceOmitted,
	}
}

func FromProtoSymbolDetail(p *codeintelv1.SymbolDetail) domain.SymbolDetail {
	if p == nil {
		return domain.SymbolDetail{}
	}
	incoming := make(map[string]domain.RelatedSymbolList, len(p.Incoming))
	for k, v := range p.Incoming {
		if v != nil {
			syms := make([]domain.RelatedSymbol, 0, len(v.Symbols))
			for _, s := range v.Symbols {
				syms = append(syms, domain.RelatedSymbol{
					Ref:  FromProtoSymbolRef(s.Ref),
					Kind: FromProtoEdgeKind(s.Kind),
				})
			}
			incoming[k] = domain.RelatedSymbolList{Symbols: syms}
		}
	}

	outgoing := make(map[string]domain.RelatedSymbolList, len(p.Outgoing))
	for k, v := range p.Outgoing {
		if v != nil {
			syms := make([]domain.RelatedSymbol, 0, len(v.Symbols))
			for _, s := range v.Symbols {
				syms = append(syms, domain.RelatedSymbol{
					Ref:  FromProtoSymbolRef(s.Ref),
					Kind: FromProtoEdgeKind(s.Kind),
				})
			}
			outgoing[k] = domain.RelatedSymbolList{Symbols: syms}
		}
	}

	flows := make([]domain.SymbolFlowRef, 0, len(p.Flows))
	for _, f := range p.Flows {
		flows = append(flows, domain.SymbolFlowRef{
			FlowID:    f.FlowId,
			FlowLabel: f.FlowLabel,
			Step:      f.Step,
		})
	}

	var source domain.SymbolSource
	if p.Source != nil {
		source = domain.SymbolSource{
			Text:      p.Source.Text,
			StartLine: p.Source.StartLine,
			EndLine:   p.Source.EndLine,
			Truncated: p.Source.Truncated,
		}
	}

	return domain.SymbolDetail{
		Symbol:        FromProtoSymbolRef(p.Symbol),
		Incoming:      incoming,
		Outgoing:      outgoing,
		Flows:         flows,
		Source:        source,
		SourceOmitted: p.SourceOmitted,
	}
}
