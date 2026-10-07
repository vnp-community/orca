import { AgentErrorCode } from '../shared/agent-wire-protocol'

export type CodeIntelErrorCode =
  | 'CODEINTEL_INVALID_PARAMS'
  | 'CODEINTEL_PATH_NOT_ALLOWED'
  | 'CODEINTEL_TOOL_UNAVAILABLE'
  | 'CODEINTEL_REPO_NOT_REGISTERED'
  | 'CODEINTEL_INDEX_MISSING'
  | 'CODEINTEL_SYMBOL_NOT_FOUND'
  | 'CODEINTEL_AMBIGUOUS_SYMBOL'
  | 'CODEINTEL_TIMEOUT'
  | 'CODEINTEL_REINDEX_IN_PROGRESS'
  | 'CODEINTEL_OUTPUT_TOO_LARGE'
  | 'CODEINTEL_TOOL_FAILED'
  | 'CODEINTEL_PROFILE_UNKNOWN'
  | 'CODEINTEL_ENV_NOT_READY'
  | 'CODEINTEL_RUN_IN_PROGRESS'
  | 'CODEINTEL_RUN_NOT_FOUND'
  | 'CODEINTEL_RUN_CANCELLED'

export class CodeIntelError extends Error {
  constructor(
    readonly code: CodeIntelErrorCode,
    message: string,
    readonly data: Record<string, unknown> = {}
  ) {
    super(message)
    this.name = 'CodeIntelError'
  }
}

export type CodeIntelErrorPayload = {
  code: number
  message: string
  data: { code: CodeIntelErrorCode } & Record<string, unknown>
}

export function toErrorPayload(err: unknown): CodeIntelErrorPayload {
  if (err instanceof CodeIntelError || (typeof err === 'object' && err !== null && (err as any).name === 'CodeIntelError')) {
    const codeIntelErr = err as CodeIntelError
    let errorCode: number
    switch (err.code) {
      case 'CODEINTEL_INVALID_PARAMS':
      case 'CODEINTEL_SYMBOL_NOT_FOUND':
      case 'CODEINTEL_PROFILE_UNKNOWN':
      case 'CODEINTEL_RUN_NOT_FOUND':
        errorCode = AgentErrorCode.InvalidParams
        break
      case 'CODEINTEL_PATH_NOT_ALLOWED':
        errorCode = AgentErrorCode.PermissionDenied
        break
      default:
        errorCode = AgentErrorCode.ServerError
    }

    let msg = err.message
    if (msg.length > 300) {
      msg = msg.substring(0, 297) + '...'
    }

    return {
      code: errorCode,
      message: msg,
      data: {
        code: err.code,
        ...err.data,
      },
    }
  }

  return {
    code: AgentErrorCode.ServerError,
    message: 'internal error',
    data: {
      code: 'CODEINTEL_TOOL_FAILED',
    },
  }
}
