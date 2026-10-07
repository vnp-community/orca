import { describe, it, expect } from 'vitest'
import { readCodeIntelLimits, CODEINTEL_DEFAULT_LIMITS } from './codeintel-limits'

describe('codeintel-limits', () => {
  it('returns default limits when env is empty', () => {
    const log = { warn: () => {} }
    const limits = readCodeIntelLimits({}, log)
    expect(limits).toEqual(CODEINTEL_DEFAULT_LIMITS)
  })

  it('parses valid overrides', () => {
    const env = {
      ORCA_CODEINTEL_TOOL_TIMEOUT_MS: '15000',
      ORCA_CODEINTEL_DETECT_TIMEOUT_MS: '25000'
    }
    const log = { warn: () => {} }
    const limits = readCodeIntelLimits(env, log)
    expect(limits.toolTimeoutMs).toBe(15000)
    expect(limits.detectTimeoutMs).toBe(25000)
    expect(limits.methodTimeoutMs).toBe(CODEINTEL_DEFAULT_LIMITS.methodTimeoutMs)
  })

  it('ignores invalid overrides and warns', () => {
    const env = {
      ORCA_CODEINTEL_TOOL_TIMEOUT_MS: 'abc',
      ORCA_CODEINTEL_QUEUE_MAX: '-1',
      ORCA_CODEINTEL_MAX_CONCURRENT_TOOLS: '0',
    }
    const warnings: string[] = []
    const log = { warn: (msg: string) => warnings.push(msg) }
    const limits = readCodeIntelLimits(env, log)
    expect(limits.toolTimeoutMs).toBe(CODEINTEL_DEFAULT_LIMITS.toolTimeoutMs)
    expect(limits.queueMax).toBe(CODEINTEL_DEFAULT_LIMITS.queueMax)
    expect(limits.maxConcurrentTools).toBe(CODEINTEL_DEFAULT_LIMITS.maxConcurrentTools)
    expect(warnings.length).toBe(3)
  })

  it('ignores methodTimeoutMs if <= toolTimeoutMs', () => {
    const env = {
      ORCA_CODEINTEL_METHOD_TIMEOUT_MS: '10000',
      ORCA_CODEINTEL_TOOL_TIMEOUT_MS: '15000'
    }
    const warnings: string[] = []
    const log = { warn: (msg: string) => warnings.push(msg) }
    const limits = readCodeIntelLimits(env, log)
    expect(limits.methodTimeoutMs).toBe(CODEINTEL_DEFAULT_LIMITS.methodTimeoutMs)
    expect(limits.toolTimeoutMs).toBe(15000)
    expect(warnings.length).toBe(1)
  })
})
