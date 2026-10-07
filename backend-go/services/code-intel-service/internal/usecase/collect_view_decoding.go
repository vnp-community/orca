package usecase

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/domain"
)

func parsePlatform(platStr string) domain.Platform {
	lower := strings.ToLower(platStr)
	switch {
	case strings.Contains(lower, "win"):
		return domain.PlatformWin32
	case strings.Contains(lower, "darwin"), strings.Contains(lower, "mac"):
		return domain.PlatformDarwin
	default:
		return domain.PlatformLinux
	}
}

// decodeArchitectureGraph parses RawCodeIntelResult.Data into domain.ArchitectureGraph.
func decodeArchitectureGraph(raw json.RawMessage, target AgentTarget) (domain.ArchitectureGraph, error) {
	if len(raw) == 0 {
		return domain.ArchitectureGraph{}, nil
	}

	type rawClusterNode struct {
		ID               string   `json:"id"`
		Label            string   `json:"label"`
		SymbolCount      int32    `json:"symbolCount"`
		Cohesion         float64  `json:"cohesion"`
		Keywords         []string `json:"keywords"`
		TopFiles         []string `json:"topFiles"`
		DominantLanguage string   `json:"dominantLanguage"`
		Area             string   `json:"area"`
	}

	type rawClusterEdge struct {
		From       string           `json:"from"`
		To         string           `json:"to"`
		Weight     float64          `json:"weight"`
		Kinds      map[string]int32 `json:"kinds"`
		KindCounts map[string]int32 `json:"kindCounts"`
	}

	var payload struct {
		Nodes []rawClusterNode `json:"nodes"`
		Edges []rawClusterEdge `json:"edges"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.ArchitectureGraph{}, apperrors.New(apperrors.KindInternal, "CODEINTEL_DECODE_FAILED", fmt.Sprintf("failed to decode architecture graph: %v", err), err)
	}

	nodes := make([]domain.ClusterNode, 0, len(payload.Nodes))
	for _, n := range payload.Nodes {
		nodes = append(nodes, domain.ClusterNode{
			ID:               n.ID,
			Label:            n.Label,
			SymbolCount:      n.SymbolCount,
			Cohesion:         n.Cohesion,
			Keywords:         n.Keywords,
			TopFiles:         n.TopFiles,
			DominantLanguage: n.DominantLanguage,
			Area:             n.Area,
		})
	}

	edges := make([]domain.ClusterEdge, 0, len(payload.Edges))
	for _, e := range payload.Edges {
		kc := e.KindCounts
		if kc == nil {
			kc = e.Kinds
		}
		if kc == nil {
			kc = make(map[string]int32)
		}
		edges = append(edges, domain.ClusterEdge{
			From:       e.From,
			To:         e.To,
			Weight:     e.Weight,
			KindCounts: kc,
		})
	}

	return domain.ArchitectureGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// decodeSymbolGraph parses RawCodeIntelResult.Data into domain.SymbolGraph.
func decodeSymbolGraph(raw json.RawMessage, target AgentTarget) (domain.SymbolGraph, error) {
	if len(raw) == 0 {
		return domain.SymbolGraph{}, nil
	}

	type rawSymbolRef struct {
		Key           string `json:"key"`
		Kind          string `json:"kind"`
		NativeKind    string `json:"nativeKind"`
		Name          string `json:"name"`
		QualifiedName string `json:"qualifiedName"`
		FilePath      string `json:"filePath"`
		StartLine     int32  `json:"startLine"`
		EndLine       int32  `json:"endLine"`
		GitNexusID    string `json:"gitnexusId"`
		CodeGraphID   string `json:"codegraphId"`
		Language      string `json:"language"`
		Signature     string `json:"signature"`
		IsExported    bool   `json:"isExported"`
		Docstring     string `json:"docstring"`
		Ordinal       int    `json:"ordinal"`
	}

	type rawSymbolNode struct {
		Ref        *rawSymbolRef `json:"ref,omitempty"`
		Key        string        `json:"key,omitempty"`
		Kind       string        `json:"kind,omitempty"`
		Name       string        `json:"name,omitempty"`
		FilePath   string        `json:"filePath,omitempty"`
		StartLine  int32         `json:"startLine,omitempty"`
		EndLine    int32         `json:"endLine,omitempty"`
		Language   string        `json:"language,omitempty"`
		IsExported bool          `json:"isExported"`
		Signature  string        `json:"signature"`
		Cluster    string        `json:"cluster"`
	}

	type rawSymbolEdge struct {
		FromKey    string   `json:"fromKey"`
		ToKey      string   `json:"toKey"`
		Kind       string   `json:"kind"`
		Sources    []string `json:"sources"`
		Confidence float32  `json:"confidence"`
	}

	var payload struct {
		Center rawSymbolRef    `json:"center"`
		Nodes  []rawSymbolNode `json:"nodes"`
		Edges  []rawSymbolEdge `json:"edges"`
		Depth  int32           `json:"depth"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.SymbolGraph{}, apperrors.New(apperrors.KindInternal, "CODEINTEL_DECODE_FAILED", fmt.Sprintf("failed to decode symbol graph: %v", err), err)
	}

	plat := parsePlatform(target.HostPlatform)

	toDomainRef := func(r rawSymbolRef) domain.SymbolRef {
		ref := domain.SymbolRef{
			Key:           r.Key,
			Kind:          domain.SymbolKindFromString(r.Kind),
			NativeKind:    r.NativeKind,
			Name:          r.Name,
			QualifiedName: r.QualifiedName,
			FilePath:      r.FilePath,
			StartLine:     r.StartLine,
			EndLine:       r.EndLine,
			GitNexusID:    r.GitNexusID,
			CodeGraphID:   r.CodeGraphID,
			Language:      r.Language,
			Signature:     r.Signature,
			IsExported:    r.IsExported,
			Docstring:     r.Docstring,
			Ordinal:       r.Ordinal,
		}
		validated, _, _ := domain.ValidateAgentSymbolRef(ref, target.WorkspaceRoot, plat)
		return validated
	}

	centerRef := toDomainRef(payload.Center)

	nodes := make([]domain.SymbolNode, 0, len(payload.Nodes))
	for _, n := range payload.Nodes {
		var ref domain.SymbolRef
		if n.Ref != nil {
			ref = toDomainRef(*n.Ref)
		} else {
			rawRef := rawSymbolRef{
				Key:        n.Key,
				Kind:       n.Kind,
				Name:       n.Name,
				FilePath:   n.FilePath,
				StartLine:  n.StartLine,
				EndLine:    n.EndLine,
				Language:   n.Language,
				IsExported: n.IsExported,
				Signature:  n.Signature,
			}
			ref = toDomainRef(rawRef)
		}
		nodes = append(nodes, domain.SymbolNode{
			Ref:        ref,
			IsExported: n.IsExported,
			Signature:  n.Signature,
			Cluster:    n.Cluster,
		})
	}

	edges := make([]domain.SymbolEdge, 0, len(payload.Edges))
	for _, e := range payload.Edges {
		edges = append(edges, domain.SymbolEdge{
			FromKey:    e.FromKey,
			ToKey:      e.ToKey,
			Kind:       domain.ParseEdgeKind(e.Kind),
			Sources:    e.Sources,
			Confidence: e.Confidence,
		})
	}

	return domain.SymbolGraph{
		Center: centerRef,
		Nodes:  nodes,
		Edges:  edges,
		Depth:  payload.Depth,
	}, nil
}

// convertSymbolGraphToModuleGraph implements C3 for STRUCTURE view.
func convertSymbolGraphToModuleGraph(sg domain.SymbolGraph) domain.ModuleGraph {
	seenNodes := make(map[string]bool)
	keyToID := make(map[string]string)
	var nodes []domain.ModuleNode

	for _, n := range sg.Nodes {
		id := n.Ref.FilePath
		if id == "" {
			id = n.Ref.Key
		}
		if id == "" {
			continue
		}
		if n.Ref.Key != "" {
			keyToID[n.Ref.Key] = id
		}
		if n.Ref.FilePath != "" {
			keyToID[n.Ref.FilePath] = id
		}
		if seenNodes[id] {
			continue
		}
		seenNodes[id] = true

		kind := n.Ref.Kind.String()
		if kind == "unspecified" {
			kind = "file"
		}

		nodes = append(nodes, domain.ModuleNode{
			ID:          id,
			Kind:        kind,
			Language:    n.Ref.Language,
			SymbolCount: 0,
			Loc:         0,
			Cluster:     n.Cluster,
		})
	}

	resolveID := func(k string) string {
		if id, ok := keyToID[k]; ok {
			return id
		}
		if seenNodes[k] {
			return k
		}
		// If key is in format kind:filePath:name, check if filePath matches a node
		parts := strings.Split(k, ":")
		if len(parts) >= 2 && seenNodes[parts[1]] {
			return parts[1]
		}
		return k
	}

	seenEdges := make(map[string]bool)
	var edges []domain.ModuleEdge

	for _, e := range sg.Edges {
		from := resolveID(e.FromKey)
		to := resolveID(e.ToKey)

		edgeKey := fmt.Sprintf("%s->%s:%s", from, to, e.Kind.String())
		if seenEdges[edgeKey] {
			continue
		}
		seenEdges[edgeKey] = true

		edges = append(edges, domain.ModuleEdge{
			From:  from,
			To:    to,
			Kind:  e.Kind.String(),
			Count: 1,
		})
	}

	return domain.ModuleGraph{
		Nodes: nodes,
		Edges: edges,
	}
}



// decodeImpactGraph parses RawCodeIntelResult.Data into domain.ImpactGraph.
func decodeImpactGraph(raw json.RawMessage, target AgentTarget) (domain.ImpactGraph, error) {
	if len(raw) == 0 {
		return domain.ImpactGraph{}, nil
	}

	type rawSymbolRef struct {
		Key           string `json:"key"`
		Kind          string `json:"kind"`
		NativeKind    string `json:"nativeKind"`
		Name          string `json:"name"`
		QualifiedName string `json:"qualifiedName"`
		FilePath      string `json:"filePath"`
		StartLine     int32  `json:"startLine"`
		EndLine       int32  `json:"endLine"`
		Language      string `json:"language"`
	}

	type rawImpactSymbol struct {
		Symbol     rawSymbolRef `json:"symbol"`
		Via        string       `json:"via"`
		Direct     bool         `json:"direct"`
		Confidence float32      `json:"confidence"`
	}

	type rawImpactLevel struct {
		Depth   int32             `json:"depth"`
		Symbols []rawImpactSymbol `json:"symbols"`
	}

	type rawAffectedFlow struct {
		FlowID      string `json:"flowId"`
		Label       string `json:"label"`
		StepCount   int32  `json:"stepCount"`
		ChangedStep *int32 `json:"changedStep,omitempty"`
	}

	type rawAffectedCluster struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Label  string `json:"label"`
		Hits   int32  `json:"hits"`
		Impact string `json:"impact"`
	}

	var payload struct {
		Target           rawSymbolRef         `json:"target"`
		Direction        string               `json:"direction"`
		Risk             string               `json:"risk"`
		ImpactedCount    int32                `json:"impactedCount"`
		Levels           []rawImpactLevel     `json:"levels"`
		AffectedFlows    []rawAffectedFlow    `json:"affectedFlows"`
		AffectedClusters []rawAffectedCluster `json:"affectedClusters"`
		AffectedModules  []rawAffectedCluster `json:"affectedModules"`
		TestsCovering    []any                `json:"testsCovering"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.ImpactGraph{}, apperrors.New(apperrors.KindInternal, "CODEINTEL_DECODE_FAILED", fmt.Sprintf("failed to decode impact graph: %v", err), err)
	}

	plat := parsePlatform(target.HostPlatform)

	toDomainRef := func(r rawSymbolRef) domain.SymbolRef {
		ref := domain.SymbolRef{
			Key:           r.Key,
			Kind:          domain.SymbolKindFromString(r.Kind),
			NativeKind:    r.NativeKind,
			Name:          r.Name,
			QualifiedName: r.QualifiedName,
			FilePath:      r.FilePath,
			StartLine:     r.StartLine,
			EndLine:       r.EndLine,
			Language:      r.Language,
		}
		validated, _, _ := domain.ValidateAgentSymbolRef(ref, target.WorkspaceRoot, plat)
		return validated
	}

	var risk domain.Risk
	switch strings.ToUpper(payload.Risk) {
	case "LOW":
		risk = domain.RiskLow
	case "MEDIUM":
		risk = domain.RiskMedium
	case "HIGH":
		risk = domain.RiskHigh
	case "CRITICAL":
		risk = domain.RiskCritical
	default:
		risk = domain.RiskUnknown
	}

	levels := make([]domain.ImpactLevel, 0, len(payload.Levels))
	for _, l := range payload.Levels {
		syms := make([]domain.ImpactSymbol, 0, len(l.Symbols))
		for _, s := range l.Symbols {
			syms = append(syms, domain.ImpactSymbol{
				Symbol:     toDomainRef(s.Symbol),
				Via:        domain.ParseEdgeKind(s.Via),
				Direct:     s.Direct,
				Confidence: s.Confidence,
			})
		}
		levels = append(levels, domain.ImpactLevel{
			Depth:   l.Depth,
			Symbols: syms,
		})
	}

	flows := make([]domain.AffectedFlow, 0, len(payload.AffectedFlows))
	for _, f := range payload.AffectedFlows {
		flows = append(flows, domain.AffectedFlow{
			FlowID:      f.FlowID,
			Label:       f.Label,
			StepCount:   f.StepCount,
			ChangedStep: f.ChangedStep,
		})
	}

	clusterItems := payload.AffectedClusters
	if len(clusterItems) == 0 {
		clusterItems = payload.AffectedModules
	}
	clusters := make([]domain.AffectedCluster, 0, len(clusterItems))
	for _, c := range clusterItems {
		lbl := c.Label
		if lbl == "" {
			lbl = c.Name
		}
		clusters = append(clusters, domain.AffectedCluster{
			ID:     c.ID,
			Label:  lbl,
			Hits:   c.Hits,
			Impact: c.Impact,
		})
	}

	var testPaths []string
	for _, tItem := range payload.TestsCovering {
		switch v := tItem.(type) {
		case string:
			testPaths = append(testPaths, v)
		case map[string]any:
			if fp, ok := v["filePath"].(string); ok && fp != "" {
				testPaths = append(testPaths, fp)
			}
		}
	}

	return domain.ImpactGraph{
		Target:           toDomainRef(payload.Target),
		Direction:        payload.Direction,
		Risk:             risk,
		Levels:           levels,
		AffectedFlows:    flows,
		AffectedClusters: clusters,
		TestsCovering:    testPaths,
		ImpactedCount:    payload.ImpactedCount,
	}, nil
}

// decodeSymbolDetail parses RawCodeIntelResult.Data into domain.SymbolDetail.
func decodeSymbolDetail(raw json.RawMessage, target AgentTarget) (domain.SymbolDetail, error) {
	if len(raw) == 0 {
		return domain.SymbolDetail{}, nil
	}

	type rawSymbolRef struct {
		Key           string `json:"key"`
		Kind          string `json:"kind"`
		NativeKind    string `json:"nativeKind"`
		Name          string `json:"name"`
		QualifiedName string `json:"qualifiedName"`
		FilePath      string `json:"filePath"`
		StartLine     int32  `json:"startLine"`
		EndLine       int32  `json:"endLine"`
		Language      string `json:"language"`
	}

	type rawRelatedSymbol struct {
		Ref      *rawSymbolRef `json:"ref,omitempty"`
		Key      string        `json:"key,omitempty"`
		Name     string        `json:"name,omitempty"`
		FilePath string        `json:"filePath,omitempty"`
		Kind     string        `json:"kind,omitempty"`
	}

	type rawFlowRef struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Step  int32  `json:"step"`
	}

	type rawSource struct {
		Text      string `json:"text"`
		StartLine int32  `json:"startLine"`
		EndLine   int32  `json:"endLine"`
		Truncated bool   `json:"truncated"`
	}

	var payload struct {
		Symbol        rawSymbolRef                    `json:"symbol"`
		Incoming      map[string][]rawRelatedSymbol   `json:"incoming"`
		Outgoing      map[string][]rawRelatedSymbol   `json:"outgoing"`
		Flows         []rawFlowRef                    `json:"flows"`
		Source        *rawSource                      `json:"source"`
		SourceOmitted string                          `json:"sourceOmitted"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.SymbolDetail{}, apperrors.New(apperrors.KindInternal, "CODEINTEL_DECODE_FAILED", fmt.Sprintf("failed to decode symbol detail: %v", err), err)
	}

	plat := parsePlatform(target.HostPlatform)

	toDomainRef := func(r rawSymbolRef) domain.SymbolRef {
		ref := domain.SymbolRef{
			Key:           r.Key,
			Kind:          domain.SymbolKindFromString(r.Kind),
			NativeKind:    r.NativeKind,
			Name:          r.Name,
			QualifiedName: r.QualifiedName,
			FilePath:      r.FilePath,
			StartLine:     r.StartLine,
			EndLine:       r.EndLine,
			Language:      r.Language,
		}
		validated, _, _ := domain.ValidateAgentSymbolRef(ref, target.WorkspaceRoot, plat)
		return validated
	}

	inMap := make(map[string]domain.RelatedSymbolList)
	for k, list := range payload.Incoming {
		syms := make([]domain.RelatedSymbol, 0, len(list))
		for _, item := range list {
			var ref domain.SymbolRef
			if item.Ref != nil {
				ref = toDomainRef(*item.Ref)
			} else {
				ref = toDomainRef(rawSymbolRef{Key: item.Key, Name: item.Name, FilePath: item.FilePath})
			}
			syms = append(syms, domain.RelatedSymbol{
				Ref:  ref,
				Kind: domain.ParseEdgeKind(item.Kind),
			})
		}
		inMap[k] = domain.RelatedSymbolList{Symbols: syms}
	}

	outMap := make(map[string]domain.RelatedSymbolList)
	for k, list := range payload.Outgoing {
		syms := make([]domain.RelatedSymbol, 0, len(list))
		for _, item := range list {
			var ref domain.SymbolRef
			if item.Ref != nil {
				ref = toDomainRef(*item.Ref)
			} else {
				ref = toDomainRef(rawSymbolRef{Key: item.Key, Name: item.Name, FilePath: item.FilePath})
			}
			syms = append(syms, domain.RelatedSymbol{
				Ref:  ref,
				Kind: domain.ParseEdgeKind(item.Kind),
			})
		}
		outMap[k] = domain.RelatedSymbolList{Symbols: syms}
	}

	flowRefs := make([]domain.SymbolFlowRef, 0, len(payload.Flows))
	for _, f := range payload.Flows {
		flowRefs = append(flowRefs, domain.SymbolFlowRef{
			FlowID:    f.ID,
			FlowLabel: f.Label,
			Step:      f.Step,
		})
	}

	var src domain.SymbolSource
	if payload.Source != nil {
		src = domain.SymbolSource{
			Text:      payload.Source.Text,
			StartLine: payload.Source.StartLine,
			EndLine:   payload.Source.EndLine,
			Truncated: payload.Source.Truncated,
		}
	}

	return domain.SymbolDetail{
		Symbol:        toDomainRef(payload.Symbol),
		Incoming:      inMap,
		Outgoing:      outMap,
		Flows:         flowRefs,
		Source:        src,
		SourceOmitted: payload.SourceOmitted,
	}, nil
}

// decodeRouteMap parses RawCodeIntelResult.Data into domain.RouteMap.
func decodeRouteMap(raw json.RawMessage, target AgentTarget) (domain.RouteMap, error) {
	if len(raw) == 0 {
		return domain.RouteMap{}, nil
	}

	type rawRouteNode struct {
		ID           string   `json:"id"`
		Path         string   `json:"path"`
		Method       string   `json:"method"`
		FilePath     string   `json:"filePath"`
		Middleware   []string `json:"middleware"`
		ResponseKeys []string `json:"responseKeys"`
		ErrorKeys    []string `json:"errorKeys"`
		Side         string   `json:"side"`
	}

	type rawRouteEdge struct {
		RouteID string `json:"routeId"`
		Route   string `json:"route"`
		Handler string `json:"handler"`
		Kind    string `json:"kind"`
	}

	var payload struct {
		Routes []rawRouteNode `json:"routes"`
		Edges  []rawRouteEdge `json:"edges"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.RouteMap{}, apperrors.New(apperrors.KindInternal, "CODEINTEL_DECODE_FAILED", fmt.Sprintf("failed to decode route map: %v", err), err)
	}

	routes := make([]domain.RouteNode, 0, len(payload.Routes))
	for _, r := range payload.Routes {
		routes = append(routes, domain.RouteNode{
			ID:           r.ID,
			Path:         r.Path,
			Method:       r.Method,
			FilePath:     r.FilePath,
			Middleware:   r.Middleware,
			ResponseKeys: r.ResponseKeys,
			ErrorKeys:    r.ErrorKeys,
			Side:         r.Side,
		})
	}

	edges := make([]domain.RouteEdge, 0, len(payload.Edges))
	for _, e := range payload.Edges {
		rid := e.RouteID
		if rid == "" {
			rid = e.Route
		}
		edges = append(edges, domain.RouteEdge{
			RouteID: rid,
			Handler: e.Handler,
			Kind:    e.Kind,
		})
	}

	return domain.RouteMap{
		Routes: routes,
		Edges:  edges,
	}, nil
}
