import { describe, it, expect } from 'vitest'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { CodeIntelError, CodeIntelErrorCode, toErrorPayload } from './codeintel-errors'

describe('codeintel-errors', () => {
  it('maps specific errors to correct AgentErrorCode', () => {
    const errorMap: Array<{ code: CodeIntelErrorCode; expected: number }> = [
      { code: 'CODEINTEL_INVALID_PARAMS', expected: AgentErrorCode.InvalidParams },
      { code: 'CODEINTEL_SYMBOL_NOT_FOUND', expected: AgentErrorCode.InvalidParams },
      { code: 'CODEINTEL_PROFILE_UNKNOWN', expected: AgentErrorCode.InvalidParams },
      { code: 'CODEINTEL_RUN_NOT_FOUND', expected: AgentErrorCode.InvalidParams },
      { code: 'CODEINTEL_PATH_NOT_ALLOWED', expected: AgentErrorCode.PermissionDenied },
      { code: 'CODEINTEL_TOOL_UNAVAILABLE', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_REPO_NOT_REGISTERED', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_INDEX_MISSING', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_AMBIGUOUS_SYMBOL', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_TIMEOUT', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_REINDEX_IN_PROGRESS', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_OUTPUT_TOO_LARGE', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_TOOL_FAILED', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_ENV_NOT_READY', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_RUN_IN_PROGRESS', expected: AgentErrorCode.ServerError },
      { code: 'CODEINTEL_RUN_CANCELLED', expected: AgentErrorCode.ServerError },
    ]

    for (const { code, expected } of errorMap) {
      const err = new CodeIntelError(code, 'some message')
      const payload = toErrorPayload(err)
      expect(payload.code).toBe(expected)
      expect(payload.data.code).toBe(code)
    }
  })

  it('truncates message to 300 characters', () => {
    const longMsg = 'a'.repeat(400)
    const err = new CodeIntelError('CODEINTEL_INVALID_PARAMS', longMsg)
    const payload = toErrorPayload(err)
    expect(payload.message.length).toBe(300)
    expect(payload.message.endsWith('...')).toBe(true)
  })

  it('handles unknown errors without leaking path', () => {
    const err = new Error('failed to read /home/u/y/file.txt')
    const payload = toErrorPayload(err)
    expect(payload.code).toBe(AgentErrorCode.ServerError)
    expect(payload.message).toBe('internal error')
    expect(payload.data.code).toBe('CODEINTEL_TOOL_FAILED')
    expect((payload as any).stack).toBeUndefined()
  })

  it('includes data field', () => {
    const err = new CodeIntelError('CODEINTEL_INVALID_PARAMS', 'msg', { field: 'workspaceRoot' })
    const payload = toErrorPayload(err)
    expect(payload.data.field).toBe('workspaceRoot')
  })
})
