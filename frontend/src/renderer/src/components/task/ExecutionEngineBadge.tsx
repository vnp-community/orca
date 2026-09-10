import { useAppStore } from '../../store'
import type { OrcaTask } from '../../../../shared/task-types'

export type InferredExecutionEngine =
  | { kind: 'workflow'; templateName: string }
  | { kind: 'orchestration' }
  | { kind: 'direct_agent' }

// Client-side inference only — CR-FLOW-TASK-001's `execution_links` table + backend
// `selectEngine()` are not implemented yet, so there is no `task.engine` field to read.
// Priority order (workflow > orchestration > direct_agent) MUST stay in sync with the
// order CR-001 already committed to server-side, so the badge doesn't "flip" once a
// real `task.engine` field lands.
export function useInferredExecutionEngine(task: OrcaTask): InferredExecutionEngine {
  const templates = useAppStore((s) => s.templates)
  const hasChildren = useAppStore((s) => s.tasks.some((t) => t.parentId === task.id))

  if (task.workflowTemplateId) {
    const template = templates.find((t) => t.id === task.workflowTemplateId)
    return { kind: 'workflow', templateName: template?.name ?? task.workflowTemplateId }
  }
  if (hasChildren) {
    return { kind: 'orchestration' }
  }
  return { kind: 'direct_agent' }
}

// Style tokens reused verbatim from TaskStatusBadge/TaskPriorityBadge (text-xs px-1.5
// py-0.5 rounded border + text-*-600/border-*-200) — no new tokens invented (AGENTS.md).
export function ExecutionEngineBadge({ task }: { task: OrcaTask }) {
  const engine = useInferredExecutionEngine(task)

  const label =
    engine.kind === 'workflow'
      ? `Workflow: ${engine.templateName}`
      : engine.kind === 'orchestration'
        ? 'Orchestration'
        : 'Direct Agent'

  const className =
    engine.kind === 'workflow'
      ? 'text-purple-600 border-purple-200'
      : engine.kind === 'orchestration'
        ? 'text-blue-600 border-blue-200'
        : 'text-gray-600 border-gray-200'

  return (
    <span
      className={`inline-flex items-center text-xs px-1.5 py-0.5 rounded border ${className}`}
      data-testid="execution-engine-badge"
    >
      {label}
    </span>
  )
}
