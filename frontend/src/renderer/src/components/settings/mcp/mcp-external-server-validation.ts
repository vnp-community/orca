import type {
  McpExternalServer,
  McpExternalServerUpsertInput,
  McpErrorCode
} from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'

export type ServerDraft = {
  id?: string
  name: string
  scope: McpExternalServer['scope']
  scopeId?: string
  transport: McpExternalServer['transport']
  url: string
  command: string
  args: string[]
  envNames: string[]
  headerNames: string[]
}

export type ServerDraftErrors = Partial<
  Record<'name' | 'url' | 'command' | 'team' | 'envNames' | 'headerNames', string>
>

// Matches the backend name rule.
export const SERVER_NAME_PATTERN = /^[a-z0-9][a-z0-9_-]{0,62}$/
const REF_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_-]{0,127}$/
const SECRET_LIKE_ARG =
  /(?:sk-[A-Za-z0-9]{16,}|gh[pousr]_[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|eyJ[A-Za-z0-9_-]{20,}\.)/
const IP_LITERAL = /^(?:\d{1,3}(?:\.\d{1,3}){3}|\[.*\])$/

export function emptyServerDraft(isAdmin: boolean): ServerDraft {
  return {
    name: '',
    scope: isAdmin ? 'tenant' : 'user',
    transport: 'http',
    url: '',
    command: '',
    args: [],
    envNames: [],
    headerNames: []
  }
}

export function draftFromServer(s: McpExternalServer): ServerDraft {
  return {
    id: s.id,
    name: s.name,
    scope: s.scope,
    scopeId: s.scopeId,
    transport: s.transport,
    url: s.url ?? '',
    command: s.command ?? '',
    args: [...(s.args ?? [])],
    envNames: s.envRefs.map((r) => r.name),
    headerNames: s.headerRefs.map((r) => r.name)
  }
}

/** Advisory client check only; the server (SSRF guard) decides. */
export function validateServerUrl(raw: string): string | null {
  let u: URL
  try {
    u = new URL(raw.trim())
  } catch {
    return translate(
      'auto.mcp.external.err.urlInvalid',
      'Enter a full URL, e.g. https://mcp.example.com/mcp'
    )
  }
  if (u.protocol !== 'https:') {
    return translate('auto.mcp.external.err.urlHttps', 'The URL must start with https://')
  }
  if (u.username || u.password) {
    return translate(
      'auto.mcp.external.err.urlUserinfo',
      'Remove the user name or password from the URL; use a header secret instead.'
    )
  }
  if (IP_LITERAL.test(u.hostname)) {
    return translate('auto.mcp.external.err.urlIp', 'Use a host name rather than an IP address.')
  }
  return null
}

function validateRefNames(names: string[]): string | null {
  const seen = new Set<string>()
  for (const n of names) {
    if (!REF_NAME_PATTERN.test(n)) {
      return translate(
        'auto.mcp.external.err.refName',
        'Names may use letters, digits, "_" and "-" and cannot be empty.'
      )
    }
    const k = n.toLowerCase()
    if (seen.has(k)) {
      return translate('auto.mcp.external.err.refDuplicate', 'Names must be unique.')
    }
    seen.add(k)
  }
  return null
}

export function validateServerDraft(d: ServerDraft): ServerDraftErrors {
  const e: ServerDraftErrors = {}
  if (!SERVER_NAME_PATTERN.test(d.name)) {
    e.name = translate(
      'auto.mcp.external.err.name',
      'Use 1-63 lowercase letters, digits, "_" or "-", starting with a letter or digit.'
    )
  }
  if (d.scope === 'team' && !d.scopeId) {
    e.team = translate('auto.mcp.external.err.team', 'Choose a team.')
  }
  if (d.transport === 'http') {
    const u = validateServerUrl(d.url)
    if (u) {
      e.url = u
    }
    const h = validateRefNames(d.headerNames)
    if (h) {
      e.headerNames = h
    }
  } else {
    if (!d.command.trim() || /[\s/\\]/.test(d.command)) {
      e.command = translate(
        'auto.mcp.external.err.command',
        'Enter a single command name, without spaces or a path (e.g. npx).'
      )
    }
    const v = validateRefNames(d.envNames)
    if (v) {
      e.envNames = v
    }
  }
  return e
}

export const hasServerErrors = (e: ServerDraftErrors): boolean => Object.keys(e).length > 0

export const looksLikeSecretArg = (arg: string): boolean => SECRET_LIKE_ARG.test(arg)

/** Names only: hasSecret/status/digest are server-computed; scopeId only for team scope. */
export function buildUpsertBody(d: ServerDraft): McpExternalServerUpsertInput {
  const body: McpExternalServerUpsertInput = {
    name: d.name,
    scope: d.scope,
    transport: d.transport
  }
  if (d.id) {
    body.id = d.id
  }
  if (d.scope === 'team') {
    body.scopeId = d.scopeId
  }
  if (d.transport === 'http') {
    body.url = d.url.trim()
    body.headerRefs = d.headerNames.map((name) => ({ name }))
    body.envRefs = []
  } else {
    body.command = d.command.trim()
    body.args = d.args.filter((a) => a !== '')
    body.envRefs = d.envNames.map((name) => ({ name }))
    body.headerRefs = []
  }
  return body
}

/** True when an edit changes what an admin approved (spec fields or ref names). */
export function editNeedsReReview(before: ServerDraft, after: ServerDraft): boolean {
  return (
    before.transport !== after.transport ||
    before.url.trim() !== after.url.trim() ||
    before.command.trim() !== after.command.trim() ||
    JSON.stringify(before.args) !== JSON.stringify(after.args) ||
    JSON.stringify(before.envNames) !== JSON.stringify(after.envNames) ||
    JSON.stringify(before.headerNames) !== JSON.stringify(after.headerNames)
  )
}

/** Fixed i18n copy for known codes; unknown codes fall back to the server detail. */
export function externalServerErrorText(code: McpErrorCode | null, detail: string): string {
  switch (code) {
    case 'MCP_SERVER_SSRF_BLOCKED':
      return translate(
        'auto.mcp.external.err.ssrf',
        "This address isn't allowed (private, local or cloud-metadata addresses are blocked)."
      )
    case 'MCP_SERVER_INVALID':
      return translate(
        'auto.mcp.external.err.invalid',
        'The server definition is invalid: {{detail}}',
        { detail }
      )
    case 'MCP_SERVER_STDIO_NOT_ALLOWED':
      return translate(
        'auto.mcp.external.err.stdioNotAllowed',
        'Your organization has not enabled stdio servers.'
      )
    case 'MCP_SERVER_NAME_CONFLICT':
      return translate(
        'auto.mcp.external.err.nameConflict',
        'A server with this name already exists in this scope.'
      )
    case 'MCP_SERVER_NOT_APPROVED':
      return translate('auto.mcp.external.err.notApproved', 'This server is not approved yet.')
    case 'MCP_SERVER_DIGEST_MISMATCH':
      return translate(
        'auto.mcp.external.err.digestMismatch',
        'The server changed while you were reviewing.'
      )
    case 'MCP_NOT_ADMIN':
      return translate('auto.mcp.external.err.notAdmin', 'You need admin rights for this action.')
    case 'MCP_KILL_SWITCH_ACTIVE':
      return translate('auto.mcp.external.err.killSwitch', 'MCP is paused by an admin.')
    default:
      return detail
  }
}
