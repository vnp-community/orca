import { useState, useEffect } from 'react'
import { useAppStore } from '../../store'
import { useTask } from '../../hooks/useTask'
import { useTaskActivity } from '../../hooks/useTaskActivity'
import { useWorkspace } from '../../context/WorkspaceContext'
import { openTaskWorktreeInGit } from './open-task-worktree-in-git'
import { Input } from '../ui/input'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '../ui/select'
import { TaskAIDecompose } from './TaskAIDecompose'
import { TaskPromptEditor } from './TaskPromptEditor'
import { TaskStatusBadge } from './TaskStatusBadge'
import { TaskComments } from './TaskComments'
import { TaskDispatchStatusPanel } from './TaskDispatchStatusPanel'
import { TaskAgentActivityPanel } from './agent-activity/TaskAgentActivityPanel'
import { ExecutionEngineBadge } from './ExecutionEngineBadge'
import { TaskSourceBadge } from './TaskSourceBadge'
import { AttachWorkflowTemplateAction } from './AttachWorkflowTemplateAction'
import { TaskGrantModal } from './TaskGrantModal'
import { TaskDependenciesSection } from './TaskDependenciesSection'
import { Button } from '../ui/button'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useTaskPermission } from '../../hooks/useTaskPermission'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { Tracers } from '../../../../shared/trace/tracers'
import { useTaskReadiness } from '../../hooks/useTaskReadiness'
import { useExecutionResult } from '../../hooks/useExecutionResult'
import { ReadinessBadge } from '../request/readiness/ReadinessBadge'
import { ReadinessReportSheet } from '../request/readiness/ReadinessReportSheet'
import { isRunBlockedByReadiness } from '../request/readiness/readiness-action-rules'
import { ExecutionResultPanel } from '../request/execution/ExecutionResultPanel'
import { hasSavedExecutionOutput } from '../request/execution/execution-saved-output'
import { useOpenDevServerSettings } from '../request/use-open-dev-server-settings'
import type { OrcaTask, TaskPriority, TaskStatus } from '../../../../shared/task-types'

// Right-panel detail view for active task
// Fields: title (editable), description (textarea), type, status, priority, assignee, progress
// Tabs: Details | Subtasks | AI Agent | Comments | Access

const TASK_STATUSES: OrcaTask['status'][] = [
  'open',
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
  const { project } = useWorkspace()
  const [openingInGit, setOpeningInGit] = useState(false)
  const [localTitle, setLocalTitle] = useState(task?.title ?? '')
  const [activeTab, setActiveTab] = useState<
    'details' | 'subtasks' | 'ai' | 'comments' | 'access' | 'result'
  >('details')
  // Polling for task status (FE-TASK-003); the agent's live output comes from the push channel
  // via TaskAgentActivityPanel. `polledTask` reflects status changes (e.g. 'in_progress' → 'done')
  // without the user needing to F5.
  const { task: polledTask } = useTaskActivity(task?.id ?? null)
  // isDispatching covers the gap between clicking "Execute with Agent" and
  // the next poll tick actually reporting in_progress (BUG-027 follow-up —
  // see handleRunAgent's doc comment). Reset once polling confirms the task
  // is no longer running, whether it finished or the backgrounded dispatch
  // reverted the status on failure.
  const [isDispatching, setIsDispatching] = useState(false)
  useEffect(() => {
    if (polledTask && polledTask.status !== 'in_progress') {
      setIsDispatching(false)
    }
  }, [polledTask])

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
  // Set by the Request Plan tab when this task's Plan/Phase is not approved yet.
  const executionGate = useAppStore((s) => s.executionGateByTaskId?.[activeTaskId ?? ''])
  // Why: readiness and structured results only exist for tasks created by a Request.
  const requestOwned = Boolean(task?.requestId)
  const readinessApi = useTaskReadiness({
    requestId: task?.requestId ?? null,
    taskId: requestOwned ? task?.id : undefined
  })
  const execApi = useExecutionResult(
    requestOwned ? (task?.id ?? null) : null,
    task?.requestId ?? null
  )
  // Why: show the tab only when there is something to read (a record or the last output); `legacy`
  // (TASK_NOT_FOUND) and `unsupported` (older gateway, desktop-local) leave nothing to show.
  const showResultTab =
    requestOwned &&
    (execApi.status === 'ready' ||
      execApi.status === 'error' ||
      (execApi.status === 'loading' && execApi.result !== null))
  // Why: the live panel is empty after a reload (no history on the backend); point to the saved output.
  const hasSavedResult = showResultTab && hasSavedExecutionOutput(execApi.status, execApi.result)
  useEffect(() => {
    if (!showResultTab && activeTab === 'result') {
      setActiveTab('details')
    }
  }, [showResultTab, activeTab])
  const [readinessOpen, setReadinessOpen] = useState(false)
  const openDevServerSettings = useOpenDevServerSettings()
  const readinessBlocked = isRunBlockedByReadiness(
    readinessApi.report,
    readinessApi.status !== 'unsupported'
  )
  const { level: myLevel, isSupported: permissionSupported } = useTaskPermission(
    task?.id ?? '',
    currentUserId
  )
  const canExecute =
    !permissionSupported || myLevel === null || ['owner', 'admin', 'user'].includes(myLevel)

  if (!task) {
    return <div className="p-4 text-sm text-muted-foreground">Select a task</div>
  }

  // BUG-027 follow-up: task.execute now dispatches async (returns in
  // milliseconds instead of blocking for the whole agent run), so nothing
  // server-side stops a second click from firing another concurrent
  // dispatch for the SAME task while the first is still running — found
  // live: a user repeatedly clicking (because the first click gave no
  // visible feedback) fired several concurrent dispatches against the same
  // dev server connection and crashed it. task-service now rejects a
  // re-dispatch while already in_progress (TASK_EXECUTE_ALREADY_IN_PROGRESS),
  // but that's a safety net, not a substitute for the button reflecting
  // reality — isDispatching covers the gap between click and the next
  // poll tick (useTaskActivity polls every 4s) actually reporting
  // in_progress; effectiveStatus (falls back to task.status when the poll
  // hasn't landed yet, e.g. right after navigating to a task) covers the
  // rest.
  const effectiveStatus = polledTask?.status ?? task.status
  const isRunning = isDispatching || effectiveStatus === 'in_progress'

  const handleRunAgent = async () => {
    setIsDispatching(true)
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // field `entryPoint: 'task-detail'` phân biệt với TaskPromptEditor (TASK-FE-018.3) —
    // 2 nút UI khác nhau cùng dẫn vào 1 tracer chung (BL-TG-04).
    const span = Tracers.uiTaskGraphExecuteFlow.start({
      taskId: task.id,
      entryPoint: 'task-detail'
    })
    try {
      // Why no worktreePath: task.execute's real handler (channels_automation_task.go)
      // never reads it — SimpleExecutor resolves the task's own worktree
      // server-side (task.projectId → ProjectExecutionResolver), independent
      // of whichever worktree happens to be selected in the sidebar. Sending
      // `currentWorktree!.path` here crashed with no worktree selected (a
      // real, common state on the Tasks tab, which has no worktree
      // requirement of its own) — found live.
      await callRuntimeRpc(target, 'task.execute', {
        taskId: task.id,
        // Why: the Workspace context only knows a project after the user picked one there; a
        // Request's task sheet has none, but the task itself always knows its project.
        projectId: task.projectId || project?.id,
        traceId: span.id
      })
      span.ok({ taskId: task.id })
      toast.success(`Agent started for: ${task.title}`)
      // Optionally emit workspace event:
      // emit('agent.started', { taskId: task.id })
    } catch (err) {
      span.fail(err, { taskId: task.id })
      toast.error(`Failed to start agent: ${err instanceof Error ? err.message : String(err)}`)
      setIsDispatching(false) // dispatch itself failed synchronously — safe to let the user retry right away
    }
  }

  // Jumps to this task's own worktree with its Source Control open, so the user sees the files the
  // agent changed. Every outcome that does not navigate says why (nothing is silent).
  const handleOpenInGit = async () => {
    const projectId = task.projectId || project?.id
    if (!task.worktreeId || !projectId) {
      toast.error(
        translate(
          'auto.components.task.TaskDetail.openInGit.noProject',
          'Cannot open Git: this task is not linked to a project'
        )
      )
      return
    }
    setOpeningInGit(true)
    try {
      const outcome = await openTaskWorktreeInGit(projectId, task.worktreeId)
      if (outcome === 'unknown') {
        toast.error(
          translate(
            'auto.components.task.TaskDetail.openInGit.unknown',
            "This task's worktree no longer exists"
          )
        )
      } else if (outcome === 'not-on-host') {
        toast.error(
          translate(
            'auto.components.task.TaskDetail.openInGit.notOnHost',
            "This task's worktree is no longer on this host. It may have been cleaned up."
          )
        )
      }
    } catch (err) {
      toast.error(
        translate(
          'auto.components.task.TaskDetail.openInGit.failed',
          'Failed to open worktree: {{message}}',
          { message: err instanceof Error ? err.message : String(err) }
        )
      )
    } finally {
      setOpeningInGit(false)
    }
  }

  return (
    <div className="task-detail flex min-w-0 flex-col h-full p-4" data-testid="task-detail">
      {/* Title */}
      <Input
        value={localTitle}
        onChange={(e) => setLocalTitle(e.target.value)}
        onBlur={() => localTitle !== task.title && updateTask({ title: localTitle })}
        className="min-w-0 truncate text-base font-semibold border-0 px-0 shadow-none"
        title={localTitle}
        data-testid="task-title-input"
      />

      {/* Action Buttons — wrap onto further rows in a narrow sheet instead of overflowing it */}
      <div className="flex flex-wrap gap-2 mt-2 items-center">
        <ExecutionEngineBadge task={task} />
        {requestOwned && (
          <ReadinessBadge report={readinessApi.report} onOpen={() => setReadinessOpen(true)} />
        )}
        {requestOwned && readinessApi.status !== 'unsupported' && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setReadinessOpen(true)}
            data-testid="check-readiness-btn"
          >
            {translate('auto.components.request.readiness.check', 'Check readiness')}
          </Button>
        )}
        <TaskSourceBadge taskId={task.id} />
        <AttachWorkflowTemplateAction task={task} />
        {canExecute && (
          <Button
            variant="default"
            onClick={handleRunAgent}
            disabled={isRunning || !!executionGate || readinessBlocked}
            title={
              executionGate
                ? translate(
                    'auto.components.task.TaskDetail.phaseNotApproved',
                    'Approve the Phase or Plan before running this task'
                  )
                : readinessBlocked
                  ? translate(
                      'auto.components.request.readiness.runBlocked',
                      'This task is not ready yet. Open the readiness report for details.'
                    )
                  : undefined
            }
            data-testid="run-agent-btn"
          >
            {isRunning ? '⏳ Agent running…' : '▶ Execute with Agent'}
          </Button>
        )}
        {task.worktreeId && (
          <Button
            variant="outline"
            onClick={handleOpenInGit}
            disabled={openingInGit}
            data-testid="open-in-git-btn"
          >
            {openingInGit ? 'Opening…' : '🔀 Open in Git'}
          </Button>
        )}
      </div>

      {/* Why: a Request's tasks always run directly (Engine 1), so "no orchestration dispatch" would be noise. */}
      {!requestOwned && <TaskDispatchStatusPanel taskId={task.id} />}

      {/* Live status from useTaskActivity's polling (FE-TASK-003) — reflects status changes
          (e.g. 'in_progress' → 'done') without the user needing to F5. */}
      <div className="text-xs text-muted-foreground mt-1" data-testid="task-live-status">
        Status: {polledTask?.status ?? task.status}
      </div>

      <TaskAgentActivityPanel
        task={polledTask ?? task}
        isRunning={isRunning}
        onOpenSavedResult={hasSavedResult ? () => setActiveTab('result') : undefined}
      />

      {/* Tabs */}
      <Tabs
        value={activeTab}
        onValueChange={(v) =>
          setActiveTab(v as 'details' | 'subtasks' | 'ai' | 'comments' | 'access' | 'result')
        }
        className="mt-4 min-w-0"
      >
        <TabsList className="max-w-full justify-start overflow-x-auto scrollbar-sleek">
          <TabsTrigger value="details">Details</TabsTrigger>
          <TabsTrigger value="subtasks">Subtasks</TabsTrigger>
          <TabsTrigger value="ai">AI Agent</TabsTrigger>
          <TabsTrigger value="comments">Comments</TabsTrigger>
          <TabsTrigger value="access">Access</TabsTrigger>
          {showResultTab && (
            <TabsTrigger value="result">
              {translate('auto.components.request.execution.tab', 'Result')}
            </TabsTrigger>
          )}
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

            <TaskDependenciesSection dependencies={blockedBy} loadFailed={depsError} />
          </div>
        </TabsContent>
        <TabsContent value="subtasks">
          <TaskAIDecompose parentTask={task} />
        </TabsContent>
        <TabsContent value="ai">
          {/* polledTask ?? task: TaskPromptEditor's own run-button guard
              (BUG-027 follow-up) needs the freshest known status to detect
              "already in_progress" once polling catches up, not the
              possibly-stale store copy. */}
          <TaskPromptEditor task={polledTask ?? task} />
        </TabsContent>
        <TabsContent value="comments">
          <TaskComments taskId={task.id} />
        </TabsContent>
        <TabsContent value="access">
          <TaskGrantModal taskId={task.id} />
        </TabsContent>
        {showResultTab && (
          <TabsContent value="result">
            <ExecutionResultPanel taskId={task.id} requestId={task.requestId} execution={execApi} />
          </TabsContent>
        )}
      </Tabs>
      {requestOwned && (
        <ReadinessReportSheet
          open={readinessOpen}
          onOpenChange={setReadinessOpen}
          report={readinessApi.report}
          requestId={task.requestId}
          canWrite={canExecute}
          hasDevServer={Boolean(task.worktreeId)}
          onCheck={() => readinessApi.check(task.id)}
          onConnectDevServer={() => {
            setReadinessOpen(false)
            openDevServerSettings()
          }}
        />
      )}
    </div>
  )
}
