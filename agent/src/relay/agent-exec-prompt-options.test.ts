import { describe, it, expect } from 'vitest'
import {
  parseExecPromptOptions,
  toExecPromptErrorResponse,
  DEFAULT_MAX_OUTPUT_BYTES,
  MAX_OUTPUT_BYTES_CEILING,
  MIN_OUTPUT_BYTES
} from './agent-exec-prompt-options'

describe('agent-exec-prompt-options', () => {
  it('returns defaults when no new param is present', () => {
    const res = parseExecPromptOptions({})
    expect(res.ok).toBe(true)
    if (!res.ok) return
    expect(res.value.accessMode).toBe('write')
    expect(res.value.workspaceKind).toBe('worktree')
    expect(res.value.explicit.accessMode).toBe(false)
    expect(res.value.explicit.workspaceKind).toBe(false)
    expect(res.value.reportChanges).toBe(false)
    expect(res.value.resultBlockNonce).toBeNull()
    expect(res.value.maxOutputBytes).toBe(DEFAULT_MAX_OUTPUT_BYTES)
  })

  it('accepts accessMode write and readonly and marks explicit', () => {
    const resWrite = parseExecPromptOptions({ accessMode: 'write' })
    expect(resWrite.ok).toBe(true)
    if (resWrite.ok) {
      expect(resWrite.value.accessMode).toBe('write')
      expect(resWrite.value.explicit.accessMode).toBe(true)
    }

    const resRo = parseExecPromptOptions({ accessMode: 'readonly' })
    expect(resRo.ok).toBe(true)
    if (resRo.ok) {
      expect(resRo.value.accessMode).toBe('readonly')
      expect(resRo.value.explicit.accessMode).toBe(true)
    }
  })

  it.each([
    ['READONLY', 'uppercase'],
    ['ReadOnly', 'mixed case'],
    [null, 'null'],
    [123, 'number'],
    ['', 'empty string'],
    [true, 'boolean']
  ])('rejects accessMode with wrong case/type: %s (%s)', (val) => {
    const res = parseExecPromptOptions({ accessMode: val })
    expect(res.ok).toBe(false)
    if (!res.ok) {
      expect(res.error.code).toBe('INVALID_ACCESS_MODE')
    }
  })

  it('rejects workspaceKind outside the three allowed values', () => {
    const validKinds = ['worktree', 'repo_root', 'scratch']
    for (const kind of validKinds) {
      const res = parseExecPromptOptions({ workspaceKind: kind })
      expect(res.ok).toBe(true)
      if (res.ok) {
        expect(res.value.workspaceKind).toBe(kind)
        expect(res.value.explicit.workspaceKind).toBe(true)
      }
    }

    const invalidKinds = ['invalid', 'WORKTREE', null, 1, '']
    for (const kind of invalidKinds) {
      const res = parseExecPromptOptions({ workspaceKind: kind })
      expect(res.ok).toBe(false)
      if (!res.ok) {
        expect(res.error.code).toBe('INVALID_WORKSPACE_KIND')
      }
    }
  })

  it('rejects reportChanges given as string "true"', () => {
    const resStr = parseExecPromptOptions({ reportChanges: 'true' })
    expect(resStr.ok).toBe(false)
    if (!resStr.ok) {
      expect(resStr.error.code).toBe('INVALID_REPORT_CHANGES')
    }

    const resBool = parseExecPromptOptions({ reportChanges: true })
    expect(resBool.ok).toBe(true)
    if (resBool.ok) {
      expect(resBool.value.reportChanges).toBe(true)
    }
  })

  it('rejects resultBlock that is an array or string', () => {
    expect(parseExecPromptOptions({ resultBlock: [] }).ok).toBe(false)
    expect(parseExecPromptOptions({ resultBlock: 'test' }).ok).toBe(false)
    expect(parseExecPromptOptions({ resultBlock: null }).ok).toBe(false)

    const invalid = parseExecPromptOptions({ resultBlock: [] })
    if (!invalid.ok) {
      expect(invalid.error.code).toBe('INVALID_RESULT_BLOCK')
    }
  })

  it('rejects nonce shorter than 16, longer than 64, or containing "-" or space', () => {
    const validNonce = 'a'.repeat(16)
    const resValid = parseExecPromptOptions({ resultBlock: { nonce: validNonce } })
    expect(resValid.ok).toBe(true)
    if (resValid.ok) {
      expect(resValid.value.resultBlockNonce).toBe(validNonce)
    }

    const invalidNonces = [
      'a'.repeat(15),           // too short
      'a'.repeat(65),           // too long
      'a'.repeat(15) + '-',     // contains hyphen
      'a'.repeat(15) + ' ',     // contains space
      'a'.repeat(15) + '_',     // contains underscore
      1234567890123456          // not string
    ]

    for (const nonce of invalidNonces) {
      const res = parseExecPromptOptions({ resultBlock: { nonce } })
      expect(res.ok).toBe(false)
      if (!res.ok) {
        expect(res.error.code).toBe('INVALID_RESULT_BLOCK_NONCE')
      }
    }
  })

  it('clamps maxOutputBytes above the ceiling to 12 MiB and rejects invalid values', () => {
    // Clamping above ceiling
    const resClamp = parseExecPromptOptions({ maxOutputBytes: 20 * 1024 * 1024 })
    expect(resClamp.ok).toBe(true)
    if (resClamp.ok) {
      expect(resClamp.value.maxOutputBytes).toBe(MAX_OUTPUT_BYTES_CEILING)
    }

    // Rejection below 64 KiB
    const resTooLow = parseExecPromptOptions({ maxOutputBytes: MIN_OUTPUT_BYTES - 1 })
    expect(resTooLow.ok).toBe(false)
    if (!resTooLow.ok) {
      expect(resTooLow.error.code).toBe('INVALID_MAX_OUTPUT_BYTES')
    }

    // Fractional, NaN, string
    const invalidValues = [100000.5, NaN, '1000000', null, Infinity]
    for (const val of invalidValues) {
      const res = parseExecPromptOptions({ maxOutputBytes: val })
      expect(res.ok).toBe(false)
      if (!res.ok) {
        expect(res.error.code).toBe('INVALID_MAX_OUTPUT_BYTES')
      }
    }
  })

  it('toExecPromptErrorResponse puts the code both in message and in error.data.reason', () => {
    const errResp = toExecPromptErrorResponse('agent.execPrompt', 'req-1', {
      code: 'INVALID_ACCESS_MODE',
      message: 'accessMode must be "write" or "readonly"'
    }) as any

    expect(errResp.jsonrpc).toBe('2.0')
    expect(errResp.id).toBe('req-1')
    expect(errResp.error.code).toBe(-32602)
    expect(errResp.error.message).toContain('INVALID_ACCESS_MODE')
    expect(errResp.error.data).toEqual({ reason: 'INVALID_ACCESS_MODE' })
  })
})
