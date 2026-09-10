import { useState, useEffect } from 'react'
import { useAppStore } from '../../store'
import { useTask } from '../../hooks/useTask'
import { useTaskActivity } from '../../hooks/useTaskActivity'
import { useWorkspace } from '../../context/WorkspaceContext'
import { Input } from '../ui/input'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '../ui/select'
import { TaskAIDecompose } from './TaskAIDecompose'
import { TaskPromptEditor } from './TaskPromptEditor'
import { TaskStatusBadge } from './TaskStatusBadge'
import { TaskComments } from './TaskComments'
import { TaskDispatchStatusPanel } from './TaskDispatchStatusPanel'
import { ExecutionEngineBadge } from './ExecutionEngineBadge'
import { AttachWorkflowTemplateAction } from './AttachWorkflowTemplateAction'
import { TaskGrantModal } from './TaskGrantModal'
import { Button } from '../ui/button'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useTaskPermission } from '../../hooks/useTaskPermission'
import { toast } from 'sonner'
import { Tracers } from '../../../../shared/trace/tracers'
import type { OrcaTask, TaskPriority, TaskStatus } from '../../../../shared/task-types'

// Right-panel detail view for active task
// Fields: title (editable), description (textarea), type, status, priority, assignee, progress
// Tabs: Details | Subtasks | AI Agent | Comments | Access

const TASK_STATUSES: OrcaTask['status'][] = [
  'backlog',
  'todo',
  'in_progress',
  'review',
  'done',
  'blocked',
  'cancelled'
]

export function TaskDetail() {
  const activeTaskId = useAppStore((s) => s.activeTaskId)
  const { task, updateTask } = useTask(activeTaskId!)
  const { project, currentWorktree } = useWorkspace()
  const [localTitle, setLocalTitle] = useState(task?.title ?? '')
  const [activeTab, setActiveTab] = useState<'details' | 'subtasks' | 'ai' | 'comments' | 'access'>(
    'details'
  )
  // Polling fallback (FE-TASK-003) — no push channel for task activity exists yet, see
  // useTaskActivity.ts's header comment. `polledTask` reflects status changes (e.g.
  // 'in_progress' → 'done') without the user needing to F5.
  const { task: polledTask } = useTaskActivity(task?.id ?? null)

  // task.getDependencies returns a flat Task[] (NOT { task, edgeType }[] — no edgeType field
  // exists at all, see task.proto:169-177 + channels_automation_task.go:296-308), always the
  // "this task depends on" direction. "blocks" can't be derived from a single task's call (it
  // would need every task's dependency list), so Details only shows blockedBy accurately —
  // see TaskDAGView for the aggregate view where "blocks" is derived across all tasks.
  const [blockedBy, setBlockedBy] = useState<OrcaTask[]>([])
  const [depsError, setDepsError] = useState(false)

  useEffect(() => {
    if (!task?.id) {
      return
    }
    setDepsError(false)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc<OrcaTask[]>(target, 'task.getDependencies', { taskId: task.id })
      .then((deps) => setBlockedBy(deps ?? []))
      .catch(() => setDepsError(true)) // no longer swallows the error silently
  }, [task?.id])

  // canExecute chỉ có ý nghĩa quyết định sản phẩm tạm thời (owner/admin/user đủ để thao
  // tác, team/company chỉ xem) — chưa phải quy tắc đã chốt với chủ sở hữu BL-TG-03, vì
  // GrantLevel là grantee-kind (ai được cấp), không phải action-scale (làm được gì).
  // Không bao giờ ẩn nút khi RPC chưa wire (isSupported === false, mặc định hôm nay) —
  // và cũng không ẩn trong lúc `myLevel` vẫn đang tải (null): `isSupported` khởi tạo
  // `true` cho tới khi useTaskPermission's RPC settle, nên chỉ dựa `!permissionSupported`
  // sẽ làm nút biến mất chớp nhoáng ở mọi lần mount trước khi promise resolve — chỉ ẩn
  // khi đã CÓ bằng chứng dương tính (myLevel resolved) rằng quyền không đủ.
  // Gọi trước early-return `if (!task)` bên dưới — Rules of Hooks, cùng pattern useTaskActivity ở trên.
  const currentUserId = useAppStore((s) => s.currentUser?.id)
  const { level: myLevel, isSupported: permissionSupported } = useTaskPermission(
    task?.id ?? '',
    currentUserId
  )
  const canExecute =
    !permissionSupported || myLevel === null || ['owner', 'admin', 'user'].includes(myLevel)

  if (!task) {
    return <div className="p-4 text-sm text-muted-foreground">Select a task</div>
  }

  const handleRunAgent = async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // field `entryPoint: 'task-detail'` phân biệt với TaskPromptEditor (TASK-FE-018.3) —
    // 2 nút UI khác nhau cùng dẫn vào 1 tracer chung (BL-TG-04).
    const span = Tracers.uiTaskGraphExecuteFlow.start({
      taskId: task.id,
      entryPoint: 'task-detail'
    })
    try {
      await callRuntimeRpc(target, 'task.execute', {
        taskId: task.id,
        projectId: project!.id,
        worktreePath: currentWorktree!.path,
        traceId: span.id
      })
      span.ok({ taskId: task.id })
      toast.success(`Agent started for: ${task.title}`)
      // Optionally emit workspace event:
      // emit('agent.started', { taskId: task.id })
    } catch (err) {
      span.fail(err, { taskId: task.id })
      toast.error(`Failed to start agent: ${err instanceof Error ? err.message : String(err)}`)
    }
  }

  return (
    <div className="task-detail flex flex-col h-full p-4" data-testid="task-detail">
      {/* Title */}
      <Input
        value={localTitle}
        onChange={(e) => setLocalTitle(e.target.value)}
        onBlur={() => localTitle !== task.title && updateTask({ title: localTitle })}
        className="text-base font-semibold border-0 px-0 shadow-none"
        data-testid="task-title-input"
      />

      {/* Action Buttons */}
      <div className="flex gap-2 mt-2 items-center">
        <ExecutionEngineBadge task={task} />
        <AttachWorkflowTemplateAction task={task} />
        {canExecute && (
          <Button variant="default" onClick={handleRunAgent} data-testid="run-agent-btn">
            ▶ Execute with Agent
          </Button>
        )}
      </div>

      <TaskDispatchStatusPanel taskId={task.id} />

      {/* Live status from useTaskActivity's polling fallback (FE-TASK-003) — reflects
          status changes (e.g. 'in_progress' → 'done') without the user needing to F5. */}
      <div className="text-xs text-muted-foreground mt-1" data-testid="task-live-status">
        Status: {polledTask?.status ?? task.status}
      </div>

      {/* Tabs */}
      <Tabs
        value={activeTab}
        onValueChange={(v) =>
          setActiveTab(v as 'details' | 'subtasks' | 'ai' | 'comments' | 'access')
        }
        className="mt-4"
      >
        <TabsList>
          <TabsTrigger value="details">Details</TabsTrigger>
          <TabsTrigger value="subtasks">Subtasks</TabsTrigger>
          <TabsTrigger value="ai">AI Agent</TabsTrigger>
          <TabsTrigger value="comments">Comments</TabsTrigger>
          <TabsTrigger value="access">Access</TabsTrigger>
        </TabsList>
        <TabsContent value="details">
          {/* Status, Priority, Type, Progress fields */}
          <div className="space-y-3 mt-3">
            <div className="flex items-center gap-2">
              <label className="text-sm w-24">Status</label>
              <Select
                value={task.status}
                onValueChange={(s) => updateTask({ status: s as TaskStatus })}
              >
                <SelectTrigger className="flex-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {TASK_STATUSES.map((s) => (
                    <SelectItem key={s} value={s}>
                      {s}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <TaskStatusBadge status={polledTask?.status ?? task.status} />
            </div>
            <div className="flex items-center gap-2">
              <label className="text-sm w-24">Priority</label>
              <Select
                value={task.priority}
                onValueChange={(p) => updateTask({ priority: p as TaskPriority })}
              >
                <SelectTrigger className="flex-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {['critical', 'high', 'medium', 'low'].map((p) => (
                    <SelectItem key={p} value={p}>
                      {p}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Dependencies */}
            <div className="pt-2 text-xs">
              <p className="font-semibold mb-1">Dependencies</p>
              {blockedBy.length > 0 ? (
                <div className="text-muted-foreground flex gap-1">
                  <span>← Blocked by:</span>
                  <span>{blockedBy.map((d) => d.title).join(', ')}</span>
                </div>
              ) : null}
              <div className="text-muted-foreground flex gap-1 mt-1">
                <span>→ Blocks:</span>
                <span>Not supported on the Details tab — see the DAG tab</span>
              </div>
              {blockedBy.length === 0 && (
                <div className="text-muted-foreground">No dependencies</div>
              )}
              {depsError && (
                <div className="text-destructive mt-1">Failed to load dependencies</div>
              )}
            </div>
          </div>
        </TabsContent>
        <TabsContent value="subtasks">
          <TaskAIDecompose parentTask={task} />
        </TabsContent>
        <TabsContent value="ai">
          <TaskPromptEditor task={task} />
        </TabsContent>
        <TabsContent value="comments">
          <TaskComments taskId={task.id} />
        </TabsContent>
        <TabsContent value="access">
          <TaskGrantModal taskId={task.id} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
