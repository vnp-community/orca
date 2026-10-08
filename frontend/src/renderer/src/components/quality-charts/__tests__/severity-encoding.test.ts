import { describe, expect, it } from 'vitest'
import {
  SEVERITY_ENCODING,
  VERDICT_ENCODING,
  labelOf,
  toSeverityLevel,
  toVerdictLevel
} from '../severity-encoding'

describe('severity encoding', () => {
  it('uses a distinct {shape, strokeDash} per level within each table', () => {
    for (const table of [SEVERITY_ENCODING, VERDICT_ENCODING]) {
      const keys = Object.values(table).map((e) => `${e.shape}|${e.strokeDash}`)
      expect(new Set(keys).size).toBe(keys.length)
    }
  })

  it('never gives unknown the pass icon or shape', () => {
    expect(VERDICT_ENCODING.unknown.icon).not.toBe(VERDICT_ENCODING.pass.icon)
    expect(VERDICT_ENCODING.unknown.shape).not.toBe(VERDICT_ENCODING.pass.shape)
    expect(SEVERITY_ENCODING.unknown.shape).not.toBe(VERDICT_ENCODING.pass.shape)
  })

  it('maps unexpected enum values to unknown without throwing', () => {
    expect(toSeverityLevel('critical')).toBe('unknown')
    expect(toSeverityLevel(undefined)).toBe('unknown')
    expect(toSeverityLevel(42)).toBe('unknown')
    expect(toVerdictLevel('PASS')).toBe('unknown')
    expect(toVerdictLevel('warn')).toBe('warn')
  })

  it('labels never use conclusive wording', () => {
    for (const entry of [...Object.values(SEVERITY_ENCODING), ...Object.values(VERDICT_ENCODING)]) {
      expect(labelOf(entry)).not.toMatch(/safe|clean|met/i)
    }
  })
})
