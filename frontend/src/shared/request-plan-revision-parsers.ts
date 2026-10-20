/**
 * Tolerant parsers of the Plan version channels (CR-REQ-045). None of them throws: a field that is missing or of the wrong type
 * becomes its safe empty value, an element that is not an object is dropped, and a gateway older than the CR (no `bundle`, no
 * `sha256`, no `change`) reads as "nothing changed" rather than as an error.
 *
 * @module shared/request-plan-revision-parsers
 */

import { safeEnum, safeOptNum, safeOptStr, safeStr } from './request-wire-scalars'
import type { PlanRunStatus } from './request-plan-auto'
import type {
  PlanBundleFileContent,
  PlanBundleFileEntry,
  PlanBundleView,
  PlanCheck,
  PlanCheckSeverity,
  PlanFileChange,
  PlanKnowledge,
  PlanPhaseProposal,
  PlanProposalDocument,
  PlanProposalView,
  PlanRevision,
  PlanRevisionStats,
  PlanRevisionStatus,
  PlanRevisionsPage,
  PlanTaskProposal
} from './request-plan-revision'

const REVISION_STATUSES = new Set<PlanRevisionStatus>(['current', 'superseded'])
const FILE_CHANGES = new Set<PlanFileChange>(['added', 'changed', 'removed', 'unchanged'])
const CHECK_SEVERITIES = new Set<PlanCheckSeverity>(['info', 'warning', 'error'])
const RUN_STATUSES = new Set<PlanRunStatus>(['running', 'succeeded', 'failed', ''])

function asRecord(raw: unknown): Record<string, unknown> | null {
  return raw !== null && typeof raw === 'object' && !Array.isArray(raw)
    ? (raw as Record<string, unknown>)
    : null
}

/** protojson writes an int64 as a string, so a count may arrive as `"12"`; a negative or broken one reads as 0. */
function wireCount(value: unknown): number {
  const n = typeof value === 'string' && value.trim() !== '' ? Number(value) : value
  return typeof n === 'number' && Number.isFinite(n) && n > 0 ? Math.floor(n) : 0
}

function wireOptNumber(value: unknown): number | undefined {
  const n = typeof value === 'string' && value.trim() !== '' ? Number(value) : value
  return safeOptNum(n)
}

function stringList(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string' && item !== '')
    : []
}

function indexList(value: unknown): number[] {
  return Array.isArray(value)
    ? value.filter((item): item is number => Number.isInteger(item) && (item as number) >= 0)
    : []
}

function mapElements<T>(value: unknown, parse: (item: Record<string, unknown>) => T | null): T[] {
  if (!Array.isArray(value)) {
    return []
  }
  const out: T[] = []
  for (const item of value) {
    const record = asRecord(item)
    const parsed = record ? parse(record) : null
    if (parsed) {
      out.push(parsed)
    }
  }
  return out
}

function parseStats(raw: unknown): PlanRevisionStats | undefined {
  const o = asRecord(raw)
  if (!o) {
    return undefined
  }
  const durationMs = wireOptNumber(o.durationMs)
  const turns = wireOptNumber(o.turns)
  // Why: protojson sends `costUsd: 0` with `hasCost: false` for a run that reported none; that is "not reported", not free.
  const costUsd = o.hasCost === false ? undefined : wireOptNumber(o.costUsd)
  const stats: PlanRevisionStats = {
    ...(durationMs !== undefined && durationMs > 0 ? { durationMs } : {}),
    ...(turns !== undefined && turns > 0 ? { turns } : {}),
    ...(costUsd !== undefined ? { costUsd } : {})
  }
  return Object.keys(stats).length > 0 ? stats : undefined
}

function parsePlanRevision(o: Record<string, unknown>): PlanRevision | null {
  const revision = wireCount(o.revision)
  if (revision === 0) {
    return null
  }
  const stats = parseStats(o.stats)
  return {
    revision,
    runId: safeStr(o.runId),
    // Why: an unknown status must never make an old version look like the one to approve.
    status: safeEnum(o.status, REVISION_STATUSES, 'superseded'),
    engine: safeStr(o.engine),
    revisionFeedback: safeStr(o.revisionFeedback).trim(),
    actorId: safeStr(o.actorId),
    generatedAt: safeStr(o.generatedAt),
    commit: safeStr(o.commit),
    filesChanged: wireCount(o.filesChanged),
    fileCount: wireCount(o.fileCount),
    // Why: an omitted `false` reads as kept; reading a pruned version answers REQUEST_PLAN_REVISION_TEXT_PRUNED, which the viewer shows.
    textKept: o.textKept !== false,
    ...(stats ? { stats } : {})
  }
}

/** Parses the answer of `request.planRevisions`, newest first; never throws. */
export function parsePlanRevisions(raw: unknown): PlanRevisionsPage {
  const o = asRecord(raw) ?? {}
  const seen = new Set<number>()
  const revisions = mapElements(o.revisions, parsePlanRevision)
    .filter((r) => {
      if (seen.has(r.revision)) {
        return false
      }
      seen.add(r.revision)
      return true
    })
    .sort((a, b) => b.revision - a.revision)
  return { revisions, nextPageToken: safeStr(o.nextPageToken) }
}

function parseBundleFile(o: Record<string, unknown>): PlanBundleFileEntry | null {
  const path = safeStr(o.path)
  if (path === '') {
    return null
  }
  const sha256 = safeOptStr(o.sha256)
  return {
    path,
    size: wireCount(o.size),
    omitted: o.omitted === true,
    ...(sha256 ? { sha256 } : {}),
    change: safeEnum(o.change, FILE_CHANGES, 'unchanged')
  }
}

function parseBundle(raw: unknown): PlanBundleView | null {
  const o = asRecord(raw)
  if (!o) {
    return null
  }
  return {
    changeId: safeStr(o.changeId),
    branch: safeStr(o.branch),
    commit: safeStr(o.commit),
    tasksMdDigest: safeStr(o.tasksMdDigest),
    status: safeStr(o.status),
    syncState: safeStr(o.syncState),
    files: mapElements(o.files, parseBundleFile)
  }
}

function parseCheck(o: Record<string, unknown>): PlanCheck | null {
  const code = safeStr(o.code)
  const message = safeStr(o.message)
  if (code === '' && message === '') {
    return null
  }
  return {
    code,
    where: safeStr(o.where),
    message,
    severity: safeEnum(o.severity, CHECK_SEVERITIES, 'info')
  }
}

function parseKnowledge(raw: unknown): PlanKnowledge | null {
  const o = asRecord(raw)
  if (!o) {
    return null
  }
  return {
    available: o.available === true,
    snapshot: safeStr(o.snapshot),
    read: stringList(o.read),
    gaps: stringList(o.gaps),
    conflicts: stringList(o.conflicts)
  }
}

function parseTask(o: Record<string, unknown>): PlanTaskProposal {
  const estimatedHours = wireOptNumber(o.estimatedHours)
  return {
    title: safeStr(o.title),
    description: safeStr(o.description),
    taskType: safeStr(o.taskType),
    ...(estimatedHours !== undefined && estimatedHours >= 0 ? { estimatedHours } : {}),
    promptTemplate: safeStr(o.promptTemplate),
    dependsOnIndices: indexList(o.dependsOnIndices),
    labels: stringList(o.labels),
    irreversible: o.irreversible === true
  }
}

function parsePhase(o: Record<string, unknown>): PlanPhaseProposal {
  return {
    title: safeStr(o.title),
    description: safeStr(o.description),
    tasks: mapElements(o.tasks, parseTask),
    dependsOnPhaseIndices: indexList(o.dependsOnPhaseIndices)
  }
}

function parseProposalDocument(raw: unknown): PlanProposalDocument | null {
  const o = asRecord(raw)
  if (!o) {
    return null
  }
  return {
    title: safeStr(o.title),
    summary: safeStr(o.summary),
    notes: safeStr(o.notes),
    phases: mapElements(o.phases, parsePhase),
    tasks: mapElements(o.tasks, parseTask)
  }
}

/** Parses the answer of `request.planProposal`; the run state itself is read by `parsePlanRunView`. Never throws. */
export function parsePlanProposalView(raw: unknown): PlanProposalView {
  const o = asRecord(raw) ?? {}
  return {
    runId: safeStr(o.runId),
    status: safeEnum(o.status, RUN_STATUSES, ''),
    engine: safeStr(o.engine),
    proposal: parseProposalDocument(o.proposal),
    bundle: parseBundle(o.bundle),
    checks: mapElements(o.checks, parseCheck),
    knowledge: parseKnowledge(o.knowledge),
    errorCode: safeStr(o.errorCode),
    errorMessage: safeStr(o.errorMessage)
  }
}

/** Parses the answer of `request.planBundleFile`; never throws. */
export function parsePlanBundleFile(raw: unknown): PlanBundleFileContent {
  const o = asRecord(raw) ?? {}
  return {
    path: safeStr(o.path),
    content: safeStr(o.content),
    omitted: o.omitted === true
  }
}
