import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'
import { CodeIntelError } from '../codeintel-errors'

export function parseGitNexusContextOutput(stdout: string): any {
  let parsed: any
  try {
    parsed = JSON.parse(stdout)
  } catch {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Failed to parse JSON', { reason: 'format_drift' })
  }

  if (parsed.error) {
    if (/not found/i.test(parsed.error)) {
      throw new CodeIntelError('CODEINTEL_SYMBOL_NOT_FOUND', parsed.error)
    }
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', parsed.error, { reason: 'unknown_shape' })
  }

  if (parsed.ambiguous) {
    if (!Array.isArray(parsed.candidates)) {
      throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Missing candidates array in ambiguous context', { reason: 'format_drift' })
    }
    const candidates = parsed.candidates.slice(0, 10)
    throw new CodeIntelError('CODEINTEL_AMBIGUOUS_SYMBOL', 'Ambiguous symbol query', { candidates })
  }

  if (!parsed.target) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Missing target in context output', { reason: 'format_drift' })
  }

  return parsed
}

export function parseGitNexusImpactOutput(stdout: string): any {
  let parsed: any
  try {
    parsed = JSON.parse(stdout)
  } catch {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Failed to parse JSON', { reason: 'format_drift' })
  }

  if (parsed.error) {
    if (/not found/i.test(parsed.error)) {
      throw new CodeIntelError('CODEINTEL_SYMBOL_NOT_FOUND', parsed.error)
    }
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', parsed.error, { reason: 'unknown_shape' })
  }

  if (!parsed.target || !parsed.direction || !parsed.risk) {
    throw new CodeIntelError('CODEINTEL_TOOL_FAILED', 'Missing required keys in impact output', { reason: 'format_drift' })
  }

  return parsed
}

describe('gitnexus-json-commands golden tests', () => {
  const fixtureDir = path.join(__dirname, '__fixtures__', 'gitnexus', '1.6.9')

  it('parses context-found.json correctly', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'context-found.json'), 'utf8')
    const res = parseGitNexusContextOutput(raw)
    expect(res.target.id).toBe('s1')
    expect(res.target.name).toBe('getUser')
    expect(res.incoming.calls.length).toBe(1)
  })

  it('handles context-ambiguous.json with CODEINTEL_AMBIGUOUS_SYMBOL and candidates <= 10', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'context-ambiguous.json'), 'utf8')
    let err: any = null
    try {
      parseGitNexusContextOutput(raw)
    } catch (e) {
      err = e
    }
    expect(err).toBeInstanceOf(CodeIntelError)
    expect(err.code).toBe('CODEINTEL_AMBIGUOUS_SYMBOL')
    expect(err.data.candidates.length).toBe(2)
    expect(err.data.candidates[0].name).toBe('User')
  })

  it('handles context-not-found.json with CODEINTEL_SYMBOL_NOT_FOUND', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'context-not-found.json'), 'utf8')
    expect(() => parseGitNexusContextOutput(raw)).toThrowError(CodeIntelError)
    try {
      parseGitNexusContextOutput(raw)
    } catch (e: any) {
      expect(e.code).toBe('CODEINTEL_SYMBOL_NOT_FOUND')
    }
  })

  it('parses impact-found.json correctly', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'impact-found.json'), 'utf8')
    const res = parseGitNexusImpactOutput(raw)
    expect(res.target.id).toBe('s1')
    expect(res.direction).toBe('upstream')
    expect(res.risk).toBe('HIGH')
    expect(res.byDepth.length).toBe(1)
  })

  it('handles impact-not-found.json with CODEINTEL_SYMBOL_NOT_FOUND', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'impact-not-found.json'), 'utf8')
    try {
      parseGitNexusImpactOutput(raw)
    } catch (e: any) {
      expect(e.code).toBe('CODEINTEL_SYMBOL_NOT_FOUND')
    }
  })

  it('detects format_drift on missing required keys', () => {
    const driftJson = JSON.stringify({ other: 123 })
    expect(() => parseGitNexusContextOutput(driftJson)).toThrow(/Missing target/)
    expect(() => parseGitNexusImpactOutput(driftJson)).toThrow(/Missing required keys/)
  })

  it('parses query-found.json correctly', () => {
    const raw = fs.readFileSync(path.join(fixtureDir, 'query-found.json'), 'utf8')
    const parsed = JSON.parse(raw)
    expect(parsed.results.length).toBe(1)
    expect(parsed.results[0].name).toBe('auth-flow')
  })
})
