package domain

// ClusterNode represents a high-level architectural cluster of symbols.
type ClusterNode struct {
	ID               string
	Label            string
	SymbolCount      int32
	Cohesion         float64
	Keywords         []string
	TopFiles         []string
	DominantLanguage string
	Area             string
}

// ClusterEdge represents relationships between architectural clusters.
type ClusterEdge struct {
	From       string
	To         string
	Weight     float64
	KindCounts map[string]int32
}

// ArchitectureGraph captures the cluster-level view of a repository.
type ArchitectureGraph struct {
	Nodes []ClusterNode
	Edges []ClusterEdge
}
