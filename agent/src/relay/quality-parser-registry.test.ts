import { describe, it, expect } from 'vitest'
import { getParser, createStepParser } from './quality-parser-registry'
import fs from 'fs/promises'
import path from 'path'
import os from 'os'

describe('quality-parser-registry', () => {
  it('returns parser by key', () => {
    expect(getParser('oxlint')).toBeDefined()
    expect(getParser('not-found')).toBeUndefined()
  })

  it('createStepParser handles missing parser', async () => {
    const parser = createStepParser('unknown', {} as any)
    const res = await parser.parse('stdout', 'stderr')
    expect(res.stepResult.state).toBe('failed')
    expect(res.stepResult.failureKind).toBe('parser_error')
  })

  it('createStepParser handles timeouts', async () => {
    const parser = createStepParser('oxlint', { timeout: true } as any)
    const res = await parser.parse('stdout', 'stderr')
    expect(res.stepResult.state).toBe('timeout')
  })
})
