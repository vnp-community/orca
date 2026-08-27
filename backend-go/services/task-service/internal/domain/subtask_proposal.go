package domain

// SubtaskProposal is an AI-generated, not-yet-persisted subtask suggestion
// — the review-before-commit shape AIDecompose/AIApply share (TASK-224):
// AIDecompose returns a set of these for the caller to review/edit, AIApply
// commits a (possibly edited) set as real Task rows + parent_child edges.
// Never itself written to task_edges; it has no ID until AIApply creates
// the real Task.
type SubtaskProposal struct {
	Title          string
	Description    string
	Type           string // task|bug|feature — mirrors Task.Type
	EstimatedHours *float64
	// DependsOnIndices names OTHER proposals in the SAME AIDecompose
	// response by their 0-based position, e.g. proposal[2] depends on
	// proposal[0] -> DependsOnIndices: []int{0}.
	DependsOnIndices []int
	PromptTemplate   string
}
