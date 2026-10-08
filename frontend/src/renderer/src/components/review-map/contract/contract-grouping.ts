/**
 * contract-grouping.ts — FE-CV-TASK-059-01
 *
 * Groups ContractChange rows by service then kind group and filters them. The UI never
 * classifies compatibility: it maps the backend value, and anything unrecognised is `unknown`.
 *
 * @module components/review-map/contract/contract-grouping
 */

import type { ContractChange } from '../../../../../shared/code-intel-types'

export type CompatibilityState = 'breaking' | 'risky' | 'compatible' | 'unknown'
export type ContractKindGroup = 'proto' | 'ws-channel' | 'route' | 'migration' | 'unknown'
export type ContractChangeKind = 'added' | 'removed' | 'modified' | 'unknown'

export const NO_SERVICE_GROUP_KEY = ''

const COMPAT_RANK: Record<CompatibilityState, number> = {
  breaking: 0,
  risky: 1,
  unknown: 2,
  compatible: 3
}

export function normalizeCompatibility(value: unknown): CompatibilityState {
  return value === 'breaking' || value === 'risky' || value === 'compatible' ? value : 'unknown'
}

export function normalizeChangeKind(value: unknown): ContractChangeKind {
  return value === 'added' || value === 'removed' || value === 'modified' ? value : 'unknown'
}

export function contractKindGroup(kind: string): ContractKindGroup {
  if (kind.startsWith('proto-')) {
    return 'proto'
  }
  if (kind === 'ws-channel' || kind === 'ws-channel-arg') {
    return 'ws-channel'
  }
  if (kind === 'route' || kind === 'route-field') {
    return 'route'
  }
  if (kind === 'sql-table' || kind === 'sql-column') {
    return 'migration'
  }
  return 'unknown'
}

export type CompatibilityCounts = Record<CompatibilityState, number>

export function emptyCompatibilityCounts(): CompatibilityCounts {
  return { breaking: 0, risky: 0, compatible: 0, unknown: 0 }
}

export function countByCompatibility(changes: readonly ContractChange[]): CompatibilityCounts {
  const counts = emptyCompatibilityCounts()
  for (const change of changes) {
    counts[normalizeCompatibility(change.compatibility)] += 1
  }
  return counts
}

export type ContractKindBucket = {
  kindGroup: ContractKindGroup
  changes: ContractChange[]
}

export type ContractServiceGroup = {
  /** '' when the backend gave no service. */
  service: string
  kinds: ContractKindBucket[]
  counts: CompatibilityCounts
  total: number
}

function compareChanges(a: ContractChange, b: ContractChange): number {
  const byCompat =
    COMPAT_RANK[normalizeCompatibility(a.compatibility)] -
    COMPAT_RANK[normalizeCompatibility(b.compatibility)]
  if (byCompat !== 0) {
    return byCompat
  }
  return a.name < b.name ? -1 : a.name > b.name ? 1 : a.id < b.id ? -1 : a.id > b.id ? 1 : 0
}

/** Services with breaking changes first, then risky, then alphabetical; "no service" last. */
export function groupContractChanges(changes: readonly ContractChange[]): ContractServiceGroup[] {
  const byService = new Map<string, Map<ContractKindGroup, ContractChange[]>>()
  for (const change of changes) {
    const service = change.service ?? NO_SERVICE_GROUP_KEY
    const kindGroup = contractKindGroup(change.kind)
    let kinds = byService.get(service)
    if (!kinds) {
      kinds = new Map()
      byService.set(service, kinds)
    }
    const list = kinds.get(kindGroup)
    if (list) {
      list.push(change)
    } else {
      kinds.set(kindGroup, [change])
    }
  }
  const groups: ContractServiceGroup[] = []
  for (const [service, kinds] of byService) {
    const buckets: ContractKindBucket[] = [...kinds.entries()]
      .map(([kindGroup, list]) => ({ kindGroup, changes: [...list].sort(compareChanges) }))
      .sort((a, b) => (a.kindGroup < b.kindGroup ? -1 : a.kindGroup > b.kindGroup ? 1 : 0))
    const all = buckets.flatMap((b) => b.changes)
    groups.push({ service, kinds: buckets, counts: countByCompatibility(all), total: all.length })
  }
  return groups.sort((a, b) => {
    if ((a.service === NO_SERVICE_GROUP_KEY) !== (b.service === NO_SERVICE_GROUP_KEY)) {
      return a.service === NO_SERVICE_GROUP_KEY ? 1 : -1
    }
    if (a.counts.breaking !== b.counts.breaking) {
      return b.counts.breaking - a.counts.breaking
    }
    if (a.counts.risky !== b.counts.risky) {
      return b.counts.risky - a.counts.risky
    }
    return a.service < b.service ? -1 : a.service > b.service ? 1 : 0
  })
}

export type ContractFilter = {
  kinds?: readonly ContractKindGroup[]
  service?: string | null
  onlyBreaking?: boolean
  /** Keep only changes whose files intersect `changedFiles` (touched by this change set). */
  onlyChangedByAgent?: boolean
  changedFiles?: ReadonlySet<string>
  /** Chip filter from the summary bar. */
  compatibility?: CompatibilityState | null
  query?: string
}

export function filterContractChanges(
  changes: readonly ContractChange[],
  filter: ContractFilter
): ContractChange[] {
  const query = filter.query?.trim().toLowerCase() ?? ''
  return changes.filter((change) => {
    const compat = normalizeCompatibility(change.compatibility)
    if (filter.onlyBreaking && compat !== 'breaking') {
      return false
    }
    if (filter.compatibility && compat !== filter.compatibility) {
      return false
    }
    if (filter.kinds && filter.kinds.length > 0 && !filter.kinds.includes(contractKindGroup(change.kind))) {
      return false
    }
    if (filter.service != null && filter.service !== '' && (change.service ?? '') !== filter.service) {
      return false
    }
    if (filter.onlyChangedByAgent) {
      const changed = filter.changedFiles
      if (!changed || !change.files.some((file) => changed.has(file))) {
        return false
      }
    }
    if (query) {
      const haystack = [change.name, change.service ?? '', change.ruleId, ...change.files]
        .join('\n')
        .toLowerCase()
      if (!haystack.includes(query)) {
        return false
      }
    }
    return true
  })
}

export function listContractServices(changes: readonly ContractChange[]): string[] {
  return [...new Set(changes.map((c) => c.service).filter((s): s is string => Boolean(s)))].sort()
}
