import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { OrcaTask } from '../types/task-types'

export function useTask(taskId: string) {
  const task = useAppStore(s => s.tasks.find((t: OrcaTask) => t.id === taskId))

  const updateTask = async (patch: Partial<OrcaTask>) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'tasks.update', { taskId, ...patch }) // Corrected to tasks.update
    useAppStore.getState().updateTask(taskId, patch)
  }

  const deleteTask = async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'tasks.delete', { taskId }) // Corrected to tasks.delete
    useAppStore.getState().removeTask(taskId)
  }

  const aiDecompose = async (instruction?: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // Corrected to tasks.aiPlan per TASK-FE-015
    const result = await callRuntimeRpc(target, 'tasks.aiPlan', { taskId, instruction }) as {
      subtasks: Partial<OrcaTask>[]
    }
    return result.subtasks
  }

  const acceptSubtasks = async (subtasks: Partial<OrcaTask>[], projectId: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // Corrected to tasks.createSubtasks per TASK-FE-015
    const createdSubtasks = await callRuntimeRpc(target, 'tasks.createSubtasks', {
      taskId,
      subtasks
    }) as OrcaTask[]
    for (const created of createdSubtasks) {
      useAppStore.getState().addTask(created)
    }
  }

  return { task, updateTask, deleteTask, aiDecompose, acceptSubtasks }
}
