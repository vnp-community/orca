/**
 * Outcome Review diff view model — FE-OR-T201..T206
 *
 * Pure functions from a review to the sub-tabs, the Design / After code / Side by side switch, and the rows of a section. No
 * React and no text. A flag is raised only by what the data says: "Not in design" only for something found in code alone, "In SAD
 * text only" only for something read from the SAD prose, "Advisory" only where the design element was not approved.
 *
 * @module components/request/outcome-review/diff/outcome-diff-view-model
 */

import { REVIEW_SECTIONS, isKnown } from '../../../../../../shared/outcome-review-enums'
import type {
  DiagramRef,
  OutcomeReview,
  ReviewItem,
  ReviewSection
} from '../../../../../../shared/outcome-review-types'
import type { RequestStatus } from '../../../../../../shared/request-types'
import { executionFinished, phaseOf } from '../outcome-review-view-model'

export const SECTION_ORDER: readonly ReviewSection[] = [
  'architecture',
  'flow',
  'behavior',
  'data',
  'contracts',
  'requirements',
  'config'
]

/** Sections that have a diagram; the others are tables. */
export const DRAWN_SECTIONS: ReadonlySet<string> = new Set([
  'architecture',
  'flow',
  'behavior',
  'data'
])

export function isDrawnSection(section: string): boolean {
  return DRAWN_SECTIONS.has(section)
}

export type DiffMode = 'design' | 'after_code' | 'side_by_side'
export type ModeReason = 'execution_not_finished' | 'generate_first' | 'design_unavailable'
export type ModeState = { enabled: boolean; reason?: ModeReason }
export type ModeAvailability = Record<DiffMode, ModeState> & { initial: DiffMode }

/**
 * Before the code exists only Design is offered. After Execution the after-code review has to be generated first. Without the
 * knowledge graph there is no design to show or to put beside the code.
 */
export function modeAvailability(
  review: OutcomeReview,
  requestStatus: RequestStatus
): ModeAvailability {
  const delivered = phaseOf(review) === 'delivered'
  const designReady = review.model.kg.available
  const afterReason: ModeReason | undefined = delivered
    ? undefined
    : executionFinished(requestStatus)
      ? 'generate_first'
      : 'execution_not_finished'
  return {
    design: designReady ? { enabled: true } : { enabled: false, reason: 'design_unavailable' },
    after_code: afterReason ? { enabled: false, reason: afterReason } : { enabled: true },
    side_by_side: afterReason
      ? { enabled: false, reason: afterReason }
      : designReady
        ? { enabled: true }
        : { enabled: false, reason: 'design_unavailable' },
    initial: delivered ? 'after_code' : 'design'
  }
}

// ---------------------------------------------------------------------------
// Rows
// ---------------------------------------------------------------------------

export type RowSymbol = '+' | '~' | '−' | '∅' | '·'
export type Tri = 'yes' | 'no' | 'unknown'
export type RowFlag =
  | 'deviates'
  | 'missing_from_code'
  | 'not_in_design'
  | 'sad_text_only'
  | 'advisory'
  | 'breaking'
  | 'regression'
  | 'unverified'

export type DiffRowModel = {
  item: ReviewItem
  symbol: RowSymbol
  flags: RowFlag[]
  /** Whether the thing is in the design, on the base and on the head; unknown when the review cannot tell. */
  dots: { design: Tri; base: Tri; head: Tri }
  /** The agent said it (versus Orca measured it). */
  agentReported: boolean
}

export function symbolOf(change: string): RowSymbol {
  switch (change) {
    case 'added':
      return '+'
    case 'modified':
      return '~'
    case 'removed':
      return '−'
    case 'missing':
      return '∅'
    default:
      return '·'
  }
}

function flagsOf(item: ReviewItem): RowFlag[] {
  const flags: RowFlag[] = []
  if (item.change === 'missing') {
    flags.push('missing_from_code')
  } else if (item.severity === 'deviation') {
    flags.push('deviates')
  }
  if (item.source === 'code') {
    flags.push('not_in_design')
  }
  if (item.source === 'sad-text') {
    flags.push('sad_text_only')
  }
  if (item.severity === 'breaking') {
    flags.push('breaking')
  }
  if (item.severity === 'regression') {
    flags.push('regression')
  }
  if (item.advisory) {
    flags.push('advisory')
  }
  if (item.unverified) {
    flags.push('unverified')
  }
  return flags
}

function triOf(value: boolean | undefined): Tri {
  return value === undefined ? 'unknown' : value ? 'yes' : 'no'
}

/** D, B, H: derived from what the item says it did to the base; a change the model does not know is unknown on both sides. */
export function dotsOf(item: ReviewItem): DiffRowModel['dots'] {
  const design = triOf(item.expected)
  switch (item.change) {
    case 'added':
      return { design, base: 'no', head: 'yes' }
    case 'removed':
      return { design, base: 'yes', head: 'no' }
    case 'modified':
    case 'unchanged':
      return { design, base: 'yes', head: 'yes' }
    case 'missing':
      return { design, base: 'no', head: 'no' }
    default:
      return { design, base: 'unknown', head: 'unknown' }
  }
}

export function rowOf(item: ReviewItem): DiffRowModel {
  return {
    item,
    symbol: symbolOf(item.change),
    flags: flagsOf(item),
    dots: dotsOf(item),
    agentReported: item.origin === 'agent'
  }
}

export function sectionItems(review: OutcomeReview, section: ReviewSection): ReviewItem[] {
  return review.model.items.filter((i) => i.section === section)
}

/**
 * The rows a section shows. Design mode lists the design itself (what the design says exists); After code lists what the code
 * changed, and hides the unchanged unless asked (they are most of a review and none of its news). Requirements always list every
 * criterion: their "unchanged" change is not a change at all.
 */
export function rowsOf(
  review: OutcomeReview,
  section: ReviewSection,
  mode: DiffMode,
  showUnchanged: boolean
): DiffRowModel[] {
  const items = sectionItems(review, section)
  if (section === 'requirements') {
    return items.map(rowOf)
  }
  if (mode === 'design') {
    return items.filter((i) => i.change === 'unchanged').map(rowOf)
  }
  return items.filter((i) => showUnchanged || i.change !== 'unchanged').map(rowOf)
}

export function diagramsOf(
  review: OutcomeReview,
  section: ReviewSection,
  variant: 'design' | 'delta'
): DiagramRef[] {
  return review.model.diagrams.filter((d) => d.section === section && d.variant === variant)
}

// ---------------------------------------------------------------------------
// Tabs
// ---------------------------------------------------------------------------

export type SubTabModel = {
  section: ReviewSection
  /** Items the viewer has not marked seen; Architecture and Flow show `unplanned` instead. */
  unseen: number
  /** Architecture and Flow only: how many things are not in the design; null when there is no design to compare with. */
  unplanned: number | null
  /** Contracts and Config exist only once the code does. */
  afterCodeOnly: boolean
}

export function subTabsOf(review: OutcomeReview, seen: ReadonlySet<string>): SubTabModel[] {
  const { architecture, flow } = review.model.verdict
  return SECTION_ORDER.map((section) => {
    const items = sectionItems(review, section).filter(
      (i) => section === 'requirements' || i.change !== 'unchanged'
    )
    const unplanned =
      section === 'architecture'
        ? architecture.designChecked
          ? architecture.unplannedEdges
          : null
        : section === 'flow'
          ? flow.designChecked
            ? flow.unplannedCalls
            : null
          : undefined
    return {
      section,
      unseen: items.filter((i) => !seen.has(i.key)).length,
      unplanned: unplanned === undefined ? null : unplanned,
      afterCodeOnly: section === 'contracts' || section === 'config'
    }
  })
}

export function showsUnplanned(section: ReviewSection): boolean {
  return section === 'architecture' || section === 'flow'
}

export function sectionOrDefault(value: string): ReviewSection {
  return isKnown<ReviewSection>(REVIEW_SECTIONS, value) ? value : 'architecture'
}

// ---------------------------------------------------------------------------
// Containers (the list outside the drawing)
// ---------------------------------------------------------------------------

/** The containers the architecture items mention, with how many items name each: the drill-down that cannot live inside a frame. */
export function containersOf(review: OutcomeReview): { id: string; items: number }[] {
  const counts = new Map<string, number>()
  for (const item of sectionItems(review, 'architecture')) {
    if (item.change === 'unchanged') {
      continue
    }
    for (const id of item.containers) {
      counts.set(id, (counts.get(id) ?? 0) + 1)
    }
  }
  return [...counts.entries()]
    .map(([id, items]) => ({ id, items }))
    .sort((a, b) => b.items - a.items || a.id.localeCompare(b.id))
}
