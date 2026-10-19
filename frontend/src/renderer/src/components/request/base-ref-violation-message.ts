/**
 * base-ref-violation-message.ts — FE-REQ-TASK-040-03
 *
 * Words for the first rule a typed branch name breaks (`validateBaseRefSyntax`). Each rule names what is wrong and never
 * echoes the whole value, mirroring the backend which does not repeat it either.
 *
 * @module components/request/base-ref-violation-message
 */

import { translate } from '@/i18n/i18n'
import type { BaseRefSyntaxResult } from '../../../../shared/request-base-ref'

const V = 'auto.components.request.BaseRefViolation.'

export function baseRefViolationMessage(
  result: Extract<BaseRefSyntaxResult, { ok: false }>
): string {
  switch (result.reason) {
    case 'blank':
      return translate(`${V}blank`, 'Enter a branch or tag name.')
    case 'too_long':
      return translate(`${V}tooLong`, 'This name is too long. Use 255 bytes or fewer.')
    case 'invalid_utf8':
      return translate(`${V}invalidUtf8`, 'This name contains characters that cannot be stored.')
    case 'leading_dash':
      return translate(`${V}leadingDash`, 'A branch name cannot start with a dash.')
    case 'bare_at':
      return translate(`${V}bareAt`, 'A branch name cannot be only {{token}}.', { token: '@' })
    case 'nul_byte':
      return translate(`${V}nulByte`, 'A branch name cannot contain a null character.')
    case 'control_character':
      return translate(`${V}controlCharacter`, 'A branch name cannot contain control characters.')
    case 'whitespace':
      return translate(`${V}whitespace`, 'A branch name cannot contain spaces.')
    case 'forbidden_character':
      return translate(`${V}forbiddenCharacter`, 'A branch name cannot contain {{token}}.', {
        token: result.character ?? ''
      })
    case 'double_dot':
      return translate(`${V}doubleDot`, 'A branch name cannot contain {{token}}.', { token: '..' })
    case 'reflog_syntax':
      return translate(`${V}reflogSyntax`, 'A branch name cannot contain {{token}}.', {
        token: '@{'
      })
    case 'double_slash':
      return translate(`${V}doubleSlash`, 'A branch name cannot contain {{token}}.', {
        token: '//'
      })
    case 'leading_slash':
      return translate(`${V}leadingSlash`, 'A branch name cannot start with {{token}}.', {
        token: '/'
      })
    case 'trailing_slash':
      return translate(`${V}trailingSlash`, 'A branch name cannot end with {{token}}.', {
        token: '/'
      })
    case 'trailing_dot':
      return translate(`${V}trailingDot`, 'A branch name cannot end with {{token}}.', {
        token: '.'
      })
    case 'dot_component':
      return translate(`${V}dotComponent`, 'No part of a branch name can start with a dot.')
    case 'lock_component':
      return translate(`${V}lockComponent`, 'No part of a branch name can end with {{token}}.', {
        token: '.lock'
      })
  }
}
