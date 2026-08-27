package domain

import "errors"

// EdgeKind distinguishes the two relations task_edges rows can encode — see
// specs/backend-go/services/task-service.md §4/§5. One table, one
// discriminator column, so GetSubtree/GetAncestors/GetDependencies reuse one
// recursive-CTE shape instead of duplicating it per relation.
type EdgeKind string

const (
	// EdgeKindParentChild is the hierarchy relation, walked by
	// GetSubtree/GetAncestors. At most one parent_child edge exists per
	// child task (DB-enforced via a unique index on to_task_id, see
	// migrations/0001_init.up.sql).
	EdgeKindParentChild EdgeKind = "parent_child"
	// EdgeKindDependsOn is the ordering relation, walked by GetDependencies
	// and the complex-Execute coordination path. Unlike parent_child, a
	// task may have many depends_on edges, and it's this relation the
	// CycleDetector protects — a dependency DAG with a cycle can never be
	// scheduled.
	EdgeKindDependsOn EdgeKind = "depends_on"
)

func (k EdgeKind) Valid() bool {
	switch k {
	case EdgeKindParentChild, EdgeKindDependsOn:
		return true
	default:
		return false
	}
}

// ErrInvalidEdgeKind is returned by NewTaskEdge for an unrecognized kind.
var ErrInvalidEdgeKind = errors.New("domain: invalid task edge kind")

// ErrSelfEdge guards against a task depending on (or parenting) itself — the
// smallest possible cycle, cheap to reject before ever touching the graph
// walk.
var ErrSelfEdge = errors.New("domain: a task cannot have an edge to itself")

// TaskEdge is one row of the task_edges table — a directed edge from
// FromTaskID to ToTaskID of a given Kind. For EdgeKindDependsOn, "from
// depends on to" reads as "from must wait for to" (TS TaskDAGValidator's
// convention, carried forward as-is per §10).
type TaskEdge struct {
	FromTaskID string
	ToTaskID   string
	Kind       EdgeKind
}

// NewTaskEdge constructs a TaskEdge, enforcing the two invariants cheap
// enough to check without the rest of the graph: a known kind, and no
// self-edges.
func NewTaskEdge(fromTaskID, toTaskID string, kind EdgeKind) (TaskEdge, error) {
	if !kind.Valid() {
		return TaskEdge{}, ErrInvalidEdgeKind
	}
	if fromTaskID == toTaskID {
		return TaskEdge{}, ErrSelfEdge
	}
	return TaskEdge{FromTaskID: fromTaskID, ToTaskID: toTaskID, Kind: kind}, nil
}

// ErrCyclicDependency is returned by the AddEdge usecase (via
// apperrors.KindFailedPrecondition) when committing a proposed depends_on
// edge would create a cycle in the dependency graph.
var ErrCyclicDependency = errors.New("domain: adding this edge would create a cyclic dependency")

// DetectCycle reports whether adding newEdge to the graph described by edges
// would create a cycle. Same algorithm as TS TaskDAGValidator, carried
// forward as-is per task-service.md §4/§10: a directed edge from -> to
// creates a cycle exactly when "to" can already reach "from" via the
// existing edge set — walk that reachability with a plain BFS/DFS over
// edges of newEdge.Kind only (a depends_on edge can't be short-circuited by
// an unrelated parent_child edge, and vice versa).
//
// Pure function: no DB, no context.Context, unit-testable against an
// in-memory edge list per task-service.md §6's note on
// domain/grant_resolution.go applying equally here.
func DetectCycle(edges []TaskEdge, newEdge TaskEdge) bool {
	if newEdge.FromTaskID == newEdge.ToTaskID {
		return true // a self-edge is trivially cyclic
	}

	adjacency := make(map[string][]string, len(edges))
	for _, e := range edges {
		if e.Kind != newEdge.Kind {
			continue
		}
		adjacency[e.FromTaskID] = append(adjacency[e.FromTaskID], e.ToTaskID)
	}

	// BFS from newEdge.ToTaskID: if we can reach newEdge.FromTaskID, then
	// the existing graph already has a path to -> ... -> from, and adding
	// from -> to would close the loop back to "from".
	visited := map[string]bool{newEdge.ToTaskID: true}
	queue := []string{newEdge.ToTaskID}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if node == newEdge.FromTaskID {
			return true
		}
		for _, next := range adjacency[node] {
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}
	return false
}
