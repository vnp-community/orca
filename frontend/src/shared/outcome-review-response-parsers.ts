/**
 * Parsers of the answers of `request.review.*` — FE-OR-T111
 *
 * Same rules as the model parsers: tolerant, never throwing, enums kept raw.
 *
 * @module shared/outcome-review-response-parsers
 */

import { safeArr, safeNum, safeOptNum, safeOptStr, safeStr } from './request-wire-scalars'
import { bool, obj, openEnum, strings } from './outcome-review-wire-readers'
import { parseOutcomeReview } from './outcome-review-wire-parsers'
import type {
  DecideOutcome,
  DiagramVariant,
  GenerateOutcome,
  ModelDiff,
  OutcomeReview,
  OutcomeReviewRun,
  RegenerateOutcome,
  RegenerationEstimate,
  ReviewArtifactContent,
  ReviewArtifactMeta,
  ReviewDecision,
  ReviewDecisionList,
  ReviewDecisionValue,
  ReviewEstimate,
  ReviewPhase,
  ReviewRunKind,
  ReviewRunStatus,
  ReviewSection,
  ReviewState
} from './outcome-review-types'

export function parseReviewRun(raw: unknown): OutcomeReviewRun | null {
  const r = obj(raw)
  const runId = safeStr(r.runId)
  if (runId === '') {
    return null
  }
  const run: OutcomeReviewRun = {
    runId,
    reviewId: safeStr(r.reviewId),
    kind: openEnum<ReviewRunKind>(r.kind),
    status: openEnum<ReviewRunStatus>(r.status),
    scope: safeStr(r.scope),
    startedAt: safeStr(r.startedAt)
  }
  for (const key of ['errorCode', 'costEstimate', 'costActual', 'finishedAt'] as const) {
    const value = safeOptStr(r[key])
    if (value) {
      run[key] = value
    }
  }
  return run
}

export function parseRuns(raw: unknown): OutcomeReviewRun[] {
  return safeArr(obj(raw).runs, parseReviewRun).filter((r): r is OutcomeReviewRun => r !== null)
}

export function parseRegenerationEstimate(raw: unknown): RegenerationEstimate | undefined {
  if (!raw || typeof raw !== 'object') {
    return undefined
  }
  const r = obj(raw)
  const estimate: RegenerationEstimate = {
    agentRuns: safeNum(r.agentRuns, 0),
    diagramsToDraw: safeNum(r.diagramsToDraw, 0),
    costKnown: bool(r.costKnown),
    confirmRequired: bool(r.confirmRequired),
    drawingsDependOnResult: bool(r.drawingsDependOnResult)
  }
  const cost = safeOptNum(r.costUsd)
  if (cost !== undefined) {
    estimate.costUsd = cost
  }
  for (const key of ['lastContextCost', 'lastDrawCost'] as const) {
    const value = safeOptStr(r[key])
    if (value) {
      estimate[key] = value
    }
  }
  return estimate
}

export function parseReviewEstimate(raw: unknown): ReviewEstimate | null {
  if (!raw || typeof raw !== 'object') {
    return null
  }
  const r = obj(raw)
  const estimate: ReviewEstimate = {
    phase: openEnum<ReviewPhase>(r.phase),
    executionFinished: bool(r.executionFinished),
    reviewExists: bool(r.reviewExists),
    stale: bool(r.stale),
    currentInputsDigest: safeStr(r.currentInputsDigest),
    warnings: strings(r.warnings),
    contextCached: bool(r.contextCached),
    designLocked: bool(r.designLocked),
    contextRunnable: bool(r.contextRunnable),
    contextGaps: strings(r.contextGaps)
  }
  const reviewId = safeOptStr(r.reviewId)
  if (reviewId) {
    estimate.reviewId = reviewId
  }
  const regeneration = parseRegenerationEstimate(r.regeneration)
  if (regeneration) {
    estimate.regeneration = regeneration
  }
  return estimate
}

/** `request.review.get`: a Request with no review yet answers `found: false`, which is not an error. */
export function parseGetReview(raw: unknown): { found: boolean; review: OutcomeReview | null } {
  const r = obj(raw)
  if (r.found !== true) {
    return { found: false, review: null }
  }
  const review = parseOutcomeReview(r.review)
  return { found: review !== null, review }
}

export function parseGenerateOutcome(raw: unknown): GenerateOutcome | null {
  const r = obj(raw)
  const review = parseOutcomeReview(r.review)
  if (!review) {
    return null
  }
  const outcome: GenerateOutcome = {
    review,
    reused: bool(r.reused),
    created: bool(r.created),
    contextCached: bool(r.contextCached),
    drawingsCached: safeNum(r.drawingsCached, 0),
    drawingsPending: safeNum(r.drawingsPending, 0)
  }
  const run = parseReviewRun(r.run)
  if (run) {
    outcome.run = run
  }
  const drawRun = parseReviewRun(r.drawRun)
  if (drawRun) {
    outcome.drawRun = drawRun
  }
  for (const key of ['contextSkipped', 'drawError'] as const) {
    const value = safeOptStr(r[key])
    if (value) {
      outcome[key] = value
    }
  }
  return outcome
}

export function parseReviewDecision(raw: unknown): ReviewDecision | null {
  const r = obj(raw)
  const decisionId = safeStr(r.decisionId)
  if (decisionId === '') {
    return null
  }
  const decision: ReviewDecision = {
    decisionId,
    reviewId: safeStr(r.reviewId),
    itemKey: safeStr(r.itemKey),
    decision: openEnum<ReviewDecisionValue>(r.decision),
    decidedBy: safeStr(r.decidedBy),
    decidedAt: safeStr(r.decidedAt),
    active: bool(r.active),
    orphaned: bool(r.orphaned)
  }
  for (const key of ['note', 'childRequestId'] as const) {
    const value = safeOptStr(r[key])
    if (value) {
      decision[key] = value
    }
  }
  return decision
}

export function parseDecisionList(raw: unknown): ReviewDecisionList {
  const r = obj(raw)
  return {
    decisions: safeArr(r.decisions, parseReviewDecision).filter(
      (d): d is ReviewDecision => d !== null
    ),
    openDecisions: safeNum(r.openDecisions, 0)
  }
}

export function parseDecideOutcome(raw: unknown): DecideOutcome | null {
  const r = obj(raw)
  const decision = parseReviewDecision(r.decision)
  if (!decision) {
    return null
  }
  return {
    decision,
    reviewVersion: safeNum(r.reviewVersion, 0),
    reviewState: openEnum<ReviewState>(r.reviewState),
    openDecisions: safeNum(r.openDecisions, 0)
  }
}

export function parseArtifactMeta(raw: unknown): ReviewArtifactMeta | null {
  const r = obj(raw)
  const artifactId = safeStr(r.artifactId)
  if (artifactId === '') {
    return null
  }
  const meta: ReviewArtifactMeta = {
    artifactId,
    section: openEnum<ReviewSection>(r.section),
    variant: openEnum<DiagramVariant>(r.variant),
    diagramType: safeStr(r.diagramType),
    inputDigest: safeStr(r.inputDigest),
    presetDigest: safeStr(r.presetDigest),
    skillVersion: safeStr(r.skillVersion),
    revision: safeNum(r.revision, 0),
    state: safeStr(r.state),
    origin: safeStr(r.origin),
    sizeBytes: safeNum(r.sizeBytes, 0),
    createdAt: safeStr(r.createdAt)
  }
  for (const key of [
    'name',
    'reason',
    'errorCode',
    'blobDigest',
    'viewBox',
    'title',
    'desc'
  ] as const) {
    const value = safeOptStr(r[key])
    if (value) {
      meta[key] = value
    }
  }
  return meta
}

export function parseArtifactRevisions(raw: unknown): ReviewArtifactMeta[] {
  return safeArr(obj(raw).revisions, parseArtifactMeta).filter(
    (a): a is ReviewArtifactMeta => a !== null
  )
}

export function parseArtifactContent(raw: unknown): ReviewArtifactContent | null {
  const r = obj(raw)
  const artifact = parseArtifactMeta(r.artifact)
  if (!artifact) {
    return null
  }
  const out: ReviewArtifactContent = { artifact, notModified: bool(r.notModified) }
  const mediaType = safeOptStr(r.mediaType)
  if (mediaType) {
    out.mediaType = mediaType
  }
  if (typeof r.content === 'string') {
    out.content = r.content
  }
  return out
}

export function parseModelDiff(raw: unknown): ModelDiff | undefined {
  if (!raw || typeof raw !== 'object') {
    return undefined
  }
  const r = obj(raw)
  return {
    addedNodes: strings(r.addedNodes),
    removedNodes: strings(r.removedNodes),
    changedNodes: strings(r.changedNodes),
    addedEdges: strings(r.addedEdges),
    removedEdges: strings(r.removedEdges),
    changedEdges: strings(r.changedEdges)
  }
}

export function parseRegenerateOutcome(raw: unknown): RegenerateOutcome {
  const r = obj(raw)
  const out: RegenerateOutcome = { replayed: bool(r.replayed) }
  const contextRun = parseReviewRun(r.contextRun)
  if (contextRun) {
    out.contextRun = contextRun
  }
  const drawRun = parseReviewRun(r.drawRun)
  if (drawRun) {
    out.drawRun = drawRun
  }
  const estimate = parseRegenerationEstimate(r.estimate)
  if (estimate) {
    out.estimate = estimate
  }
  const skipped = safeOptStr(r.skipped)
  if (skipped) {
    out.skipped = skipped
  }
  return out
}
