/**
 * useC4Architecture.ts — FE-CV-TASK-055-02
 *
 * `architecture` returns {containers, view}. First call carries no container so the server
 * answers with the container list (view may be null); once a container is chosen it is
 * requested again. Cache key is (container, includeHidden) via the shared view loader.
 */

import { useMemo } from 'react'
import type {
  C4ComponentView,
  ContainerRef
} from '../../../shared/code-intel-architecture-types'
import { CODE_INTEL_RPC_METHODS } from '../../../shared/code-intel-rpc-methods'
import { useCodeIntelViewLoad } from './useCodeIntelViewLoad'
import type { ViewLoadResult } from './useCodeIntelViewLoad'

export type C4ArchitectureData = { containers: ContainerRef[]; view: C4ComponentView | null }

function arr<T>(v: unknown): T[] {
  return Array.isArray(v) ? (v as T[]) : []
}

export function parseC4Architecture(raw: unknown): C4ArchitectureData {
  const r = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>
  const v = r.view
  if (typeof v !== 'object' || v === null) {
    return { containers: arr<ContainerRef>(r.containers), view: null }
  }
  const view = v as Record<string, unknown>
  return {
    containers: arr<ContainerRef>(r.containers),
    view: {
      container: view.container as ContainerRef,
      components: arr(view.components),
      relations: arr(view.relations),
      externals: arr(view.externals),
      warnings: arr(view.warnings),
      overridesVersion: typeof view.overridesVersion === 'string' ? view.overridesVersion : '',
      hasOverrides: view.hasOverrides === true
    }
  }
}

export const C4_ARCHITECTURE_METHOD = CODE_INTEL_RPC_METHODS.ARCHITECTURE

export function useC4Architecture(
  worktreeId: string,
  environmentId: string | null,
  opts: { container: string | null; includeHidden: boolean; enabled?: boolean }
): ViewLoadResult<C4ArchitectureData> {
  const { container, includeHidden, enabled = true } = opts
  const params = useMemo(
    () => (enabled ? { ...(container ? { container } : {}), ...(includeHidden ? { includeHidden: true } : {}) } : null),
    [enabled, container, includeHidden]
  )
  return useCodeIntelViewLoad<C4ArchitectureData>({
    worktreeId,
    environmentId,
    method: C4_ARCHITECTURE_METHOD,
    params,
    parse: parseC4Architecture
  })
}
