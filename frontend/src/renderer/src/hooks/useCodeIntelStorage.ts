/**
 * useCodeIntelStorage.ts — FE-CV-TASK-058-03
 *
 * Loads the `storage` channel for one environment. A `changed` push only raises
 * `isStale`; the lens shows a chip and never reloads mid-interaction.
 */

import { useMemo } from 'react'
import type { StorageMap } from '../../../shared/code-intel-architecture-types'
import { useCodeIntelQuery } from './useCodeIntelQuery'
import type {
  CodeIntelCallFn,
  CodeIntelQueryError,
  CodeIntelQueryMeta,
  CodeIntelQueryStatus
} from './useCodeIntelQuery'

export type StorageEnv = 'dev' | 'prod'

function arr<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : []
}

export function parseStorageMap(raw: unknown): StorageMap {
  if (typeof raw !== 'object' || raw === null) {
    throw new Error('storage: not an object')
  }
  const m = raw as StorageMap
  return {
    ...m,
    stores: arr(m.stores),
    bindings: arr(m.bindings),
    topics: arr<StorageMap['topics'][number]>(m.topics).map((t) => ({
      ...t,
      publishers: arr(t.publishers),
      subscribers: arr(t.subscribers),
      evidence: arr(t.evidence)
    })),
    sources: arr(m.sources),
    redactedCount: typeof m.redactedCount === 'number' ? m.redactedCount : 0,
    warnings: arr<string>(m.warnings).filter((w) => typeof w === 'string')
  }
}

/** False when the backend cannot serve `storage`: the lens then shows a plain notice, no retry. */
export function isStorageLensAvailable(errorKind: string | null | undefined): boolean {
  return errorKind !== 'unsupported' && errorKind !== 'disabled'
}

export type UseCodeIntelStorageArgs = {
  worktreeId: string
  environmentId: string | null
  env?: StorageEnv
  includeLegacy?: boolean
  enabled?: boolean
  /** Test seam; production uses the code-intel client. */
  callFn?: CodeIntelCallFn
}

export type UseCodeIntelStorage = {
  status: CodeIntelQueryStatus
  data: StorageMap | null
  meta: CodeIntelQueryMeta | null
  error: CodeIntelQueryError | null
  truncated: boolean
  isStale: boolean
  available: boolean
  reload: () => void
}

export function useCodeIntelStorage(args: UseCodeIntelStorageArgs): UseCodeIntelStorage {
  const {
    worktreeId,
    environmentId,
    env = 'dev',
    includeLegacy = false,
    enabled = true,
    callFn
  } = args
  const params = useMemo(
    () => ({ env, ...(includeLegacy ? { includeLegacy: true } : {}) }),
    [env, includeLegacy]
  )
  const query = useCodeIntelQuery<StorageMap>(
    worktreeId,
    environmentId,
    { method: 'storage', params, enabled, parseResult: parseStorageMap },
    callFn
  )
  return {
    status: query.status,
    // Why: the previous environment's map must not show while the next one loads.
    data: query.status === 'success' ? query.data : null,
    meta: query.meta,
    error: query.error,
    truncated: query.truncated,
    isStale: query.staleSignal,
    available: isStorageLensAvailable(query.error?.kind),
    reload: query.refetch
  }
}
