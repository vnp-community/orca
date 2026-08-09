import { useState, useEffect } from 'react'
import { useAppStore } from '../../store'
import { useTask } from '../../hooks/useTask'
import { Input } from '../ui/input'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '../ui/select'
import { TaskAIDecompose } from './TaskAIDecompose'
import { TaskPromptEditor } from './TaskPromptEditor'
import { Button } from '../ui/button'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { toast } from 'sonner'
import { Tracers } from '../../../../shared/trace/tracers'
import type { OrcaTask } from '../../types/task-types'

// Right-panel detail view for active task
// Fields: title (editable), description (textarea), type, status, priority, assignee, progress
// Tabs: Details | Subtasks | AI Agent

export function TaskDetail() {
  const activeTaskId = useAppStore(s => s.activeTaskId)
  const { task, updateTask } = useTask(activeTaskId!)
  const [localTitle, setLocalTitle] = useState(task?.title ?? '')
  const [activeTab, setActiveTab]   = useState<'details' | 'subtasks' | 'ai'>('details')
  
  const [deps, setDeps] = useState<{ blockedBy: OrcaTask[]; blocks: OrcaTask[] }>({
    blockedBy: [], blocks: []
  })

  useEffect(() => {
    if (!task?.id) {return}
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    callRuntimeRpc(target, 'tasks.getDependencies', { taskId: task.id })
      .then(d => setDeps(d as any))
      .catch(() => {})
  }, [task?.id])

  if (!task) {return <div className="p-4 text-sm text-muted-foreground">Select a task</div>}

  const handleRunAgent = async () => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // field `entryPoint: 'task-detail'` phân biệt với TaskPromptEditor (TASK-FE-018.3) —
    // 2 nút UI khác nhau cùng dẫn vào 1 tracer chung (BL-TG-04).
    const span = Tracers.uiTaskGraphExecuteFlow.start({ taskId: task.id, entryPoint: 'task-detail' })
    try {
      await callRuntimeRpc(target, 'tasks.runAgent', { taskId: task.id, traceId: span.id })
      span.ok({ taskId: task.id })
      toast.success(`Agent started for: ${task.title}`)
      // Optionally emit workspace event:
      // emit('agent.started', { taskId: task.id })
    } catch (err: any) {
      span.fail(err, { taskId: task.id })
      toast.error(`Failed to start agent: ${  err.message}`)
    }
  }

  return (
    <div className="task-detail flex flex-col h-full p-4" data-testid="task-detail">
      {/* Title */}
      <Input
        value={localTitle}
        onChange={e => setLocalTitle(e.target.value)}
        onBlur={() => localTitle !== task.title && updateTask({ title: localTitle })}
        className="text-base font-semibold border-0 px-0 shadow-none"
        data-testid="task-title-input"
      />
      
      {/* Action Buttons */}
      <div className="flex gap-2 mt-2">
        <Button variant="default" onClick={handleRunAgent} data-testid="run-agent-btn">
          ▶ Execute with Agent
        </Button>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={v => setActiveTab(v as any)} className="mt-4">
        <TabsList>
          <TabsTrigger value="details">Details</TabsTrigger>
          <TabsTrigger value="subtasks">Subtasks</TabsTrigger>
          <TabsTrigger value="ai">AI Agent</TabsTrigger>
        </TabsList>
        <TabsContent value="details">
          {/* Status, Priority, Type, Progress fields */}
          <div className="space-y-3 mt-3">
            <div className="flex items-center gap-2">
              <label className="text-sm w-24">Status</label>
              <Select value={task.status} onValueChange={s => updateTask({ status: s as any })}>
                <SelectTrigger className="flex-1"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {['todo','in_progress','done','cancelled'].map(s => <SelectItem key={s} value={s}>{s}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="flex items-center gap-2">
              <label className="text-sm w-24">Priority</label>
              <Select value={task.priority} onValueChange={p => updateTask({ priority: p as any })}>
                <SelectTrigger className="flex-1"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {['critical','high','medium','low'].map(p => <SelectItem key={p} value={p}>{p}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            
            {/* Dependencies */}
            <div className="pt-2 text-xs">
              <p className="font-semibold mb-1">Dependencies</p>
              {deps.blockedBy.length > 0 ? (
                <div className="text-muted-foreground flex gap-1">
                  <span>← Blocked by:</span>
                  <span>{deps.blockedBy.map(d => d.title).join(', ')}</span>
                </div>
              ) : null}
              {deps.blocks.length > 0 ? (
                <div className="text-muted-foreground flex gap-1 mt-1">
                  <span>→ Blocks:</span>
                  <span>{deps.blocks.map(d => d.title).join(', ')}</span>
                </div>
              ) : null}
              {deps.blockedBy.length === 0 && deps.blocks.length === 0 && (
                <div className="text-muted-foreground">No dependencies</div>
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
      </Tabs>
    </div>
  )
}
