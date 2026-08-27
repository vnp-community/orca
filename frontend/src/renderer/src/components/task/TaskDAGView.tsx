import { useMemo, useCallback } from 'react'
import { ReactFlow, type Node, type Edge, Background, Controls, MiniMap } from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { OrcaTask } from '../../../../shared/task-types'

type TaskDAGViewProps = {
  tasks: OrcaTask[]
  onSelect: (taskId: string) => void
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

function buildDAGLayout(tasks: OrcaTask[]): { nodes: Node[]; edges: Edge[] } {
  if (tasks.length === 0) {
    return { nodes: [], edges: [] }
  }

  // Build dependency map: taskId -> list of taskIds this depends on
  // NOTE: OrcaTask has no embedded `dependsOn` — edges live in a separate table
  // (task.getDependencies per task). Always [] until that's wired in (out of scope here).
  const dependsOnMap = new Map<string, string[]>()
  for (const task of tasks) {
    dependsOnMap.set(task.id, (task as any).dependsOn ?? [])
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
    const deps = (task as any).dependsOn ?? []
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

export function TaskDAGView({ tasks, onSelect }: TaskDAGViewProps) {
  const { nodes, edges } = useMemo(() => buildDAGLayout(tasks), [tasks])

  const onNodeClick = useCallback(
    (_event: any, node: any) => {
      onSelect(node.id)
    },
    [onSelect]
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
    <div className="task-dag-view h-full min-h-[300px]" data-testid="task-dag-view">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodeClick={onNodeClick}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={true}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={16} />
        <Controls showInteractive={false} />
        <MiniMap nodeStrokeWidth={2} />
      </ReactFlow>
    </div>
  )
}

export default TaskDAGView
