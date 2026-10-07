package domain

// ModuleNode represents a module, package, or directory in the module graph.
type ModuleNode struct {
	ID          string
	Kind        string
	Language    string
	SymbolCount int32
	Loc         int32
	Cluster     string
	Area        string
}

// ModuleEdge represents a dependency or reference between modules.
type ModuleEdge struct {
	From  string
	To    string
	Kind  string
	Count int32
}

// ModuleGraph captures module-level dependency relationships.
type ModuleGraph struct {
	Nodes []ModuleNode
	Edges []ModuleEdge
}
