package domain

// SymbolNode represents a node in a symbol neighborhood graph.
type SymbolNode struct {
	Ref        SymbolRef
	IsExported bool
	Signature  string
	Cluster    string
}

// SymbolGraph captures the neighborhood of a center symbol up to a specific depth.
type SymbolGraph struct {
	Center SymbolRef
	Nodes  []SymbolNode
	Edges  []SymbolEdge
	Depth  int32
}
