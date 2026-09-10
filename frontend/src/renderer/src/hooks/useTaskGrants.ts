import { useState, useCallback } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { toast } from 'sonner'
import type { RealGrantLevel } from '../../../shared/task-types'

export function useTaskGrants(taskId: string) {
  const [isGranting, setIsGranting] = useState(false)

  const addGrant = useCallback(
    async (subjectId: string, level: RealGrantLevel, applyTree: boolean) => {
      setIsGranting(true)
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      try {
        // Shape thật channels_automation_task.go:369-388 — level là string ('owner'|'admin'|
        // 'user'|'team'|'company'), parse server-side qua taskv1.GrantLevel_value.
        await callRuntimeRpc(target, 'task.grant', { taskId, subjectId, level, applyTree })
        toast.success(`Granted ${level} to ${subjectId}`)
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err)
        toast.error(`Failed to grant: ${message}`)
        throw err
      } finally {
        setIsGranting(false)
      }
    },
    [taskId]
  )

  // task.listGrants CHƯA tồn tại (BE-SOL-003 📋 Proposed) — trả rỗng tạm thời, không giả lập
  // dữ liệu giả để tránh hiểu nhầm là đã hoạt động thật.
  const grants: { subjectId: string; level: RealGrantLevel }[] = []

  // revoke/generateShareLink CHƯA có RPC — no-op + toast thông báo rõ, không im lặng thất bại.
  const revoke = useCallback((_subjectId: string) => {
    toast.info('Revoke grant is not available yet (pending BE-SOL-003)')
  }, [])
  const generateShareLink = useCallback(() => {
    toast.info('Share link is not available yet (pending BE-SOL-003)')
  }, [])

  return { grants, addGrant, revoke, generateShareLink, isGranting }
}
