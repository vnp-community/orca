/**
 * Outcome Review words — FE-OR-T124
 *
 * Every label of the review that depends on a value from the server. Three levels of claim, kept apart on purpose: what Orca
 * measured (stated plainly), what an agent reported ("Agent reported"), and what could not be checked ("Unverified" / "Unknown").
 * Nothing here says "verified", "safe to release" or "requirements met"; a test keeps it that way.
 *
 * @module components/request/outcome-review/outcome-review-copy
 */

import { translate } from '@/i18n/i18n'
import type {
  Open,
  ReviewChange,
  ReviewSection,
  ReviewSeverity
} from '../../../../../shared/outcome-review-types'
import type {
  DesignRowId,
  GateId,
  ReadinessCell,
  ReadinessRowId,
  RiskId
} from './outcome-review-view-model'

const T = 'auto.components.request.outcomeReview.'

export function unknownText(): string {
  return translate(`${T}unknown`, 'Unknown')
}

export function titleText(): string {
  return translate(`${T}title`, 'Outcome review')
}

export function agentReportedText(): string {
  return translate(`${T}agentReported`, 'Agent reported')
}

export function unverifiedText(): string {
  return translate(`${T}unverified`, 'Unverified')
}

export function advisoryText(): string {
  return translate(`${T}advisory`, 'Advisory')
}

export function phaseText(phase: string): string {
  switch (phase) {
    case 'design':
      return translate(`${T}phase.design`, 'Design review')
    case 'delivered':
      return translate(`${T}phase.delivered`, 'After-code review')
    default:
      return unknownText()
  }
}

export function stateText(state: string): string {
  switch (state) {
    case 'ready':
      return translate(`${T}state.ready`, 'Ready')
    case 'stale':
      return translate(`${T}state.stale`, 'Out of date')
    case 'approved':
      return translate(`${T}state.approved`, 'Approved')
    case 'draft':
      return translate(`${T}state.draft`, 'Draft')
    case 'superseded':
      return translate(`${T}state.superseded`, 'Replaced')
    default:
      return unknownText()
  }
}

export function sectionText(section: Open<ReviewSection>): string {
  switch (section) {
    case 'architecture':
      return translate(`${T}section.architecture`, 'Architecture')
    case 'flow':
      return translate(`${T}section.flow`, 'Flow')
    case 'behavior':
      return translate(`${T}section.behavior`, 'Behavior')
    case 'data':
      return translate(`${T}section.data`, 'Data')
    case 'contracts':
      return translate(`${T}section.contracts`, 'Contracts')
    case 'requirements':
      return translate(`${T}section.requirements`, 'Requirements')
    case 'config':
      return translate(`${T}section.config`, 'Config')
    default:
      return unknownText()
  }
}

export function changeText(change: Open<ReviewChange>): string {
  switch (change) {
    case 'added':
      return translate(`${T}change.added`, 'Added')
    case 'modified':
      return translate(`${T}change.modified`, 'Changed')
    case 'removed':
      return translate(`${T}change.removed`, 'Removed')
    case 'unchanged':
      return translate(`${T}change.unchanged`, 'Unchanged')
    case 'missing':
      return translate(`${T}change.missing`, 'Missing')
    default:
      return unknownText()
  }
}

export function severityText(severity: Open<ReviewSeverity>): string {
  switch (severity) {
    case 'info':
      return translate(`${T}severity.info`, 'Info')
    case 'warning':
      return translate(`${T}severity.warning`, 'Warning')
    case 'breaking':
      return translate(`${T}severity.breaking`, 'Breaking')
    case 'deviation':
      return translate(`${T}severity.deviation`, 'Deviates from design')
    case 'regression':
      return translate(`${T}severity.regression`, 'Regression')
    default:
      return unknownText()
  }
}

export function designRowText(id: DesignRowId): string {
  switch (id) {
    case 'containers_changed':
      return translate(`${T}designRow.containers_changed`, 'Containers changed')
    case 'new_edges':
      return translate(`${T}designRow.new_edges`, 'New connections')
    case 'removed_edges':
      return translate(`${T}designRow.removed_edges`, 'Removed connections')
    case 'unplanned_edges':
      return translate(`${T}designRow.unplanned_edges`, 'Not in design')
    case 'new_calls':
      return translate(`${T}designRow.new_calls`, 'New calls')
    case 'removed_calls':
      return translate(`${T}designRow.removed_calls`, 'Removed calls')
    case 'reordered':
      return translate(`${T}designRow.reordered`, 'Reordered calls')
    case 'unplanned_calls':
      return translate(`${T}designRow.unplanned_calls`, 'Not in design')
    case 'design_items':
      return translate(`${T}designRow.design_items`, 'Reported design items')
    default:
      return translate(`${T}designRow.drawings`, 'Design diagrams')
  }
}

export function gateText(gate: GateId): string {
  switch (gate) {
    case 'pass':
      return translate(`${T}gate.pass`, 'Checks passed')
    case 'warn':
      return translate(`${T}gate.warn`, 'Checks raised warnings')
    case 'fail':
      return translate(`${T}gate.fail`, 'Checks failed')
    case 'absent':
      return translate(`${T}gate.absent`, 'No quality gate is set up')
    default:
      return translate(`${T}gate.unknown`, 'Not enough data to conclude')
  }
}

export function riskText(risk: RiskId): string {
  switch (risk) {
    case 'low':
      return translate(`${T}risk.low`, 'Low')
    case 'medium':
      return translate(`${T}risk.medium`, 'Medium')
    case 'high':
      return translate(`${T}risk.high`, 'High')
    case 'critical':
      return translate(`${T}risk.critical`, 'Critical')
    default:
      return translate(`${T}risk.unknown`, 'Risk could not be measured')
  }
}

export function readinessRowText(id: ReadinessRowId): string {
  switch (id) {
    case 'migration':
      return translate(`${T}readiness.migration`, 'Database migration')
    case 'breaking':
      return translate(`${T}readiness.breaking`, 'Breaking change')
    default:
      return translate(`${T}readiness.config`, 'Configuration change')
  }
}

export function readinessCellText(cell: ReadinessCell): string {
  switch (cell) {
    case 'present':
      return translate(`${T}readiness.present`, 'Present: look at it before release')
    case 'none_found':
      return translate(`${T}readiness.noneFound`, 'None found')
    default:
      return unknownText()
  }
}
