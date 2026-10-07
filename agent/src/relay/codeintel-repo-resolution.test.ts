import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { execSync } from 'child_process'
import { resolveCodeIntelRepo, invalidateRepoBindings } from './codeintel-repo-resolution'
import { AgentConfig } from './agent-config'
import { CodeIntelError } from './codeintel-errors'
import { getCodeIntelGitCapabilities } from './codeintel-git-capabilities'

describe('codeintel-repo-resolution', () => {
  const tmpDir = os.tmpdir()
  let mockConfig: AgentConfig
  let testRepoDir: string
  let homeDir: string

  beforeEach(() => {
    testRepoDir = fs.mkdtempSync(path.join(tmpDir, 'orca-repo-'))
    execSync('git init', { cwd: testRepoDir })
    
    homeDir = fs.mkdtempSync(path.join(tmpDir, 'orca-home-'))
    fs.mkdirSync(path.join(homeDir, '.gitnexus'), { recursive: true })
    fs.writeFileSync(
      path.join(homeDir, '.gitnexus', 'registry.json'),
      JSON.stringify([{ path: fs.realpathSync.native(testRepoDir), indexedAt: 100 }])
    )

    mockConfig = {
      toolEnv: { HOME: homeDir }
    } as any

    getCodeIntelGitCapabilities().clear()
    invalidateRepoBindings()
  })

  afterEach(() => {
    try { fs.rmSync(testRepoDir, { recursive: true, force: true }) } catch {}
    try { fs.rmSync(homeDir, { recursive: true, force: true }) } catch {}
  })

  it('throws PATH_NOT_ALLOWED if path does not exist', async () => {
    await expect(resolveCodeIntelRepo('/nonexistent', mockConfig)).rejects.toThrowError(CodeIntelError)
  })

  it('throws PATH_NOT_ALLOWED if not a git worktree', async () => {
    const emptyDir = fs.mkdtempSync(path.join(tmpDir, 'empty-'))
    try {
      await expect(resolveCodeIntelRepo(emptyDir, mockConfig)).rejects.toThrowError(/Failed to read git repository/)
    } finally {
      fs.rmSync(emptyDir, { recursive: true, force: true })
    }
  })

  it('resolves normal git repo in registry', async () => {
    const res = await resolveCodeIntelRepo(testRepoDir, mockConfig)
    expect(res.toplevel).toBe(fs.realpathSync.native(testRepoDir))
    expect(res.gitNexusRegistryPath).toBe(fs.realpathSync.native(testRepoDir))
    expect(res.linkedWorktree).toBe(false)
  })

  it('throws REPO_NOT_REGISTERED if neither registry nor codegraph', async () => {
    const repo2 = fs.mkdtempSync(path.join(tmpDir, 'repo2-'))
    execSync('git init', { cwd: repo2 })
    
    try {
      await expect(resolveCodeIntelRepo(repo2, mockConfig)).rejects.toThrowError(/Repository not indexed/)
    } finally {
      fs.rmSync(repo2, { recursive: true, force: true })
    }
  })

  it('resolves if codegraph db exists', async () => {
    const repo2 = fs.mkdtempSync(path.join(tmpDir, 'repo2-'))
    execSync('git init', { cwd: repo2 })
    fs.mkdirSync(path.join(repo2, '.codegraph'), { recursive: true })
    fs.writeFileSync(path.join(repo2, '.codegraph', 'codegraph.db'), 'mock')

    try {
      const res = await resolveCodeIntelRepo(repo2, mockConfig)
      expect(res.toplevel).toBe(fs.realpathSync.native(repo2))
      expect(res.codeGraphDbPath).toBe(path.join(fs.realpathSync.native(repo2), '.codegraph', 'codegraph.db'))
    } finally {
      fs.rmSync(repo2, { recursive: true, force: true })
    }
  })
})
