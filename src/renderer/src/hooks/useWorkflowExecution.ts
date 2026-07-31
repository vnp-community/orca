import { useEffect, useCallback } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'

export function useWorkflowExecution(executionId: string) {
  const { stepStatuses, streamingOutput } = useAppStore(s => ({
    stepStatuses:    s.stepStatuses[executionId] ?? {},
    streamingOutput: s.streamingOutput,
  }))
  const execution = useAppStore(s => s.executions.find(e => e.id === executionId))

  useEffect(() => {
    if (!(window as any).api?.on) return
    const unsubs = [
      (window as any).api.on('workflow:stepStatus', ({ execId, stepId, status }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().setStepStatus(execId, stepId, status)
      }),
      (window as any).api.on('workflow:stepOutput', ({ execId, line }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().appendStreamLine(execId, line)
      }),
      (window as any).api.on('workflow:complete', ({ execId, status }: any) => {
        if (execId !== executionId) return
        useAppStore.getState().updateExecutionStatus(execId, status)
      }),
    ]
    return () => unsubs.forEach((u: any) => u?.())
  }, [executionId])

  const cancelExecution = useCallback(async () => {
    await callRuntimeRpc('workflow.cancel', { executionId })
    useAppStore.getState().updateExecutionStatus(executionId, 'cancelled')
  }, [executionId])

  return { execution, stepStatuses, streamingOutput, cancelExecution }
}
