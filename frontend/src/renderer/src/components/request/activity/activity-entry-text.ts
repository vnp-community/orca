/**
 * activity-entry-text — FE-REQ-TASK-037-07
 *
 * The words of one activity line, built from `kind` and `details` (the backend `summary` is only the fallback).
 * Keys are spelled out in translate() calls, not a lookup table, so the locale scan finds them.
 *
 * @module components/request/activity/activity-entry-text
 */

import { translate } from '@/i18n/i18n'
import { getRequestTypePresentation } from '../request-status-presentation'
import type { RequestType } from '../../../../../shared/request-types'
import type {
  ActivityActorKind,
  RequestActivityEntry
} from '../../../../../shared/request-execution-types'
import {
  activityAttempt,
  activityTaskName,
  detailString,
  type ActivityTone
} from './activity-entry-presentation'

const T = 'auto.components.request.activity.'

function typeLabel(type: string | undefined): string {
  if (!type) {
    return translate(`${T}type.unknown`, 'unclassified')
  }
  return translate(getRequestTypePresentation(type as RequestType).labelKey, type)
}

function taskOrFallback(entry: RequestActivityEntry): string {
  return activityTaskName(entry) ?? translate(`${T}task.unnamed`, 'A task')
}

export function activityActorLabel(actor: ActivityActorKind): string {
  switch (actor) {
    case 'user':
      return translate(`${T}actor.user`, 'User')
    case 'ai':
      return translate(`${T}actor.ai`, 'AI')
    case 'agent':
      return translate(`${T}actor.agent`, 'Agent')
    default:
      return translate(`${T}actor.system`, 'System')
  }
}

export function activitySeverityLabel(tone: ActivityTone): string | null {
  if (tone === 'error') {
    return translate(`${T}severity.error`, 'Stopped')
  }
  if (tone === 'attention') {
    return translate(`${T}severity.attention`, 'Needs attention')
  }
  return null
}

export function activityFailureCauseLabel(cause: string): string {
  switch (cause) {
    case 'timed_out':
      return translate(`${T}cause.timedOut`, 'Timed out')
    case 'model_error':
      return translate(`${T}cause.modelError`, 'Model error')
    case 'verify_failed':
      return translate(`${T}cause.verifyFailed`, 'Verification failed')
    case 'env_defect':
      return translate(`${T}cause.envDefect`, 'Environment problem')
    case 'spec_defect':
      return translate(`${T}cause.specDefect`, 'Problem in the task spec')
    case 'agent_error':
      return translate(`${T}cause.agentError`, 'Agent error')
    case 'needs_info':
      return translate(`${T}cause.needsInfo`, 'Needed more information')
    case 'cancelled':
      return translate(`${T}cause.cancelled`, 'Cancelled')
    case 'base_ref_not_found':
      return translate(`${T}cause.baseRefNotFound`, 'Starting branch not found')
    case 'unknown':
      return translate(`${T}cause.unknown`, 'Unknown cause')
    default:
      return cause
  }
}

/** The short headline of a line. A kind this build does not know shows the backend summary, then the raw kind. */
export function activityTitle(entry: RequestActivityEntry): string {
  const d = entry.details
  switch (entry.kind) {
    case 'request.created':
      return translate(`${T}title.requestCreated`, 'Request created')
    case 'request.classified':
      return translate(`${T}title.requestClassified`, 'Classified as {{type}}', {
        type: typeLabel(detailString(d, 'to'))
      })
    case 'request.type_changed':
      return translate(`${T}title.typeChanged`, 'Type changed from {{from}} to {{to}}', {
        from: typeLabel(detailString(d, 'from')),
        to: typeLabel(detailString(d, 'to'))
      })
    case 'analysis.generated': {
      const kind = detailString(d, 'analysisKind') ?? translate(`${T}analysis.generic`, 'Analysis')
      const version = typeof d.version === 'number' ? d.version : undefined
      return version
        ? translate(
            `${T}title.analysisGeneratedVersion`,
            '{{kind}} version {{version}} generated',
            { kind, version }
          )
        : translate(`${T}title.analysisGenerated`, '{{kind}} generated', { kind })
    }
    // Why: the reviewer's text is never in the line (CONTRACT C9); it is read from the version it produced.
    case 'analysis.revision_requested':
      return translate(`${T}title.analysisRevisionRequested`, 'Requested changes to the analysis')
    case 'plan.revision_requested':
      return translate(`${T}title.planRevisionRequested`, 'Requested changes to the plan')
    case 'clarification.asked':
      return translate(`${T}title.clarificationAsked`, 'Clarification asked')
    case 'clarification.answered':
      return translate(`${T}title.clarificationAnswered`, 'Clarification answered')
    case 'approval.requested':
      return translate(`${T}title.approvalRequested`, 'Approval requested for the {{subject}}', {
        subject: entry.subjectType ?? translate(`${T}approval.subjectFallback`, 'item')
      })
    case 'approval.decided': {
      const subject = entry.subjectType ?? translate(`${T}approval.subjectFallback`, 'item')
      const result = detailString(d, 'result')
      if (result === 'approved') {
        return translate(`${T}title.approvalApproved`, 'The {{subject}} was approved', { subject })
      }
      if (result === 'rejected') {
        return translate(`${T}title.approvalRejected`, 'The {{subject}} was rejected', { subject })
      }
      return translate(`${T}title.approvalDecided`, 'Decision on the {{subject}}', { subject })
    }
    case 'phase.started':
      return detailString(d, 'trigger') === 'auto'
        ? translate(`${T}title.phaseStartedAuto`, 'Phase started automatically')
        : translate(`${T}title.phaseStarted`, 'Phase started')
    case 'phase.done':
      return translate(`${T}title.phaseDone`, 'Phase finished')
    case 'plan.done':
      return translate(`${T}title.planDone`, 'All tasks finished')
    case 'task.started':
      return translate(`${T}title.taskStarted`, '{{task}} started', { task: taskOrFallback(entry) })
    case 'task.succeeded':
      return translate(`${T}title.taskSucceeded`, '{{task}} finished', {
        task: taskOrFallback(entry)
      })
    case 'task.accepted':
      return translate(`${T}title.taskAccepted`, '{{task}} marked done', {
        task: taskOrFallback(entry)
      })
    case 'task.failed':
      return translate(`${T}title.taskFailed`, '{{task}} failed', { task: taskOrFallback(entry) })
    case 'task.cancelled':
      return translate(`${T}title.taskCancelled`, '{{task}} cancelled', {
        task: taskOrFallback(entry)
      })
    case 'request.returned':
      return translate(`${T}title.requestReturned`, 'Returned to backlog')
    case 'request.reopened':
      return translate(`${T}title.requestReopened`, 'Reopened')
    case 'request.cancelled':
      return translate(`${T}title.requestCancelled`, 'Cancelled')
    default:
      return entry.summary.trim() || entry.kind
  }
}

/** "Attempt 2 of 3", or "Attempt 2" when the backend did not say how many there are. */
export function activityAttemptLabel(entry: RequestActivityEntry): string | null {
  const attempt = activityAttempt(entry)
  if (!attempt) {
    return null
  }
  return attempt.maxAttempts
    ? translate(`${T}attemptOf`, 'Attempt {{attempt}} of {{max}}', {
        attempt: attempt.attempt,
        max: attempt.maxAttempts
      })
    : translate(`${T}attempt`, 'Attempt {{attempt}}', { attempt: attempt.attempt })
}

/** One line of context under the headline: the reason, comment or error. Plain text only; never the Request body. */
export function activityDetailText(entry: RequestActivityEntry): string | null {
  const d = entry.details
  switch (entry.kind) {
    case 'task.failed': {
      const cause = detailString(d, 'cause')
      const error = detailString(d, 'error')
      const causeText = cause ? activityFailureCauseLabel(cause) : null
      if (causeText && error) {
        return translate(`${T}detail.failure`, '{{cause}}: {{error}}', { cause: causeText, error })
      }
      return causeText ?? error ?? null
    }
    case 'approval.decided':
      return detailString(d, 'comment') ?? null
    case 'request.classified':
    case 'request.type_changed':
    case 'request.returned':
    case 'request.reopened':
    case 'request.cancelled':
      return detailString(d, 'reason') ?? null
    default:
      return null
  }
}
