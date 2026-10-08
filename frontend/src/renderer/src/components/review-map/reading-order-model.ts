/**
 * reading-order-model.ts — FE-CV-TASK-052-01
 *
 * Pure row model: one row = one ReadingStep (file), grouped by ComponentGroup.
 * Filters hide rows but never change totals, so progress always counts the whole list.
 */

import type {
  ChangedFileView,
  ComponentGroupView,
  ReadingProgress,
  ReadingStepView
} from './review-wire-types'

export const OTHER_GROUP_ID = '__other__'

export type ReadingOrderItem = ReadingStepView & {
  fileStatus: ChangedFileView['status']
  component: { id: string; label: string } | null
}

export type ReadingOrderGroupRow = {
  type: 'group'
  id: string
  label: string
  collapsed: boolean
  /** Whole-group totals (unaffected by filter/collapse). */
  total: number
  seen: number
  stepKeys: string[]
}

export type ReadingOrderStepRow = {
  type: 'step'
  id: string
  groupId: string
  item: ReadingOrderItem
}

export type ReadingOrderRow = ReadingOrderGroupRow | ReadingOrderStepRow

const FILE_STATUSES = new Set(['added', 'modified', 'deleted', 'renamed', 'copied', 'untracked'])

/** Duplicate stepKey keeps the first; a step in several groups goes to the first group. */
export function buildReadingOrderItems(
  steps: readonly ReadingStepView[],
  components: readonly ComponentGroupView[],
  changedFiles: readonly ChangedFileView[]
): ReadingOrderItem[] {
  const componentByStep = new Map<string, { id: string; label: string }>()
  for (const group of components) {
    for (const key of group.stepKeys) {
      if (!componentByStep.has(key)) {
        componentByStep.set(key, { id: group.componentId, label: group.label })
      }
    }
  }
  const statusByPath = new Map(changedFiles.map((f) => [f.path, f.status]))
  const seen = new Set<string>()
  const items: ReadingOrderItem[] = []
  for (const step of steps) {
    if (seen.has(step.stepKey)) {
      continue
    }
    seen.add(step.stepKey)
    const status = statusByPath.get(step.file)
    items.push({
      ...step,
      fileStatus: status && FILE_STATUSES.has(status) ? status : 'unknown',
      component: componentByStep.get(step.stepKey) ?? null
    })
  }
  return items.sort((a, b) => a.n - b.n)
}

export type BuildRowsOptions = {
  collapsedGroupIds: ReadonlySet<string>
  /** File paths kept visible; null = no filter. */
  filter: ReadonlySet<string> | null
  progress?: ReadingProgress
}

export function buildReadingOrderRows(
  items: readonly ReadingOrderItem[],
  { collapsedGroupIds, filter, progress }: BuildRowsOptions
): ReadingOrderRow[] {
  const groups = new Map<string, { label: string; items: ReadingOrderItem[]; minN: number }>()
  for (const item of items) {
    const id = item.component?.id ?? OTHER_GROUP_ID
    const g = groups.get(id) ?? { label: item.component?.label ?? '', items: [], minN: item.n }
    g.items.push(item)
    g.minN = Math.min(g.minN, item.n)
    groups.set(id, g)
  }
  const ordered = [...groups.entries()].sort(([ida, a], [idb, b]) => {
    if (ida === OTHER_GROUP_ID) {
      return 1
    }
    if (idb === OTHER_GROUP_ID) {
      return -1
    }
    return a.minN - b.minN
  })

  const rows: ReadingOrderRow[] = []
  for (const [id, group] of ordered) {
    const visible = filter ? group.items.filter((i) => filter.has(i.file)) : group.items
    if (visible.length === 0) {
      continue
    }
    const collapsed = collapsedGroupIds.has(id)
    const stepKeys = group.items.map((i) => i.stepKey)
    rows.push({
      type: 'group',
      id,
      label: group.label,
      collapsed,
      total: stepKeys.length,
      seen: progress ? stepKeys.filter((k) => progress.entries[k]?.state === 'seen').length : 0,
      stepKeys
    })
    if (collapsed) {
      continue
    }
    for (const item of visible) {
      rows.push({ type: 'step', id: item.stepKey, groupId: id, item })
    }
  }
  return rows
}

export function countReadingOrderTotals(
  items: readonly ReadingOrderItem[],
  progress: ReadingProgress
): { total: number; seen: number } {
  // overflow marker is not a reviewable file
  const reviewable = items.filter((i) => i.reason !== 'overflow')
  return {
    total: reviewable.length,
    seen: reviewable.filter((i) => progress.entries[i.stepKey]?.state === 'seen').length
  }
}
