import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { startRun, QualityRunStartError, QualityRunStartDeps } from './quality-run-start'
import * as pf from './quality-environment-preflight'
import fs from 'fs'
import path from 'path'
import os from 'os'

describe('quality-run-start', () => {
  let tmpRoot: string
  let mockRunManager: any
  let deps: QualityRunStartDeps
  let preflightSpy: any

  beforeEach(() => {
    tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-run-start-'))
    fs.mkdirSync(path.join(tmpRoot, '.git'))
    // dummy file
    fs.writeFileSync(path.join(tmpRoot, 'index.ts'), '')

    mockRunManager = {
      submit: vi.fn().mockResolvedValue({
        runId: 'run-1', state: 'queued', queuePosition: 1, dirtyFingerprint: 'x'
      })
    }
    
    deps = {
      runManager: mockRunManager,
      warn: vi.fn(),
      resolveBin: (b) => `/bin/${b}`,
      gitCommonDir: path.join(tmpRoot, '.git'),
      readGoWork: () => [],
      now: () => Date.now()
    }
    
    preflightSpy = vi.spyOn(pf, 'preflightProfile').mockResolvedValue({ ready: true, missing: [] })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    fs.rmSync(tmpRoot, { recursive: true, force: true })
  })

  it('PROFILE_UNKNOWN', async () => {
    await expect(startRun({ workspaceRoot: tmpRoot, profile: 'unknown' }, deps))
      .rejects.toThrowError('PROFILE_UNKNOWN')
    expect(mockRunManager.submit).not.toHaveBeenCalled()
  })

  it('ENV_NOT_READY when all steps fail preflight', async () => {
    preflightSpy.mockResolvedValue({ ready: false, missing: [{ reason: 'tool_incompatible' }] })
    
    try {
      await startRun({ workspaceRoot: tmpRoot, profile: 'ts-lint' }, deps)
      expect.fail('should throw')
    } catch (e: any) {
      expect(e).toBeInstanceOf(QualityRunStartError)
      expect(e.code).toBe('CODEINTEL_ENV_NOT_READY')
      expect(e.reason).toBe('tool_incompatible')
      expect(mockRunManager.submit).not.toHaveBeenCalled()
    }
  })

  it('Run created if partial preflight failure', async () => {
    // We request a suite with multiple profiles.
    // If one fails and one passes, run should be created.
    preflightSpy.mockImplementation(async (prof: any) => {
      if (prof.id === 'ts-lint') return { ready: false, missing: [{ reason: 'tool_too_old' }] }
      return { ready: true, missing: [] }
    })
    
    // removed unused res // assume suite has ts-lint and ts-typecheck...
    // wait, I don't know the suite contents. Let's pass multiple profiles instead!
    const res2 = await startRun({ workspaceRoot: tmpRoot, profiles: ['ts-lint', 'ts-typecheck-desktop-node'] }, deps)
    
    expect(mockRunManager.submit).toHaveBeenCalled()
    expect(res2.steps.find(s => s.id === 'ts-lint')?.state).toBe('env_not_ready')
    expect(res2.steps.find(s => s.id === 'ts-typecheck-desktop-node')?.state).toBeUndefined()
  })

  it('Timeouts preflight if > 800ms', async () => {
    preflightSpy.mockImplementation(async () => {
      return new Promise(resolve => setTimeout(() => resolve({ ready: false, missing: [] }), 5000))
    })

    const t0 = Date.now()
    const res = await startRun({ workspaceRoot: tmpRoot, profile: 'ts-lint' }, deps)
    const elapsed = Date.now() - t0
    
    expect(elapsed).toBeLessThan(1500) // Much less than 5s!
    expect(mockRunManager.submit).toHaveBeenCalled()
    // It assumes ready if timed out
    expect(res.steps[0].state).toBeUndefined()
  })
  
  it('Handles worktree busy', async () => {
    mockRunManager.submit.mockRejectedValue(Object.assign(new Error('Worktree is busy'), { code: 'worktree_busy', runId: 'run-2' }))
    
    try {
      await startRun({ workspaceRoot: tmpRoot, profile: 'ts-lint' }, deps)
      expect.fail('should throw')
    } catch (e: any) {
      expect(e.code).toBe('WORKTREE_BUSY')
      expect(e.data.runId).toBe('run-2')
    }
  })
})
