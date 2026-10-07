import { describe, it, expect, vi } from 'vitest'
import { classifyGitNexusStdout, classifyCodeGraphStdout } from './codeintel-tool-output-classification'
import { CodeIntelError } from './codeintel-errors'

describe('classifyGitNexusStdout', () => {
  it('classifies not found errors', () => {
    const out1 = classifyGitNexusStdout('{"error":"Table Nope does not exist"}')
    expect(out1).toEqual({ notFound: true })

    const out2 = classifyGitNexusStdout('{"error":"Symbol \'X\' not found"}')
    expect(out2).toEqual({ notFound: true })
  })

  it('classifies write blocked error', () => {
    const log = { error: vi.fn() }
    expect(() => classifyGitNexusStdout('{"error":"Write operations (CREATE…) are not allowed…"}', log))
      .toThrowError(CodeIntelError)
    expect(log.error).toHaveBeenCalled()
  })

  it('classifies generic error', () => {
    expect(() => classifyGitNexusStdout('{"error":"Something went wrong"}'))
      .toThrowError(CodeIntelError)
  })

  it('throws truncated_stdout on invalid json', () => {
    try {
      classifyGitNexusStdout('{"error":"So')
      expect.fail('Should throw')
    } catch (e: any) {
      expect(e).toBeInstanceOf(CodeIntelError)
      expect(e.data.reason).toBe('truncated_stdout')
    }
  })

  it('returns valid json objects/arrays', () => {
    expect(classifyGitNexusStdout('[]')).toEqual({ kind: 'json', value: [] })
    expect(classifyGitNexusStdout('{"markdown":"","row_count":0}')).toEqual({ kind: 'json', value: { markdown: '', row_count: 0 } })
  })
})

describe('classifyCodeGraphStdout', () => {
  it('strips ANSI and throws SYMBOL_NOT_FOUND', () => {
    try {
      classifyCodeGraphStdout('\u001b[36mℹ Symbol "X" not found\u001b[0m')
      expect.fail()
    } catch (e: any) {
      expect(e).toBeInstanceOf(CodeIntelError)
      expect(e.code).toBe('CODEINTEL_SYMBOL_NOT_FOUND')
    }
  })

  it('throws INDEX_MISSING', () => {
    try {
      classifyCodeGraphStdout('✗ CodeGraph not initialized in /tmp')
      expect.fail()
    } catch (e: any) {
      expect(e).toBeInstanceOf(CodeIntelError)
      expect(e.code).toBe('CODEINTEL_INDEX_MISSING')
    }
  })

  it('returns json', () => {
    const res = classifyCodeGraphStdout('{"initialized":false}')
    expect(res).toEqual({ kind: 'json', value: { initialized: false } })
  })

  it('throws truncated_stdout on invalid json starting with {', () => {
    try {
      classifyCodeGraphStdout('{"initialized":fal')
      expect.fail()
    } catch (e: any) {
      expect(e.data.reason).toBe('truncated_stdout')
    }
  })
})
