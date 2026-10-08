/**
 * RequirementTracePanel.tsx — FE-CV-TASK-092-05
 *
 * The `requirements` review lens: requirements grouped "no evidence" first, inferred
 * suggestions kept apart, unlinked changes at the bottom. Hidden while the quality
 * flag is off. Never says a requirement is satisfied.
 *
 * @module components/review-map/requirements/RequirementTracePanel
 */

import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useAppStore } from '@/store'
import { translateCatalogKey } from '@/i18n/catalog-key-translate'
import { findWorktreeById } from '../../../store/slices/worktree-helpers'
import type { ReviewLensProps } from '../review-lens-registry'
import { RequirementRow } from './RequirementRow'
import { UnlinkedChangesList } from './UnlinkedChangesList'
import { WorktreeTaskLinkPicker } from './WorktreeTaskLinkPicker'
import { useRequirementTrace } from './use-requirement-trace'
import type { EvidenceRowViewModel } from './requirement-trace-view-model'

const BASE = 'auto.components.reviewMap.requirements.trace'

export type RequirementTracePanelProps = {
  worktreeId: string
  projectId: string | null | undefined
  onOpenDiff: (path: string, line?: number) => void
  translate?: (key: string, params?: Record<string, unknown>) => string
}

export function RequirementTracePanel({
  worktreeId,
  projectId,
  onOpenDiff,
  translate = translateCatalogKey
}: RequirementTracePanelProps): React.JSX.Element | null {
  const trace = useRequirementTrace({ projectId, worktreeId })
  const [pickerOpen, setPickerOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  if (trace.status === 'disabled') {
    return null
  }
  if (trace.status === 'idle' || trace.status === 'loading') {
    return (
      <div role="status" className="flex items-center gap-2 p-3 text-xs text-muted-foreground">
        <Loader2 className="size-3.5 animate-spin" aria-hidden />
        {translate(`${BASE}.loading`)}
      </div>
    )
  }
  if (trace.status === 'error' || !trace.view) {
    return (
      <div className="space-y-2 p-3 text-xs">
        <p className="text-muted-foreground">{translate(`${BASE}.error`)}</p>
        <Button type="button" variant="outline" size="xs" onClick={trace.refetch}>
          {translate(`${BASE}.retry`)}
        </Button>
      </div>
    )
  }

  const { view } = trace
  const guard = async (run: () => Promise<void>): Promise<void> => {
    setBusy(true)
    try {
      await run()
    } finally {
      setBusy(false)
    }
  }
  const openEvidence = (e: EvidenceRowViewModel): void => {
    if (e.kind === 'change' || e.kind === 'test') {onOpenDiff(e.ref)}
  }

  return (
    <section aria-label={translate(`${BASE}.label`)} className="flex min-h-0 flex-col gap-2 overflow-y-auto p-3">
      <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
        <span>{translate(`${BASE}.link.${view.linkConfidence}`)}</span>
        <label className="flex items-center gap-1">
          <input
            type="checkbox"
            checked={trace.showInferred}
            onChange={(event) => trace.setShowInferred(event.target.checked)}
          />
          {translate(`${BASE}.showInferred`)}
        </label>
        {!trace.readOnly && projectId && (view.canLinkTask || pickerOpen) ? (
          <WorktreeTaskLinkPicker
            projectId={projectId}
            linkedTaskId={null}
            disabled={busy}
            onLink={(taskId) => void guard(() => trace.linkTask(taskId))}
            onUnlink={() => void guard(() => trace.unlinkTask())}
          />
        ) : null}
        {!trace.readOnly && !view.canLinkTask && !pickerOpen ? (
          <Button type="button" variant="ghost" size="xs" onClick={() => setPickerOpen(true)}>
            {translate('auto.components.reviewMap.requirements.taskPicker.change')}
          </Button>
        ) : null}
        {!trace.readOnly && !view.canLinkTask ? (
          <Button type="button" variant="ghost" size="xs" disabled={busy} onClick={() => void guard(() => trace.unlinkTask())}>
            {translate('auto.components.reviewMap.requirements.taskPicker.unlink')}
          </Button>
        ) : null}
      </div>

      {trace.stale ? <p className="text-[11px] text-muted-foreground">{translate(`${BASE}.stale`)}</p> : null}
      {trace.readOnly ? <p className="text-[11px] text-muted-foreground">{translate(`${BASE}.readOnly`)}</p> : null}
      {trace.actionError && trace.actionError !== 'forbidden' ? (
        <p role="status" className="text-[11px] text-muted-foreground">{translate(`${BASE}.actionError`)}</p>
      ) : null}
      {view.warnings.map((w) => (
        <p key={w.code} className="text-[11px] text-muted-foreground">
          {translate(w.labelKey)}
        </p>
      ))}

      {view.isEmpty ? (
        <p className="text-xs text-muted-foreground">{translate(`${BASE}.empty`)}</p>
      ) : (
        view.groups.map((group) => (
          <section key={group.key} aria-label={translate(group.labelKey)}>
            <h3 className="px-1 pb-1 text-[11px] font-medium text-muted-foreground">
              {translate(group.labelKey)} ({group.rows.length})
            </h3>
            <ul>
              {group.rows.map((row) => (
                <RequirementRow
                  key={row.key}
                  row={row}
                  readOnly={trace.readOnly}
                  busy={busy}
                  onOpenEvidence={openEvidence}
                  onConfirm={(key, evidence) => void guard(() => trace.confirm(key, evidence))}
                  onReject={(key, evidence) => void guard(() => trace.reject(key, evidence))}
                  translate={translate}
                />
              ))}
            </ul>
          </section>
        ))
      )}

      <UnlinkedChangesList changes={view.unlinked} onOpenFile={(file) => onOpenDiff(file)} translate={translate} />
    </section>
  )
}

/** Lens entry: resolves the project from the worktree record. */
export default function RequirementTraceLens({ worktreeId, onOpenDiff }: ReviewLensProps): React.JSX.Element | null {
  const projectId = useAppStore((s) => findWorktreeById(s.worktreesByRepo, worktreeId)?.projectId)
  return <RequirementTracePanel worktreeId={worktreeId} projectId={projectId} onOpenDiff={onOpenDiff} />
}
