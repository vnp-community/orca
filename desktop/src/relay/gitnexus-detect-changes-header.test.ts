import { describe, it, expect } from 'vitest'
import {
  parseGitNexusDetectChangesHeader,
  evaluateCrossCheck
} from './gitnexus-detect-changes-header'

describe('gitnexus-detect-changes-header', () => {
  it('parses standard 3-line header correctly', () => {
    const output = `Changes: 446 files, 1508 symbols
Affected processes: 12
Risk level: MEDIUM
Additional details that should not be parsed...
- Symbol 1
- Symbol 2
`
    const hint = parseGitNexusDetectChangesHeader(output)
    expect(hint).toEqual({
      source: 'gitnexus detect-changes',
      level: 'MEDIUM',
      files: 446,
      symbols: 1508,
      processes: 12
    })
  })

  it('handles singular file and symbol', () => {
    const output = `Changes: 1 file, 1 symbol
Affected processes: 0
Risk level: low
`
    const hint = parseGitNexusDetectChangesHeader(output)
    expect(hint).toEqual({
      source: 'gitnexus detect-changes',
      level: 'LOW',
      files: 1,
      symbols: 1,
      processes: 0
    })
  })

  it('returns null gracefully on empty or malformed output without throwing', () => {
    expect(parseGitNexusDetectChangesHeader('')).toBeNull()
    expect(parseGitNexusDetectChangesHeader('Random text\nLine 2')).toBeNull()
    expect(parseGitNexusDetectChangesHeader('Changes: 10 files\nNot matched')).toBeNull()
  })

  it('detects crosscheck_mismatch when difference exceeds 20%', () => {
    const hint = {
      source: 'gitnexus detect-changes' as const,
      level: 'HIGH',
      files: 100,
      symbols: 500,
      processes: 5
    }

    // Agent has 60 files -> diff 40/100 = 40% > 20%
    const res = evaluateCrossCheck(hint, { files: 60, symbols: 500 })
    expect(res.warnings).toContain('crosscheck_mismatch')
    expect(res.riskHint).toBe(hint)
  })

  it('does not warn when difference is within 20%', () => {
    const hint = {
      source: 'gitnexus detect-changes' as const,
      level: 'LOW',
      files: 100,
      symbols: 500,
      processes: 5
    }

    // Agent has 90 files (10% diff) and 450 symbols (10% diff)
    const res = evaluateCrossCheck(hint, { files: 90, symbols: 450 })
    expect(res.warnings).toHaveLength(0)
  })
})
