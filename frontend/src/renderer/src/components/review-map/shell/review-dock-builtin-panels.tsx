/**
 * review-dock-builtin-panels.tsx — FE-CV-TASK-059-05 / 060-03
 *
 * Side-effect module: registers the structural-findings and review-notes panels in the dock.
 *
 * @module components/review-map/shell/review-dock-builtin-panels
 */

import { useMemo } from 'react'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { openReviewFileInEditor } from '@/lib/review-diff-navigation'
import { FindingsPanel } from '../findings/FindingsPanel'
import { ReviewNotesPanel } from '../notes/ReviewNotesPanel'
import { registerReviewDockPanel } from './review-dock-registry'
import type { ReviewDockPanelProps } from './review-dock-registry'

function StructuralFindingsDockPanel(props: ReviewDockPanelProps): React.JSX.Element {
  const { worktreeId, environmentId, overlay, availableLensIds, onOpenDiff } = props
  const changedFiles = useMemo(
    () => new Set(overlay.changedFiles.map((f) => f.path)),
    [overlay.changedFiles]
  )
  return (
    <FindingsPanel
      worktreeId={worktreeId}
      environmentId={environmentId}
      changedFiles={changedFiles}
      graphSymbolKeys={null}
      availableLensIds={availableLensIds}
      onOpenDiff={onOpenDiff}
      onOpenFile={(path, line) => {
        const r = openReviewFileInEditor(worktreeId, { filePath: path }, { line })
        if (!r.ok) {
          toast.error(
            translate(
              'auto.components.reviewMap.impact.openPathBlocked',
              'This path is outside the worktree and was not opened.'
            )
          )
        }
      }}
    />
  )
}

registerReviewDockPanel({
  id: 'findings',
  order: 10,
  labelKey: 'auto.components.reviewMap.shell.dock.findings',
  labelFallback: 'Findings',
  render: (p) => <StructuralFindingsDockPanel {...p} />
})

registerReviewDockPanel({
  id: 'notes',
  order: 20,
  labelKey: 'auto.components.reviewMap.shell.dock.notes',
  labelFallback: 'Notes',
  render: (p) => <ReviewNotesPanel worktreeId={p.worktreeId} onOpenDiff={p.onOpenDiff} />
})
