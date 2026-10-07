import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { handleStatus } from './codeintel-status'
import { CodeIntelRequestContext } from './codeintel-method-table'
import { resetCodeIntelToolDetectionCache } from './codeintel-tool-detection'
import * as repoModule from './codeintel-repo-resolution'
import { PerfCollector } from './codeintel-result-envelope'
import { execSync } from 'child_process'
import * as probeModule from './codeintel-index-basis-probe'
import * as methodTableModule from './codeintel-method-table'
import { CodeIntelError } from './codeintel-errors'

vi.mock('./codeintel-index-basis-probe', async () => {
  const actual = await vi.importActual('./codeintel-index-basis-probe')
  return {
    ...actual as any,
    probeIndexBasis: vi.fn()
  }
})

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
    repoModule.invalidateRepoBindings()
    Object.defineProperty(process, 'platform', { value: 'linux' })

    vi.spyOn(repoModule, 'resolveCodeIntelRepo').mockResolvedValue({ indexRoot: repoDir, worktreeMismatch: false })
    vi.spyOn(methodTableModule, 'getIndexProbes').mockReturnValue([
      {
        tool: 'gitnexus',
        probe: async () => ({
          state: 'ready',
          version: '1.6.9',
          indexedAt: Date.now(),
          commit: 'abcdef',
          lineBase: 1,
          indexRoot: repoDir
        }) as any
      },
      {
        tool: 'codegraph',
        probe: async () => ({
          state: 'ready',
          version: '1.5.0',
          indexedAt: Date.now(),
          commit: null,
          lineBase: 1,
          indexRoot: repoDir,
          pendingChanges: { added: 0, modified: 0, removed: 0 }
        }) as any
      }
    ])
  })

  afterEach(() => {
    vi.restoreAllMocks()
    try { fs.rmSync(repoDir, { recursive: true, force: true }) } catch {}
    try { fs.rmSync(binDir, { recursive: true, force: true }) } catch {}
    Object.defineProperty(process, 'platform', { value: originalPlatform })
  })

  it('returns success even if repo not registered', async () => {
    vi.spyOn(repoModule, 'resolveCodeIntelRepo').mockRejectedValueOnce(new CodeIntelError('CODEINTEL_REPO_NOT_REGISTERED', 'repo not found', { reason: 'not_found' }));
    const res = await handleStatus({ workspaceRoot: '/non/existent/repo' }, mockCtx)
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

  it('checkout chính sạch -> exact/fresh', async () => {
    vi.mocked(probeModule.probeIndexBasis).mockResolvedValue({
      headCommit: 'abcdef',
      headCommitTimeMs: Date.now() - 1000,
      mergeBase: 'abcdef',
      mergeBaseCommitTimeMs: Date.now() - 1000,
      changedFilesNotInIndex: 0,
      dirtySinceIndex: false
    })

    const res = await handleStatus({ workspaceRoot: repoDir }, mockCtx)
    
    const gn = res.sources.find(s => s.tool === 'gitnexus')
    expect(gn?.indexScope).toBe('exact')
    expect(gn?.freshness).toBe('fresh')
    
    const cg = res.sources.find(s => s.tool === 'codegraph')
    expect(cg?.indexScope).toBe('exact')
    expect(cg?.freshness).toBe('fresh')
  })

  it('worktree liên kết -> repo_root', async () => {
    const mainRepoDir = fs.mkdtempSync(path.join(tmpDir, 'orca-repo-main-'))
    
    vi.spyOn(methodTableModule, 'getIndexProbes').mockReturnValue([
      {
        tool: 'gitnexus',
        probe: async () => ({
          state: 'ready',
          version: '1.6.9',
          indexedAt: Date.now(),
          commit: 'abcdef',
          lineBase: 1,
          indexRoot: mainRepoDir // different from repoDir
        }) as any
      }
    ])

    vi.mocked(probeModule.probeIndexBasis).mockResolvedValue({
      headCommit: '123456',
      headCommitTimeMs: Date.now(),
      mergeBase: 'abcdef',
      mergeBaseCommitTimeMs: Date.now() - 1000,
      changedFilesNotInIndex: 1,
      dirtySinceIndex: true
    })

    const res = await handleStatus({ workspaceRoot: repoDir }, mockCtx)
    
    const gn = res.sources.find(s => s.tool === 'gitnexus')
    expect(gn?.indexScope).toBe('repo_root')
    expect(gn?.freshness).toBe('fresh_base')
    expect(gn?.rootMismatch).toBeNull() // because binding is mocked internally in resolveCodeIntelRepo without worktreeMismatch? Wait, let's see.

    // To test worktreeMismatch from binding, we need to mock resolveCodeIntelRepo or just set up a real worktree.
    fs.rmSync(mainRepoDir, { recursive: true, force: true })
  })

  it('baseRef xấu -> INVALID_PARAMS', () => {
    expect(() => methodTableModule.CODEINTEL_METHODS['codeintel.status'].validate({ workspaceRoot: repoDir, baseRef: '-invalid' })).toThrowError(/cannot start with '-'|Invalid baseRef format/)
    expect(() => methodTableModule.CODEINTEL_METHODS['codeintel.status'].validate({ workspaceRoot: repoDir, baseRef: 'origin/HEAD' })).not.toThrow()
  })
})
