/**
 * request-error-message.ts — CR-REQ-018-05
 *
 * Maps a RequestRpcErrorKind (as stored by the request hooks) to user-facing
 * copy. Hooks never toast; components call this to render inline errors.
 * `requestErrorMessageForError` adds code-specific copy where one sentence per
 * kind would hide the action the user needs (AI setup, throttling, flow off).
 *
 * @module components/request/request-error-message
 */

import { translate } from '@/i18n/i18n'
import type { RequestRpcError, RequestRpcErrorKind } from '../../../../shared/request-errors'

type Copy = { key: string; fallback: string }

// Why: typed over the full union so adding a kind without copy fails the type check.
const ERROR_COPY: Record<RequestRpcErrorKind, Copy> = {
  forbidden: {
    key: 'auto.components.request.error.forbidden',
    fallback: 'You do not have permission to do this.'
  },
  not_found: {
    key: 'auto.components.request.error.notFound',
    fallback: 'This item no longer exists.'
  },
  conflict: {
    key: 'auto.components.request.error.conflict',
    fallback: 'Someone else changed this. Refresh and try again.'
  },
  invalid_state: {
    key: 'auto.components.request.error.invalidState',
    fallback: 'This action is not allowed in the current state.'
  },
  blocking_questions: {
    key: 'auto.components.request.error.blockingQuestions',
    fallback:
      'This cannot be approved yet: the analysis has blocking open questions that still need an answer.'
  },
  unconfirmed_assumptions: {
    key: 'auto.components.request.error.unconfirmedAssumptions',
    fallback:
      'This plan cannot be approved yet: it has assumptions that need confirmation and still have no recorded answer.'
  },
  validation: {
    key: 'auto.components.request.error.validation',
    fallback: 'Check the highlighted fields and try again.'
  },
  network: {
    key: 'auto.components.request.error.network',
    fallback: 'Network error. Check your connection.'
  },
  unavailable: {
    key: 'auto.components.request.error.unavailable',
    fallback: 'The request service is unavailable. Try again shortly.'
  },
  rate_limited: {
    key: 'auto.components.request.error.rateLimited',
    fallback: 'Too many attempts. Wait a moment and retry.'
  },
  unsupported: {
    key: 'auto.components.request.error.unsupported',
    fallback: 'This server does not support this action yet.'
  },
  expired: { key: 'auto.components.request.error.expired', fallback: 'This item has expired.' },
  pending: {
    key: 'auto.components.request.error.pending',
    fallback: 'This is still being prepared. Try again shortly.'
  },
  no_dev_server: {
    key: 'auto.components.request.error.noDevServer',
    fallback: 'No dev server is connected to this project. Connect one and try again.'
  },
  no_ai: {
    key: 'auto.components.request.error.noAi',
    fallback: 'AI is not available on the dev server. Check its AI configuration.'
  },
  feature_off: {
    key: 'auto.components.request.error.featureOff',
    fallback: 'The Request flow is not enabled for this organization.'
  },
  unknown: { key: 'auto.components.request.error.unknown', fallback: 'Something went wrong.' }
}

const CODE_COPY: Record<string, Copy> = {
  REQUEST_RATE_LIMITED: {
    key: 'auto.components.request.error.code.rateLimited',
    fallback: 'You are going too fast. Wait a moment and try again.'
  },
  REQUEST_PROJECT_NOT_FOUND: {
    key: 'auto.components.request.error.code.projectNotFound',
    fallback: 'This project no longer exists.'
  },
  REQUEST_AI_QUOTA_EXHAUSTED: {
    key: 'auto.components.request.error.code.aiQuotaExhausted',
    fallback: 'The AI quota of the dev server is used up. Try again later.'
  },
  REQUEST_AI_BUSY: {
    key: 'auto.components.request.error.code.aiBusy',
    fallback: 'The AI is busy with another job. Try again shortly.'
  },
  REQUEST_AI_COMPLETE_TIMEOUT: {
    key: 'auto.components.request.error.code.aiTimeout',
    fallback: 'The AI took too long to answer. Try again.'
  },
  REQUEST_AI_AGENT_UNAVAILABLE: {
    key: 'auto.components.request.error.code.aiAgentUnavailable',
    fallback: 'No dev server is connected, so the AI cannot run.'
  }
}

function render({ key, fallback }: Copy): string {
  return translate(key, fallback)
}

export function requestErrorMessage(kind: string | null | undefined): string {
  const entry =
    (kind && (ERROR_COPY as Record<string, Copy | undefined>)[kind]) || ERROR_COPY.unknown
  return render(entry)
}

/** Message for a full error: code-specific copy when one exists, else the kind's copy. */
export function requestErrorMessageForError(
  error: Pick<RequestRpcError, 'kind'> & Partial<Pick<RequestRpcError, 'backendCode' | 'code'>>
): string {
  const code = (error.backendCode ?? error.code ?? '').toUpperCase()
  const byCode = CODE_COPY[code]
  return byCode ? render(byCode) : requestErrorMessage(error.kind)
}
