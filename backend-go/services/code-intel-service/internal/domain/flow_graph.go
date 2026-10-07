package domain

// FlowSummary summarizes an end-to-end execution flow.
type FlowSummary struct {
	ID          string
	Label       string
	ProcessType string
	StepCount   int32
	Communities []string
	Entry       string
	Terminal    string
}

// FlowStep represents a step along a flow path.
type FlowStep struct {
	Step      int32
	Symbol    SymbolRef
	Cluster   string
	FilePath  string
	StartLine int32
}

// FlowGraph captures a trace or flow with steps and edges.
type FlowGraph struct {
	Flow  FlowSummary
	Steps []FlowStep
	Edges []SymbolEdge
}
