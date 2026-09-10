import { useState, useEffect } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { TaskGrantLevel } from '../../../shared/task-types'

// isSupported: false khi target đang chạy chưa wire task.resolvePermission ở wscompat
// (hôm nay: LUÔN false — task.* channel đã wire chỉ có create/get/execute/list/update/
// delete/getDependencies/aiDecompose/aiApply, xem channels.go + channels_automation_task.go).
// Hook tự phát hiện qua lỗi RPC thay vì hard-code true/false theo deploy target, để tự
// động hết cảnh báo khi backend wire xong mà không cần sửa lại hook này.
export function useTaskPermission(taskId: string, userId: string | undefined) {
  const [level, setLevel] = useState<TaskGrantLevel | null>(null)
  const [isSupported, setIsSupported] = useState(true)

  useEffect(() => {
    if (!userId) {
      return
    }
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<{ effectiveLevel: string }>(target, 'task.resolvePermission', {
      taskId,
      userId
    })
      .then((r) => {
        if (cancelled) {
          return
        }
        setLevel(r.effectiveLevel.replace('GRANT_LEVEL_', '').toLowerCase() as TaskGrantLevel)
      })
      .catch(() => {
        if (cancelled) {
          return
        }
        setIsSupported(false)
      })
    return () => {
      cancelled = true
    }
  }, [taskId, userId])

  return { level, isSupported }
}
