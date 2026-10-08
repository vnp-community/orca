/**
 * finding-view-model.ts — FE-CV-TASK-059-02
 *
 * Finding (structural source) -> display row. Titles are built from `titleKey` + `params`
 * through a closed lookup; an unknown titleKey falls back to the raw `rule`. QualityFinding
 * is a different source (PQ-06) and never passes through here.
 *
 * @module components/review-map/findings/finding-view-model
 */

import type { Finding } from '../../../../../shared/code-intel-types'
import { maskSensitiveText } from '../sensitive-text-masking'

export type FindingTranslate = (
  key: string,
  fallback: string,
  params?: Record<string, unknown>
) => string

export type FindingSeverityState = 'error' | 'warning' | 'info' | 'unknown'
export type FindingOriginState = 'introduced' | 'touched' | 'preexisting' | 'unknown'
export type FindingKindState =
  | 'layer_violation'
  | 'dependency_cycle'
  | 'hotspot'
  | 'missing_tenant_id'
  | 'dead_code'
  | 'rls_removed'
  | 'unknown'

const KINDS: readonly FindingKindState[] = [
  'layer_violation',
  'dependency_cycle',
  'hotspot',
  'missing_tenant_id',
  'dead_code',
  'rls_removed'
]

export const FINDING_BASE = 'auto.components.reviewMap.findings'

// Why closed: titleKey values are owned by the backend rules; only these are translated here.
const TITLE_FALLBACKS: Record<string, string> = {
  layer_violation: 'Layer violation: {{from}} depends on {{to}}',
  dependency_cycle: 'Dependency cycle',
  hotspot: 'Hotspot: frequently changed and complex',
  missing_tenant_id: 'Query may be missing a tenant_id filter',
  dead_code: 'Possibly unused code',
  rls_removed: 'Row-level security removed'
}

export function normalizeFindingSeverity(value: unknown): FindingSeverityState {
  return value === 'error' || value === 'warning' || value === 'info' ? value : 'unknown'
}

export function normalizeFindingOrigin(value: unknown): FindingOriginState {
  return value === 'introduced' || value === 'touched' || value === 'preexisting' ? value : 'unknown'
}

export function normalizeFindingKind(value: unknown): FindingKindState {
  return KINDS.find((k) => k === value) ?? 'unknown'
}

export type FindingRowModel = {
  findingKey: string
  rule: string
  kind: FindingKindState
  severity: FindingSeverityState
  origin: FindingOriginState
  confidence: string
  title: string
  subject: string
  /** Masked copy of `params`; safe to render. */
  params: Record<string, string>
  locationPath: string | null
  locationLine: number | null
  locationLabel: string | null
  symbolKey: string | null
  ownerLabel: string | null
  isDismissed: boolean
  disposition: 'ignored' | 'resolved' | null
  dismissReason: string | null
  dismissNote: string | null
  finding: Finding
}

function maskParams(params: Record<string, string> | undefined): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(params ?? {})) {
    out[key] = maskSensitiveText(String(value)).text
  }
  return out
}

export function buildFindingTitle(
  finding: Pick<Finding, 'titleKey' | 'rule' | 'kind' | 'params'>,
  params: Record<string, string>,
  t: FindingTranslate
): string {
  const kind = normalizeFindingKind(finding.kind)
  const expected = `finding.${kind}.title`
  if (kind === 'unknown' || finding.titleKey !== expected) {
    return finding.rule
  }
  return t(`${FINDING_BASE}.title.${kind}`, TITLE_FALLBACKS[kind], params)
}

export function toFindingRow(finding: Finding, t: FindingTranslate): FindingRowModel {
  const params = maskParams(finding.params)
  const first = finding.evidence?.[0]
  const line = first?.line ?? null
  const dismissed = finding.dismissed
  return {
    findingKey: finding.findingKey,
    rule: finding.rule,
    kind: normalizeFindingKind(finding.kind),
    severity: normalizeFindingSeverity(finding.severity),
    origin: normalizeFindingOrigin(finding.origin),
    confidence: finding.confidence,
    title: buildFindingTitle(finding, params, t),
    subject: maskSensitiveText(finding.subject ?? '').text,
    params,
    locationPath: first?.path ?? null,
    locationLine: line,
    locationLabel: first ? (line ? `${first.path}:${line}` : first.path) : null,
    symbolKey: first?.symbol?.key ?? null,
    ownerLabel: finding.owner?.names?.length ? finding.owner.names.slice(0, 2).join(', ') : null,
    isDismissed: Boolean(dismissed),
    disposition: dismissed ? dismissed.disposition : null,
    dismissReason: dismissed ? maskSensitiveText(dismissed.reason ?? '').text : null,
    dismissNote: dismissed?.note ? maskSensitiveText(dismissed.note).text : null,
    finding
  }
}
