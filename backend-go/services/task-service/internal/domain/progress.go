package domain

// CalculateProgress computes, for one task, the percentage of its direct
// children marked Done — 100 for a leaf task whose OWN status is Done, 0
// for a leaf that isn't, and the average of children's own (already
// recursively computed) percentages for a task with children. The caller
// (usecase.RecalculateProgress) walks bottom-up over a subtree fetched via
// GetSubtree, calling this once per task in post-order. Pure function: no
// DB, no context.Context — same discipline as DetectCycle/ResolveGrant.
func CalculateProgress(task Task, childPercents []int) int {
	if len(childPercents) == 0 {
		if task.Status == StatusDone {
			return 100
		}
		return 0
	}
	sum := 0
	for _, p := range childPercents {
		sum += p
	}
	return sum / len(childPercents)
}
