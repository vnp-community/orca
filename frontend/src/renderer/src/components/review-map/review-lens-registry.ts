/**
 * review-lens-registry.ts — FE-CV-TASK-051-05
 *
 * Lens plug-in point of the Review workspace. A lens agent adds ONE entry here (or calls
 * registerReviewLens from its own module) and never edits ReviewLensTabs/ReviewWorkspace.
 * Canonical lenses without a `load` render a localized "not available yet" placeholder, so
 * the tab order is stable while lenses land one by one.
 *
 * HOW TO REGISTER A LENS
 *   1. Create the lens component taking ReviewLensProps (default export for React.lazy).
 *   2. In REVIEW_LENS_DEFINITIONS set `load: () => import('./<lens>/<LensPanel>')` on the
 *      entry with your id (impact | architecture | dataflow | erd | storage | structure |
 *      contract), or call registerReviewLens({...}) for a new id (requirements, quality...).
 *   3. Put label strings under auto.components.reviewMap.lens.<id>.* in the locale files.
 */

import type { ComponentType, ReactNode } from 'react'
import type { ReviewScope } from './review-scope-model'
import type { ReviewChipId } from './review-chip-filter'
import type { ChangeOverlayView, IndexStatusView } from './review-wire-types'
import type { AmbiguousSymbolCandidate, AmbiguousSymbolChoice } from './shell/AmbiguousSymbolDialog'
import { requirementsLensDefinition } from './requirements/requirements-lens-registration'
import { qualityLensDefinition } from './quality/quality-lens-registration'

export type ReviewLensProps = {
  worktreeId: string
  environmentId: string | null
  scope: ReviewScope
  overlay: ChangeOverlayView
  selectedSymbolKey: string | null
  chipFilter: ReviewChipId | null
  onSelectSymbol: (symbolKey: string | null) => void
  /** Opens the diff of `path`, optionally at a 1-based line. */
  onOpenDiff: (path: string, line?: number) => void
  /** Opens the ambiguous-symbol dialog; resolves with the user's pick or null when cancelled. */
  requestSymbolChoice: (
    candidates: readonly AmbiguousSymbolCandidate[]
  ) => Promise<AmbiguousSymbolChoice | null>
}

export type ReviewLensComponent = ComponentType<ReviewLensProps>

export type ReviewLensDefinition = {
  id: string
  /** Ascending tab order. Canonical lenses use 10, 20, ... so new ones can slot between. */
  order: number
  labelKey: string
  labelFallback: string
  /** Absent => placeholder body. Loaded lazily (React.lazy + Suspense + Skeleton). */
  load?: () => Promise<{ default: ReviewLensComponent }>
  /** Hidden unless the quality-gate flag is on. */
  requiresQuality?: boolean
}

const label = (id: string): string => `auto.components.reviewMap.lens.${id}.label`

/** Canonical order: Impact, Architecture, Flow, ERD, Storage, Structure, Contract. */
export const REVIEW_LENS_DEFINITIONS: ReviewLensDefinition[] = [
  {
    id: 'impact',
    order: 10,
    labelKey: label('impact'),
    labelFallback: 'Impact',
    load: () => import('./impact/ImpactLens')
  },
  {
    id: 'architecture',
    order: 20,
    labelKey: label('architecture'),
    labelFallback: 'Architecture',
    load: () => import('./c4/ArchitectureLens')
  },
  {
    id: 'dataflow',
    order: 30,
    labelKey: label('dataflow'),
    labelFallback: 'Flows',
    load: () => import('./dataflow/DataFlowLens')
  },
  {
    id: 'erd',
    order: 40,
    labelKey: label('erd'),
    labelFallback: 'ERD',
    load: () => import('./erd/ErdLens')
  },
  {
    id: 'storage',
    order: 50,
    labelKey: label('storage'),
    labelFallback: 'Storage',
    load: () => import('./storage/StorageLens')
  },
  {
    id: 'structure',
    order: 60,
    labelKey: label('structure'),
    labelFallback: 'Structure',
    load: () => import('./structure/StructureLens')
  },
  {
    id: 'contract',
    order: 70,
    labelKey: label('contract'),
    labelFallback: 'Contracts',
    load: () => import('./contract/ContractDiffLens')
  },
  // Quality-gated: hidden (and never loaded) while the quality flag is off.
  qualityLensDefinition,
  requirementsLensDefinition
]

/** Replaces the entry with the same id, otherwise adds it. Idempotent. */
export function registerReviewLens(def: ReviewLensDefinition): void {
  const i = REVIEW_LENS_DEFINITIONS.findIndex((d) => d.id === def.id)
  if (i >= 0) {
    REVIEW_LENS_DEFINITIONS[i] = def
  } else {
    REVIEW_LENS_DEFINITIONS.push(def)
  }
}

export function getReviewLenses(flags: { quality: boolean }): ReviewLensDefinition[] {
  return REVIEW_LENS_DEFINITIONS.filter((d) => !d.requiresQuality || flags.quality).sort(
    (a, b) => a.order - b.order
  )
}

/** Chip -> lens navigation only lands on a lens the user can actually see. */
export function resolveActiveLensId(
  wanted: string | null,
  lenses: readonly ReviewLensDefinition[]
): string | null {
  if (wanted && lenses.some((l) => l.id === wanted)) {
    return wanted
  }
  return lenses[0]?.id ?? null
}

// ---------------------------------------------------------------------------
// Drawer content (selected-symbol detail). SOL-053 registers SymbolDetailPanel here.
// ---------------------------------------------------------------------------

export type ReviewDrawerContentProps = {
  worktreeId: string
  environmentId: string | null
  scope: ReviewScope
  selectedSymbolKey: string
  /** Index status of the worktree; lets the panel note when line numbers follow another commit. */
  indexStatus?: IndexStatusView | null
  onClose: () => void
  onOpenDiff: (path: string, line?: number) => void
}

let drawerRenderer: ((props: ReviewDrawerContentProps) => ReactNode) | null = null

export function setReviewDrawerRenderer(
  renderer: ((props: ReviewDrawerContentProps) => ReactNode) | null
): void {
  drawerRenderer = renderer
}

export function getReviewDrawerRenderer(): ((props: ReviewDrawerContentProps) => ReactNode) | null {
  return drawerRenderer
}
