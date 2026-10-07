import { describe, it, expect, vi } from 'vitest'
import fs from 'fs'
import cp from 'child_process'
import { gateDisabled, buildDisabledStatusResult } from './disabled-gate'
import { readRuntimeSwitches } from './runtime-switches'
import { CODEINTEL_METHODS } from '../codeintel-method-table'
import { QUALITY_METHODS } from '../quality-method-table'

describe('disabled-gate (Task 073-02)', () => {
  const fullDisabledSwitches = readRuntimeSwitches({ ORCA_CODEINTEL_DISABLED: '1' })

  it('blocks every codeintel method except codeintel.status with -32000 codeintel_disabled', () => {
    for (const methodName of Object.keys(CODEINTEL_METHODS)) {
      if (methodName === 'codeintel.status') continue

      const gated = gateDisabled({ method: methodName, id: 1, params: { badKey: 'injection' } }, fullDisabledSwitches)
      expect(gated).toBeDefined()
      expect(gated.error.code).toBe(-32000)
      expect(gated.error.data.code).toBe('CODEINTEL_TOOL_UNAVAILABLE')
      expect(gated.error.data.reason).toBe('codeintel_disabled')
      expect(gated.error.message.length).toBeLessThanOrEqual(300)
      expect(gated.error.message).not.toContain('/')
      expect(gated.error.message).not.toContain('\\')
    }
  })

  it('blocks every quality method when codeintelDisabled is true', () => {
    for (const methodName of QUALITY_METHODS) {
      const gated = gateDisabled({ method: methodName, id: 2 }, fullDisabledSwitches)
      expect(gated).toBeDefined()
      expect(gated.error.code).toBe(-32000)
      expect(gated.error.data.reason).toBe('codeintel_disabled')
    }
  })

  it('codeintel.status returns disabled envelope without invoking fs, child_process, or git', () => {
    const fsStatSpy = vi.spyOn(fs, 'statSync')
    const cpSpawnSpy = vi.spyOn(cp, 'spawn')

    const gated = gateDisabled(
      { method: 'codeintel.status', id: 3, params: { workspaceRoot: '/my/repo' } },
      fullDisabledSwitches
    )

    expect(fsStatSpy).not.toHaveBeenCalled()
    expect(cpSpawnSpy).not.toHaveBeenCalled()

    expect(gated).toBeDefined()
    expect(gated.error).toBeUndefined()
    expect(gated.result).toEqual(buildDisabledStatusResult('/my/repo'))
    expect(gated.result.warnings).toContain('codeintel_disabled')
    expect(gated.result.tools.gitnexus.available).toBe(false)
    expect(gated.result.indexes.gitnexus.state).toBe('unknown')

    fsStatSpy.mockRestore()
    cpSpawnSpy.mockRestore()
  })

  it('reindexDisabled selectively blocks reindex methods only', () => {
    const reindexOff = readRuntimeSwitches({ ORCA_CODEINTEL_REINDEX: 'off' })

    const gatedReindex = gateDisabled({ method: 'codeintel.reindex', id: 4 }, reindexOff)
    expect(gatedReindex).toBeDefined()
    expect(gatedReindex.error.data.reason).toBe('reindex_disabled')

    // Overview should NOT be blocked by reindexDisabled
    const gatedOverview = gateDisabled({ method: 'codeintel.overview', id: 5 }, reindexOff)
    expect(gatedOverview).toBeNull()
  })

  it('qualityDisabled selectively blocks quality methods only', () => {
    const qualityOff = readRuntimeSwitches({ ORCA_QUALITY_RUN: 'off' })

    const gatedQuality = gateDisabled({ method: 'quality.run', id: 6 }, qualityOff)
    expect(gatedQuality).toBeDefined()
    expect(gatedQuality.error.data.reason).toBe('quality_disabled')

    // Codeintel should NOT be blocked by qualityDisabled alone
    const gatedOverview = gateDisabled({ method: 'codeintel.overview', id: 7 }, qualityOff)
    expect(gatedOverview).toBeNull()
  })
})
