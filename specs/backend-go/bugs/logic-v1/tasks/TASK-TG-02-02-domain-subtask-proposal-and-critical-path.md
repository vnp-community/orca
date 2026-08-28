# TASK-TG-02-02: Domain — widen `SubtaskProposal`, add `CalculateCriticalPath`

**From Solution:** SOL-TG-02
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/subtask_proposal.go`, `backend-go/services/task-service/internal/domain/critical_path.go` (new)
**Depends on:** none (pure domain)
**Status:** `[x]` DONE — domain.SubtaskProposal widened; CalculateCriticalPath (Kahn topo-sort + longest-path) added with a tie-break fix (>= not >) so an all-zero-hours chain still walks the full path; go test ./internal/domain/... -run TestCalculateCriticalPath passes (diamond DAG, parallel chains, empty, all-zero-hours, parent_child-ignored cases).

---

## Context

`domain.SubtaskProposal` only has `Title`/`Description`. `CalculateCriticalPath`
is the spec's DAG/topological-sort algorithm over `estimated_hours` — a pure
function of already-fetched `TaskEdge`s and a task-ID→hours map, same
discipline as `DetectCycle`/`ResolveGrant`.

## Changes to make

Replace `backend-go/services/task-service/internal/domain/subtask_proposal.go`:

```go
package domain

// SubtaskProposal is an AI-generated, not-yet-persisted subtask suggestion
// — the review-before-commit shape AIDecompose/AIApply share. Never itself
// written to task_edges; it has no ID until AIApply creates the real Task.
type SubtaskProposal struct {
	Title       string
	Description string
	Type        string // task|bug|feature — mirrors Task.Type
	EstimatedHours *float64
	// DependsOnIndices names OTHER proposals in the SAME AIDecompose
	// response by their 0-based position, e.g. proposal[2] depends on
	// proposal[0] -> DependsOnIndices: []int{0}.
	DependsOnIndices []int
	PromptTemplate   string
}
```

Create `backend-go/services/task-service/internal/domain/critical_path.go`:

```go
package domain

// CalculateCriticalPath returns the longest-duration path through the
// depends_on DAG (rooted implicitly at whichever nodes have no incoming
// depends_on edge), plus its total duration — the standard CPM longest-path
// algorithm. Assumes the edge set is already acyclic (DetectCycle is the
// enforcement point, at AddEdge/AIApply time) — this function does not
// re-validate. hours entries default to 0 when a task ID is absent from the
// map (spec: "if AI/user leaves estimate blank" is never a computation
// error).
func CalculateCriticalPath(edges []TaskEdge, hours map[string]float64) (path []string, totalHours float64) {
	adjacency, indegree, nodes := buildDependsOnGraph(edges)
	order := topologicalSort(adjacency, indegree, nodes) // Kahn's algorithm; empty if a cycle slipped through (defensive, not expected)

	longest := make(map[string]float64, len(order))
	predecessor := make(map[string]string, len(order))
	best := ""
	for _, id := range order {
		longest[id] = hours[id]
		for _, from := range incomingOf(id, edges) {
			if candidate := longest[from] + hours[id]; candidate > longest[id] {
				longest[id] = candidate
				predecessor[id] = from
			}
		}
		if best == "" || longest[id] > longest[best] {
			best = id
		}
	}
	if best == "" {
		return nil, 0
	}

	for id := best; id != ""; id = predecessor[id] {
		path = append([]string{id}, path...)
	}
	return path, longest[best]
}

// buildDependsOnGraph mirrors DetectCycle's per-kind-filtered adjacency
// build (task_edge.go), scoped to EdgeKindDependsOn, plus the full node set
// (every task ID appearing as either endpoint) and each node's in-degree
// for Kahn's algorithm.
func buildDependsOnGraph(edges []TaskEdge) (adjacency map[string][]string, indegree map[string]int, nodes []string) {
	adjacency = map[string][]string{}
	indegree = map[string]int{}
	seen := map[string]bool{}
	addNode := func(id string) {
		if !seen[id] {
			seen[id] = true
			nodes = append(nodes, id)
		}
	}
	for _, e := range edges {
		if e.Kind != EdgeKindDependsOn {
			continue
		}
		// "from depends on to" -> to must complete before from -> edge to->from in the DAG-walk sense
		adjacency[e.ToTaskID] = append(adjacency[e.ToTaskID], e.FromTaskID)
		indegree[e.FromTaskID]++
		addNode(e.FromTaskID)
		addNode(e.ToTaskID)
		if _, ok := indegree[e.ToTaskID]; !ok {
			indegree[e.ToTaskID] = 0
		}
	}
	return adjacency, indegree, nodes
}

func topologicalSort(adjacency map[string][]string, indegree map[string]int, nodes []string) []string {
	queue := make([]string, 0, len(nodes))
	remaining := make(map[string]int, len(indegree))
	for id, d := range indegree {
		remaining[id] = d
		if d == 0 {
			queue = append(queue, id)
		}
	}
	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, next := range adjacency[id] {
			remaining[next]--
			if remaining[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(order) != len(nodes) {
		return nil // a cycle slipped through — defensive, not expected per DetectCycle's enforcement
	}
	return order
}

func incomingOf(id string, edges []TaskEdge) []string {
	var out []string
	for _, e := range edges {
		if e.Kind == EdgeKindDependsOn && e.FromTaskID == id {
			out = append(out, e.ToTaskID)
		}
	}
	return out
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/internal/domain/...
go test ./services/task-service/internal/domain/... -run TestCalculateCriticalPath -v
```

Expected: new `critical_path_test.go` covers a diamond DAG, parallel
independent chains (longest one wins), a single-node graph, and an
all-zero-hours graph (degenerates to path length = node count, not a
divide-by-zero) — all pass.
