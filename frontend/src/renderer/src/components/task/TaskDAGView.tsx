import { useMemo, useCallback, useEffect, useState } from 'react'
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
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { useAppStore } from '../../store'
import type { OrcaTask, TaskEdgeType } from '../../../../shared/task-types'

type TaskDAGViewProps = {
  tasks: OrcaTask[]
  onSelect: (taskId: string) => void
}

// FE-TASK-002 (task-graph v4): task.getDependencies real response shape is a flat
// { task, edgeType }[] (channels_automation_task.go:330-347) — same as TaskDetail.tsx's
// existing usage, NOT { dependencies: { taskId }[] } as an earlier draft assumed.
function useDependencyEdges(tasks: OrcaTask[]) {
  const [depsById, setDepsById] = useState<Map<string, string[]>>(new Map())

  useEffect(() => {
    let cancelled = false
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    Promise.all(
      tasks.map(async (t) => {
        const edges = (await callRuntimeRpc(target, 'task.getDependencies', {
          taskId: t.id
        })) as { task: OrcaTask; edgeType: TaskEdgeType }[]
        return [
          t.id,
          edges.filter((e) => e.edgeType === 'depends_on').map((e) => e.task.id)
        ] as const
      })
    )
      .then((pairs) => {
        if (!cancelled) {
          setDepsById(new Map(pairs))
        }
      })
      .catch(() => {
        // Giữ depsById cũ nếu 1 request lỗi — DAG vẫn render với dữ liệu đã có, không crash toàn bộ view.
      })
    return () => {
      cancelled = true
    }
  }, [tasks])

  return depsById
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
  depsById: Map<string, string[]>
): { nodes: Node[]; edges: Edge[] } {
  if (tasks.length === 0) {
    return { nodes: [], edges: [] }
  }

  // Build dependency map: taskId -> list of taskIds this depends on — from the real
  // task.getDependencies RPC (via useDependencyEdges above), not the old always-[]
  // (task as any).dependsOn placeholder.
  const dependsOnMap = new Map<string, string[]>()
  for (const task of tasks) {
    dependsOnMap.set(task.id, depsById.get(task.id) ?? [])
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
    const deps = depsById.get(task.id) ?? []
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
  const depsById = useDependencyEdges(tasks)
  const { nodes, edges } = useMemo(() => buildDAGLayout(tasks, depsById), [tasks, depsById])
  const [addingFor, setAddingFor] = useState<string | null>(null)

  const addDependency = useCallback(async (fromTaskId: string, toTaskId: string) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    try {
      await callRuntimeRpc(target, 'task.addEdge', { fromTaskId, toTaskId, type: 'depends_on' })
      setAddingFor(null)
      // Không có cách nào refetch depsById từ đây khác ngoài đợi `tasks` prop đổi (trigger
      // useEffect ở useDependencyEdges) — chấp nhận được vì `tasks` re-render mỗi khi
      // useTasks's refetch chạy; nếu cần cập nhật ngay lập tức, thêm 1 forceRefresh cục bộ
      // ở bước follow-up (ngoài phạm vi task này).
    } catch (err) {
      // Backend đã atomic + tự cycle-detect (BE-SOL-001) — không tự lặp lại validation ở
      // client, chỉ hiện lỗi. addingFor KHÔNG reset để người dùng thử lại ngay.
      toast.error(`Cannot add dependency: ${(err as Error).message}`)
    }
  }, [])

  const onNodeClick = useCallback<NodeMouseHandler>(
    (_event, node) => {
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
      <div className="flex-1">
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
    </div>
  )
}

export default TaskDAGView
