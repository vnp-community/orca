/**
 * Outcome Review wire parsers — FE-OR-T111
 *
 * Tolerant like every request parser: a field the server leaves out gets a neutral value and nothing throws. Enums are never
 * folded into a neighbouring value (the `task_list` → `plan` trap of the request parsers): an unknown value is kept as the raw
 * string and the screen shows "Unknown".
 *
 * @module shared/outcome-review-wire-parsers
 */

import { safeArr, safeNum, safeOptNum, safeOptStr, safeStr } from './request-wire-scalars'
import { bool, obj, openEnum, strings } from './outcome-review-wire-readers'
import type {
  DesignVerdict,
  DiagramRef,
  DiagramStatus,
  DiagramVariant,
  EvidenceKind,
  KgInfo,
  OutcomeReview,
  OutcomeReviewModel,
  ReviewChange,
  ReviewEvidence,
  ReviewItem,
  ReviewOrigin,
  ReviewPhase,
  ReviewSection,
  ReviewSeverity,
  ReviewState,
  ReviewSubject,
  Verdict
} from './outcome-review-types'

function parseEvidence(raw: unknown): ReviewEvidence {
  const r = obj(raw)
  const evidence: ReviewEvidence = { kind: openEnum<EvidenceKind>(r.kind), ref: safeStr(r.ref) }
  for (const key of ['snapshot', 'doc', 'section'] as const) {
    const value = safeOptStr(r[key])
    if (value) {
      evidence[key] = value
    }
  }
  const line = safeOptNum(r.line)
  if (line) {
    evidence.line = line
  }
  return evidence
}

function parseItem(raw: unknown): ReviewItem {
  const r = obj(raw)
  const item: ReviewItem = {
    key: safeStr(r.key),
    section: openEnum<ReviewSection>(r.section),
    change: openEnum<ReviewChange>(r.change),
    origin: openEnum<ReviewOrigin>(r.origin),
    title: safeStr(r.title),
    detail: safeStr(r.detail),
    severity: openEnum<ReviewSeverity>(r.severity),
    needsDecision: bool(r.needsDecision),
    advisory: bool(r.advisory),
    unverified: bool(r.unverified),
    containers: strings(r.containers),
    protocol: safeStr(r.protocol),
    evidence: safeArr(r.evidence, parseEvidence),
    before: safeStr(r.before),
    after: safeStr(r.after)
  }
  const source = safeStr(r.source)
  if (source) {
    item.source = source
  }
  const reviewState = safeStr(r.reviewState)
  if (reviewState) {
    item.reviewState = reviewState
  }
  if (typeof r.expected === 'boolean') {
    item.expected = r.expected
  }
  return item
}

function parseDesignVerdict(raw: unknown): DesignVerdict {
  const r = obj(raw)
  return {
    containersChanged: safeNum(r.containersChanged, 0),
    newEdges: safeNum(r.newEdges, 0),
    removedEdges: safeNum(r.removedEdges, 0),
    unplannedEdges: safeNum(r.unplannedEdges, 0),
    newCalls: safeNum(r.newCalls, 0),
    removedCalls: safeNum(r.removedCalls, 0),
    reordered: safeNum(r.reordered, 0),
    unplannedCalls: safeNum(r.unplannedCalls, 0),
    reportedDeviations: safeNum(r.reportedDeviations, 0),
    withEvidence: safeNum(r.withEvidence, 0),
    advisory: safeNum(r.advisory, 0),
    designChecked: bool(r.designChecked),
    reviewedRatio: safeNum(r.reviewedRatio, 0)
  }
}

function parseVerdict(raw: unknown): Verdict {
  const r = obj(raw)
  const req = obj(r.requirements)
  const quality = obj(r.quality)
  const release = obj(r.release)
  return {
    requirements: {
      delivered: safeNum(req.delivered, 0),
      partial: safeNum(req.partial, 0),
      notCovered: safeNum(req.notCovered, 0),
      unknown: safeNum(req.unknown, 0),
      planned: safeNum(req.planned, 0),
      total: safeNum(req.total, 0)
    },
    architecture: parseDesignVerdict(r.architecture),
    flow: parseDesignVerdict(r.flow),
    quality: { gate: safeStr(quality.gate) },
    release: { risk: safeStr(release.risk), flags: strings(release.flags) }
  }
}

function parseSubject(raw: unknown): ReviewSubject {
  const r = obj(raw)
  const subject: ReviewSubject = {
    baseRef: safeStr(r.baseRef),
    heads: safeArr(r.heads, (h) => {
      const o = obj(h)
      const head: ReviewSubject['heads'][number] = {
        branch: safeStr(o.branch),
        commit: safeStr(o.commit)
      }
      const taskId = safeOptStr(o.taskId)
      if (taskId) {
        head.taskId = taskId
      }
      return head
    })
  }
  for (const key of ['baseCommit', 'headCommit', 'engine', 'executionMode'] as const) {
    const value = safeOptStr(r[key])
    if (value) {
      subject[key] = value
    }
  }
  return subject
}

function parseKg(raw: unknown): KgInfo {
  const r = obj(raw)
  const kg: KgInfo = {
    available: bool(r.available),
    gaps: strings(r.gaps),
    conflicts: strings(r.conflicts)
  }
  const snapshot = safeOptStr(r.snapshot)
  if (snapshot) {
    kg.snapshot = snapshot
  }
  return kg
}

function parseDiagramRef(raw: unknown): DiagramRef {
  const r = obj(raw)
  const ref: DiagramRef = {
    section: openEnum<ReviewSection>(r.section),
    variant: openEnum<DiagramVariant>(r.variant),
    type: safeStr(r.type),
    inputDigest: safeStr(r.inputDigest),
    status: openEnum<DiagramStatus>(r.status)
  }
  const name = safeOptStr(r.name)
  if (name) {
    ref.name = name
  }
  const artifactId = safeOptStr(r.artifactId)
  if (artifactId) {
    ref.artifactId = artifactId
  }
  return ref
}

export function parseReviewModel(raw: unknown): OutcomeReviewModel {
  const r = obj(raw)
  return {
    schemaVersion: safeNum(r.schemaVersion, 0),
    phase: openEnum<ReviewPhase>(r.phase),
    subject: parseSubject(r.subject),
    inputsDigest: safeStr(r.inputsDigest),
    factsDigest: safeStr(r.factsDigest),
    state: openEnum<ReviewState>(r.state),
    verdict: parseVerdict(r.verdict),
    items: safeArr(r.items, parseItem),
    kg: parseKg(r.kg),
    diagrams: safeArr(r.diagrams, parseDiagramRef),
    warnings: strings(r.warnings),
    truncated: bool(r.truncated)
  }
}

/** A review needs its id to be acted on; anything else is a gap the screen cannot repair. */
export function parseOutcomeReview(raw: unknown): OutcomeReview | null {
  const r = obj(raw)
  const reviewId = safeStr(r.reviewId)
  if (reviewId === '') {
    return null
  }
  const review: OutcomeReview = {
    reviewId,
    requestId: safeStr(r.requestId),
    phase: openEnum<ReviewPhase>(r.phase),
    state: openEnum<ReviewState>(r.state),
    headKey: safeStr(r.headKey),
    version: safeNum(r.version, 0),
    createdAt: safeStr(r.createdAt),
    updatedAt: safeStr(r.updatedAt),
    model: parseReviewModel(r.model)
  }
  for (const key of ['kgSnapshot', 'approvedBy', 'approvedAt', 'approvalNote'] as const) {
    const value = safeOptStr(r[key])
    if (value) {
      review[key] = value
    }
  }
  return review
}
