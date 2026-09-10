package domain

// SubtaskProposal is an AI-generated, not-yet-persisted subtask suggestion
// — the review-before-commit shape AIDecompose/AIApply share (TASK-224):
// AIDecompose returns a set of these for the caller to review/edit, AIApply
// commits a (possibly edited) set as real Task rows + parent_child edges.
// Never itself written to task_edges; it has no ID until AIApply creates
// the real Task.
type SubtaskProposal struct {
	Title       string
	Description string
	Type        string
	// EstimatedHours is nil when the AI response omits it — not defaulted
	// to 0 by parseSubtaskProposals, so a zero-valued estimate and "no
	// estimate given" stay distinguishable through to CreateTaskInput.
	EstimatedHours *float64
	// DependsOnIndex holds indices into the SAME proposal batch (not real
	// task IDs — those don't exist until AIApply creates them) that this
	// proposal depends on. Resolved to real depends_on edges in AIApply,
	// after every proposal in the batch has been created.
	DependsOnIndex []int
	PromptTemplate string
}
