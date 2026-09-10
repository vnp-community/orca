package domain

// CalculateCriticalPath returns the longest-duration chain of depends_on
// edges through tasks, by EstimatedHours — Kahn's-algorithm topological
// sort + longest-path DP, same primitive style as workflow-service's
// DAG.BuildWaves (workflow-service.md §4, reused conceptually only — this
// package imports nothing from that separate service/module). Tasks with a
// nil EstimatedHours are treated as zero-duration for this calculation (a
// task with no estimate contributes nothing to path length, but is not
// excluded from the graph — cutting it out entirely could silently break a
// real depends_on chain through it).
//
// Returns nil if edges don't form a DAG (a real cycle should never reach
// this function, since AddEdge/AIApply both reject cycles at write time —
// this is a defensive fallback, not an expected path) or if tasks is empty.
// Ties (multiple paths of equal total duration) are broken by picking the
// first-discovered path in topological order, deterministic across repeated
// calls against identical input.
func CalculateCriticalPath(tasks []Task, edges []TaskEdge) []string {
	if len(tasks) == 0 {
		return nil
	}

	// hoursByID: EstimatedHours per task, defaulting nil to 0.
	hoursByID := make(map[string]float64, len(tasks))
	for _, t := range tasks {
		if t.EstimatedHours != nil {
			hoursByID[t.ID] = *t.EstimatedHours
		} else {
			hoursByID[t.ID] = 0
		}
	}

	// adjacency: fromID -> []toID (fromID is a dependency of toID, matching
	// AddEdge's existing depends_on semantics: fromID must complete before
	// toID can start), inDegree: toID -> count of incoming depends_on edges.
	adjacency := make(map[string][]string, len(tasks))
	inDegree := make(map[string]int, len(tasks))
	for _, t := range tasks {
		inDegree[t.ID] = 0
	}
	for _, e := range edges {
		if e.Kind != EdgeKindDependsOn {
			continue
		}
		if _, ok := hoursByID[e.FromTaskID]; !ok {
			continue // edge references a task outside this call's task set — ignore
		}
		if _, ok := hoursByID[e.ToTaskID]; !ok {
			continue
		}
		adjacency[e.FromTaskID] = append(adjacency[e.FromTaskID], e.ToTaskID)
		inDegree[e.ToTaskID]++
	}

	// Kahn's algorithm: process zero-in-degree nodes in a stable (task
	// slice) order for determinism, decrementing successors' in-degree as
	// each is processed.
	var queue []string
	for _, t := range tasks {
		if inDegree[t.ID] == 0 {
			queue = append(queue, t.ID)
		}
	}
	topoOrder := make([]string, 0, len(tasks))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		topoOrder = append(topoOrder, id)
		for _, next := range adjacency[id] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(topoOrder) != len(tasks) {
		// A real cycle exists — shouldn't happen given AddEdge/AIApply's
		// write-time cycle rejection, but never loop forever or return a
		// misleading partial path.
		return nil
	}

	// Longest-path DP over the topological order: dist[node] = task's own
	// hours + max(dist[dep]) across all incoming depends_on edges (0 for a
	// node with no dependencies). prev[node] tracks the predecessor on the
	// best path found so far, for path reconstruction.
	dist := make(map[string]float64, len(tasks))
	prev := make(map[string]string, len(tasks))
	for _, id := range topoOrder {
		dist[id] = hoursByID[id]
	}
	for _, id := range topoOrder {
		for _, next := range adjacency[id] {
			candidate := dist[id] + hoursByID[next]
			if candidate > dist[next] {
				dist[next] = candidate
				prev[next] = id
			}
		}
	}

	// Find the max-dist node — ties broken by topological order (first
	// discovered wins, since we only overwrite on strictly-greater).
	best := topoOrder[0]
	for _, id := range topoOrder {
		if dist[id] > dist[best] {
			best = id
		}
	}

	// Walk back from best to reconstruct the path, then reverse it.
	var reversed []string
	for id := best; ; {
		reversed = append(reversed, id)
		p, ok := prev[id]
		if !ok {
			break
		}
		id = p
	}
	path := make([]string, len(reversed))
	for i, id := range reversed {
		path[len(reversed)-1-i] = id
	}
	return path
}
