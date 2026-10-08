/**
 * review-overlay-model.ts — FE-CV-TASK-053-01
 *
 * Single encoding table for the change overlay shared by every lens (impact, structure,
 * reading order...). Why one table: colour is never the only cue, and the legend, node
 * borders and row dots must not drift apart.
 *
 * @module components/review-map/review-overlay-model
 */

import { CircleDot, FlaskConicalOff, TriangleAlert, Waypoints } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { ChangeOverlayView } from './review-wire-types'
import type { TurnChangeLabel, TurnCompareResult } from './turns/turn-compare-model'

export type OverlayFlag = 'changed' | 'affected' | 'untested' | 'violation'

export type OverlayEncoding = {
  labelKey: string
  labelFallback: string
  /** Tooltip / legend explanation; wording must not overclaim ("not found in index"). */
  descriptionKey: string
  descriptionFallback: string
  /** CSS custom property defined in main.css (--review-*). */
  tokenVar: string
  icon: LucideIcon
  strokeWidth: number
  dashed: boolean
  /** Tailwind class carrying the token (no raw colour values). */
  textClass: string
  borderClass: string
}

const k = (flag: OverlayFlag, part: 'label' | 'description'): string =>
  `auto.components.reviewMap.overlay.${flag}.${part}`

export const OVERLAY_FLAG_ORDER: readonly OverlayFlag[] = [
  'changed',
  'affected',
  'untested',
  'violation'
]

export const OVERLAY_ENCODING: Record<OverlayFlag, OverlayEncoding> = {
  changed: {
    labelKey: k('changed', 'label'),
    labelFallback: 'Changed',
    descriptionKey: k('changed', 'description'),
    descriptionFallback: 'Changed in this review scope',
    tokenVar: '--review-changed',
    icon: CircleDot,
    strokeWidth: 2,
    dashed: false,
    textClass: 'text-[color:var(--review-changed)]',
    borderClass: 'border-[color:var(--review-changed)]'
  },
  affected: {
    labelKey: k('affected', 'label'),
    labelFallback: 'Affected',
    descriptionKey: k('affected', 'description'),
    descriptionFallback: 'Reached by a changed symbol according to the index',
    tokenVar: '--review-affected',
    icon: Waypoints,
    strokeWidth: 1,
    dashed: false,
    textClass: 'text-[color:var(--review-affected)]',
    borderClass: 'border-[color:var(--review-affected)]'
  },
  untested: {
    labelKey: k('untested', 'label'),
    labelFallback: 'No test found',
    descriptionKey: k('untested', 'description'),
    descriptionFallback: 'No covering test found in the index',
    tokenVar: '--review-untested',
    icon: FlaskConicalOff,
    strokeWidth: 1,
    dashed: true,
    textClass: 'text-[color:var(--review-untested)]',
    borderClass: 'border-[color:var(--review-untested)]'
  },
  violation: {
    labelKey: k('violation', 'label'),
    labelFallback: 'Rule finding',
    descriptionKey: k('violation', 'description'),
    descriptionFallback: 'The containing file has a rule finding',
    tokenVar: '--review-violation',
    icon: TriangleAlert,
    strokeWidth: 1,
    dashed: false,
    textClass: 'text-[color:var(--review-violation)]',
    borderClass: 'border-[color:var(--review-violation)]'
  }
}

export type OverlayFlagSet = ReadonlySet<OverlayFlag>

export type OverlayImpactInput = {
  /** Symbol keys returned by an `impact` query (center excluded by the caller if desired). */
  affectedKeys: ReadonlySet<string>
} | null

/** Flags for one symbol (by key) and/or the file that contains it. */
export function computeOverlayFlags(
  target: { symbolKey?: string; file?: string },
  overlay: Pick<
    ChangeOverlayView,
    'changedSymbols' | 'uncoveredSymbols' | 'violations' | 'changedFiles'
  > | null,
  impact?: OverlayImpactInput
): OverlayFlagSet {
  const flags = new Set<OverlayFlag>()
  if (!overlay) {
    return flags
  }
  const { symbolKey, file } = target
  const changedSymbol = symbolKey
    ? overlay.changedSymbols.find((c) => c.symbol.key === symbolKey)
    : undefined
  const changed =
    Boolean(changedSymbol) ||
    (!symbolKey && Boolean(file) && overlay.changedFiles.some((f) => f.path === file))
  if (changed) {
    flags.add('changed')
  } else if (symbolKey && impact?.affectedKeys.has(symbolKey)) {
    flags.add('affected')
  }
  // 'unknown' is "not yet known", never drawn as untested.
  const untested =
    changedSymbol?.tested === 'no' ||
    (symbolKey ? overlay.uncoveredSymbols.some((u) => u.key === symbolKey) : false)
  if (untested) {
    flags.add('untested')
  }
  if (file && overlay.violations.some((v) => v.file === file)) {
    flags.add('violation')
  }
  return flags
}

/** Strongest border source: changed > affected. `untested` only adds the dashed style. */
function primaryFlag(flags: OverlayFlagSet): OverlayFlag | null {
  if (flags.has('changed')) {
    return 'changed'
  }
  if (flags.has('affected')) {
    return 'affected'
  }
  if (flags.has('untested')) {
    return 'untested'
  }
  return flags.has('violation') ? 'violation' : null
}

export function overlayClassNames(flags: OverlayFlagSet): string {
  const primary = primaryFlag(flags)
  if (!primary) {
    return ''
  }
  const enc = OVERLAY_ENCODING[primary]
  const classes = [enc.borderClass, enc.strokeWidth >= 2 ? 'border-2' : 'border']
  if (flags.has('untested')) {
    classes.push('border-dashed')
  }
  return classes.join(' ')
}

export type OverlaySvgProps = {
  stroke: string
  strokeWidth: number
  strokeDasharray?: string
}

export function overlaySvgProps(flags: OverlayFlagSet): OverlaySvgProps | null {
  const primary = primaryFlag(flags)
  if (!primary) {
    return null
  }
  const enc = OVERLAY_ENCODING[primary]
  return {
    stroke: `var(${enc.tokenVar})`,
    strokeWidth: enc.strokeWidth,
    ...(flags.has('untested') ? { strokeDasharray: '4 3' } : {})
  }
}

/** Icon flags in display order (changed marker first). */
export function overlayIconFlags(flags: OverlayFlagSet): OverlayFlag[] {
  return OVERLAY_FLAG_ORDER.filter((f) => flags.has(f))
}

/** Which flags have any data at all, to build the legend from the same table. */
export function overlayFlagsWithData(
  overlay: Pick<
    ChangeOverlayView,
    'changedSymbols' | 'uncoveredSymbols' | 'violations' | 'changedFiles'
  > | null,
  opts: { hasImpact: boolean }
): OverlayFlag[] {
  if (!overlay) {
    return []
  }
  const has: Record<OverlayFlag, boolean> = {
    changed: overlay.changedSymbols.length > 0 || overlay.changedFiles.length > 0,
    affected: opts.hasImpact,
    untested:
      overlay.uncoveredSymbols.length > 0 || overlay.changedSymbols.some((c) => c.tested === 'no'),
    violation: overlay.violations.length > 0
  }
  return OVERLAY_FLAG_ORDER.filter((f) => has[f])
}

/**
 * Flags worth a mark on a reading-order row. `changed` is omitted on purpose: every step is a
 * changed file, so the mark would carry no information there.
 */
export function computeReadingStepOverlayFlags(
  step: { file: string; symbols: readonly { key: string }[] },
  overlay: Pick<
    ChangeOverlayView,
    'changedSymbols' | 'uncoveredSymbols' | 'violations' | 'changedFiles'
  > | null
): OverlayFlag[] {
  const out = new Set<OverlayFlag>()
  if (!overlay) {
    return []
  }
  for (const s of step.symbols) {
    if (computeOverlayFlags({ symbolKey: s.key }, overlay).has('untested')) {
      out.add('untested')
    }
  }
  if (computeOverlayFlags({ file: step.file }, overlay).has('violation')) {
    out.add('violation')
  }
  return overlayIconFlags(out)
}

// ---- Turn comparison layer (FE-CV-TASK-060-08) ----

/** Per-node label of the "since previous turn" view; symbol labels win over file labels. */
export function turnOverlayLabel(
  target: { symbolKey?: string; file?: string },
  turn: Pick<TurnCompareResult, 'files' | 'symbols'> | null
): TurnChangeLabel | null {
  if (!turn) {
    return null
  }
  const bySymbol = target.symbolKey ? turn.symbols?.[target.symbolKey] : undefined
  if (bySymbol) {
    return bySymbol
  }
  return (target.file ? turn.files[target.file] : undefined) ?? null
}

/** Nodes the last turn did not touch are dimmed, not hidden, so the graph keeps its shape. */
export function turnOverlayDimmed(label: TurnChangeLabel | null): boolean {
  return label === 'unchanged_since'
}
