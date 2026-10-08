/**
 * quality-lens-registration.tsx — FE-CV-TASK-087-07 / 087-13
 *
 * Registers the `quality` lens and its dock panel. Both are quality-gated (hidden, and the lazy
 * modules never loaded, while the flag is off). The check findings are their own dock panel:
 * structural Findings and QualityFindings are different sources and are never merged.
 *
 * @module components/review-map/quality/quality-lens-registration
 */

import { Suspense, lazy } from 'react'
import type { ReviewLensDefinition } from '../review-lens-registry'
import { registerReviewDockPanel } from '../shell/review-dock-registry'

export const QUALITY_LENS_ID = 'quality'
export const QUALITY_FINDINGS_DOCK_ID = 'quality-findings'

// Why: order 80 sits after the canonical lenses (10..70) and before requirements (90).
export const qualityLensDefinition: ReviewLensDefinition = {
  id: QUALITY_LENS_ID,
  order: 80,
  labelKey: 'auto.components.reviewMap.lens.quality.label',
  labelFallback: 'Quality',
  requiresQuality: true,
  load: () => import('./QualityLens')
}

const QualityFindingsDockPanel = lazy(() => import('./findings/QualityFindingsDockPanel'))

registerReviewDockPanel({
  id: QUALITY_FINDINGS_DOCK_ID,
  order: 15,
  labelKey: 'auto.components.reviewMap.shell.dock.qualityFindings',
  labelFallback: 'Checks',
  requiresQuality: true,
  render: (p) => (
    <Suspense fallback={null}>
      <QualityFindingsDockPanel worktreeId={p.worktreeId} onOpenDiff={p.onOpenDiff} />
    </Suspense>
  )
})
