/**
 * review-report-model-parser.ts — FE-CV-TASK-090-01
 *
 * Parses `quality.report` results into `ReviewReportModel` (CONTRACT ui-api 4.7).
 * Never throws: missing fields become empty defaults plus a warning, unknown enums
 * become 'unknown' / 'UNKNOWN' (U4), so the exporter can always render something.
 *
 * @module components/review-map/report/review-report-model-parser
 */

export type ReportGateVerdict = 'pass' | 'warn' | 'fail' | 'unknown'
export type ReportRiskLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL' | 'UNKNOWN'
export type ReportDiagramKind = 'components' | 'erd' | 'flow'

export type ReportGateReason = {
  check: string
  result: ReportGateVerdict
  code: string | null
  params: Record<string, string>
}

export type ReportFinding = {
  ruleId: string
  severity: string
  category: string
  file: string
  line: number
  message: string
  origin: 'quality' | 'structure' | 'unknown'
}

export type ReportChangeRow = { name: string; change: string; breaking: boolean }
export type ReportTableRow = { table: string; service: string; op: string; breaking: boolean }
export type ReportReadingStep = { n: number; file: string; reason: string; symbols: string[] }
export type ReportDiagram = { kind: ReportDiagramKind | 'unknown'; mermaid: string; alt: string[]; truncated: boolean }

export type ReviewReportModel = {
  schemaVersion: number
  subject: {
    branch: string
    baseRef: string
    headCommit: string
    baseCommit: string
    indexCommit: string
    indexStale: boolean
    provider: string
    turnKey: string | null
  }
  reproducibility: { profileRef: string; toolVersions: Record<string, string>; runIds: string[]; modelDigest: string }
  summary: { files: number; added: number; removed: number; symbols: number; flows: number; components: string[] }
  risk: { level: ReportRiskLevel; reasons: { code: string; params: Record<string, string> }[] }
  gate: {
    verdict: ReportGateVerdict
    reasons: ReportGateReason[]
    waivers: { count: number; earliestExpiry: string | null }
  }
  findings: {
    counts: { error: number; warning: number; info: number; byCategory: Record<string, number> }
    top: ReportFinding[]
  }
  contracts: { protoRpc: ReportChangeRow[]; wsChannels: ReportChangeRow[]; tables: ReportTableRow[] }
  readingOrder: ReportReadingStep[]
  diagrams: ReportDiagram[]
  limits: { truncated: { findings: boolean; readingOrder: boolean; diagrams: boolean }; totalCounts: Record<string, number> }
  warnings: string[]
}

type Rec = Record<string, unknown>

function rec(v: unknown): Rec {
  return typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Rec) : {}
}
function str(v: unknown, fallback = ''): string {
  return typeof v === 'string' ? v : fallback
}
function nonNegInt(v: unknown): number {
  return typeof v === 'number' && Number.isFinite(v) && v >= 0 ? Math.floor(v) : 0
}
function arr(v: unknown): unknown[] {
  return Array.isArray(v) ? v : []
}
function strMap(v: unknown): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, val] of Object.entries(rec(v))) {
    if (typeof val === 'string') {out[k] = val}
  }
  return out
}
function numMap(v: unknown): Record<string, number> {
  const out: Record<string, number> = {}
  for (const [k, val] of Object.entries(rec(v))) {
    if (typeof val === 'number' && Number.isFinite(val) && val >= 0) {out[k] = Math.floor(val)}
  }
  return out
}

const VERDICTS = new Set(['pass', 'warn', 'fail', 'unknown'])
const RISKS = new Set(['LOW', 'MEDIUM', 'HIGH', 'CRITICAL', 'UNKNOWN'])
const DIAGRAM_KINDS = new Set(['components', 'erd', 'flow'])

function verdict(v: unknown): ReportGateVerdict {
  return typeof v === 'string' && VERDICTS.has(v) ? (v as ReportGateVerdict) : 'unknown'
}

export function parseReviewReportModel(raw: unknown): ReviewReportModel {
  const r = rec(raw)
  const warnings = arr(r.warnings).filter((w): w is string => typeof w === 'string')
  // Why: a payload without a recognizable shape must still be flagged so the export says data is missing.
  if (typeof raw !== 'object' || raw === null || r.gate === undefined) {
    warnings.push('no_gate')
  }
  const subject = rec(r.subject)
  const repro = rec(r.reproducibility)
  const summary = rec(r.summary)
  const risk = rec(r.risk)
  const gate = rec(r.gate)
  const waivers = rec(gate.waivers)
  const findings = rec(r.findings)
  const counts = rec(findings.counts)
  const contracts = rec(r.contracts)
  const limits = rec(r.limits)
  const truncated = rec(limits.truncated)

  return {
    schemaVersion: nonNegInt(r.schemaVersion) || 1,
    subject: {
      branch: str(subject.branch),
      baseRef: str(subject.baseRef),
      headCommit: str(subject.headCommit),
      baseCommit: str(subject.baseCommit),
      indexCommit: str(subject.indexCommit),
      indexStale: subject.indexStale === true,
      provider: str(subject.provider),
      turnKey: typeof subject.turnKey === 'string' ? subject.turnKey : null
    },
    reproducibility: {
      profileRef: str(repro.profileRef),
      toolVersions: strMap(repro.toolVersions),
      runIds: arr(repro.runIds).filter((x): x is string => typeof x === 'string'),
      modelDigest: str(repro.modelDigest)
    },
    summary: {
      files: nonNegInt(summary.files),
      added: nonNegInt(summary.added),
      removed: nonNegInt(summary.removed),
      symbols: nonNegInt(summary.symbols),
      flows: nonNegInt(summary.flows),
      components: arr(summary.components).filter((x): x is string => typeof x === 'string')
    },
    risk: {
      level: typeof risk.level === 'string' && RISKS.has(risk.level) ? (risk.level as ReportRiskLevel) : 'UNKNOWN',
      reasons: arr(risk.reasons).map((x) => ({ code: str(rec(x).code), params: strMap(rec(x).params) }))
    },
    gate: {
      verdict: verdict(gate.verdict),
      reasons: arr(gate.reasons).map((x) => {
        const reason = rec(x)
        return {
          check: str(reason.check),
          result: verdict(reason.result),
          code: typeof reason.code === 'string' ? reason.code : null,
          params: strMap(reason.params)
        }
      }),
      waivers: {
        count: nonNegInt(waivers.count),
        earliestExpiry: typeof waivers.earliestExpiry === 'string' ? waivers.earliestExpiry : null
      }
    },
    findings: {
      counts: {
        error: nonNegInt(counts.error),
        warning: nonNegInt(counts.warning),
        info: nonNegInt(counts.info),
        byCategory: numMap(counts.byCategory)
      },
      top: arr(findings.top).map((x) => {
        const f = rec(x)
        return {
          ruleId: str(f.ruleId),
          severity: str(f.severity, 'unknown'),
          category: str(f.category),
          file: str(f.file),
          line: nonNegInt(f.line),
          message: str(f.message),
          origin: f.origin === 'quality' || f.origin === 'structure' ? f.origin : 'unknown'
        }
      })
    },
    contracts: {
      protoRpc: arr(contracts.protoRpc).map(changeRow),
      wsChannels: arr(contracts.wsChannels).map(changeRow),
      tables: arr(contracts.tables).map((x) => {
        const t = rec(x)
        return { table: str(t.table), service: str(t.service), op: str(t.op), breaking: t.breaking === true }
      })
    },
    readingOrder: arr(r.readingOrder).map((x) => {
      const s = rec(x)
      return {
        n: nonNegInt(s.n),
        file: str(s.file),
        reason: str(s.reason),
        symbols: arr(s.symbols).filter((y): y is string => typeof y === 'string')
      }
    }),
    diagrams: arr(r.diagrams).map((x) => {
      const d = rec(x)
      return {
        kind: typeof d.kind === 'string' && DIAGRAM_KINDS.has(d.kind) ? (d.kind as ReportDiagramKind) : 'unknown',
        mermaid: str(d.mermaid),
        alt: arr(d.alt).filter((y): y is string => typeof y === 'string'),
        truncated: d.truncated === true
      }
    }),
    limits: {
      truncated: {
        findings: truncated.findings === true,
        readingOrder: truncated.readingOrder === true,
        diagrams: truncated.diagrams === true
      },
      totalCounts: numMap(limits.totalCounts)
    },
    warnings
  }
}

function changeRow(x: unknown): ReportChangeRow {
  const row = rec(x)
  return { name: str(row.name), change: str(row.change), breaking: row.breaking === true }
}
