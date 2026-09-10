package domain

// TopologicalWaves groups taskIDs into waves of mutually-independent tasks
// over the depends_on DAG, scoped to ONLY the given taskIDs (edges pointing
// outside that set are ignored) — reuses the same "from depends_on to"
// edge-direction convention critical_path.go's buildDependsOnGraph uses,
// rather than a second graph-construction implementation with its own
// (possibly inconsistent) direction convention. A task with no dependency
// among the batch set is wave 0; each subsequent wave contains every task
// whose depends_on targets are all in an earlier wave.
func TopologicalWaves(edges []TaskEdge, taskIDs []string) [][]string {
	inSet := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		inSet[id] = true
	}
	scoped := make([]TaskEdge, 0, len(edges))
	for _, e := range edges {
		if e.Kind == EdgeKindDependsOn && inSet[e.FromTaskID] && inSet[e.ToTaskID] {
			scoped = append(scoped, e)
		}
	}

	remainingDeps := make(map[string]map[string]bool, len(taskIDs)) // taskID -> set of not-yet-satisfied dependency IDs
	for _, id := range taskIDs {
		remainingDeps[id] = map[string]bool{}
	}
	for _, e := range scoped {
		remainingDeps[e.FromTaskID][e.ToTaskID] = true
	}

	var waves [][]string
	placed := map[string]bool{}
	for len(placed) < len(taskIDs) {
		var wave []string
		for _, id := range taskIDs {
			if placed[id] || len(remainingDeps[id]) > 0 {
				continue
			}
			wave = append(wave, id)
		}
		if len(wave) == 0 {
			break // defensive: a cycle slipped through (DetectCycle is the real enforcement point)
		}
		for _, id := range wave {
			placed[id] = true
		}
		for _, deps := range remainingDeps {
			for _, id := range wave {
				delete(deps, id)
			}
		}
		waves = append(waves, wave)
	}
	return waves
}
