import { lazy, Suspense, useState } from 'react'
import { useTasks } from '../../hooks/useTasks'
import { TaskTreeView } from './TaskTreeView'
import { Input } from '../ui/input'

const TaskDAGView = lazy(() => import('./TaskDAGView'))

export function TaskGraph({ projectId }: { projectId: string }) {
  const { filteredTasks, expandedNodes, toggleExpanded, setActiveTask, filterStatus, setFilterStatus, searchQuery, setSearchQuery } = useTasks(projectId)
  const [viewMode, setViewMode] = useState<'tree' | 'dag'>('tree')

  return (
    <div className="task-graph flex flex-col h-full" data-testid="task-graph">
      <div className="flex items-center gap-2 p-2 border-b">
        <Input placeholder="Search tasks..." value={searchQuery} onChange={e => setSearchQuery(e.target.value)} className="flex-1 h-7 text-sm" />
        <select value={filterStatus} onChange={e => setFilterStatus(e.target.value as any)} className="text-sm border rounded px-2 py-1">
          <option value="all">All Status</option>
          <option value="todo">Todo</option>
          <option value="in_progress">In Progress</option>
          <option value="done">Done</option>
        </select>
        <div className="flex border rounded overflow-hidden">
          <button onClick={() => setViewMode('tree')} className={`px-2 py-1 text-xs ${viewMode === 'tree' ? 'bg-primary text-primary-foreground' : ''}`} data-testid="view-tree">Tree</button>
          <button onClick={() => setViewMode('dag')} className={`px-2 py-1 text-xs ${viewMode === 'dag' ? 'bg-primary text-primary-foreground' : ''}`} data-testid="view-dag">DAG</button>
        </div>
      </div>
      <div className="flex-1 overflow-auto">
        {viewMode === 'tree' ? (
          <TaskTreeView tasks={filteredTasks} expandedNodes={expandedNodes} toggleExpanded={toggleExpanded} setActiveTask={setActiveTask} />
        ) : (
          <Suspense fallback={<div className="p-4 text-sm">Loading DAG...</div>}>
            <TaskDAGView tasks={filteredTasks} onSelect={setActiveTask} />
          </Suspense>
        )}
      </div>
    </div>
  )
}
