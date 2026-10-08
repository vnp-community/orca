/**
 * storage-change-marks.ts — FE-CV-TASK-058-01
 *
 * Which nodes/edges the change touches. Order of trust: backend `change`, then evidence
 * paths intersected with changed files, then `related` neighbours. Topics have no backend
 * `change`, so their mark is inferred from evidence and may be wrong when evidence is thin.
 */

import type { StorageEdge, StorageNode, StorageViewModel } from './storage-view-model'

export type StorageMark = 'added' | 'modified' | 'removed' | 'related'

/** Symbols travel with the data so components never rely on colour alone. */
export const STORAGE_MARK_SYMBOL: Record<StorageMark, string> = {
  added: '+',
  modified: '~',
  removed: '−',
  related: '·'
}

export type StorageChangedFile = { path: string; status?: string }

export type StorageChangeMarks = {
  marks: Map<string, StorageMark>
  /** Neither backend `change` nor evidence exists, so nothing can be attributed. */
  unknown: boolean
}

export function normalizeStoragePath(path: string): string {
  return path.replace(/\\/g, '/').replace(/^\.\//, '')
}

function markFromFile(status: string | undefined): StorageMark {
  if (status === 'added' || status === 'untracked') {
    return 'added'
  }
  if (status === 'deleted') {
    return 'removed'
  }
  return 'modified'
}

function directMark(
  item: Pick<StorageNode | StorageEdge, 'evidencePaths' | 'backendChange'>,
  files: Map<string, StorageChangedFile>
): StorageMark | null {
  if (item.backendChange) {
    return item.backendChange
  }
  for (const p of item.evidencePaths) {
    const file = files.get(normalizeStoragePath(p))
    if (file) {
      return markFromFile(file.status)
    }
  }
  return null
}

export function computeStorageChangeMarks(
  vm: Pick<StorageViewModel, 'nodes' | 'edges'>,
  changedFiles: readonly StorageChangedFile[]
): StorageChangeMarks {
  const files = new Map(changedFiles.map((f) => [normalizeStoragePath(f.path), f]))
  const marks = new Map<string, StorageMark>()
  const items = [...vm.nodes, ...vm.edges]
  for (const item of items) {
    const mark = directMark(item, files)
    if (mark) {
      marks.set(item.id, mark)
    }
  }
  // Why: services have no evidence of their own; they are context for the edges around them.
  for (const edge of vm.edges) {
    if (!marks.has(edge.id)) {
      continue
    }
    for (const end of [edge.from, edge.to]) {
      if (!marks.has(end)) {
        marks.set(end, 'related')
      }
    }
  }
  const direct = new Set([...marks.entries()].filter(([, m]) => m !== 'related').map(([id]) => id))
  for (const edge of vm.edges) {
    const touchesDirect = direct.has(edge.from) || direct.has(edge.to)
    if (touchesDirect && !marks.has(edge.id)) {
      marks.set(edge.id, 'related')
    }
    if (touchesDirect) {
      for (const end of [edge.from, edge.to]) {
        if (!marks.has(end)) {
          marks.set(end, 'related')
        }
      }
    }
  }
  const hasEvidence = items.some((i) => i.evidencePaths.length > 0 || i.backendChange)
  return { marks, unknown: changedFiles.length > 0 && !hasEvidence }
}
