import type { McpScopeId } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'

export const EXPIRY_PRESET_DAYS = [7, 30, 60, 90] as const
export const TOKEN_NAME_MAX = 80

export function expiryOptions(maxTokenDays: number): number[] {
  if (!Number.isFinite(maxTokenDays) || maxTokenDays <= 0) {
    return []
  }
  const opts: number[] = EXPIRY_PRESET_DAYS.filter((d) => d <= maxTokenDays)
  if (maxTokenDays < 90 && !opts.includes(maxTokenDays)) {
    opts.push(maxTokenDays)
  }
  return opts.sort((a, b) => a - b)
}

export function defaultExpiryDays(maxTokenDays: number): number {
  return Math.min(30, maxTokenDays)
}

export type TokenFormInput = {
  name: string
  scopes: McpScopeId[]
  expiresInDays: number
}
export type TokenFormErrors = {
  name?: string
  scopes?: string
  expiresInDays?: string
}

export function validateTokenForm(i: TokenFormInput, maxTokenDays: number): TokenFormErrors {
  const errors: TokenFormErrors = {}
  const name = i.name.trim()
  if (name.length < 1 || name.length > TOKEN_NAME_MAX) {
    errors.name = translate('auto.mcp.tokens.errName', 'Enter a name up to 80 characters.')
  }
  if (i.scopes.length === 0) {
    errors.scopes = translate('auto.mcp.tokens.errScopes', 'Select at least one permission.')
  }
  if (!Number.isInteger(i.expiresInDays) || i.expiresInDays < 1 || i.expiresInDays > maxTokenDays) {
    errors.expiresInDays = translate(
      'auto.mcp.tokens.errTooLong',
      'Maximum lifetime is {{days}} days.',
      { days: maxTokenDays }
    )
  }
  return errors
}

export const hasFormErrors = (e: TokenFormErrors): boolean => Object.keys(e).length > 0

// Why: only admins may request orca:admin; the server still enforces it (MCP_SCOPE_NOT_ALLOWED).
export function isScopeSelectable(id: string, role: string | undefined): boolean {
  return id !== 'orca:admin' || role === 'admin'
}

export type CreateErrorEffect = {
  field?: keyof TokenFormErrors
  message: string
  /** Banner-level error disables further creates. */
  banner?: boolean
  disableCreate?: boolean
  refreshServerInfo?: boolean
}

export function mapCreateError(
  code: string | null,
  detail: string,
  maxTokenDays: number
): CreateErrorEffect {
  switch (code) {
    case 'MCP_TOKEN_TOO_LONG':
      return {
        field: 'expiresInDays',
        message: translate('auto.mcp.tokens.errTooLong', 'Maximum lifetime is {{days}} days.', {
          days: maxTokenDays
        }),
        refreshServerInfo: true
      }
    case 'MCP_SCOPE_NOT_ALLOWED':
      return {
        field: 'scopes',
        message: translate(
          'auto.mcp.tokens.errScopeNotAllowed',
          "Your role can't grant one or more of these permissions."
        )
      }
    case 'MCP_SCOPE_INVALID':
      return {
        field: 'scopes',
        message: translate(
          'auto.mcp.tokens.errScopeInvalid',
          'Select at least one valid permission.'
        )
      }
    case 'MCP_TOKEN_LIMIT':
      return {
        banner: true,
        message: translate(
          'auto.mcp.tokens.errLimit',
          "You've reached the maximum number of active tokens. Revoke one, then try again."
        )
      }
    case 'MCP_KILL_SWITCH_ACTIVE':
    case 'MCP_DISABLED':
      return {
        banner: true,
        disableCreate: true,
        message: translate(
          'auto.mcp.tokens.errDisabled',
          'MCP access is currently disabled for this organization.'
        )
      }
    default:
      return { banner: true, message: detail }
  }
}
