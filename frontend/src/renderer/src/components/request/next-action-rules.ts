/**
 * next-action-rules.ts — FE-REQ-TASK-037-06
 *
 * The one thing a viewer should look at next, picked by priority: a failed task, something that waits for them, work that is
 * running, work that waits for something else, done. One pure function so the order is easy to change.
 *
 * @module components/request/next-action-rules
 */

import { translate } from '@/i18n/i18n'
import type { ExecutionSummary } from '../../../../shared/request-execution-types'
import type { OrcaRequest } from '../../../../shared/request-types'

const T = 'auto.components.request.NextAction.'

export type NextActionKind = 'failed' | 'needs_you' | 'running' | 'waiting' | 'done' | 'none'
export type DetailTabId = 'overview' | 'analysis' | 'plan' | 'execution' | 'outcome-review'
export type NextActionCommand = 'open_tab' | 'reopen' | 'answer'

export type NextAction = {
  kind: NextActionKind
  title: string
  detail?: string
  /** The tab that holds the thing; the bar's button goes there. */
  tab?: DetailTabId
  primary?: { label: string; command: NextActionCommand }
  /** When the button goes to another tab than `tab` (a soft hint about the Outcome Review beside the main message). */
  primaryTab?: DetailTabId
}

export type NextActionInput = {
  request: Pick<OrcaRequest, 'status' | 'type'>
  summary: ExecutionSummary | null
  /** The open Clarification can be answered by this viewer. */
  clarificationAnswerable: boolean
  /** Which tabs this request type has. */
  tabs: { analysis: boolean; plan: boolean; execution: boolean }
}

const NONE: NextAction = { kind: 'none', title: '' }

function approvalTab(status: OrcaRequest['status'], tabs: NextActionInput['tabs']): DetailTabId {
  if (status === 'awaiting_analysis_approval' && tabs.analysis) {
    return 'analysis'
  }
  if (status === 'awaiting_plan_approval') {
    // Why: a single-task flow (hotfix) has no Plan tab; its pre-deploy card lives in Execution.
    return tabs.plan ? 'plan' : 'execution'
  }
  return 'overview'
}

export function pickNextAction({
  request,
  summary,
  clarificationAnswerable,
  tabs
}: NextActionInput): NextAction {
  const { status } = request
  const failed = summary?.failed.find((f) => f.attempts >= f.maxAttempts) ?? summary?.failed[0]

  if (status === 'request_backlog' && failed) {
    return {
      kind: 'failed',
      title: translate(`${T}failed`, '“{{task}}” failed {{attempts}}/{{max}} times', {
        task: failed.title,
        attempts: failed.attempts,
        max: failed.maxAttempts
      }),
      detail: translate(
        `${T}failedDetail`,
        'The request is back in the backlog. Everything before it is kept.'
      ),
      tab: 'execution',
      primary: { label: translate(`${T}reopen`, 'Reopen and rerun'), command: 'reopen' }
    }
  }

  if (status === 'awaiting_information' && clarificationAnswerable) {
    return {
      kind: 'needs_you',
      title: translate(`${T}question`, 'A question is waiting for your answer'),
      primary: { label: translate(`${T}answer`, 'Answer'), command: 'answer' }
    }
  }

  const first = summary?.needsYou.find((n) => n.kind !== 'failed_task')
  if (first) {
    if (first.kind === 'clarification') {
      return {
        kind: 'needs_you',
        title: translate(`${T}question`, 'A question is waiting for your answer'),
        primary: { label: translate(`${T}answer`, 'Answer'), command: 'answer' }
      }
    }
    const tab =
      status === 'executing' || first.kind === 'drift' ? 'execution' : approvalTab(status, tabs)
    return {
      kind: 'needs_you',
      title:
        first.kind === 'pre_deploy'
          ? translate(`${T}preDeploy`, 'Pre-deploy approval is waiting for you')
          : first.kind === 'drift'
            ? translate(`${T}drift`, 'A change drifted from the plan and needs your review')
            : translate(`${T}approval`, 'An approval is waiting for you'),
      tab,
      primary: { label: translate(`${T}review`, 'Review'), command: 'open_tab' }
    }
  }

  if (status === 'awaiting_type_confirmation') {
    return {
      kind: 'needs_you',
      title: translate(`${T}confirmType`, 'Confirm the type of this request'),
      tab: 'overview'
    }
  }

  if (status === 'executing') {
    const running = summary?.running[0]
    if (running) {
      return {
        kind: 'running',
        title: translate(`${T}running`, 'The agent is working on “{{task}}”', {
          task: running.title
        }),
        detail: translate(
          `${T}runningDetail`,
          'Attempt {{attempt}} of {{max}}. Nothing needs you right now.',
          { attempt: running.attempt, max: running.maxAttempts }
        ),
        tab: 'execution'
      }
    }
    return {
      kind: 'waiting',
      title: translate(`${T}nextTask`, 'Waiting for the next task to start'),
      tab: 'execution'
    }
  }

  switch (status) {
    case 'classifying':
      return {
        kind: 'waiting',
        title: translate(`${T}classifying`, 'Orca is classifying this request')
      }
    case 'analyzing':
      return {
        kind: 'waiting',
        title: translate(`${T}analyzing`, 'Orca is analysing this request'),
        tab: tabs.analysis ? 'analysis' : undefined
      }
    case 'planning':
      return {
        kind: 'waiting',
        title: translate(`${T}planning`, 'Orca is writing the plan'),
        tab: tabs.plan ? 'plan' : undefined
      }
    case 'awaiting_analysis_approval':
    case 'awaiting_plan_approval':
      return {
        kind: 'waiting',
        title: translate(`${T}waitingApproval`, 'Waiting for someone to approve'),
        tab: approvalTab(status, tabs)
      }
    case 'awaiting_information':
      return {
        kind: 'waiting',
        title: translate(`${T}waitingInfo`, 'Waiting for someone to answer a question')
      }
    case 'completed':
      return {
        kind: 'done',
        title: translate(`${T}done`, 'This request is done'),
        tab: tabs.execution ? 'execution' : undefined
      }
    default:
      return NONE
  }
}
