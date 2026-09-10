import { lazy, Suspense, useState } from 'react'
import { useTasks } from '../../hooks/useTasks'
import { useTaskDependencyEdges } from '../../hooks/useTaskDependencyEdges'
import { useTaskBatchExecution } from '../../hooks/useTaskBatchExecution'
import { TaskTreeView } from './TaskTreeView'
import { TaskCreateDialog } from './TaskCreateDialog'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { toast } from 'sonner'

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
    const ids = [...selectedIds]
    runSelected(ids).then((results) => {
      const okCount = [...results.values()].filter((r) => r === 'ok').length
      toast.success(`${okCount}/${ids.length} task(s) dispatched successfully`)
      clearSelection()
    })
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
            <TaskBoardView tasks={filteredTasks} onSelect={setActiveTask} />
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
    </div>
  )
}
