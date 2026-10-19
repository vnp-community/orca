/**
 * execution-failure-cause.ts — FE-REQ-TASK-037-05
 *
 * The words for why a task attempt ended, and what the viewer can do about it. The cause is structured by the backend; the raw
 * last line is only shown as extra detail.
 *
 * @module components/request/execution/execution-failure-cause
 */

import { translate } from '@/i18n/i18n'
import type { FailureCause } from '../../../../../shared/request-execution-types'

const C = 'auto.components.request.execution.cause.'

export function failureCauseLabel(cause: FailureCause): string {
  switch (cause) {
    case 'timed_out':
      return translate(`${C}timedOut`, 'Timed out')
    case 'model_error':
      return translate(`${C}modelError`, 'Model error')
    case 'verify_failed':
      return translate(`${C}verifyFailed`, 'Verification failed')
    case 'env_defect':
      return translate(`${C}envDefect`, 'Environment problem')
    case 'spec_defect':
      return translate(`${C}specDefect`, 'Problem in the task spec')
    case 'agent_error':
      return translate(`${C}agentError`, 'Agent error')
    case 'needs_info':
      return translate(`${C}needsInfo`, 'Needs information')
    case 'cancelled':
      return translate(`${C}cancelled`, 'Cancelled')
    case 'base_ref_not_found':
      return translate(`${C}baseRefNotFound`, 'Starting branch not found')
    default:
      return translate(`${C}unknown`, 'Unknown cause')
  }
}

/** One line on what to try next for this cause. */
export function failureCauseAdvice(cause: FailureCause): string {
  switch (cause) {
    case 'timed_out':
      return translate(
        `${C}adviceTimedOut`,
        'The agent ran out of time. Rerunning usually helps; if it repeats, split the task.'
      )
    case 'model_error':
      return translate(
        `${C}adviceModelError`,
        'The model could not be used. Check the agent and model settings, then rerun.'
      )
    case 'verify_failed':
      return translate(
        `${C}adviceVerifyFailed`,
        'The work did not pass its checks. Read the log, then rerun.'
      )
    case 'env_defect':
      return translate(
        `${C}adviceEnvDefect`,
        'The workspace or tools are broken. Fix the environment, then rerun.'
      )
    case 'spec_defect':
      return translate(
        `${C}adviceSpecDefect`,
        'The task spec is wrong or incomplete. Change the plan before rerunning.'
      )
    case 'agent_error':
      return translate(
        `${C}adviceAgentError`,
        'The agent stopped without a usable result. Read the log, then rerun.'
      )
    case 'needs_info':
      return translate(
        `${C}adviceNeedsInfo`,
        'The agent needs an answer from you before it can continue.'
      )
    case 'base_ref_not_found':
      return translate(
        `${C}adviceBaseRefNotFound`,
        'The branch this request starts from no longer exists in the repository. Choose another starting branch, then rerun.'
      )
    default:
      return translate(`${C}adviceUnknown`, 'Read the log to see what happened, then rerun.')
  }
}

const MAX_ERROR_CHARS = 240

/** The last error line as plain, shortened text. */
export function shortError(text: string | undefined): string | undefined {
  const t = text?.trim()
  if (!t) {
    return undefined
  }
  return t.length > MAX_ERROR_CHARS ? `${t.slice(0, MAX_ERROR_CHARS)}…` : t
}
