import { useEffect, useCallback } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { Tracers } from '../../../shared/trace/tracers'

export function useWorkflowExecution(executionId: string) {
  const { stepStatuses, streamingOutput } = useAppStore(s => ({
    stepStatuses:    s.stepStatuses[executionId] ?? {},
    streamingOutput: s.streamingOutput,
  }))
  const execution = useAppStore(s => s.executions.find(e => e.id === executionId))

  useEffect(() => {
    if (!(window as any).api?.on) {return}
    const unsubs = [
      (window as any).api.on('workflow:stepStatus', ({ execId, stepId, status }: any) => {
        if (execId !== executionId) {return}
        useAppStore.getState().setStepStatus(execId, stepId, status)
      }),
      (window as any).api.on('workflow:stepOutput', ({ execId, line }: any) => {
        if (execId !== executionId) {return}
        useAppStore.getState().appendStreamLine(execId, line)
      }),
      (window as any).api.on('workflow:complete', ({ execId, status }: any) => {
        if (execId !== executionId) {return}
        useAppStore.getState().updateExecutionStatus(execId, status)
      }),
    ]
    return () => unsubs.forEach((u: any) => u?.())
  }, [executionId])

  const cancelExecution = useCallback(async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    const rootTraceId = useAppStore.getState().executions.find(e => e.id === executionId)?.rootTraceId
    // Field `parentTraceId` (không phải resume) nhóm thao tác cancel này vào cùng
    // execution trong TracePanel, dù span này có id riêng.
    const span = Tracers.uiWorkflowCancelFlow.start({ executionId, parentTraceId: rootTraceId })
    try {
      await callRuntimeRpc(target, 'workflow.cancel', { executionId, traceId: span.id })
      useAppStore.getState().updateExecutionStatus(executionId, 'cancelled')
      span.ok({ executionId })
    } catch (err) {
      span.fail(err, { executionId })
      throw err
    }
  }, [executionId])

  return { execution, stepStatuses, streamingOutput, cancelExecution }
}
