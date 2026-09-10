import { useEffect, useReducer } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaTask } from '../../../shared/task-types'

// CR-FLOW-TASK-005 mục 2 thiết kế subscribe 1 kênh WS hợp nhất `task.activity:{taskId}`
// (CR-FLOW-TASK-003). Cả 2 điều kiện tiên quyết đều CHƯA có hôm nay:
//   1. Backend: CR-FLOW-TASK-003 (outbox orchestration/workflow + kênh WS) — 🔵 Proposed.
//   2. Client: `RuntimeClientEvent` (shared/runtime-client-events.ts) là 1 union đóng, không có
//      variant `taskActivity`; và `subscribeRuntimeClientEvents` chỉ nhận `environmentId`, không
//      có đường tương đương cho `target.kind === 'local'` (Desktop IPC dùng channel riêng từng
//      loại event, xem useIpcEvents.ts — không có channel `taskActivity` nào ở đó).
// Cho tới khi cả 2 xong, hook này CHỈ poll `task.get` — đúng fallback CR-005 tự cho phép ("Rủi ro"
// mục 2). Khi CR-003 xong VÀ `RuntimeClientEvent` có variant `taskActivity`, thay thân effect dưới
// đây bằng subscribeRuntimeClientEvents(...) cho target 'environment' + 1 IPC bridge tương đương
// cho 'local' — KHÔNG xoá field `isLive` (UI dùng nó phân biệt "đang poll" / "đang nhận real-time").
const TASK_ACTIVITY_POLL_INTERVAL_MS = 4_000 // cùng hằng số CR-PW-005 dùng cho workflow execution polling

export type TaskActivityState = {
  task: OrcaTask | null
  isLive: false // luôn false cho tới khi kênh WS thật tồn tại — xem comment trên
  lastPolledAt: number | null
}

type Action = { type: 'polled'; task: OrcaTask }

function reducer(state: TaskActivityState, action: Action): TaskActivityState {
  switch (action.type) {
    case 'polled':
      return { ...state, task: action.task, lastPolledAt: Date.now() }
  }
}

export function useTaskActivity(taskId: string | null): TaskActivityState {
  const [state, dispatch] = useReducer(reducer, { task: null, isLive: false, lastPolledAt: null })

  useEffect(() => {
    if (!taskId) {
      return
    }
    let cancelled = false
    const poll = async () => {
      try {
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        // task.get: RPC đã tồn tại (dùng chung với refetch thủ công hiện có), không phải RPC mới —
        // chỉ đổi cách gọi từ "thủ công theo action người dùng" (BUG-FE-TASKV1-005) sang interval.
        const task = await callRuntimeRpc<OrcaTask>(target, 'task.get', { taskId })
        if (!cancelled) {
          dispatch({ type: 'polled', task })
        }
      } catch {
        // Lỗi tạm thời — tick sau retry, không cần error state riêng cho 1 poll nền.
      }
    }
    void poll()
    const intervalId = setInterval(() => void poll(), TASK_ACTIVITY_POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(intervalId)
    }
  }, [taskId])

  return state
}
