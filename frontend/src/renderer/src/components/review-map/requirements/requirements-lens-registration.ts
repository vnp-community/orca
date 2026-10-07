/**
 * requirements-lens-registration.ts — FE-CV-TASK-092-06
 *
 * Registration entry for the 'requirements' lens in the Review workspace.
 * Follows the lens registry pattern from FE-CV-SOL-051.
 *
 * The lens is only shown when flags.quality is true.
 * Panel component is lazy-loaded to avoid bundling when unused.
 *
 * @module components/review-map/requirements/requirements-lens-registration
 */

import type React from 'react'

// ---------------------------------------------------------------------------
// Types (aligned with SOL-051 lens registry contract)
// ---------------------------------------------------------------------------

export type LensId = string

export type LensRegistrationEntry = {
  id: LensId
  /** i18n key for the tab label */
  labelKey: string
  /** i18n key for the tooltip */
  tooltipKey: string
  /** Lazy factory for the panel component */
  loadPanel: () => Promise<{ default: React.ComponentType<LensPanelProps> }>
  /** If true, the lens is only shown when the quality flag is enabled */
  requiresQuality: boolean
  /** If true, tab is hidden (but registered) so lens ID doesn't become a dead URL */
  hidden?: boolean
}

export type LensPanelProps = {
  worktreeId: string
  environmentId: string | null
  projectId?: string | null
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

export const REQUIREMENTS_LENS_ID: LensId = 'requirements'

export const requirementsLensRegistration: LensRegistrationEntry = {
  id: REQUIREMENTS_LENS_ID,
  labelKey: 'auto.components.reviewMap.requirements.lens.label',
  tooltipKey: 'auto.components.reviewMap.requirements.lens.tooltip',

  // Lazy load to avoid bundling when quality is disabled
  loadPanel: () =>
    import('./RequirementTracePanel').then((m) => ({ default: m.RequirementTracePanel })),

  requiresQuality: true,
}
