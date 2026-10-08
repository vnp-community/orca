/**
 * requirements-lens-registration.ts — FE-CV-TASK-092-06
 *
 * Definition of the `requirements` lens, listed in REVIEW_LENS_DEFINITIONS. The lens is
 * quality-gated (hidden, and never loaded, while the flag is off) and lazy-loaded.
 *
 * Kept free of runtime imports from the registry (types only) so the registry can import it.
 *
 * @module components/review-map/requirements/requirements-lens-registration
 */

import type { ReviewLensDefinition } from '../review-lens-registry'

export const REQUIREMENTS_LENS_ID = 'requirements'

// Why: order 90 puts it after the canonical lenses (10..70) and the quality lens (80) without reshuffling them.
export const requirementsLensDefinition: ReviewLensDefinition = {
  id: REQUIREMENTS_LENS_ID,
  order: 90,
  labelKey: 'auto.components.reviewMap.lens.requirements.label',
  labelFallback: 'Requirements',
  requiresQuality: true,
  load: () => import('./RequirementTracePanel')
}
