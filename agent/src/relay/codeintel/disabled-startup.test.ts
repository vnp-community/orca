import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import fs from 'fs'
import { enableWatch, getWatchingRoots } from '../codeintel-index-watcher'
import { emitCodeIntelNotification, setCodeIntelNotifier } from '../codeintel-notification-sink'
import { writeJobStart, readActiveJobs } from '../codeintel-reindex-journal'
import { logCodeIntelDisabledOnce, resetDisabledLogState } from './runtime-switches'

describe('disabled-startup (Task 073-04)', () => {
  const origDisabled = process.env.ORCA_CODEINTEL_DISABLED

  beforeEach(() => {
    process.env.ORCA_CODEINTEL_DISABLED = '1'
    resetDisabledLogState()
  })

  afterEach(() => {
    if (origDisabled !== undefined) {
      process.env.ORCA_CODEINTEL_DISABLED = origDisabled
    } else {
      delete process.env.ORCA_CODEINTEL_DISABLED
    }
    resetDisabledLogState()
  })

  it('enableWatch has no side-effects and does not call fs.watch when disabled', () => {
    const fsWatchSpy = vi.spyOn(fs, 'watch')
    enableWatch({ toplevel: '/some/repo', gitNexusRegistryPath: '/some/repo' })

    expect(fsWatchSpy).not.toHaveBeenCalled()
    expect(getWatchingRoots()).not.toContain('/some/repo')
    fsWatchSpy.mockRestore()
  })

  it('emitCodeIntelNotification is a no-op when disabled', () => {
    const sinkSpy = vi.fn()
    setCodeIntelNotifier(sinkSpy)

    emitCodeIntelNotification('codeintel.indexChanged', {
      workspaceRoot: '/some/repo',
      reason: 'test',
      tool: 'gitnexus'
    })

    expect(sinkSpy).not.toHaveBeenCalled()
  })

  it('reindex journal does not write any files or create jobs directory when disabled', () => {
    const fsWriteSpy = vi.spyOn(fs, 'writeFileSync')
    writeJobStart({
      jobId: 'test-job',
      tool: 'gitnexus',
      mode: 'init',
      workspaceRoot: '/some/repo',
      startedAt: Date.now()
    })

    expect(fsWriteSpy).not.toHaveBeenCalled()
    fsWriteSpy.mockRestore()
  })

  it('logs exactly one warning line on startup/dispatch without leaking env values', () => {
    const logWarnSpy = vi.fn()
    const mockLogger = { warn: logWarnSpy }

    logCodeIntelDisabledOnce(mockLogger)
    expect(logWarnSpy).toHaveBeenCalledTimes(1)
    expect(logWarnSpy).toHaveBeenCalledWith('codeintel disabled by ORCA_CODEINTEL_DISABLED')

    // Subsequent calls do not log duplicate lines
    logCodeIntelDisabledOnce(mockLogger)
    expect(logWarnSpy).toHaveBeenCalledTimes(1)
  })
})
