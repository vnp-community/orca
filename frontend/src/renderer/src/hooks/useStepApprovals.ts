import { useCallback, useEffect, useState } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'

// A decision a running workflow step waits for from a person (workflow.stepApproval.*,
// docs/integrate/TDD-WF-AGENT-001 §7.5), e.g. a tdd-agent gate approval.
export type StepApproval = {
  id: string
  executionId: string
  stepId: string
  projectId: string
  kind: string
  subject: { runId?: string; gate?: string; role?: string } | null
  decision: unknown
  decisionHash: string
  status: 'pending' | 'approved' | 'rejected' | 'superseded'
  resolvedBy?: string
  rationale?: string
  createdAt: string
  canDecide?: boolean
}

const POLL_INTERVAL_MS = 4_000

function target() {
  return getActiveRuntimeTarget(useAppStore.getState().settings)
}

/** Pending step approvals of one execution, polled while `active` (the execution is running). */
export function useStepApprovals(executionId: string, active: boolean) {
  const [approvals, setApprovals] = useState<StepApproval[]>([])
  const [unavailable, setUnavailable] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const rows = await callRuntimeRpc<StepApproval[]>(target(), 'workflow.stepApproval.list', {
        executionId
      })
      setApprovals(rows ?? [])
      setUnavailable(false)
    } catch {
      // Older gateways have no step approvals; the monitor simply shows none.
      setUnavailable(true)
    }
  }, [executionId])

  useEffect(() => {
    if (!executionId || !active) {
      return
    }
    void refresh()
    const id = setInterval(() => void refresh(), POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [executionId, active, refresh])

  // The decision and the right to decide come from a fresh read: the list may be older than the decision shown.
  const load = useCallback(
    (id: string) => callRuntimeRpc<StepApproval>(target(), 'workflow.stepApproval.get', { id }),
    []
  )

  const resolve = useCallback(
    async (approval: StepApproval, decision: 'approved' | 'rejected', rationale: string) => {
      await callRuntimeRpc<StepApproval>(target(), 'workflow.stepApproval.resolve', {
        id: approval.id,
        decision,
        decisionHash: approval.decisionHash,
        rationale
      })
      await refresh()
    },
    [refresh]
  )

  return { approvals, unavailable, refresh, load, resolve }
}
