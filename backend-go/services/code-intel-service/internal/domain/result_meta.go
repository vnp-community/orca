package domain

import (
	"fmt"
	"time"
)

// ViewKind enumerates the graph projection kinds.
type ViewKind int

const (
	ViewKindUnspecified ViewKind = iota
	ViewKindStructure
	ViewKindArchitecture
	ViewKindFlows
	ViewKindFlow
	ViewKindSubgraph
	ViewKindImpact
	ViewKindSymbol
	ViewKindRoutes
	ViewKindChangeOverlay
	ViewKindStatus
	ViewKindClusters
	ViewKindReadingOrder
	ViewKindErd
	ViewKindStorage
	ViewKindDataflows
	ViewKindDataflow
	ViewKindFindings
	ViewKindContractDiff
	ViewKindContractCatalog
	ViewKindRequirementTrace
	ViewKindAiSummary
)

// CacheName returns the contract §4/T3 cache name representation of ViewKind.
func (v ViewKind) CacheName() string {
	switch v {
	case ViewKindStructure:
		return "structure"
	case ViewKindArchitecture:
		return "architecture"
	case ViewKindFlows:
		return "flows"
	case ViewKindFlow:
		return "flow"
	case ViewKindSubgraph:
		return "subgraph"
	case ViewKindImpact:
		return "impact"
	case ViewKindSymbol:
		return "symbol"
	case ViewKindRoutes:
		return "routes"
	case ViewKindChangeOverlay:
		return "changeOverlay"
	case ViewKindStatus:
		return "status"
	case ViewKindClusters:
		return "clusters"
	case ViewKindReadingOrder:
		return "readingOrder"
	case ViewKindErd:
		return "erd"
	case ViewKindStorage:
		return "storage"
	case ViewKindDataflows:
		return "dataflows"
	case ViewKindDataflow:
		return "dataflow"
	case ViewKindFindings:
		return "findings"
	case ViewKindContractDiff:
		return "contractDiff"
	case ViewKindContractCatalog:
		return "contractCatalog"
	case ViewKindRequirementTrace:
		return "requirementTrace"
	case ViewKindAiSummary:
		return "aiSummary"
	default:
		return "unspecified"
	}
}

// ParseViewKind parses a cache name into its ViewKind enum value.
func ParseViewKind(s string) (ViewKind, error) {
	switch s {
	case "structure":
		return ViewKindStructure, nil
	case "architecture":
		return ViewKindArchitecture, nil
	case "flows":
		return ViewKindFlows, nil
	case "flow":
		return ViewKindFlow, nil
	case "subgraph":
		return ViewKindSubgraph, nil
	case "impact":
		return ViewKindImpact, nil
	case "symbol":
		return ViewKindSymbol, nil
	case "routes":
		return ViewKindRoutes, nil
	case "changeOverlay":
		return ViewKindChangeOverlay, nil
	case "status":
		return ViewKindStatus, nil
	case "clusters":
		return ViewKindClusters, nil
	case "readingOrder":
		return ViewKindReadingOrder, nil
	case "erd":
		return ViewKindErd, nil
	case "storage":
		return ViewKindStorage, nil
	case "dataflows":
		return ViewKindDataflows, nil
	case "dataflow":
		return ViewKindDataflow, nil
	case "findings":
		return ViewKindFindings, nil
	case "contractDiff":
		return ViewKindContractDiff, nil
	case "contractCatalog":
		return ViewKindContractCatalog, nil
	case "requirementTrace":
		return ViewKindRequirementTrace, nil
	case "aiSummary":
		return ViewKindAiSummary, nil
	default:
		return ViewKindUnspecified, fmt.Errorf("unknown view kind cache name: %q", s)
	}
}

// SourceInfo describes indexing metadata for a code intelligence tool source.
type SourceInfo struct {
	Tool      string
	Version   string
	IndexedAt time.Time
	Commit    string
	LineBase  int
}

// ResultMeta captures envelope metadata for graph query responses.
type ResultMeta struct {
	Repo         string
	WorktreeRef  string
	DevServerID  string
	Commit       string
	View         ViewKind
	Cached       bool
	ETag         string
	GeneratedAt  time.Time
	Sources      []SourceInfo
	Stale        bool
	Truncated    bool
	TotalCount   int64
	TotalNodes   int64
	TotalEdges   int64
	TotalBefore  int64
	NotModified  bool
	Warnings     []string
}
