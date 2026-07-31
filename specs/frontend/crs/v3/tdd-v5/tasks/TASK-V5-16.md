# TASK-V5-16: TaskDetail + TaskAIDecompose + TaskPromptEditor

**Order:** 16 | **Prerequisite:** TASK-V5-15 | **Tests:** 11

---

## Files Cần Tạo

### 1. `src/renderer/src/components/task/TaskDetail.tsx`

```typescript
// Right-panel detail view for active task
// Fields: title (editable), description (textarea), type, status, priority, assignee, progress
// Tabs: Details | Subtasks | AI Agent

export function TaskDetail() {
  const activeTaskId = useAppStore(s => s.activeTaskId)
  const { task, updateTask } = useTask(activeTaskId!)
  const [localTitle, setLocalTitle] = useState(task?.title ?? '')
  const [activeTab, setActiveTab]   = useState<'details' | 'subtasks' | 'ai'>('details')

  if (!task) return <div className="p-4 text-sm text-muted-foreground">Select a task</div>

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
```

### 2. `src/renderer/src/components/task/TaskAIDecompose.tsx`

```typescript
export function TaskAIDecompose({ parentTask }: { parentTask: OrcaTask }) {
  const { aiDecompose, acceptSubtasks } = useTask(parentTask.id)
  const [instruction,      setInstruction]    = useState('')
  const [isDecomposing,    setIsDecomposing]  = useState(false)
  const [proposedSubtasks, setProposedSubtasks] = useState<Partial<OrcaTask>[]>([])

  const decompose = async () => {
    setIsDecomposing(true)
    try {
      const subtasks = await aiDecompose(instruction || undefined)
      setProposedSubtasks(subtasks)
    } finally {
      setIsDecomposing(false)
    }
  }

  const accept = async () => {
    await acceptSubtasks(proposedSubtasks, parentTask.projectId)
    setProposedSubtasks([])
  }

  return (
    <div className="task-ai-decompose space-y-3 mt-3" data-testid="task-ai-decompose">
      <Input
        value={instruction}
        onChange={e => setInstruction(e.target.value)}
        placeholder="Optional: decompose instructions..."
      />
      <Button onClick={decompose} disabled={isDecomposing} data-testid="decompose-btn">
        {isDecomposing ? <><Loader2 size={12} className="animate-spin mr-1" /> Decomposing...</> : '🤖 Decompose with AI'}
      </Button>

      {proposedSubtasks.length > 0 && (
        <div className="proposed-subtasks space-y-1" data-testid="proposed-subtasks">
          {proposedSubtasks.map((st, i) => (
            <div key={i} className="flex items-center gap-2 px-2 py-1 rounded bg-muted/50 text-sm">
              <span className="text-xs font-mono text-muted-foreground">{st.type}</span>
              <span>{st.title}</span>
            </div>
          ))}
          <div className="flex gap-2 pt-2">
            <Button size="sm" onClick={accept} data-testid="accept-subtasks-btn">Accept All</Button>
            <Button size="sm" variant="ghost" onClick={() => setProposedSubtasks([])}>Cancel</Button>
          </div>
        </div>
      )}
    </div>
  )
}
```

### 3. `src/renderer/src/components/task/TaskPromptEditor.tsx`

```typescript
export function TaskPromptEditor({ task }: { task: OrcaTask }) {
  const [prompt, setPrompt]       = useState(task.agentPrompt ?? '')
  const [isRunning, setIsRunning] = useState(false)
  const { project }               = useWorkspace()

  const runWithAgent = async () => {
    setIsRunning(true)
    try {
      await callRuntimeRpc('task.runAgent', {
        taskId: task.id,
        prompt: prompt || task.agentPrompt,
        projectId: project!.id,
      })
    } finally {
      setIsRunning(false)
    }
  }

  return (
    <div className="task-prompt-editor space-y-3 mt-3" data-testid="task-prompt-editor">
      <Textarea
        value={prompt}
        onChange={e => setPrompt(e.target.value)}
        placeholder="Describe what the agent should do for this task..."
        rows={4}
      />
      <Button onClick={runWithAgent} disabled={isRunning || !prompt.trim()} data-testid="run-agent-btn">
        {isRunning ? <><Loader2 size={12} className="animate-spin mr-1" />Running...</> : '▶ Run with Agent'}
      </Button>
    </div>
  )
}
```

---

## Tests (11 total)

```
__tests__/task/TaskDetail.test.tsx        (4 tests)
  renders "Select a task" when no activeTask
  title input blur → calls updateTask
  status change → updateTask called
  AI Agent tab shows TaskPromptEditor

__tests__/task/TaskAIDecompose.test.tsx   (5 tests)
  "Decompose" calls task.aiDecompose
  shows proposed subtasks list
  "Accept All" calls acceptSubtasks
  "Cancel" clears proposed subtasks
  shows spinner during decomposing

__tests__/task/TaskPromptEditor.test.tsx  (2 tests)
  Run button disabled when prompt empty
  Run button calls task.runAgent RPC
```

## Acceptance Criteria

- [x] `TaskDetail` shows "Select a task" fallback
- [x] Title edit on blur saves to server
- [x] `TaskAIDecompose` calls `task.aiDecompose` RPC
- [x] Accept creates all subtasks in store
- [x] `TaskPromptEditor` run button disabled when empty
- [x] 11/11 tests pass
