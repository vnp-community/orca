/**
 * Outcome Review wire types — FE-OR-T110
 *
 * What `request.review.*` returns, as the screen holds it. Every enum is open: a value the server adds later is kept as the raw
 * string and shown as "Unknown", never folded into a neighbouring value.
 *
 * @module shared/outcome-review-types
 */

/** A known value, or the raw string the server sent. Compare with the `is*` guards in outcome-review-wire-parsers. */
export type Open<T extends string> = T | (string & {})

export type ReviewPhase = 'design' | 'delivered'
export type ReviewState = 'draft' | 'ready' | 'stale' | 'approved' | 'superseded'
export type ReviewSection =
  | 'architecture'
  | 'flow'
  | 'behavior'
  | 'data'
  | 'contracts'
  | 'requirements'
  | 'config'
export type ReviewChange = 'added' | 'modified' | 'removed' | 'unchanged' | 'missing'
export type ReviewOrigin = 'fact' | 'agent'
export type ReviewSeverity = 'info' | 'warning' | 'breaking' | 'deviation' | 'regression'
/** Where a component or edge of the design was found; only `code` means "not in the design". */
export type ComponentSource = 'kg' | 'sad-text' | 'code'
export type KgReviewState = 'approved' | 'needs_review' | 'unknown'
export type EvidenceKind =
  | 'kg'
  | 'source'
  | 'file'
  | 'rpc'
  | 'channel'
  | 'migration'
  | 'check'
  | 'task'
export type DiagramVariant = 'design' | 'delta'
export type DiagramStatus = 'cached' | 'missing' | 'drawing' | 'failed' | 'disabled'
export type ReviewRunKind = 'context' | 'draw'
export type ReviewRunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
export type ReviewDecisionValue = 'accept_exception' | 'send_back' | 'dismiss'
export type QualityGate = 'pass' | 'warn' | 'fail' | 'unknown' | 'absent'
export type ReleaseRisk = 'low' | 'medium' | 'high' | 'critical' | 'unknown'

export type ReviewEvidence = {
  kind: Open<EvidenceKind>
  ref: string
  snapshot?: string
  doc?: string
  section?: string
  line?: number
}

export type ReviewItem = {
  /** Stable between builds, so a decision outlives a regeneration. */
  key: string
  section: Open<ReviewSection>
  change: Open<ReviewChange>
  origin: Open<ReviewOrigin>
  title: string
  detail: string
  severity: Open<ReviewSeverity>
  needsDecision: boolean
  /** Rests on elements nobody approved; never asks for a decision. */
  advisory: boolean
  /** The agent said it but no evidence could be checked. */
  unverified: boolean
  containers: string[]
  protocol: string
  source?: Open<ComponentSource>
  reviewState?: Open<KgReviewState>
  /** Undefined when there is no design to compare with. */
  expected?: boolean
  evidence: ReviewEvidence[]
  before: string
  after: string
}

export type RequirementsVerdict = {
  delivered: number
  partial: number
  notCovered: number
  unknown: number
  planned: number
  total: number
}

export type DesignVerdict = {
  containersChanged: number
  newEdges: number
  removedEdges: number
  unplannedEdges: number
  newCalls: number
  removedCalls: number
  reordered: number
  unplannedCalls: number
  reportedDeviations: number
  withEvidence: number
  advisory: number
  /** False means there was nothing to compare with: the card says Unknown, never "0 deviations". */
  designChecked: boolean
  /** Share of the graph elements read that a person approved. */
  reviewedRatio: number
}

export type Verdict = {
  requirements: RequirementsVerdict
  architecture: DesignVerdict
  flow: DesignVerdict
  quality: { gate: Open<QualityGate> }
  release: { risk: Open<ReleaseRisk>; flags: string[] }
}

export type ReviewHead = { branch: string; commit: string; taskId?: string }

export type ReviewSubject = {
  baseRef: string
  baseCommit?: string
  headCommit?: string
  heads: ReviewHead[]
  engine?: string
  executionMode?: string
}

export type KgInfo = { available: boolean; snapshot?: string; gaps: string[]; conflicts: string[] }

export type DiagramRef = {
  section: Open<ReviewSection>
  variant: Open<DiagramVariant>
  name?: string
  type: string
  inputDigest: string
  artifactId?: string
  status: Open<DiagramStatus>
}

export type OutcomeReviewModel = {
  schemaVersion: number
  phase: Open<ReviewPhase>
  subject: ReviewSubject
  inputsDigest: string
  factsDigest: string
  state: Open<ReviewState>
  verdict: Verdict
  items: ReviewItem[]
  kg: KgInfo
  diagrams: DiagramRef[]
  warnings: string[]
  truncated: boolean
}

export type OutcomeReview = {
  reviewId: string
  requestId: string
  phase: Open<ReviewPhase>
  state: Open<ReviewState>
  headKey: string
  kgSnapshot?: string
  /** Sent back on every write; the server refuses a write based on an older one. */
  version: number
  createdAt: string
  updatedAt: string
  approvedBy?: string
  approvedAt?: string
  approvalNote?: string
  model: OutcomeReviewModel
}

export type OutcomeReviewRun = {
  runId: string
  reviewId: string
  kind: Open<ReviewRunKind>
  status: Open<ReviewRunStatus>
  scope: string
  errorCode?: string
  costEstimate?: string
  costActual?: string
  startedAt: string
  finishedAt?: string
}

export type RegenerationEstimate = {
  agentRuns: number
  diagramsToDraw: number
  costKnown: boolean
  costUsd?: number
  lastContextCost?: string
  lastDrawCost?: string
  confirmRequired: boolean
  drawingsDependOnResult: boolean
}

export type ReviewEstimate = {
  phase: Open<ReviewPhase>
  executionFinished: boolean
  reviewExists: boolean
  reviewId?: string
  stale: boolean
  currentInputsDigest: string
  warnings: string[]
  contextCached: boolean
  designLocked: boolean
  contextRunnable: boolean
  contextGaps: string[]
  regeneration?: RegenerationEstimate
}

export type GenerateOutcome = {
  review: OutcomeReview
  reused: boolean
  created: boolean
  run?: OutcomeReviewRun
  contextCached: boolean
  contextSkipped?: string
  drawRun?: OutcomeReviewRun
  drawingsCached: number
  drawingsPending: number
  drawError?: string
}

export type ReviewDecision = {
  decisionId: string
  reviewId: string
  itemKey: string
  decision: Open<ReviewDecisionValue>
  note?: string
  childRequestId?: string
  decidedBy: string
  decidedAt: string
  /** The decision in force for its item; earlier ones stay as history. */
  active: boolean
  /** The item is gone from the latest review. */
  orphaned: boolean
}

export type ReviewArtifactMeta = {
  artifactId: string
  section: Open<ReviewSection>
  variant: Open<DiagramVariant>
  name?: string
  diagramType: string
  inputDigest: string
  presetDigest: string
  skillVersion: string
  revision: number
  state: string
  origin: string
  reason?: string
  errorCode?: string
  blobDigest?: string
  sizeBytes: number
  viewBox?: string
  title?: string
  desc?: string
  createdAt: string
}

export type ReviewArtifactContent = {
  artifact: ReviewArtifactMeta
  notModified: boolean
  mediaType?: string
  content?: string
}

export type ModelDiff = {
  addedNodes: string[]
  removedNodes: string[]
  changedNodes: string[]
  addedEdges: string[]
  removedEdges: string[]
  changedEdges: string[]
}

export type RegenerateOutcome = {
  contextRun?: OutcomeReviewRun
  drawRun?: OutcomeReviewRun
  replayed: boolean
  estimate?: RegenerationEstimate
  skipped?: string
}

export type DecideOutcome = {
  decision: ReviewDecision
  reviewVersion: number
  reviewState: Open<ReviewState>
  openDecisions: number
}

export type ReviewDecisionList = { decisions: ReviewDecision[]; openDecisions: number }
