/**
 * quality-lens-blocks.ts — FE-CV-TASK-087-07
 *
 * Registry of the collapsible blocks under the scorecard. Each block loads lazily and only when
 * opened, so closed blocks import no chart code and issue no requests.
 *
 * @module components/review-map/quality/quality-lens-blocks
 */

import type { ComponentType } from 'react'
import type { QualityBlockProps } from './quality-lens-block-types'
import type { QualityScorecardCopyKey } from './quality-scorecard-copy'

export type QualityLensBlock = {
  id: string
  titleKey: QualityScorecardCopyKey
  /** Contract phase that supplies the data (2: coverage/trend, 3: hotspot/dependency). */
  phase: 2 | 3
  load: () => Promise<{ default: ComponentType<QualityBlockProps> }>
}

export const QUALITY_LENS_BLOCKS: readonly QualityLensBlock[] = [
  {
    id: 'coverage',
    titleKey: 'blocks.coverage',
    phase: 2,
    load: () => import('./QualityCoveragePanel')
  },
  { id: 'trend', titleKey: 'blocks.trend', phase: 2, load: () => import('./QualityTrendPanel') },
  {
    id: 'hotspot',
    titleKey: 'blocks.hotspot',
    phase: 3,
    load: () => import('./QualityHotspotPanel')
  },
  {
    id: 'dependency',
    titleKey: 'blocks.dependency',
    phase: 3,
    load: () => import('./QualityDependencyPanel')
  }
]
