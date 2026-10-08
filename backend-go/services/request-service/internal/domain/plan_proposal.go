package domain

type PlanProposal struct {
	Title   string
	Summary string
	Phases  []PhaseProposal
	Tasks   []TaskProposal
	Notes   string
}

type PhaseProposal struct {
	Title       string
	Description string
	Tasks       []TaskProposal
}

type TaskProposal struct {
	ID               string
	Title            string
	Description      string
	TaskType         string
	EstimatedHours   float64
	PromptTemplate   string
	DependsOnIndices []int
	Labels           []string
	Irreversible     bool
	Satisfies        []string
	Done             bool
}

// Violation is one problem found in an artifact or projection. Path is a JSON Pointer
// (empty when the problem is positional); Line is 1-based and 0 when unknown.
type Violation struct {
	Path    string
	Line    int
	Code    string
	Message string
}
