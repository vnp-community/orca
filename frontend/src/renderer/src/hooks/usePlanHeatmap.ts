/**
 * usePlanHeatmap — FE-REQ-TASK-032-06
 *
 * `impact.heatmap {planTaskId}` -> risk per task. The wire shape is not final
 * (CR-REQ-030 2.8), so the parser accepts a map or a list and degrades to null.
 *
 * @module hooks/usePlanHeatmap
 */

import { useEffect, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseGraphRisk } from '../../../shared/graph-wire-parsers'
import type { PlanHeatmap } from '../components/graph/graph-client-lens-adapters'

export function parsePlanHeatmap(raw: unknown): PlanHeatmap | null {
  if (typeof raw !== 'object' || raw === null) {return null}
  const r = raw as Record<string, unknown>
  const byTaskId: Record<string, string> = {}
  const list = Array.isArray(r.tasks) ? r.tasks : Array.isArray(r.items) ? r.items : []
  for (const item of list) {
    if (typeof item !== 'object' || item === null) {continue}
    const o = item as Record<string, unknown>
    const id = o.taskId ?? o.task_id ?? o.id
    if (typeof id === 'string') {byTaskId[id] = parseGraphRisk(o.risk ?? o.level)}
  }
  const map = r.byTaskId
  if (typeof map === 'object' && map !== null) {
    for (const [k, v] of Object.entries(map as Record<string, unknown>)) {byTaskId[k] = parseGraphRisk(v)}
  }
  if (Object.keys(byTaskId).length === 0) {return null}
  const at = r.assessedAt ?? r.assessed_at
  return { byTaskId, assessedAt: typeof at === 'string' ? at : null, tool: typeof r.tool === 'string' ? r.tool : null }
}

export function usePlanHeatmap(planTaskId: string | undefined, enabled: boolean): { heatmap: PlanHeatmap | null; unsupported: boolean } {
  const [heatmap, setHeatmap] = useState<PlanHeatmap | null>(null)
  const [unsupported, setUnsupported] = useState(false)

  useEffect(() => {
    if (!planTaskId || !enabled) {return}
    let cancelled = false
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.IMPACT_HEATMAP, { planTaskId }).then((res) => {
      if (cancelled) {return}
      if (res.ok) {
        setHeatmap(parsePlanHeatmap(res.value))
        setUnsupported(false)
      } else {
        // Why: no heatmap means every node stays `unknown`, never `low`.
        setHeatmap(null)
        setUnsupported(res.error.kind === 'unsupported')
      }
    })
    return () => { cancelled = true }
  }, [planTaskId, enabled])

  return { heatmap, unsupported }
}
