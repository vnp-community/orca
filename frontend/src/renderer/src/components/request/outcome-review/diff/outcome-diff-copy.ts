/**
 * Words of the system diff — FE-OR-T201..T206
 *
 * @module components/request/outcome-review/diff/outcome-diff-copy
 */

import { translate } from '@/i18n/i18n'
import type { ReviewEvidence } from '../../../../../../shared/outcome-review-types'
import type { DiffMode, ModeReason, RowFlag, Tri } from './outcome-diff-view-model'

const T = 'auto.components.request.outcomeReview.diff.'

export function flagText(flag: RowFlag): string {
  switch (flag) {
    case 'deviates':
      return translate(`${T}flag.deviates`, 'Deviates from design')
    case 'missing_from_code':
      return translate(`${T}flag.missingFromCode`, 'Missing from code')
    case 'not_in_design':
      return translate(`${T}flag.notInDesign`, 'Not in design')
    case 'sad_text_only':
      return translate(`${T}flag.sadTextOnly`, 'In SAD text only')
    case 'advisory':
      return translate(`${T}flag.advisory`, 'Advisory')
    case 'breaking':
      return translate(`${T}flag.breaking`, 'Breaking')
    case 'regression':
      return translate(`${T}flag.regression`, 'Regression')
    default:
      return translate(`${T}flag.unverified`, 'Unverified')
  }
}

export function modeText(mode: DiffMode): string {
  switch (mode) {
    case 'design':
      return translate(`${T}mode.design`, 'Design')
    case 'after_code':
      return translate(`${T}mode.afterCode`, 'After code')
    default:
      return translate(`${T}mode.sideBySide`, 'Side by side')
  }
}

export function modeReasonText(reason: ModeReason): string {
  switch (reason) {
    case 'execution_not_finished':
      return translate(`${T}modeReason.executionNotFinished`, 'Available when execution completes')
    case 'generate_first':
      return translate(`${T}modeReason.generateFirst`, 'Generate the after-code review first')
    default:
      return translate(`${T}modeReason.designUnavailable`, 'Design not available')
  }
}

export function dotText(where: 'design' | 'base' | 'head', value: Tri): string {
  const place =
    where === 'design'
      ? translate(`${T}dot.design`, 'In the design')
      : where === 'base'
        ? translate(`${T}dot.base`, 'On the base')
        : translate(`${T}dot.head`, 'On the head')
  const answer =
    value === 'yes'
      ? translate(`${T}dot.yes`, 'yes')
      : value === 'no'
        ? translate(`${T}dot.no`, 'no')
        : translate(`${T}dot.unknown`, 'unknown')
  return `${place}: ${answer}`
}

export function dotLetter(where: 'design' | 'base' | 'head'): string {
  return where === 'design' ? 'D' : where === 'base' ? 'B' : 'H'
}

export function tabBadgeText(kind: 'unplanned' | 'unseen', count: number): string {
  return kind === 'unplanned'
    ? translate(`${T}tab.unplanned`, '{{count}} not in design', { count })
    : translate(`${T}tab.unseen`, '{{count}} new', { count })
}

export function emptyText(): string {
  return translate(`${T}empty`, 'Nothing here.')
}

export function emptyHiddenText(count: number): string {
  return translate(`${T}emptyHidden`, 'Nothing changed here ({{count}} unchanged hidden).', {
    count
  })
}

export function laterText(): string {
  return translate(`${T}later`, 'This appears after execution completes.')
}

export function showUnchangedText(): string {
  return translate(`${T}showUnchanged`, 'Show unchanged')
}

export function seenText(): string {
  return translate(`${T}seen`, 'Seen')
}

export function evidenceText(e: ReviewEvidence): string {
  switch (e.kind) {
    case 'kg':
      return e.snapshot
        ? translate(`${T}evidence.kgSnapshot`, '{{id}} · {{snapshot}}', {
            id: e.ref,
            snapshot: e.snapshot
          })
        : e.ref
    case 'source': {
      const where = e.line
        ? `${e.doc || e.ref}:${e.line}`
        : e.section
          ? `${e.doc || e.ref} § ${e.section}`
          : e.doc || e.ref
      return where
    }
    case 'file':
      return e.ref
    case 'rpc':
    case 'channel':
    case 'migration':
    case 'check':
    case 'task':
      return `${e.kind}: ${e.ref}`
    default:
      return `${translate(`${T}evidence.unknown`, 'Unknown')}: ${e.ref}`
  }
}

export function copyText(id: string): string {
  return translate(`${T}copy`, 'Copy {{id}}', { id })
}

export function copiedText(): string {
  return translate(`${T}copied`, 'Copied')
}

export function openFileText(path: string): string {
  return translate(`${T}openFile`, 'Open the diff of {{path}}', { path })
}

export function requirementColumnText(
  column: 'requirement' | 'before' | 'after' | 'evidence'
): string {
  switch (column) {
    case 'requirement':
      return translate(`${T}req.requirement`, 'Requirement')
    case 'before':
      return translate(`${T}req.before`, 'Before')
    case 'after':
      return translate(`${T}req.after`, 'After')
    default:
      return translate(`${T}req.evidence`, 'Evidence')
  }
}

export function requirementStateText(state: string): string {
  switch (state) {
    case 'planned':
      return translate(`${T}req.planned`, 'Planned')
    case 'delivered':
      return translate(`${T}req.delivered`, 'Delivered')
    case 'partial':
      return translate(`${T}req.partial`, 'Partial')
    case 'notCovered':
      return translate(`${T}req.notCovered`, 'Not covered')
    case 'unknown':
      return translate(`${T}req.unknown`, 'Unknown')
    default:
      return translate(`${T}req.other`, 'Unknown')
  }
}

export function containersText(): { title: string } {
  return { title: translate(`${T}containers.title`, 'Inside containers') }
}

export function containerItemsText(count: number): string {
  return translate(`${T}containers.items`, '{{count}} item(s)', { count })
}

export function diagramLabelText(section: string, variant: string, name?: string): string {
  const where =
    variant === 'design'
      ? translate(`${T}diagram.design`, 'design')
      : translate(`${T}diagram.afterCode`, 'after code')
  return name ? `${section} · ${name} · ${where}` : `${section} · ${where}`
}

export function sideText(side: 'design' | 'after_code'): string {
  return side === 'design'
    ? translate(`${T}side.design`, 'Design')
    : translate(`${T}side.afterCode`, 'After code')
}
