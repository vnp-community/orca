import { MCP_ERROR_CODES, type McpErrorCode } from '../../../shared/mcp-types'

export class McpRpcError extends Error {
  readonly code: McpErrorCode | null
  readonly detail: string
  readonly cause?: unknown

  constructor(code: McpErrorCode | null, detail: string, cause?: unknown) {
    super(detail)
    this.name = 'McpRpcError'
    this.code = code
    this.detail = detail
    this.cause = cause
  }
}

// Why: C4 — gateway should strip the gRPC prefix, but accept both forms defensively.
const PATTERN = /(?:^|desc = )(MCP_[A-Z0-9_]+): ([\s\S]*)$/

export function parseMcpError(error: unknown): McpRpcError {
  if (error instanceof McpRpcError) {
    return error
  }
  const message = error instanceof Error ? error.message : String(error)
  const m = PATTERN.exec(message)
  // Unknown MCP_* codes keep the server detail but code=null (UI shows the message, no crash).
  const code =
    m && (MCP_ERROR_CODES as readonly string[]).includes(m[1]) ? (m[1] as McpErrorCode) : null
  return new McpRpcError(code, m ? m[2] : message, error)
}

export const isMcpDisabledError = (e: unknown): boolean =>
  e instanceof McpRpcError && e.code === 'MCP_DISABLED'

export const isMcpNotAdminError = (e: unknown): boolean =>
  e instanceof McpRpcError && e.code === 'MCP_NOT_ADMIN'

export const isMcpNotImplementedError = (e: unknown): boolean =>
  /is not yet implemented/.test((e as Error | undefined)?.message ?? '')
