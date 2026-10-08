/**
 * request-flow-summary.ts — CR-REQ-019-04
 *
 * One-line description of what a request type goes through, derived from the
 * flow registry so the type picker never hard-codes per-type behaviour.
 *
 * @module components/request/request-flow-summary
 */

import { translate } from '@/i18n/i18n'
import { REQUEST_FLOW_REGISTRY } from '../../../../shared/request-flow-registry'
import type { RequestType } from '../../../../shared/request-types'

const T = 'auto.components.request.TypeConfirmationCard.flow.'

export function summarizeRequestFlow(type: RequestType): string {
  if (type === 'unknown') {return ''}
  const entry = REQUEST_FLOW_REGISTRY[type]
  const gates = Object.values(entry.gates).filter(Boolean).length
  return [
    entry.analysisKind ? translate(`${T}analysis`, 'Analysis') : translate(`${T}noAnalysis`, 'No analysis'),
    entry.plan !== 'none' ? translate(`${T}plan`, 'Plan') : translate(`${T}noPlan`, 'No plan'),
    entry.phase !== 'never' ? translate(`${T}phase`, 'Phases') : null,
    gates > 0 ? translate(`${T}gates`, '{{count}} approval gates', { count: gates }) : translate(`${T}noGates`, 'No approval gates')
  ]
    .filter(Boolean)
    .join(' · ')
}
