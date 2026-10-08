/**
 * finding-graph-target.ts — FE-CV-TASK-059-06
 *
 * Where "View in graph" goes for a finding. Pure: the graph's node set and the lenses that can
 * actually be shown are passed in. Never guesses: no table param means no ERD target.
 *
 * @module components/review-map/findings/finding-graph-target
 */

import type { Finding } from '../../../../../shared/code-intel-types'
import { normalizeFindingKind } from './finding-view-model'

export type FindingGraphTarget =
  | { lens: 'structure' | 'impact'; symbolKey: string | null }
  | { lens: 'erd'; table: string; service: string | null }

export function resolveFindingGraphTarget(
  finding: Pick<Finding, 'kind' | 'evidence' | 'params' | 'scope'>,
  ctx: { availableLensIds: ReadonlySet<string>; graphSymbolKeys: ReadonlySet<string> | null }
): FindingGraphTarget | null {
  const kind = normalizeFindingKind(finding.kind)
  const symbolKey = finding.evidence?.[0]?.symbol?.key ?? null
  switch (kind) {
    case 'layer_violation':
    case 'dependency_cycle': {
      if (!symbolKey || !ctx.graphSymbolKeys?.has(symbolKey)) {
        return null
      }
      if (ctx.availableLensIds.has('structure')) {
        return { lens: 'structure', symbolKey }
      }
      return ctx.availableLensIds.has('impact') ? { lens: 'impact', symbolKey } : null
    }
    case 'hotspot':
    case 'dead_code':
      return ctx.availableLensIds.has('structure') ? { lens: 'structure', symbolKey } : null
    case 'missing_tenant_id':
    case 'rls_removed': {
      const table = finding.params?.table
      if (!table || !ctx.availableLensIds.has('erd')) {
        return null
      }
      return { lens: 'erd', table, service: finding.scope?.service ?? null }
    }
    default:
      return null
  }
}

export type FindingGraphActions = {
  setReviewLens: (worktreeId: string, lens: string) => void
  selectReviewSymbol: (worktreeId: string, symbolKey: string | null) => void
  setErdService: (worktreeId: string, service: string | null) => void
  selectErdTable: (worktreeId: string, tableKey: string | null) => void
}

export function openFindingInGraph(
  worktreeId: string,
  target: FindingGraphTarget,
  actions: FindingGraphActions
): void {
  actions.setReviewLens(worktreeId, target.lens)
  if (target.lens === 'erd') {
    if (target.service) {
      actions.setErdService(worktreeId, target.service)
    }
    actions.selectErdTable(worktreeId, target.table)
    return
  }
  if (target.symbolKey) {
    actions.selectReviewSymbol(worktreeId, target.symbolKey)
  }
}
