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
	ID                 string
	Title              string
	Description        string
	TaskType           string
	EstimatedHours     float64
	PromptTemplate     string
	DependsOnIndices   []int
	Labels             []string
	Irreversible       bool
	Satisfies          []string
	Done               bool
}

type Violation struct {
	Line    int
	Code    string
	Message string
}
