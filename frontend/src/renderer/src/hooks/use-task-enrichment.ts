// Hook auto-fetch KG context khi task có kg_context (CR-ORCA-002)
// Pattern: concurrent fetch CRS + feature, best-effort, 5min cache

import { useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import type { TaskKGContext, TaskEnrichment } from '../types/task-types'

// Task shape được hỗ trợ bởi hook này
type TaskWithKGContext = {
  id: string
  kg_context?: TaskKGContext
}

// 5 phút cache — consistent với backend best-effort approach (SOL-ORCA-002 §5)
const KG_CACHE_TTL_MS = 5 * 60 * 1000

export function useTaskEnrichment(task: TaskWithKGContext, environmentId?: string) {
  const { taskEnrichments, setTaskEnrichmentLoading, setTaskEnrichment, setTaskEnrichmentError } =
    useAppStore()

  const entry: TaskEnrichment | undefined = taskEnrichments[task.id]
  const kgCtx = task.kg_context

  useEffect(() => {
    // Skip nếu không có KG refs (backend: CRSID == "" → skip enrichment)
    if (kgCtx?.crs_id == null && kgCtx?.feature_ref == null) {
      return
    }

    // Cache: skip nếu đã fetch < 5 phút
    if (entry?.fetchedAt != null && Date.now() - entry.fetchedAt < KG_CACHE_TTL_MS) {
      return
    }

    const target =
      environmentId != null
        ? { kind: 'environment' as const, environmentId }
        : { kind: 'local' as const }

    setTaskEnrichmentLoading(task.id)

    // Concurrent fetch — mirror backend concurrent approach (SOL-ORCA-002 §5 risk)
    // Promise.allSettled: tiếp tục dù 1 trong 2 fail
    Promise.allSettled([
      kgCtx?.crs_id != null
        ? callRuntimeRpc(target, 'agent.tool_call', {
            tool: 'pis_query_crs',
            input: { crs_id: kgCtx.crs_id }
          })
        : Promise.resolve(undefined),
      kgCtx?.feature_ref != null
        ? callRuntimeRpc(target, 'agent.tool_call', {
            tool: 'pis_query_feature_context',
            input: { feature_id: kgCtx.feature_ref }
          })
        : Promise.resolve(undefined)
    ])
      .then(([crsResult, featureResult]) => {
        // Best-effort: luôn gọi setTaskEnrichment dù có failure
        setTaskEnrichment(
          task.id,
          crsResult.status === 'fulfilled' ? crsResult.value : undefined,
          featureResult.status === 'fulfilled' ? featureResult.value : undefined
        )
      })
      .catch((err: unknown) => {
        setTaskEnrichmentError(task.id, String(err))
      })
  }, [task.id, kgCtx?.crs_id, kgCtx?.feature_ref, environmentId]) // eslint-disable-line react-hooks/exhaustive-deps

  return { entry, loading: entry?.loading ?? false }
}
