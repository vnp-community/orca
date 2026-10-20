/**
 * Plan versions and the documents behind them (CR-REQ-045): the wire shapes of `request.planRevisions`,
 * `request.planBundleFile` and the parts of `request.planProposal` the Plan tab reads, and the gateway flag that says they exist.
 *
 * Every field has a safe empty value because the gateway returns Go struct JSON: proto3 zero values may be omitted, and a
 * gateway older than CR-REQ-045 has none of this (see request-plan-revision-parsers.ts).
 *
 * @module shared/request-plan-revision
 */

import type { PlanRunStatus } from './request-plan-auto'

/** `request.flowStatus.features` entry of a gateway that lists Plan versions and reads a document of any of them (CR-REQ-045). */
export const REQUEST_PLAN_REVISIONS_FEATURE = 'request.planRevisions'

/** Whether the connected gateway serves `request.planRevisions`; without it the tab shows the current version only. */
export function isPlanRevisionsSupported(features: ReadonlySet<string>): boolean {
  return features.has(REQUEST_PLAN_REVISIONS_FEATURE)
}

export type PlanRevisionStatus = 'current' | 'superseded'

/** What a version did to one file, against the version before it. */
export type PlanFileChange = 'added' | 'changed' | 'removed' | 'unchanged'

export type PlanRevisionStats = {
  durationMs?: number
  /** Only when the agent reported a cost: a missing cost is not a free run. */
  costUsd?: number
  turns?: number
}

export type PlanRevision = {
  /** 1-based; the number shown as "Version n". */
  revision: number
  runId: string
  status: PlanRevisionStatus
  /** `native` or `openspec`; empty when the gateway did not say. */
  engine: string
  /** What the reviewer asked for when this version was written; empty for the first proposal. */
  revisionFeedback: string
  actorId: string
  generatedAt: string
  /** Commit of the proposal branch; empty for native Plans and older gateways. */
  commit: string
  filesChanged: number
  fileCount: number
  /** The text of the documents is still stored; false once it was pruned (or never kept for a Request older than the CR). */
  textKept: boolean
  stats?: PlanRevisionStats
}

export type PlanRevisionsPage = {
  /** Newest first. */
  revisions: PlanRevision[]
  /** Empty on the last page. */
  nextPageToken: string
}

export type PlanBundleFileEntry = {
  /** Relative to the change directory, for example `specs/ai/spec.md`. */
  path: string
  /** The real size in bytes, also for a file too large to keep. */
  size: number
  omitted: boolean
  sha256?: string
  change: PlanFileChange
}

export type PlanBundleView = {
  changeId: string
  branch: string
  commit: string
  tasksMdDigest: string
  status: string
  syncState: string
  files: PlanBundleFileEntry[]
}

export type PlanCheckSeverity = 'info' | 'warning' | 'error'

export type PlanCheck = {
  code: string
  where: string
  message: string
  severity: PlanCheckSeverity
}

/** What the authoring agent reported about the project's design knowledge: its word, not a fact Orca checked. */
export type PlanKnowledge = {
  available: boolean
  snapshot: string
  read: string[]
  gaps: string[]
  conflicts: string[]
}

export type PlanBundleFileContent = {
  path: string
  content: string
  omitted: boolean
}

export type PlanTaskProposal = {
  title: string
  description: string
  taskType: string
  estimatedHours?: number
  promptTemplate: string
  /** 0-based indices into the tasks of the same Phase (or of the Plan when it has no Phase), as the backend wrote them. */
  dependsOnIndices: number[]
  labels: string[]
  irreversible: boolean
}

export type PlanPhaseProposal = {
  title: string
  description: string
  tasks: PlanTaskProposal[]
  dependsOnPhaseIndices: number[]
}

/** The native Plan as the AI proposed it (already scrubbed of secrets by the backend). */
export type PlanProposalDocument = {
  title: string
  summary: string
  notes: string
  phases: PlanPhaseProposal[]
  /** Tasks directly under the Plan when it has no Phase. */
  tasks: PlanTaskProposal[]
}

/** The parts of `request.planProposal` the Plan tab reads beyond the run state (see request-plan-auto.ts for that). */
export type PlanProposalView = {
  runId: string
  status: PlanRunStatus
  engine: string
  proposal: PlanProposalDocument | null
  bundle: PlanBundleView | null
  checks: PlanCheck[]
  knowledge: PlanKnowledge | null
  errorCode: string
  errorMessage: string
}
