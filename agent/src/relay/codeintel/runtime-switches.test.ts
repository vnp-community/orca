import { describe, it, expect } from 'vitest'
import { readRuntimeSwitches } from './runtime-switches'

describe('runtime-switches (Task 073-01)', () => {
  it('parses truthy values as disabled', () => {
    const truthy = ['1', 'true', 'TRUE', 'yes', 'YES', 'on', ' ON ', '  1  ']
    for (const val of truthy) {
      const switches = readRuntimeSwitches({ ORCA_CODEINTEL_DISABLED: val })
      expect(switches.codeintelDisabled).toBe(true)
      expect(switches.warnings.length).toBe(0)
    }
  })

  it('parses absent, empty, and falsy values as enabled', () => {
    const falsy = [undefined, '', '   ', '0', 'false', 'FALSE', 'no', 'NO', 'off', 'OFF']
    for (const val of falsy) {
      const env = val !== undefined ? { ORCA_CODEINTEL_DISABLED: val } : {}
      const switches = readRuntimeSwitches(env)
      expect(switches.codeintelDisabled).toBe(false)
      expect(switches.warnings.length).toBe(0)
    }
  })

  it('fails closed with warning on unrecognized value', () => {
    const invalidValues = ['maybe', 'unknown', '2', 'disabled', 'potato']
    for (const val of invalidValues) {
      const switches = readRuntimeSwitches({ ORCA_CODEINTEL_DISABLED: val })
      expect(switches.codeintelDisabled).toBe(true)
      expect(switches.warnings).toEqual([
        { var: 'ORCA_CODEINTEL_DISABLED', code: 'invalid_value' }
      ])
    }
  })

  it('parses ORCA_CODEINTEL_REINDEX=off and handles invalid values with warning', () => {
    const offSwitches = readRuntimeSwitches({ ORCA_CODEINTEL_REINDEX: 'off' })
    expect(offSwitches.reindexDisabled).toBe(true)

    const invalidSwitches = readRuntimeSwitches({ ORCA_CODEINTEL_REINDEX: 'not_a_valid_flag' })
    expect(invalidSwitches.reindexDisabled).toBe(false)
    expect(invalidSwitches.warnings).toEqual([
      { var: 'ORCA_CODEINTEL_REINDEX', code: 'invalid_value' }
    ])
  })

  it('parses ORCA_QUALITY_RUN=off and cascades codeintelDisabled to reindex and quality', () => {
    const qualityOff = readRuntimeSwitches({ ORCA_QUALITY_RUN: 'off' })
    expect(qualityOff.qualityDisabled).toBe(true)

    // Cascade: when codeintelDisabled is true, reindex and quality are also disabled
    const cascadeSwitches = readRuntimeSwitches({ ORCA_CODEINTEL_DISABLED: '1' })
    expect(cascadeSwitches.codeintelDisabled).toBe(true)
    expect(cascadeSwitches.reindexDisabled).toBe(true)
    expect(cascadeSwitches.qualityDisabled).toBe(true)
  })

  it('returns frozen object that cannot be mutated', () => {
    const switches = readRuntimeSwitches({})
    expect(Object.isFrozen(switches)).toBe(true)
    expect(Object.isFrozen(switches.warnings)).toBe(true)

    expect(() => {
      ;(switches as any).codeintelDisabled = true
    }).toThrow()
  })
})
