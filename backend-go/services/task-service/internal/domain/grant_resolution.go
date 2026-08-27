package domain

// DefaultMaxAncestorDepth bounds the ancestor walk ResolveGrant performs —
// task-service.md §8's "configurable max-depth guard so a malformed
// hierarchy can't turn a check into an unbounded walk." The usecase layer
// may pass a smaller value (e.g. from config); 0 or negative means
// "unbounded" (walk the whole chain the caller supplied).
const DefaultMaxAncestorDepth = 64

// ResolveGrant is the BFS ancestor walk from task-service.md §4.1 — a pure
// function of (ancestorChain, grantsByTask, caller) -> resolved level,
// carried forward faithfully from TS TaskGrantService.resolvePermission().
// No SQL, no gRPC, no context.Context: the usecase layer fetches
// ancestorChain (via TaskRepository.GetAncestors) and grantsByTask (via
// GrantRepository.ListGrantsForAncestors) and resolves caller's team
// membership (via TeamScopeResolver) before calling in.
//
// ancestorChain[0] must be the target task itself; ancestorChain[1:] are
// its ancestors in walk order (parent, grandparent, ... root) — exactly
// what TaskRepository.GetAncestors returns. Algorithm (§4.1):
//
//  1. At the target task (index 0), every grant on that task counts,
//     regardless of ApplyTree — a grant only needs apply_tree to be
//     inherited by descendants, not to apply to the task it's directly on.
//  2. At every ancestor above it (index > 0), only ApplyTree=true grants
//     count — a non-inherited grant on an ancestor has no bearing on a
//     descendant task.
//  3. The walk continues across the WHOLE chain (bounded by maxDepth, not
//     stopped at the first match) collecting every matching grant.
//  4. The highest-priority match across the whole walk wins:
//     owner > admin > user > team > company — priority wins over
//     proximity, matching TS semantics (a distant "owner" grant beats a
//     nearby "user" grant).
//  5. No match anywhere -> (GrantLevelUnspecified, false): default-deny,
//     left to the caller (ResolvePermission usecase / OPA per §9) to turn
//     into an actual allow/deny decision.
func ResolveGrant(ancestorChain []string, grantsByTask map[string][]Grant, caller CallerIdentity, maxDepth int) (GrantLevel, bool) {
	if maxDepth <= 0 {
		maxDepth = len(ancestorChain)
	}

	var best GrantLevel
	found := false

	for depth, taskID := range ancestorChain {
		if depth >= maxDepth {
			break
		}
		for _, g := range grantsByTask[taskID] {
			if depth > 0 && !g.ApplyTree {
				continue // only inherited grants count above the target task
			}
			if !g.Matches(caller) {
				continue
			}
			if !found || g.Level.priority() < best.priority() {
				best = g.Level
				found = true
			}
		}
	}

	if !found {
		return GrantLevelUnspecified, false
	}
	return best, true
}
