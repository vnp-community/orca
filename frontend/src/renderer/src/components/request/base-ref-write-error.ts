/**
 * base-ref-write-error.ts — FE-REQ-TASK-040-03/04
 *
 * Turns a failed `request.create` / `request.setBaseRef` into what the base-branch UI can act on: a typed failure, a
 * localized sentence, the gateway's branch suggestions and the lock reason. The machine-readable part rides in a
 * ` {json}` trailer of the error message (CONTRACT 2.11), so it is read here and never shown raw.
 *
 * @module components/request/base-ref-write-error
 */

import { translate } from '@/i18n/i18n'
import { requestErrorMessageForError } from './request-error-message'
import type { RequestRpcError } from '../../../../shared/request-errors'
import type { RequestBaseRefLockReason } from '../../../../shared/request-types'

const E = 'auto.components.request.BaseRefError.'
const MAX_SUGGESTIONS = 5

export type BaseRefWriteFailure =
  | { kind: 'invalid'; message: string }
  | { kind: 'notFound'; message: string; suggestions: string[] }
  | { kind: 'locked'; message: string; reason: RequestBaseRefLockReason | undefined }
  | { kind: 'conflict'; message: string }
  | { kind: 'forbidden'; message: string }
  | { kind: 'other'; message: string }

const LOCK_REASONS: ReadonlySet<string> = new Set(['proposal', 'task', 'execution', 'terminal'])

/** The `{json}` object a gateway appends after the human text; null when absent or not an object. */
function errorTrailer(message: string): Record<string, unknown> | null {
  const start = message.lastIndexOf(' {')
  if (start < 0) {
    return null
  }
  try {
    const parsed: unknown = JSON.parse(message.slice(start + 1))
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null
  } catch {
    return null
  }
}

/** Suggested refs of REQUEST_BASE_REF_NOT_FOUND; empty when the trailer is missing or malformed. */
export function baseRefSuggestionsOf(message: string): string[] {
  const list = errorTrailer(message)?.suggestions
  if (!Array.isArray(list)) {
    return []
  }
  return list
    .filter((item): item is string => typeof item === 'string' && item.trim() !== '')
    .slice(0, MAX_SUGGESTIONS)
}

/** Lock reason of REQUEST_BASE_REF_LOCKED; a reason this build does not know is still a lock ('unknown'). */
export function baseRefLockReasonOf(message: string): RequestBaseRefLockReason | undefined {
  const reason = errorTrailer(message)?.reason
  if (typeof reason !== 'string' || reason === '') {
    return undefined
  }
  return LOCK_REASONS.has(reason) ? (reason as RequestBaseRefLockReason) : 'unknown'
}

/** Why the branch can no longer be changed; also the tooltip of the lock on the Overview row. */
export function baseRefLockReasonText(reason: RequestBaseRefLockReason | undefined): string {
  switch (reason) {
    case 'proposal':
      return translate(
        `${E}lockProposal`,
        'The analysis already created a worktree from this branch, so it can no longer be changed.'
      )
    case 'task':
      return translate(
        `${E}lockTask`,
        'A task worktree already exists for this request, so the branch can no longer be changed.'
      )
    case 'execution':
      return translate(
        `${E}lockExecution`,
        'Execution has started, so the branch can no longer be changed.'
      )
    case 'terminal':
      return translate(
        `${E}lockTerminal`,
        'This request is finished, so the branch can no longer be changed.'
      )
    case 'unknown':
    case '':
    case undefined:
      return translate(
        `${E}lockGeneric`,
        'Worktrees already exist for this request, so the branch can no longer be changed.'
      )
  }
}

function codeOf(error: Pick<RequestRpcError, 'backendCode' | 'code'>): string {
  return (error.backendCode ?? error.code ?? '').toUpperCase()
}

export function classifyBaseRefWriteError(error: RequestRpcError): BaseRefWriteFailure {
  const code = codeOf(error)
  switch (code) {
    case 'REQUEST_BASE_REF_INVALID':
      return {
        kind: 'invalid',
        message: translate(
          `${E}invalid`,
          'This is not a valid branch or tag name. Check it and try again.'
        )
      }
    case 'REQUEST_BASE_REF_NOT_FOUND':
      return {
        kind: 'notFound',
        message: translate(
          `${E}notFound`,
          'This branch or tag was not found in the repository. Pick another one.'
        ),
        suggestions: baseRefSuggestionsOf(error.message)
      }
    case 'REQUEST_BASE_REF_LOCKED': {
      const reason = baseRefLockReasonOf(error.message)
      return { kind: 'locked', message: baseRefLockReasonText(reason), reason }
    }
    case 'REQUEST_BASE_REF_CHECK_NOT_CONFIGURED':
      return {
        kind: 'other',
        message: translate(
          `${E}checkNotConfigured`,
          'This server cannot check branches yet, so the branch was not saved. Ask an administrator.'
        )
      }
    case 'REQUEST_ACTOR_NOT_ALLOWED':
      return { kind: 'forbidden', message: requestErrorMessageForError({ kind: 'forbidden' }) }
    default:
      break
  }
  if (error.kind === 'conflict') {
    return { kind: 'conflict', message: requestErrorMessageForError(error) }
  }
  if (error.kind === 'forbidden') {
    return { kind: 'forbidden', message: requestErrorMessageForError(error) }
  }
  return { kind: 'other', message: requestErrorMessageForError(error) }
}
