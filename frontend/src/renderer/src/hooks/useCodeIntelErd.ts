/**
 * useCodeIntelErd.ts — FE-CV-TASK-057-04
 *
 * Two-phase ERD load: `erd` without `service` lists services, then `erd` for the chosen
 * service/dialect returns the model. Stale-response dropping and the CODEINTEL_TIMEOUT
 * retry live in useCodeIntelQuery; a plain `changed` push only raises `isStale`.
 */

import { useMemo } from 'react'
import type {
  ErdModel,
  ErdTable,
  ErdServiceInfo
} from '../../../shared/code-intel-architecture-types'
import type { ReviewScope } from '../components/review-map/review-scope-model'
import { toChangeOverlayParams } from '../components/review-map/review-scope-model'
import { useCodeIntelQuery } from './useCodeIntelQuery'
import type {
  CodeIntelCallFn,
  CodeIntelQueryError,
  CodeIntelQueryMeta,
  CodeIntelQueryStatus
} from './useCodeIntelQuery'

export type ErdDialect = 'postgres' | 'mysql'

function arr<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : []
}

export function parseErdServices(raw: unknown): ErdServiceInfo[] {
  const services = (raw as { services?: unknown } | null)?.services
  return arr<ErdServiceInfo>(services).map((s) => ({
    ...s,
    dialects: arr(s.dialects),
    tableCount: s.tableCount ?? 0
  }))
}

export function parseErdModel(raw: unknown): ErdModel {
  if (typeof raw !== 'object' || raw === null) {
    throw new Error('erd: not an object')
  }
  const m = raw as ErdModel
  return {
    ...m,
    tables: arr<ErdTable>(m.tables).map((t) => ({
      ...t,
      columns: arr(t.columns),
      pk: arr(t.pk),
      indexes: arr(t.indexes),
      checks: arr(t.checks),
      rls: arr(t.rls),
      accessedBy: arr(t.accessedBy)
    })),
    relations: arr(m.relations),
    externalRefs: arr(m.externalRefs),
    changes: arr(m.changes),
    warnings: arr(m.warnings)
  }
}

/** First service that has tables; its first dialect unless the user picked one. */
export function resolveErdSelection(
  services: readonly ErdServiceInfo[] | null,
  service: string | null,
  dialect: ErdDialect | null
): { service: string | null; dialect: ErdDialect | null } {
  if (!services || services.length === 0) {
    return { service: null, dialect: null }
  }
  const chosen =
    services.find((s) => s.name === service) ?? services.find((s) => s.tableCount > 0) ?? null
  if (!chosen) {
    return { service: null, dialect: null }
  }
  const picked =
    dialect && chosen.dialects.includes(dialect) ? dialect : (chosen.dialects[0] ?? null)
  return { service: chosen.name, dialect: picked }
}

export type UseCodeIntelErdArgs = {
  worktreeId: string
  environmentId: string | null
  scope: ReviewScope | null
  service: string | null
  dialect: ErdDialect | null
  enabled?: boolean
  /** Test seam; production uses the code-intel client. */
  callFn?: CodeIntelCallFn
}

export type UseCodeIntelErd = {
  services: {
    status: CodeIntelQueryStatus
    data: ErdServiceInfo[] | null
    error: CodeIntelQueryError | null
  }
  model: {
    status: CodeIntelQueryStatus
    data: ErdModel | null
    meta: CodeIntelQueryMeta | null
    error: CodeIntelQueryError | null
    truncated: boolean
  }
  resolved: { service: string | null; dialect: ErdDialect | null }
  /** The index changed after the last load; the lens shows a chip, never auto-reloads. */
  isStale: boolean
  reload: () => void
}

export function useCodeIntelErd(args: UseCodeIntelErdArgs): UseCodeIntelErd {
  const { worktreeId, environmentId, scope, service, dialect, enabled = true, callFn } = args
  const servicesQuery = useCodeIntelQuery<ErdServiceInfo[]>(
    worktreeId,
    environmentId,
    { method: 'erd', params: {}, scopeKey: 'services', enabled, parseResult: parseErdServices },
    callFn
  )
  const resolved = useMemo(
    () => resolveErdSelection(servicesQuery.data, service, dialect),
    [servicesQuery.data, service, dialect]
  )
  const scopeParams = useMemo(() => {
    if (!scope) {
      return {}
    }
    const { base, head } = toChangeOverlayParams(scope)
    return { ...(base ? { base } : {}), ...(head ? { head } : {}) }
  }, [scope])
  const params = useMemo(
    () => ({
      service: resolved.service,
      ...(resolved.dialect ? { dialect: resolved.dialect } : {}),
      ...scopeParams,
      includeAccess: true,
      includeInferred: false
    }),
    [resolved, scopeParams]
  )
  const modelQuery = useCodeIntelQuery<ErdModel>(
    worktreeId,
    environmentId,
    {
      method: 'erd',
      params,
      scopeKey: 'model',
      enabled: enabled && resolved.service !== null,
      parseResult: parseErdModel
    },
    callFn
  )
  const reload = (): void => {
    servicesQuery.refetch()
    modelQuery.refetch()
  }
  return {
    services: {
      status: servicesQuery.status,
      data: servicesQuery.data,
      error: servicesQuery.error
    },
    model: {
      status: modelQuery.status,
      // Why: the previous service's model must not show while the next one loads.
      data: modelQuery.status === 'success' ? modelQuery.data : null,
      meta: modelQuery.meta,
      error: modelQuery.error,
      truncated: modelQuery.truncated
    },
    resolved,
    isStale: modelQuery.staleSignal || servicesQuery.staleSignal,
    reload
  }
}
