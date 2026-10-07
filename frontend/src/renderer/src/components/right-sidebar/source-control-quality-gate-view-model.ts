/**
 * source-control-quality-gate-view-model.ts — FE-CV-TASK-085-02
 *
 * Pure view model for the source-control quality gate notice block.
 * No React, no store imports.
 *
 * Rules:
 * - pass → hidden (no notice shown, CR-085 decision 8)
 * - warn → show with severity 'warn'
 * - fail → show with severity 'fail'
 * - unknown, timeout, offline → show as 'unknown' with unavailable=true or stale=true
 * - Reasons truncated to first 3; reasonCount holds full count
 * - Reason codes mapped to i18n keys; unknown codes → .reason.unknown with check preserved
 *
 * @module components/right-sidebar/source-control-quality-gate-view-model
 */

// ---------------------------------------------------------------------------
// Input types (§4.7 — subset used here)
// ---------------------------------------------------------------------------

export type QualityGateResult = 'pass' | 'warn' | 'fail' | 'unknown' | 'timeout' | 'offline'

export type QualityGateReason = {
  check: string
  observed: unknown
  threshold: unknown
  result: QualityGateResult
  /** Semantic code for i18n mapping */
  code?: string
  params?: Record<string, unknown>
}

export type QualityGate = {
  result: QualityGateResult
  reasons: QualityGateReason[]
  mode?: 'block' | 'warn' | string
  stale?: boolean
  unavailable?: boolean
}

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

const I18N_BASE = 'auto.components.right.sidebar.qualityGateNotice'

export type QualityGateReasonViewModel = {
  /** Full i18n key */
  labelKey: string
  /** Raw check name (for fallback / custom code display) */
  check: string
  params: Record<string, unknown>
}

export type QualityGateNoticeViewModel =
  | { visible: false }
  | {
      visible: true
      severity: 'warn' | 'fail' | 'unknown'
      stale: boolean
      unavailable: boolean
      reasons: QualityGateReasonViewModel[]
      /** Total reason count before truncation */
      reasonCount: number
    }

// ---------------------------------------------------------------------------
// Known reason codes → i18n key suffix
// ---------------------------------------------------------------------------

const KNOWN_REASON_CODES: Record<string, string> = {
  coverage_below_threshold: 'reason.coverage_below_threshold',
  complexity_exceeded: 'reason.complexity_exceeded',
  duplication_exceeded: 'reason.duplication_exceeded',
  security_violations: 'reason.security_violations',
  reliability_violations: 'reason.reliability_violations',
  maintainability_violations: 'reason.maintainability_violations',
  tech_debt_exceeded: 'reason.tech_debt_exceeded',
  hotspot_count_exceeded: 'reason.hotspot_count_exceeded',
  new_violations_found: 'reason.new_violations_found',
  dependency_vulnerabilities: 'reason.dependency_vulnerabilities',
}

function mapReasonCode(code: string | undefined, check: string): string {
  if (code && KNOWN_REASON_CODES[code]) {
    return `${I18N_BASE}.${KNOWN_REASON_CODES[code]}`
  }
  // Unknown code: fall back to generic key; `check` is passed as params
  return `${I18N_BASE}.reason.unknown`
}

const MAX_DISPLAYED_REASONS = 3

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

/**
 * Build the quality gate notice view model from raw gate data.
 * `mode` does not affect the output (CR-085 decision: only result matters).
 */
export function buildQualityNoticeViewModel(gate: QualityGate | null | undefined): QualityGateNoticeViewModel {
  if (!gate) {
    return {
      visible: true,
      severity: 'unknown',
      stale: false,
      unavailable: true,
      reasons: [],
      reasonCount: 0
    }
  }

  const { result, reasons = [], stale = false, unavailable = false } = gate

  // pass → hidden
  if (result === 'pass') {
    return { visible: false }
  }

  let severity: 'warn' | 'fail' | 'unknown'
  let isUnavailable = unavailable

  if (result === 'fail') {
    severity = 'fail'
  } else if (result === 'warn') {
    severity = 'warn'
  } else {
    // unknown, timeout, offline → show as unknown
    severity = 'unknown'
    isUnavailable = true
  }

  // Sort: fail reasons first, then warn, then unknown
  const sortedReasons = [...reasons].sort((a, b) => {
    const order: Record<string, number> = { fail: 0, unknown: 1, warn: 2, pass: 3, timeout: 4, offline: 5 }
    return (order[a.result] ?? 99) - (order[b.result] ?? 99)
  })

  const truncated = sortedReasons.slice(0, MAX_DISPLAYED_REASONS)

  const reasonViewModels: QualityGateReasonViewModel[] = truncated.map((r) => ({
    labelKey: mapReasonCode(r.code, r.check),
    check: r.check,
    params: r.params ?? {}
  }))

  return {
    visible: true,
    severity,
    stale,
    unavailable: isUnavailable,
    reasons: reasonViewModels,
    reasonCount: reasons.length
  }
}
