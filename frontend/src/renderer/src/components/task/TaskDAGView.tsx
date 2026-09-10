import { useMemo, useCallback, useState } from 'react'
import {
  ReactFlow,
  type Node,
  type Edge,
  type NodeMouseHandler,
  Background,
  Controls,
  MiniMap
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { toast } from 'sonner'
import { useAppStore } from '../../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import type { TaskEdgeMap } from '../../hooks/useTaskDependencyEdges'
import type { OrcaTask } from '../../../../shared/task-types'

type TaskDAGViewProps = {
  tasks: OrcaTask[]
  dependencyEdges: TaskEdgeMap
  onSelect: (taskId: string) => void
  onEdgeAdded?: () => void
}

// Color coding by status
const STATUS_COLORS: Record<string, { bg: string; border: string }> = {
  done: { bg: '#f0fdf4', border: '#16a34a' },
  in_progress: { bg: '#eff6ff', border: '#2563eb' },
  blocked: { bg: '#fef2f2', border: '#dc2626' },
  review: { bg: '#faf5ff', border: '#9333ea' },
  todo: { bg: '#f8fafc', border: '#94a3b8' },
  backlog: { bg: '#f8fafc', border: '#cbd5e1' },
  cancelled: { bg: '#f1f5f9', border: '#64748b' }
}

function buildDAGLayout(
  tasks: OrcaTask[],
  dependencyEdges: TaskEdgeMap
): { nodes: Node[]; edges: Edge[] } {
  if (tasks.length === 0) {
    return { nodes: [], edges: [] }
  }

  // Build dependency map: taskId -> list of taskIds this depends on, sourced from
  // useTaskDependencyEdges (real task.getDependencies data, not a fictitious field
  // on OrcaTask itself — edges live in a separate table).
  const dependsOnMap = new Map<string, string[]>()
  for (const task of tasks) {
    dependsOnMap.set(task.id, dependencyEdges.get(task.id)?.blockedBy ?? [])
  }

  // Topological wave assignment
  const waveMap = new Map<string, number>()
  function getWave(id: string, visited = new Set<string>()): number {
    if (waveMap.has(id)) {
      return waveMap.get(id)!
    }
    if (visited.has(id)) {
      return 0
    } // cycle guard
    visited.add(id)
    const deps = dependsOnMap.get(id) ?? []
    const wave =
      deps.length === 0 ? 0 : Math.max(...deps.map((d) => getWave(d, new Set(visited)))) + 1
    waveMap.set(id, wave)
    return wave
  }
  tasks.forEach((t) => getWave(t.id))

  // Group tasks by wave
  const waveGroups = new Map<number, OrcaTask[]>()
  for (const task of tasks) {
    const wave = waveMap.get(task.id) ?? 0
    if (!waveGroups.has(wave)) {
      waveGroups.set(wave, [])
    }
    waveGroups.get(wave)!.push(task)
  }

  // Position and create nodes
  const nodes: Node[] = []
  const HORIZONTAL_GAP = 220
  const VERTICAL_GAP = 90

  for (const [wave, waveTasks] of waveGroups) {
    waveTasks.forEach((task, idx) => {
      const colors = STATUS_COLORS[task.status] ?? STATUS_COLORS.todo
      nodes.push({
        id: task.id,
        position: { x: wave * HORIZONTAL_GAP, y: idx * VERTICAL_GAP },
        data: {
          label: (
            <div style={{ fontSize: 11, padding: '2px 4px' }}>
              <div
                style={{
                  fontWeight: 600,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  maxWidth: 140
                }}
              >
                {task.title}
              </div>
              <div style={{ color: '#6b7280', marginTop: 2 }}>[{task.type}]</div>
            </div>
          )
        },
        style: {
          background: colors.bg,
          border: `2px solid ${colors.border}`,
          borderRadius: 8,
          width: 170,
          minHeight: 50
        }
      })
    })
  }

  // Create dependency edges
  const edges: Edge[] = []
  for (const task of tasks) {
    const deps = dependencyEdges.get(task.id)?.blockedBy ?? []
    for (const depId of deps) {
      if (tasks.find((t) => t.id === depId)) {
        edges.push({
          id: `${depId}->${task.id}`,
          source: depId,
          target: task.id,
          animated: task.status === 'in_progress',
          style: { stroke: '#94a3b8', strokeWidth: 1.5 }
        })
      }
    }
  }

  return { nodes, edges }
}

export function TaskDAGView({ tasks, dependencyEdges, onSelect, onEdgeAdded }: TaskDAGViewProps) {
  const { nodes, edges } = useMemo(
    () => buildDAGLayout(tasks, dependencyEdges),
    [tasks, dependencyEdges]
  )
  const [addingFor, setAddingFor] = useState<string | null>(null)

  const onNodeClick = useCallback<NodeMouseHandler>(
    (_event, node) => {
      onSelect(node.id)
    },
    [onSelect]
  )

  // Wires up to `task.addEdge` — the RPC exists at gRPC/proto level (AddEdgeRequest,
  // task.proto:84-90) but is not yet registered in wscompat's channel registry
  // (TASK-FE-TASKV1-04). Every connect will fail with "method not found" until
  // backend-go wires it — surfaced via toast, not hidden behind a feature flag.
  const onConnect = useCallback(
    async (connection: { source: string | null; target: string | null }) => {
      if (!connection.source || !connection.target) {
        return
      }
      const sourceTitle = tasks.find((t) => t.id === connection.source)?.title ?? connection.source
      const targetTitle = tasks.find((t) => t.id === connection.target)?.title ?? connection.target
      const confirmed = window.confirm(
        `Set dependency: "${targetTitle}" depends on "${sourceTitle}"?`
      )
      if (!confirmed) {
        return
      }
      try {
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        // Channel name assumed "task.addEdge" following the camelCase convention of
        // the other task.* channels — RECONFIRM the real name once backend-go wires
        // it (not yet tracked as its own backend-go task, see TASK-FE-TASKV1-04).
        await callRuntimeRpc(target, 'task.addEdge', {
          fromTaskId: connection.source,
          toTaskId: connection.target,
          type: 'EDGE_TYPE_DEPENDS_ON'
        })
        onEdgeAdded?.()
      } catch (err) {
        toast.error(`Could not add dependency: ${(err as Error).message}`)
      }
    },
    [tasks, onEdgeAdded]
  )

  // Dropdown-based alternative to drag-connect above — same task.addEdge RPC, useful when
  // dragging between nodes isn't practical (keyboard/touch users, dense graphs).
  const addDependency = useCallback(
    async (fromTaskId: string, toTaskId: string) => {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      try {
        await callRuntimeRpc(target, 'task.addEdge', { fromTaskId, toTaskId, type: 'depends_on' })
        setAddingFor(null)
        onEdgeAdded?.()
      } catch (err) {
        // Deliberately does NOT reset addingFor — the "to" dropdown stays open so the user
        // can retry (e.g. backend's cycle-detection rejected the edge).
        toast.error(`Failed to add dependency: ${err instanceof Error ? err.message : String(err)}`)
      }
    },
    [onEdgeAdded]
  )

  if (tasks.length === 0) {
    return (
      <div
        className="flex items-center justify-center h-full text-sm text-muted-foreground"
        data-testid="task-dag-empty"
      >
        No tasks to display
      </div>
    )
  }

  return (
    <div className="task-dag-view h-full min-h-[300px] flex flex-col" data-testid="task-dag-view">
      <div className="flex items-center gap-2 p-1 border-b text-xs">
        <select
          className="border rounded px-1 py-0.5"
          onChange={(e) => setAddingFor(e.target.value || null)}
          value={addingFor ?? ''}
          data-testid="dag-add-dependency-select-from"
        >
          <option value="">+ Add dependency: select task...</option>
          {tasks.map((t) => (
            <option key={t.id} value={t.id}>
              {t.title}
            </option>
          ))}
        </select>
        {addingFor && (
          <select
            className="border rounded px-1 py-0.5"
            onChange={(e) => e.target.value && addDependency(addingFor, e.target.value)}
            data-testid="dag-add-dependency-select-to"
          >
            <option value="">depends on...</option>
            {tasks
              .filter((t) => t.id !== addingFor)
              .map((t) => (
                <option key={t.id} value={t.id}>
                  {t.title}
                </option>
              ))}
          </select>
        )}
      </div>
      <div className="flex-1 min-h-0">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodeClick={onNodeClick}
          onConnect={onConnect}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          nodesDraggable={false}
          nodesConnectable
          elementsSelectable={true}
          proOptions={{ hideAttribution: true }}
        >
          <Background gap={16} />
          <Controls showInteractive={false} />
          <MiniMap nodeStrokeWidth={2} />
        </ReactFlow>
      </div>
    </div>
  )
}

export default TaskDAGView
