/**
 * request-error-message.ts — CR-REQ-018-05
 *
 * Maps a RequestRpcErrorKind (as stored by the request hooks) to user-facing
 * copy. Hooks never toast; components call this to render inline errors.
 *
 * @module components/request/request-error-message
 */

import { translate } from '@/i18n/i18n'

const ERROR_COPY: Record<string, { key: string; fallback: string }> = {
  forbidden: { key: 'error.forbidden', fallback: 'You do not have permission to do this.' },
  not_found: { key: 'error.notFound', fallback: 'This item no longer exists.' },
  conflict: { key: 'error.conflict', fallback: 'Someone else changed this. Refresh and try again.' },
  invalid_state: { key: 'error.invalidState', fallback: 'This action is not allowed in the current state.' },
  validation: { key: 'error.validation', fallback: 'Check the highlighted fields and try again.' },
  network: { key: 'error.network', fallback: 'Network error. Check your connection.' },
  unavailable: { key: 'error.unavailable', fallback: 'The request service is unavailable. Try again shortly.' },
  rate_limited: { key: 'error.rateLimited', fallback: 'Too many attempts. Wait a moment and retry.' }
}

export function requestErrorMessage(kind: string | null | undefined): string {
  const entry = (kind && ERROR_COPY[kind]) || { key: 'error.unknown', fallback: 'Something went wrong.' }
  return translate(`auto.components.request.${entry.key}`, entry.fallback)
}
