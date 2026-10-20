/**
 * Outcome Review state words — FE-OR-T123, T124
 *
 * Banners, invitations and the sentences for the `REQUEST_REVIEW_*` codes. A code the screen does not know gets the general
 * sentence, never the raw server text.
 *
 * @module components/request/outcome-review/outcome-review-state-copy
 */

import { translate } from '@/i18n/i18n'
import type { ReviewBannerId } from './outcome-review-view-model'

const T = 'auto.components.request.outcomeReview.'

export function bannerText(
  id: ReviewBannerId,
  params: { percent?: number; reason?: string } = {}
): string {
  switch (id) {
    case 'design_phase':
      return translate(
        `${T}banner.designPhase`,
        'Design review. The after-code view appears when execution completes.'
      )
    case 'execution_finished':
      return translate(
        `${T}banner.executionFinished`,
        'Execution finished. Generate the after-code review.'
      )
    case 'stale':
      return translate(`${T}banner.stale`, 'This review was made before the Request changed.')
    case 'design_not_reviewed':
      return translate(
        `${T}banner.designNotReviewed`,
        'Design not reviewed: {{percent}}% of the design elements read are approved. Findings based on them are advisory.',
        { percent: params.percent ?? 0 }
      )
    case 'kg_unavailable':
      return translate(
        `${T}banner.kgUnavailable`,
        'Design knowledge was not available, so architecture checks are limited.'
      )
    case 'truncated':
      return translate(`${T}banner.truncated`, 'The review was cut to fit its size limit.')
    case 'run_failed':
      return translate(
        `${T}banner.runFailed`,
        'The last agent run failed ({{reason}}). The facts Orca measured are shown.',
        { reason: params.reason ?? '' }
      )
    default:
      return translate(`${T}banner.drawingFailed`, 'Drawing failed, showing the table.')
  }
}

/** One sentence for a `REQUEST_REVIEW_*` code (the part after the prefix), or the general one. */
export function errorText(code: string | undefined): string {
  switch (code) {
    case 'REQUEST_REVIEW_DISABLED':
      return translate(`${T}error.disabled`, 'The Outcome review is turned off for this project.')
    case 'REQUEST_REVIEW_NOT_READY':
      return translate(`${T}error.notReady`, 'The review opens once the Request has a plan.')
    case 'REQUEST_REVIEW_AGENT_TOO_OLD':
      return translate(
        `${T}error.agentTooOld`,
        "The dev server's agent is too old for the review. Update it and try again."
      )
    case 'REQUEST_REVIEW_SKILL_MISSING':
      return translate(
        `${T}error.skillMissing`,
        'A skill the review needs is not installed on the dev server.'
      )
    case 'REQUEST_REVIEW_NO_DEV_SERVER':
      return translate(`${T}error.noDevServer`, 'No dev server is connected for this project.')
    case 'REQUEST_REVIEW_KNOWLEDGE_REQUIRED':
      return translate(
        `${T}error.knowledgeRequired`,
        "The design review needs the project's knowledge graph, which could not be loaded."
      )
    case 'REQUEST_REVIEW_RESULT_INVALID':
      return translate(
        `${T}error.resultInvalid`,
        "The agent's answer could not be used. Try again."
      )
    case 'REQUEST_REVIEW_RUN_IN_PROGRESS':
      return translate(
        `${T}error.runInProgress`,
        'A review run is already going. Wait for it to finish.'
      )
    case 'REQUEST_REVIEW_STALE_REVISION':
      return translate(
        `${T}error.staleRevision`,
        'Someone changed this review. It was reloaded; try again.'
      )
    case 'REQUEST_REVIEW_DECISIONS_OPEN':
      return translate(`${T}error.decisionsOpen`, 'Some decisions are still open.')
    case 'REQUEST_REVIEW_DECISION_INVALID':
      return translate(`${T}error.decisionInvalid`, 'That decision cannot be recorded.')
    case 'REQUEST_FORBIDDEN':
    case 'REQUEST_REVIEW_FORBIDDEN':
      return translate(
        `${T}error.forbidden`,
        'Only the person who reported the Request, or an administrator, can do this.'
      )
    case 'REQUEST_REVIEW_CONFIRM_COST_REQUIRED':
      return translate(`${T}error.confirmCost`, 'This costs more than the limit. Confirm to go on.')
    case 'REQUEST_REVIEW_BUDGET_EXCEEDED':
      return translate(`${T}error.budgetExceeded`, 'The AI budget for reviews is used up.')
    case 'REQUEST_REVIEW_DUPLICATE_REQUEST':
      return translate(
        `${T}error.duplicate`,
        'The same request was sent twice; the first one is being handled.'
      )
    case 'REQUEST_REVIEW_NOT_FOUND':
    case 'REQUEST_REVIEW_ARTIFACT_NOT_FOUND':
      return translate(`${T}error.notFound`, 'The review was not found. It may have been replaced.')
    default:
      return translate(`${T}error.general`, 'The review could not be loaded. Try again.')
  }
}

/** Why a context run could not start or did not finish, in a few words for the banner. */
export function reasonText(code: string | undefined): string {
  return errorText(code)
}

export function loadingText(): string {
  return translate(`${T}state.loading`, 'Loading the review…')
}

export function generatingText(phase: 'design' | 'delivered'): string {
  return phase === 'design'
    ? translate(`${T}state.generatingDesign`, 'Reading the design…')
    : translate(`${T}state.generatingDelivered`, 'Reading the design and the code…')
}

export function unavailableText(): string {
  return translate(`${T}state.unavailable`, 'The Outcome review is not available here.')
}

export function decisionsLaterText(): string {
  return translate(`${T}state.decisionsLater`, 'Decisions open after execution completes.')
}

export function inviteTitleText(kind: 'design' | 'delivered'): string {
  return kind === 'design'
    ? translate(`${T}invite.designTitle`, 'Review the design')
    : translate(`${T}invite.deliveredTitle`, 'Execution finished. Generate the after-code review.')
}

export function inviteBodyText(kind: 'design' | 'delivered'): string {
  return kind === 'design'
    ? translate(
        `${T}invite.designBody`,
        "See how this Request changes the architecture and the flows, from the project's design, before any code exists."
      )
    : translate(
        `${T}invite.deliveredBody`,
        'The design part is already read, so only the code needs reading. Drawings of the design are reused.'
      )
}

export function inviteActionText(kind: 'design' | 'delivered'): string {
  return kind === 'design'
    ? translate(`${T}invite.generateDesign`, 'Generate review')
    : translate(`${T}invite.generateDelivered`, 'Generate after-code review')
}

export function factsOnlyText(): string {
  return translate(`${T}invite.factsOnly`, 'Facts only, no agent run')
}

/** A reason the estimate gives for a run that cannot start: a lowercase tag of the server, or a REQUEST_REVIEW_ code. */
export function gapText(gap: string): string {
  switch (gap) {
    case 'no_dev_server':
      return translate(`${T}gap.noDevServer`, 'no dev server is connected')
    case 'no_repo_path':
      return translate(
        `${T}gap.noRepoPath`,
        "the project's repository has no folder on its dev server"
      )
    case 'kg_unavailable':
      return translate(`${T}gap.kgUnavailable`, "the project's knowledge graph is not set up")
    default:
      return gap.startsWith('REQUEST_REVIEW_')
        ? errorText(gap)
        : translate(`${T}gap.other`, 'a requirement is not met')
  }
}

export function estimateLines(est: {
  contextCached: boolean
  contextRunnable: boolean
  designLocked: boolean
  gaps: string[]
}): string[] {
  const lines: string[] = []
  if (est.contextCached) {
    lines.push(
      translate(`${T}estimate.cached`, 'A checked result is kept, so no new agent run is needed.')
    )
  } else if (est.contextRunnable) {
    lines.push(
      est.designLocked
        ? translate(`${T}estimate.codeRun`, 'Reading the code takes one agent run.')
        : translate(`${T}estimate.oneRun`, 'Reading the design takes one agent run.')
    )
  } else {
    lines.push(
      translate(`${T}estimate.cannotRun`, 'The agent run cannot start yet: {{gaps}}.', {
        gaps: est.gaps.map(gapText).join('; ')
      })
    )
  }
  lines.push(
    translate(
      `${T}estimate.drawings`,
      'Diagrams are drawn after the review is built, when the dev server can draw.'
    )
  )
  lines.push(translate(`${T}estimate.noCostFigures`, 'No cost figures have been measured yet.'))
  return lines
}
