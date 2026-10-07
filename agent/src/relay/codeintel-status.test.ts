import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { handleStatus } from './codeintel-status'
import { CodeIntelRequestContext } from './codeintel-method-table'
import { resetCodeIntelToolDetectionCache } from './codeintel-tool-detection'
import { invalidateRepoBindings } from './codeintel-repo-resolution'
import { PerfCollector } from './codeintel-result-envelope'
import { execSync } from 'child_process'

describe('codeintel-status', () => {
  const tmpDir = os.tmpdir()
  let repoDir: string
  let binDir: string
  let mockCtx: CodeIntelRequestContext
  const originalPlatform = process.platform

  beforeEach(() => {
    repoDir = fs.mkdtempSync(path.join(tmpDir, 'orca-repo-'))
    execSync('git init', { cwd: repoDir })
    
    binDir = fs.mkdtempSync(path.join(tmpDir, 'orca-bin-'))
    const gn = path.join(binDir, 'gitnexus')
    fs.writeFileSync(gn, '#!/bin/sh\necho "gitnexus version 1.6.9"')
    fs.chmodSync(gn, 0o755)

    mockCtx = {
      config: { toolPath: binDir, toolEnv: {} } as any,
      log: { info: vi.fn(), error: vi.fn(), warn: vi.fn() } as any,
      signal: new AbortController().signal,
      deadline: Date.now() + 10000,
      notifier: { notify: vi.fn() },
      perf: new PerfCollector()
    }

    resetCodeIntelToolDetectionCache()
    invalidateRepoBindings()
    Object.defineProperty(process, 'platform', { value: 'linux' })
  })

  afterEach(() => {
    try { fs.rmSync(repoDir, { recursive: true, force: true }) } catch {}
    try { fs.rmSync(binDir, { recursive: true, force: true }) } catch {}
    Object.defineProperty(process, 'platform', { value: originalPlatform })
  })

  it('returns success even if repo not registered', async () => {
    const res = await handleStatus({ workspaceRoot: repoDir }, mockCtx)
    expect(res.sources).toEqual(expect.arrayContaining([
      expect.objectContaining({ tool: 'gitnexus', state: 'unknown' }),
      expect.objectContaining({ tool: 'codegraph', state: 'unknown' })
    ]))
    expect(res.data.tools.gitnexus.available).toBe(true)
    expect(res.data.tools.codegraph.available).toBe(false)
  })

  it('returns unsupported_platform on win32', async () => {
    Object.defineProperty(process, 'platform', { value: 'win32' })
    await expect(handleStatus({ workspaceRoot: repoDir }, mockCtx)).rejects.toThrowError(/Code Intel is not supported on Windows/)
  })
})
