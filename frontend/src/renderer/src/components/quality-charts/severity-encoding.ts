import {
  CircleCheck,
  CircleHelp,
  Info,
  OctagonX,
  TriangleAlert,
  type LucideIcon
} from 'lucide-react'
import { chartCopy, type QualityChartCopyKey } from './quality-chart-copy'

export type QualitySeverityLevel = 'error' | 'warning' | 'info' | 'unknown'
export type QualityVerdictLevel = 'pass' | 'warn' | 'fail' | 'unknown'
export type EncodingShape =
  | 'octagon'
  | 'triangle'
  | 'circle-open'
  | 'circle-check'
  | 'circle-dashed'

export type EncodingEntry = {
  token: string
  textClass: string
  icon: LucideIcon
  shape: EncodingShape
  strokeDash: string | null
  labelKey: QualityChartCopyKey
}

// Why: the only place that maps level -> colour + shape + dash, so colour is never the sole cue.
export const SEVERITY_ENCODING: Record<QualitySeverityLevel, EncodingEntry> = {
  error: {
    token: '--quality-error',
    textClass: 'text-quality-error',
    icon: OctagonX,
    shape: 'octagon',
    strokeDash: null,
    labelKey: 'severity.error'
  },
  warning: {
    token: '--quality-warning',
    textClass: 'text-quality-warning',
    icon: TriangleAlert,
    shape: 'triangle',
    strokeDash: '4 2',
    labelKey: 'severity.warning'
  },
  info: {
    token: '--quality-info',
    textClass: 'text-quality-info',
    icon: Info,
    shape: 'circle-open',
    strokeDash: '1 2',
    labelKey: 'severity.info'
  },
  unknown: {
    token: '--quality-unknown',
    textClass: 'text-quality-unknown',
    icon: CircleHelp,
    shape: 'circle-dashed',
    strokeDash: '2 3',
    labelKey: 'severity.unknown'
  }
}

export const VERDICT_ENCODING: Record<QualityVerdictLevel, EncodingEntry> = {
  pass: {
    token: '--quality-pass',
    textClass: 'text-quality-pass',
    icon: CircleCheck,
    shape: 'circle-check',
    strokeDash: null,
    labelKey: 'verdict.pass'
  },
  warn: {
    token: '--quality-warning',
    textClass: 'text-quality-warning',
    icon: TriangleAlert,
    shape: 'triangle',
    strokeDash: '4 2',
    labelKey: 'verdict.warn'
  },
  fail: {
    token: '--quality-error',
    textClass: 'text-quality-error',
    icon: OctagonX,
    shape: 'octagon',
    strokeDash: null,
    labelKey: 'verdict.fail'
  },
  unknown: {
    token: '--quality-unknown',
    textClass: 'text-quality-unknown',
    icon: CircleHelp,
    shape: 'circle-dashed',
    strokeDash: '2 3',
    labelKey: 'verdict.unknown'
  }
}

export function toSeverityLevel(raw: unknown): QualitySeverityLevel {
  return raw === 'error' || raw === 'warning' || raw === 'info' ? raw : 'unknown'
}

export function toVerdictLevel(raw: unknown): QualityVerdictLevel {
  return raw === 'pass' || raw === 'warn' || raw === 'fail' ? raw : 'unknown'
}

// Why: translate at call time (not module scope) so the active locale is honoured.
export function labelOf(entry: EncodingEntry): string {
  return chartCopy(entry.labelKey)
}
