/**
 * review-dock-registry.ts — FE-CV-TASK-059-05 / 060-03
 *
 * Plug-in point of the bottom dock. Structural `Finding` (lens 059) and the quality gate's
 * `QualityFinding` are separate sources: each owns its own panel here and they are never merged.
 * The quality lens registers its panel with `registerReviewDockPanel({ requiresQuality: true })`.
 *
 * @module components/review-map/shell/review-dock-registry
 */

import type { ReactNode } from 'react'
import type { ReviewScope } from '../review-scope-model'
import type { ChangeOverlayView } from '../review-wire-types'

export type ReviewDockPanelProps = {
  worktreeId: string
  environmentId: string | null
  scope: ReviewScope
  overlay: ChangeOverlayView
  /** Ids of the lenses the user can currently see (for "View in graph"). */
  availableLensIds: ReadonlySet<string>
  onOpenDiff: (path: string, line?: number) => void
}

export type ReviewDockPanelDefinition = {
  id: string
  order: number
  labelKey: string
  labelFallback: string
  render: (props: ReviewDockPanelProps) => ReactNode
  /** Hidden unless the quality-gate flag is on. */
  requiresQuality?: boolean
}

const panels: ReviewDockPanelDefinition[] = []

/** Replaces the entry with the same id, otherwise adds it. Idempotent. */
export function registerReviewDockPanel(def: ReviewDockPanelDefinition): void {
  const i = panels.findIndex((p) => p.id === def.id)
  if (i >= 0) {
    panels[i] = def
  } else {
    panels.push(def)
  }
}

export function getReviewDockPanels(flags: { quality: boolean }): ReviewDockPanelDefinition[] {
  return panels.filter((p) => !p.requiresQuality || flags.quality).sort((a, b) => a.order - b.order)
}
