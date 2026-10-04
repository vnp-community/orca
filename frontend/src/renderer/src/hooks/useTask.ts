import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { Tracers } from '../../../shared/trace/tracers'
import type { OrcaTask } from '../../../shared/task-types'

export function useTask(taskId: string) {
  const task = useAppStore((s) => s.tasks.find((t: OrcaTask) => t.id === taskId))

  const updateTask = async (patch: Partial<OrcaTask>) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // BUG-024: task.update's real handler (channels_automation_task.go)
    // decodes flat {id, title, status, workflowTemplateId, labels} — no
    // `patch` wrapper, and the id key is `id`, not `taskId`. Only send keys
    // actually present in `patch` (an omitted key must leave that field
    // untouched server-side, per UpdateTaskRequest's field-mask convention).
    const args: Record<string, unknown> = { id: taskId }
    if ('title' in patch) {
      args.title = patch.title
    }
    if ('status' in patch) {
      args.status = patch.status
    }
    if ('workflowTemplateId' in patch) {
      args.workflowTemplateId = patch.workflowTemplateId
    }
    if ('labels' in patch) {
      args.labels = patch.labels
    }
    await callRuntimeRpc(target, 'task.update', args)
    useAppStore.getState().updateTask(taskId, patch)
  }

  const deleteTask = async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await callRuntimeRpc(target, 'task.delete', { taskId })
    useAppStore.getState().removeTask(taskId)
  }

  const aiDecompose = async (instruction?: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // BL-TG-02: field `promptLength` thay vì instruction đầy đủ — tránh log nội
    // dung hướng dẫn AI dài, đúng tinh thần "chỉ trace field cần cho debug".
    const span = Tracers.uiTaskGraphAiPlanFlow.start({
      taskId,
      hasInstruction: !!instruction,
      promptLength: instruction?.length ?? 0
    })
    try {
      // task.aiDecompose has no `instruction` param — the backend derives the prompt
      // from the task's own fields (TaskAIPlanner.buildDecomposePrompt) and returns
      // the proposed subtasks as OrcaTask[] directly (not wrapped in `{ subtasks }`).
      const subtasks = (await callRuntimeRpc(target, 'task.aiDecompose', {
        taskId,
        projectId: task?.projectId ?? '',
        traceId: span.id
      })) as OrcaTask[]
      span.ok({ taskId, subtaskCount: subtasks.length })
      return subtasks
    } catch (err) {
      span.fail(err, { taskId })
      throw err
    }
  }

  const acceptSubtasks = async (subtasks: Partial<OrcaTask>[], projectId: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // task.aiApply looks up the parent task's projectId server-side; kept as a param
    // here only to preserve this hook's existing call signature for callers.
    void projectId
    const createdSubtasks = (await callRuntimeRpc(target, 'task.aiApply', {
      taskId,
      subtasks
    })) as OrcaTask[]
    for (const created of createdSubtasks) {
      useAppStore.getState().addTask(created)
    }
  }

  return { task, updateTask, deleteTask, aiDecompose, acceptSubtasks }
}
