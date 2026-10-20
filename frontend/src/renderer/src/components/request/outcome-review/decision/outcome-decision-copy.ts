/**
 * Words of the decisions — FE-OR-T301..T312
 *
 * "Approved" says a person looked and decided; it never says the work is safe to release. The name of who decided is the viewer
 * ("you") or "a teammate": the screen has no directory of people to look an id up in.
 *
 * @module components/request/outcome-review/decision/outcome-decision-copy
 */

import { translate } from '@/i18n/i18n'
import type { CodeReviewAvailability } from './outcome-code-review-link'

const T = 'auto.components.request.outcomeReview.decision.'

export function whoText(isYou: boolean): string {
  return isYou ? translate(`${T}who.you`, 'you') : translate(`${T}who.teammate`, 'a teammate')
}

export function decidedText(decision: string, who: string): string {
  switch (decision) {
    case 'accept_exception':
      return translate(`${T}decided.accept`, 'Accepted as an exception by {{who}}', { who })
    case 'send_back':
      return translate(`${T}decided.sendBack`, 'Sent back by {{who}}', { who })
    case 'dismiss':
      return translate(`${T}decided.dismiss`, 'Claim dismissed by {{who}}', { who })
    default:
      return translate(`${T}decided.unknown`, 'Decided by {{who}}', { who })
  }
}

export function cardActionText(
  id: 'accept' | 'sendBack' | 'dismiss' | 'change' | 'cancel' | 'confirmAccept' | 'confirmDismiss'
): string {
  switch (id) {
    case 'accept':
      return translate(`${T}action.accept`, 'Accept as exception')
    case 'sendBack':
      return translate(`${T}action.sendBack`, 'Send back')
    case 'dismiss':
      return translate(`${T}action.dismiss`, 'Dismiss claim')
    case 'change':
      return translate(`${T}action.change`, 'Change decision')
    case 'cancel':
      return translate(`${T}action.cancel`, 'Cancel')
    case 'confirmAccept':
      return translate(`${T}action.confirmAccept`, 'Accept')
    default:
      return translate(`${T}action.confirmDismiss`, 'Dismiss')
  }
}

export function noteLabelText(required: boolean): string {
  return required
    ? translate(`${T}note.required`, 'Why (required)')
    : translate(`${T}note.optional`, 'Note (optional)')
}

export function noteErrorText(kind: 'required' | 'tooLong', max: number): string {
  return kind === 'required'
    ? translate(`${T}note.errorRequired`, 'A note is required.')
    : translate(`${T}note.errorTooLong`, 'At most {{max}} characters.', { max })
}

export function sentBackInText(): string {
  return translate(`${T}sentBackIn`, 'Sent back in a follow-up request')
}

export function openChildText(): string {
  return translate(`${T}openChild`, 'Open it')
}

export function barText(id: 'note' | 'approve' | 'requestChanges'): string {
  switch (id) {
    case 'note':
      return translate(`${T}bar.note`, 'Note for the approval (optional)')
    case 'approve':
      return translate(`${T}bar.approve`, 'Approve')
    default:
      return translate(`${T}bar.requestChanges`, 'Request changes')
  }
}

export function requestChangesCountText(count: number): string {
  return translate(`${T}bar.requestChangesCount`, 'Request changes ({{count}})', { count })
}

export function openDecisionsErrorText(count: number): string {
  return translate(`${T}bar.openDecisions`, 'Resolve {{count}} open decisions first', { count })
}

export function approvedText(who: string): string {
  return translate(
    `${T}bar.approved`,
    'Review approved by {{who}}. This is not a release sign-off.',
    { who }
  )
}

export function sendBackDialogText(
  id: 'title' | 'description' | 'noteLabel' | 'continue' | 'nothing' | 'working'
): string {
  switch (id) {
    case 'title':
      return translate(`${T}dialog.title`, 'Request changes')
    case 'description':
      return translate(
        `${T}dialog.description`,
        'Choose the findings to send back. A follow-up request is created with their text, and this request stays as it is.'
      )
    case 'noteLabel':
      return translate(`${T}dialog.noteLabel`, 'Note for the follow-up (optional)')
    case 'continue':
      return translate(`${T}dialog.continue`, 'Continue')
    case 'nothing':
      return translate(`${T}dialog.nothing`, 'Choose at least one finding.')
    default:
      return translate(`${T}dialog.working`, 'Recording…')
  }
}

export function followUpFinishedText(): string {
  return translate(
    `${T}followUpFinished`,
    'A follow-up Request finished. Regenerate this review to check.'
  )
}

export function codeReviewText(state: CodeReviewAvailability['state'] | 'open' | 'choose'): string {
  switch (state) {
    case 'unsupported':
      return translate(`${T}codeReview.unsupported`, 'Code review is not available here.')
    case 'no_worktree':
      return translate(
        `${T}codeReview.noWorktree`,
        'Code review is not available for this Request: the review has no worktree to open.'
      )
    case 'no_lens':
      return translate(`${T}codeReview.noLens`, 'This part has no view in the code review.')
    case 'choose':
      return translate(`${T}codeReview.choose`, 'Open which worktree?')
    default:
      return translate(`${T}codeReview.open`, 'Open in code review')
  }
}
