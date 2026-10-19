// Panel hiển thị KG context trong task detail (CR-ORCA-002)
//
// Rules:
// - Ẩn khi kg_context absent (khớp backend: CRSID == "" → skip enrichment)
// - Best-effort: render dù crsContext/featureContext undefined
// - Convention hint: "// implements: CRS-XXX" (từ BuildSystemPrompt — SOL-ORCA-002 §2.6)
// - CRS status badge từ raw crsContext JSON (json.RawMessage)

import type { FC } from 'react'
import type { TaskKGContext, TaskEnrichment } from '../../types/task-types'
import { useTaskEnrichment } from '../../hooks/use-task-enrichment'

interface TaskShape {
  id: string
  kg_context?: TaskKGContext
}

interface Props {
  task: TaskShape
  environmentId?: string
}

// CRS status values từ backend validation (SOL-ORCA-003 §2.2)
type CRSStatus = 'pending' | 'in-progress' | 'done' | 'cancelled'

const KGStatusBadge: FC<{ status: string }> = ({ status }) => (
  <span className={`kg-status-badge kg-status-badge--${status}`} title={`Status: ${status}`}>
    {status}
  </span>
)

export const TaskKGContextPanel: FC<Props> = ({ task, environmentId }) => {
  const { entry, loading } = useTaskEnrichment(task, environmentId)

  // Ẩn khi không có KG context (backend: skip enrichment nếu CRSID == "")
  if (task.kg_context?.crs_id == null && task.kg_context?.feature_ref == null) return null

  const kgCtx = task.kg_context
  const crsNode = (entry as TaskEnrichment | undefined)?.crsContext as
    | Record<string, unknown>
    | undefined
  const featureNode = (entry as TaskEnrichment | undefined)?.featureContext as
    | Record<string, unknown>
    | undefined

  return (
    <section
      className="task-kg-context-panel"
      aria-label="Knowledge Graph Context"
      id={`task-kg-context-${task.id}`}
    >
      <h4 className="task-kg-context-panel__title">KG Context</h4>

      {loading && (
        <div className="task-kg-context-panel__loading" aria-live="polite">
          Loading KG context…
        </div>
      )}

      {/* CRS node */}
      {kgCtx?.crs_id != null && (
        <div className="task-kg-context-panel__crs">
          <span className="kg-badge kg-badge--crs">CRS</span>
          <code>{kgCtx.crs_id}</code>
          {crsNode?.status != null && <KGStatusBadge status={crsNode.status as CRSStatus} />}
          {crsNode?.layer != null && (
            <span className="kg-meta">Layer: {String(crsNode.layer)}</span>
          )}
          {crsNode?.feature_ref != null && (
            <span className="kg-meta">Feature: {String(crsNode.feature_ref)}</span>
          )}
        </div>
      )}

      {/* Feature node */}
      {kgCtx?.feature_ref != null && (
        <div className="task-kg-context-panel__feature">
          <span className="kg-badge kg-badge--feature">Feature</span>
          <code>{kgCtx.feature_ref}</code>
          {featureNode?.description != null && (
            <span className="kg-meta">{String(featureNode.description)}</span>
          )}
        </div>
      )}

      {/* Service metadata */}
      {kgCtx?.service != null && (
        <div className="task-kg-context-panel__meta">
          <span className="kg-meta-label">Service:</span>
          <code>{kgCtx.service}</code>
        </div>
      )}

      {/* RepoTarget — dùng cho quality gate convention verify (SOL-ORCA-003) */}
      {kgCtx?.repo_target != null && (
        <div className="task-kg-context-panel__meta">
          <span className="kg-meta-label">Repo:</span>
          <code>{kgCtx.repo_target}</code>
        </div>
      )}

      {/* Convention hint từ BuildSystemPrompt() (SOL-ORCA-002 §2.6) */}
      {kgCtx?.crs_id != null && (
        <div className="task-kg-context-panel__convention-hint">
          <span className="kg-meta-label">Convention:</span>
          <code>// implements: {kgCtx.crs_id}</code>
          <span className="kg-meta"> — required in changed files</span>
        </div>
      )}
    </section>
  )
}
