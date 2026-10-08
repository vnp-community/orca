/**
 * useExecutionResult — FE-REQ-TASK-036-02
 *
 * `execution.get` is a proposed channel. `legacy` = task has no record (older
 * than CR-REQ-029); `unsupported` = runtime lacks the channel.
 *
 * @module hooks/useExecutionResult
 */

import { useCallback, useEffect, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { useRefetchOnRequestEvent } from './useRefetchOnRequestEvent'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseExecutionResult } from '../../../shared/request-artifact-parsers'
import type { ExecutionResult } from '../../../shared/request-artifact-types'

export type ExecutionResultStatus = 'idle' | 'loading' | 'ready' | 'legacy' | 'unsupported' | 'error'

export function useExecutionResult(taskId: string | null, requestId: string | null = null) {
  const [result, setResult] = useState<ExecutionResult | null>(null)
  const [status, setStatus] = useState<ExecutionResultStatus>('idle')
  const [tick, setTick] = useState(0)

  const refetch = useCallback(() => setTick((n) => n + 1), [])
  useRefetchOnRequestEvent(requestId, ['execution.verified'], refetch)

  useEffect(() => {
    if (!taskId) {
      setResult(null)
      setStatus('idle')
      return
    }
    let cancelled = false
    setStatus('loading')
    void callRequestRpc<{ result?: unknown }>(REQUEST_RPC_METHODS.EXECUTION_GET, { taskId, latestOnly: true }).then((res) => {
      if (cancelled) {return}
      if (!res.ok) {
        setResult(null)
        setStatus(res.error.kind === 'unsupported' ? 'unsupported' : res.error.kind === 'not_found' ? 'legacy' : 'error')
        return
      }
      const parsed = parseExecutionResult(res.value?.result ?? res.value)
      setResult(parsed)
      setStatus(parsed ? 'ready' : 'legacy')
    })
    return () => { cancelled = true }
  }, [taskId, tick])

  return { result, status, refetch }
}
