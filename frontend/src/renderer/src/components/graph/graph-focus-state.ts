/**
 * Focus (neighborhood) state — FE-REQ-TASK-032-04
 *
 * @module components/graph/graph-focus-state
 */

import type { GraphEdge } from '../../../../shared/graph-types'

export type GraphFocusState = { focusId: string | null }
export type GraphFocusAction = { type: 'focus'; id: string } | { type: 'clear' }

/** The node itself plus its one-step neighbors (undirected). */
export function focusNeighborhood(edges: readonly GraphEdge[], id: string): Set<string> {
  const set = new Set<string>([id])
  for (const e of edges) {
    if (e.from === id) {set.add(e.to)}
    if (e.to === id) {set.add(e.from)}
  }
  return set
}

export function focusReducer(state: GraphFocusState, action: GraphFocusAction): GraphFocusState {
  if (action.type === 'clear') {return state.focusId === null ? state : { focusId: null }}
  // Why: re-selecting the focused node toggles focus off so Esc is not the only exit.
  return state.focusId === action.id ? { focusId: null } : { focusId: action.id }
}

export function isDimmed(nodeId: string, focusSet: ReadonlySet<string> | null): boolean {
  return focusSet !== null && !focusSet.has(nodeId)
}
