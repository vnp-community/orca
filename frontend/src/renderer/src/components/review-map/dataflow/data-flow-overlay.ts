/**
 * data-flow-overlay.ts — FE-CV-TASK-056-03
 *
 * Step rows and change flags for the data-flow lens. Pure; the overlay is passed in.
 */

import type { DataFlow, DataFlowStep, DataFlowSummary } from '../../../../../shared/code-intel-architecture-types'
import type { ChangeOverlayView } from '../review-wire-types'

export type StepRowFlags = {
  changed: boolean
  untested: boolean
  unimplemented: boolean
  /** False when the step is cut off from the (truncated) diagram. */
  inDiagram: boolean
}

export type DataFlowStore = DataFlow['stores'][number]

export type DataFlowRow =
  | { type: 'step'; step: DataFlowStep; flags: StepRowFlags; stores: DataFlowStore[] }
  | { type: 'gap'; afterStep: number; code: string; message: string }

type OverlayInput = Pick<ChangeOverlayView, 'changedSymbols' | 'uncoveredSymbols' | 'touchedTables'>

function entityId(v: unknown, fields: readonly string[]): string | null {
  if (typeof v === 'string') {
    return v
  }
  if (typeof v === 'object' && v !== null) {
    for (const f of fields) {
      const x = (v as Record<string, unknown>)[f]
      if (typeof x === 'string') {
        return x
      }
    }
  }
  return null
}

function touchedTableNames(overlay: Pick<ChangeOverlayView, 'touchedTables'>): Set<string> {
  const out = new Set<string>()
  for (const t of overlay.touchedTables ?? []) {
    const name = entityId(t, ['table', 'name', 'id'])
    if (name) {
      out.add(name.toLowerCase())
    }
  }
  return out
}

function stepSymbolKeys(step: DataFlowStep): string[] {
  return [...(step.symbol ? [step.symbol.key] : []), ...(step.evidence ?? []).map((e) => e.key)]
}

function storesOf(flow: DataFlow, n: number): DataFlowStore[] {
  return (flow.stores ?? []).filter((s) => s.step === n)
}

function isStepChanged(
  flow: DataFlow,
  step: DataFlowStep,
  changedKeys: ReadonlySet<string>,
  tables: ReadonlySet<string>
): boolean {
  if (stepSymbolKeys(step).some((k) => changedKeys.has(k))) {
    return true
  }
  return storesOf(flow, step.n).some((s) => {
    const names = [s.table, s.store?.name].filter((x): x is string => typeof x === 'string')
    return names.some((n) => tables.has(n.toLowerCase()))
  })
}

/** Steps (with gap rows after `afterStep`) in flow order. */
export function buildStepRows(flow: DataFlow, overlay: OverlayInput, renderedMessages: number): DataFlowRow[] {
  const changedKeys = new Set(overlay.changedSymbols.map((c) => c.symbol.key))
  const uncovered = new Set(overlay.uncoveredSymbols.map((s) => s.key))
  const tables = touchedTableNames(overlay)
  const gaps = flow.gaps ?? []
  const rows: DataFlowRow[] = []
  const pushGaps = (after: number): void => {
    for (const g of gaps) {
      if (g.afterStep === after) {
        rows.push({ type: 'gap', afterStep: g.afterStep, code: g.code, message: g.message })
      }
    }
  }
  pushGaps(0)
  flow.steps.forEach((step, index) => {
    rows.push({
      type: 'step',
      step,
      stores: storesOf(flow, step.n),
      flags: {
        changed: isStepChanged(flow, step, changedKeys, tables),
        untested: stepSymbolKeys(step).some((k) => uncovered.has(k)),
        unimplemented: step.unimplemented === true,
        inDiagram: index < renderedMessages
      }
    })
    pushGaps(step.n)
  })
  return rows
}

/** Message numbers that changed; assumes SequenceModel.messages[].n equals DataFlowStep.n. */
export function changedMessageSet(flow: DataFlow, overlay: OverlayInput): Set<number> {
  const changedKeys = new Set(overlay.changedSymbols.map((c) => c.symbol.key))
  const tables = touchedTableNames(overlay)
  const out = new Set<number>()
  for (const step of flow.steps) {
    if (isStepChanged(flow, step, changedKeys, tables)) {
      out.add(step.n)
    }
  }
  return out
}

export type ChangeFilterResult = {
  /** Ids of listed flows that touch the change. */
  ids: Set<string>
  /** True when nothing intersects: we do not know, which is not the same as "none". */
  unknown: boolean
}

export function flowTouchesChange(summary: Pick<DataFlowSummary, 'id'>, overlay: Pick<ChangeOverlayView, 'affectedFlows'>): boolean {
  return (overlay.affectedFlows ?? []).some((f) => entityId(f, ['id', 'flowId', 'processId']) === summary.id)
}

export function filterFlowsTouchingChange(
  summaries: readonly Pick<DataFlowSummary, 'id'>[],
  overlay: Pick<ChangeOverlayView, 'affectedFlows'>
): ChangeFilterResult {
  const ids = new Set(summaries.filter((s) => flowTouchesChange(s, overlay)).map((s) => s.id))
  return { ids, unknown: ids.size === 0 }
}
