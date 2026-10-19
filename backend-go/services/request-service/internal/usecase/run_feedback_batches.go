package usecase

// RunFeedbackBatches drops empty and repeated ids, keeps first-seen order and splits the rest into lots of at most
// RunFeedbackBatchSize, so every adapter bounds its IN list the same way.
func RunFeedbackBatches(runIDs []string) [][]string {
	seen := make(map[string]struct{}, len(runIDs))
	var batches [][]string
	var cur []string
	for _, id := range runIDs {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		cur = append(cur, id)
		if len(cur) == RunFeedbackBatchSize {
			batches, cur = append(batches, cur), nil
		}
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}
