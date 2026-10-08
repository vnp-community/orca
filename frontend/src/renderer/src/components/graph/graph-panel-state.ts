/**
 * GraphPanel state — FE-REQ-TASK-032-06
 *
 * GraphPanel is the only owner of selection, open groups, lens and view mode.
 *
 * @module components/graph/graph-panel-state
 */

import type { GraphChangeView } from './graph-before-after'
import type { GraphLens, GraphNode } from '../../../../shared/graph-types'

export type GraphPanelView = 'graph' | 'list'

export type GraphPanelState = {
  lens: GraphLens
  view: GraphPanelView
  changeView: GraphChangeView
  selectedId: string | null
  focusId: string | null
  openGroups: ReadonlySet<string>
  searchOpen: boolean
  sheetNodeId: string | null
  fitViewSignal: number
}

export type GraphPanelAction =
  | { type: 'lens'; lens: GraphLens }
  | { type: 'view'; view: GraphPanelView }
  | { type: 'changeView'; changeView: GraphChangeView }
  | { type: 'select'; id: string | null }
  | { type: 'focus'; id: string | null }
  | { type: 'toggleGroup'; id: string }
  | { type: 'search'; open: boolean }
  | { type: 'sheet'; id: string | null }
  | { type: 'searchPick'; node: GraphNode }

export function createGraphPanelState(lens: GraphLens, view: GraphPanelView = 'graph'): GraphPanelState {
  return {
    lens, view, changeView: 'after', selectedId: null, focusId: null,
    openGroups: new Set(), searchOpen: false, sheetNodeId: null, fitViewSignal: 0
  }
}

export function initialView(input: { truncated: boolean; narrow: boolean; screenReaderMode?: boolean }): GraphPanelView {
  return input.truncated || input.narrow || input.screenReaderMode === true ? 'list' : 'graph'
}

export function graphPanelReducer(state: GraphPanelState, action: GraphPanelAction): GraphPanelState {
  switch (action.type) {
    case 'lens':
      if (action.lens === state.lens) {return state}
      // Why: node ids differ per lens, so selection, focus and open groups would dangle.
      return { ...state, lens: action.lens, selectedId: null, focusId: null, openGroups: new Set(), sheetNodeId: null }
    case 'view':
      return { ...state, view: action.view }
    case 'changeView':
      return { ...state, changeView: action.changeView }
    case 'select':
      return { ...state, selectedId: action.id }
    case 'focus':
      return { ...state, focusId: action.id }
    case 'toggleGroup': {
      const next = new Set(state.openGroups)
      if (next.has(action.id)) {next.delete(action.id)} else {next.add(action.id)}
      return { ...state, openGroups: next }
    }
    case 'search':
      return { ...state, searchOpen: action.open }
    case 'sheet':
      return { ...state, sheetNodeId: action.id }
    case 'searchPick':
      return selectNodeFromSearch(state, action.node)
  }
}

export function selectNodeFromSearch(state: GraphPanelState, node: GraphNode): GraphPanelState {
  const next = new Set(state.openGroups)
  if (node.group !== null) {next.add(node.group)}
  return { ...state, openGroups: next, selectedId: node.id, searchOpen: false, fitViewSignal: state.fitViewSignal + 1 }
}

function fold(s: string): string {
  return s.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/đ/g, 'd')
}

export const GRAPH_SEARCH_RESULT_LIMIT = 50

export function filterGraphNodes(nodes: readonly GraphNode[], query: string, limit = GRAPH_SEARCH_RESULT_LIMIT): GraphNode[] {
  const q = fold(query.trim())
  const out: GraphNode[] = []
  for (const n of nodes) {
    if (q === '' || fold(n.label).includes(q) || fold(n.kind).includes(q) || fold(n.group ?? '').includes(q)) {
      out.push(n)
      if (out.length >= limit) {break}
    }
  }
  return out
}

/** True when a `/` key press should open search: not while typing in a field. */
export function shouldOpenSearchOnKey(event: { key: string; target: EventTarget | null; metaKey?: boolean; ctrlKey?: boolean; altKey?: boolean }): boolean {
  if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) {return false}
  const el = event.target as HTMLElement | null
  if (!el || typeof el.closest !== 'function') {return true}
  return !el.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"]')
}

export type LensDisabledReason = 'noAssessment' | 'noPlan' | 'notExecuting' | 'unsupported'

export type LensAvailabilityContext = {
  /** undefined = unknown, treated as available. */
  impactAssessed?: boolean
  backendUnsupported: boolean
  hasPlan: boolean
  executing: boolean
}

/** Why: chips are never hidden; a disabled chip carries the reason in its tooltip. */
export function lensDisabledReason(lens: GraphLens, ctx: LensAvailabilityContext): LensDisabledReason | null {
  switch (lens) {
    case 'flow':
      return null
    case 'plan':
      return ctx.hasPlan ? null : 'noPlan'
    case 'execution':
      return ctx.executing ? null : 'notExecuting'
    default:
      if (ctx.backendUnsupported) {return 'unsupported'}
      return ctx.impactAssessed === false ? 'noAssessment' : null
  }
}
