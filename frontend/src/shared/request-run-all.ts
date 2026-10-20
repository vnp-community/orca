/**
 * "Approve and run all steps" (CR-REQ-043): the optional `executionMode` of `approval.approve`, the gateway flag that says
 * it is honoured, and the per-project engine settings the project switch reads and writes back.
 *
 * Parsers never throw: a gateway older than CR-REQ-043 simply lacks the fields, and proto3 zero values may be omitted.
 *
 * @module shared/request-run-all
 */

import { splitErrorCode } from './request-errors'
import type { RequestRpcError } from './request-errors'

/** `request.flowStatus.features` entry of a gateway that forwards and enforces `executionMode` on `approval.approve`. */
export const APPROVAL_RUN_ALL_FEATURE = 'approval.runAll'

/** The one value `approval.approve` accepts besides empty; "step by step" is never sent. */
export const RUN_ALL_EXECUTION_MODE = 'continuous'

export type ExecutionModeValue = 'continuous' | 'step_by_step'

/** The Request already runs step by step (pinned on an earlier run), so the run-all choice cannot apply. */
export const RUN_ALL_ALREADY_PINNED_CODE = 'REQUEST_EXECUTION_MODE_ALREADY_PINNED'

/** The codes below mean the UI offered the choice where it does not apply: a bug, not a user mistake. */
export const RUN_ALL_PROGRAMMING_ERROR_CODES: ReadonlySet<string> = new Set([
  'REQUEST_EXECUTION_MODE_NOT_APPLICABLE',
  'REQUEST_EXECUTION_MODE_INVALID'
])

export type ApproveResult = {
  requestStatus?: string
  /** The mode now pinned on the Request; absent when the gateway did not say. */
  executionMode?: ExecutionModeValue
}

function executionModeOf(raw: unknown): ExecutionModeValue | undefined {
  return raw === 'continuous' || raw === 'step_by_step' ? raw : undefined
}

export function parseApproveResult(raw: unknown): ApproveResult {
  if (raw === null || typeof raw !== 'object') {
    return {}
  }
  const o = raw as Record<string, unknown>
  const out: ApproveResult = {}
  if (typeof o.requestStatus === 'string') {
    out.requestStatus = o.requestStatus
  }
  const mode = executionModeOf(o.executionMode)
  if (mode) {
    out.executionMode = mode
  }
  return out
}

/** Backend code of a failed call, from the "CODE: text" message or the classified error. */
function failureCode(error: Pick<RequestRpcError, 'code' | 'message' | 'backendCode'>): string {
  return (
    error.backendCode ||
    splitErrorCode(error.message ?? '').code ||
    error.code ||
    ''
  ).toUpperCase()
}

export function isRunAllAlreadyPinned(
  error: Pick<RequestRpcError, 'code' | 'message' | 'backendCode'> | null | undefined
): boolean {
  return error ? failureCode(error) === RUN_ALL_ALREADY_PINNED_CODE : false
}

export function isRunAllProgrammingError(
  error: Pick<RequestRpcError, 'code' | 'message' | 'backendCode'>
): boolean {
  return RUN_ALL_PROGRAMMING_ERROR_CODES.has(failureCode(error))
}

export type ProjectEngineSettings = {
  projectId: string
  /** Written back as read: the backend turns an empty engine into `native`. */
  engine: string
  openspecMinVersion: string
  solutionAgent: boolean
  executionMode: ExecutionModeValue
  /** The Outcome Review is on for the project now: its choice, else the deployment default, never past the kill switch. */
  outcomeReviewEnabled: boolean
  /** 0 when the project has no settings row yet (deployment defaults apply). */
  version: number
}

export function parseEngineSettings(raw: unknown): ProjectEngineSettings | null {
  if (raw === null || typeof raw !== 'object') {
    return null
  }
  const o = raw as Record<string, unknown>
  if (typeof o.projectId !== 'string' || o.projectId === '' || typeof o.engine !== 'string') {
    return null
  }
  // Why: proto3 zero values (empty string, false, 0) may be left out of the JSON.
  const minVersion = o.openspecMinVersion ?? ''
  const solutionAgent = o.solutionAgent ?? false
  const version = o.version ?? 0
  if (
    typeof minVersion !== 'string' ||
    typeof solutionAgent !== 'boolean' ||
    typeof version !== 'number' ||
    !Number.isFinite(version)
  ) {
    return null
  }
  // Why: an empty mode means the project never chose one; the backend's own default is step by step.
  const mode =
    o.executionMode === undefined || o.executionMode === ''
      ? 'step_by_step'
      : executionModeOf(o.executionMode)
  if (!mode) {
    return null
  }
  return {
    projectId: o.projectId,
    engine: o.engine,
    openspecMinVersion: minVersion,
    solutionAgent,
    executionMode: mode,
    // Why: an older gateway does not send the flag; the review stays hidden rather than guessed.
    outcomeReviewEnabled: o.outcomeReviewEnabled === true,
    version
  }
}
