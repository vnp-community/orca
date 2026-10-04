import { lazy, Suspense, useMemo, useState } from 'react'
import { useTasks } from '../../hooks/useTasks'
import { useTaskDependencyEdges } from '../../hooks/useTaskDependencyEdges'
import { useTaskBatchExecution } from '../../hooks/useTaskBatchExecution'
import { useTaskBatchMerge } from '../../hooks/useTaskBatchMerge'
import { TaskTreeView } from './TaskTreeView'
import { TaskCreateDialog } from './TaskCreateDialog'
import { TaskMergeDialog } from './TaskMergeDialog'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { toast } from 'sonner'
import {
  derivePhase,
  buildSpecPrompt,
  buildImplementPrompt,
  PHASE_LABEL_SPEC_PENDING,
  PHASE_LABEL_CODE_PENDING
} from '../../lib/task-spec-build-loop'
import type { MergeStrategy } from '../../hooks/useTaskBatchMerge'

const TaskDAGView = lazy(() => import('./TaskDAGView'))
const TaskBoardView = lazy(() =>
  import('./TaskBoardView').then((m) => ({ default: m.TaskBoardView }))
)

export function TaskGraph({ projectId }: { projectId: string }) {
  const {
    filteredTasks,
    expandedNodes,
    toggleExpanded,
    setActiveTask,
    filterStatus,
    setFilterStatus,
    searchQuery,
    setSearchQuery,
    createTask,
    selectedIds,
    toggleSelected,
    clearSelection
  } = useTasks(projectId)
  const [viewMode, setViewMode] = useState<'tree' | 'dag' | 'board'>('tree')
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [createParentId, setCreateParentId] = useState<string | undefined>(undefined)
  const [selectMode, setSelectMode] = useState(false)

  const {
    edges: dependencyEdges,
    loading: depsLoading,
    error: depsError,
    refetch: refetchDependencyEdges
  } = useTaskDependencyEdges(filteredTasks)
  const { running, runSelected } = useTaskBatchExecution()
  const { running: merging, batchMergeWorktrees } = useTaskBatchMerge()
  const [mergeDialogOpen, setMergeDialogOpen] = useState(false)

  const selectedTasks = useMemo(
    () => filteredTasks.filter((t) => selectedIds.has(t.id)),
    [filteredTasks, selectedIds]
  )
  // BL-TG-06: only show a phase-specific batch action when EVERY selected
  // task shares the same phase — mixing phases in one batch action would
  // silently do the wrong thing for some of them (e.g. "Generate Spec" on
  // a task that's already past that phase). null = no selection, or a
  // mixed selection — no phase-specific button rendered either way; the
  // pre-existing generic "Run Selected" below still works regardless.
  const commonPhase = useMemo(() => {
    if (selectedTasks.length === 0) {
      return null
    }
    const phases = new Set(selectedTasks.map((t) => derivePhase(t)))
    return phases.size === 1 ? [...phases][0] : null
  }, [selectedTasks])
  const doneWithWorktree =
    selectedTasks.length > 0 && selectedTasks.every((t) => t.status === 'done' && !!t.worktreeId)

  const openCreateDialog = (parentId?: string) => {
    setCreateParentId(parentId)
    setCreateDialogOpen(true)
  }

  const handleCreateTask = (title: string, parentId?: string) =>
    createTask(title, parentId)
      .then(() => {
        toast.success(`Task "${title}" created`)
      })
      .catch((err: unknown) => {
        toast.error(`Failed to create task: ${err instanceof Error ? err.message : String(err)}`)
      })

  const handleRunSelected = () => {
    const count = selectedTasks.length
    runSelected(selectedTasks).then((results) => {
      const okCount = [...results.values()].filter((r) => r === 'ok').length
      toast.success(`${okCount}/${count} task(s) dispatched successfully`)
      clearSelection()
    })
  }

  // BL-TG-06 §B/§C: same underlying batch primitive as "Run Selected" —
  // just with a per-task built prompt and a phase label staged first.
  const handleBatchPhaseDispatch = (
    promptBuilder: (task: (typeof selectedTasks)[number]) => string,
    phaseLabel: string,
    verb: string
  ) => {
    const count = selectedTasks.length
    runSelected(selectedTasks, { promptBuilder, phaseLabel }).then((results) => {
      const okCount = [...results.values()].filter((r) => r === 'ok').length
      toast.success(`${verb}: ${okCount}/${count} task(s) dispatched`)
      clearSelection()
    })
  }

  const handleMerge = async (baseBranch: string, strategy: MergeStrategy) => {
    const tasks = selectedTasks
    const outcomes = await batchMergeWorktrees(tasks, baseBranch, strategy)
    const okCount = [...outcomes.values()].filter((o) => o.ok === true).length
    const conflict = [...outcomes.entries()].find(([, o]) => o.ok === 'conflict')
    const failed = [...outcomes.entries()].find(([, o]) => o.ok === false)
    if (conflict) {
      toast.error(
        `Merge stopped: conflict in task ${conflict[0]} — resolve it in the Git tab, then retry the rest`
      )
    } else if (failed) {
      const [, outcome] = failed
      toast.error(`Merge stopped: ${outcome.ok === false ? outcome.error : 'unknown error'}`)
    } else {
      toast.success(`${okCount}/${tasks.length} worktree(s) merged into ${baseBranch}`)
    }
    if (okCount > 0) {
      clearSelection()
    }
    setMergeDialogOpen(false)
  }

  return (
    <div className="task-graph flex flex-col h-full" data-testid="task-graph">
      <div className="flex items-center gap-2 p-2 border-b">
        <Input
          placeholder="Search tasks..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="flex-1 h-7 text-sm"
        />
        <select
          value={filterStatus}
          onChange={(e) => setFilterStatus(e.target.value)}
          className="text-sm border rounded px-2 py-1"
        >
          <option value="all">All Status</option>
          <option value="todo">Todo</option>
          <option value="in_progress">In Progress</option>
          <option value="done">Done</option>
        </select>
        <Button size="sm" onClick={() => openCreateDialog(undefined)} data-testid="new-task-btn">
          + New Task
        </Button>
        <button
          onClick={() => setSelectMode((v) => !v)}
          className="text-sm border rounded px-2 py-1"
          data-testid="toggle-select-mode"
        >
          {selectMode ? 'Cancel' : 'Select'}
        </button>
        {selectMode && selectedIds.size > 0 && (
          <Button
            size="sm"
            disabled={running}
            onClick={handleRunSelected}
            data-testid="run-selected-btn"
          >
            {running ? 'Running…' : `▶ Run Selected (${selectedIds.size})`}
          </Button>
        )}
        {/* BL-TG-06: phase-specific batch actions — only rendered when every
            selected task shares the same phase (commonPhase), so this never
            silently mixes actions across tasks at different points in the
            Spec → Approve → Code loop. */}
        {selectMode && commonPhase === 'not-started' && (
          <Button
            size="sm"
            variant="outline"
            disabled={running}
            onClick={() =>
              handleBatchPhaseDispatch(buildSpecPrompt, PHASE_LABEL_SPEC_PENDING, 'Generate Spec')
            }
            data-testid="batch-generate-spec-btn"
          >
            {running ? 'Running…' : `📝 Generate Spec (${selectedIds.size})`}
          </Button>
        )}
        {selectMode && commonPhase === 'spec-approved' && (
          <Button
            size="sm"
            variant="outline"
            disabled={running}
            onClick={() =>
              handleBatchPhaseDispatch(
                buildImplementPrompt,
                PHASE_LABEL_CODE_PENDING,
                'Implement Spec'
              )
            }
            data-testid="batch-implement-spec-btn"
          >
            {running ? 'Running…' : `🔨 Implement Spec (${selectedIds.size})`}
          </Button>
        )}
        {selectMode && doneWithWorktree && (
          <Button
            size="sm"
            variant="outline"
            onClick={() => setMergeDialogOpen(true)}
            data-testid="batch-merge-btn"
          >
            🔀 Merge Worktree ({selectedIds.size})
          </Button>
        )}
        <div className="flex border rounded overflow-hidden">
          <button
            onClick={() => setViewMode('tree')}
            className={`px-2 py-1 text-xs ${viewMode === 'tree' ? 'bg-primary text-primary-foreground' : ''}`}
            data-testid="view-tree"
          >
            Tree
          </button>
          <button
            onClick={() => setViewMode('dag')}
            className={`px-2 py-1 text-xs ${viewMode === 'dag' ? 'bg-primary text-primary-foreground' : ''}`}
            data-testid="view-dag"
          >
            DAG
          </button>
          <button
            onClick={() => setViewMode('board')}
            className={`px-2 py-1 text-xs ${viewMode === 'board' ? 'bg-primary text-primary-foreground' : ''}`}
            data-testid="view-board"
          >
            Board
          </button>
        </div>
      </div>
      <div className="flex-1 overflow-auto">
        {viewMode === 'board' ? (
          <Suspense fallback={<div className="p-4 text-sm">Loading Board...</div>}>
            <TaskBoardView
              tasks={filteredTasks}
              onSelect={setActiveTask}
              selectMode={selectMode}
              selectedIds={selectedIds}
              onToggleSelect={toggleSelected}
            />
          </Suspense>
        ) : viewMode === 'tree' ? (
          <TaskTreeView
            tasks={filteredTasks}
            expandedNodes={expandedNodes}
            toggleExpanded={toggleExpanded}
            setActiveTask={setActiveTask}
            onCreateSubtask={openCreateDialog}
            selectMode={selectMode}
            selectedIds={selectedIds}
            onToggleSelect={toggleSelected}
          />
        ) : (
          <Suspense fallback={<div className="p-4 text-sm">Loading DAG...</div>}>
            <TaskDAGView
              tasks={filteredTasks}
              dependencyEdges={dependencyEdges}
              onSelect={setActiveTask}
              onEdgeAdded={refetchDependencyEdges}
            />
            {depsLoading && (
              <div className="text-xs text-muted-foreground px-2">Loading dependencies…</div>
            )}
            {depsError && (
              <div className="text-xs text-destructive px-2">
                Some dependencies may have failed to load
              </div>
            )}
          </Suspense>
        )}
      </div>
      <TaskCreateDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        parentId={createParentId}
        onCreate={handleCreateTask}
      />
      <TaskMergeDialog
        open={mergeDialogOpen}
        onOpenChange={setMergeDialogOpen}
        taskCount={selectedTasks.length}
        merging={merging}
        onMerge={handleMerge}
      />
    </div>
  )
}
