import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { execSync } from 'child_process'
import { resolveCodeIntelRepo, invalidateRepoBindings } from '../codeintel-repo-resolution'
import { readGitNexusRegistry, findRegistryEntry } from '../gitnexus-registry-reader'
import { tailForStderr } from '../codeintel-secret-redaction'
import { CodeIntelError } from '../codeintel-errors'

describe('security-repo-resolution (Task 072-06)', () => {
  let tmpRoot: string
  let homeDir: string
  let parentRepoDir: string
  let nestedWorktreeDir: string
  let mockConfig: any

  beforeEach(() => {
    tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-security-repo-'))
    homeDir = path.join(tmpRoot, 'home')
    fs.mkdirSync(path.join(homeDir, '.gitnexus'), { recursive: true })

    parentRepoDir = path.join(tmpRoot, 'parent-repo')
    fs.mkdirSync(parentRepoDir, { recursive: true })
    execSync('git init -b main', { cwd: parentRepoDir })
    execSync('git config user.email "test@example.com"', { cwd: parentRepoDir })
    execSync('git config user.name "Test"', { cwd: parentRepoDir })
    fs.writeFileSync(path.join(parentRepoDir, 'file.txt'), 'hello')
    execSync('git add . && git commit -m "initial"', { cwd: parentRepoDir })

    // Create nested worktree in parent-repo/.claude/worktrees/wt1
    nestedWorktreeDir = path.join(parentRepoDir, '.claude', 'worktrees', 'wt1')
    fs.mkdirSync(path.dirname(nestedWorktreeDir), { recursive: true })
    execSync(`git worktree add "${nestedWorktreeDir}" -b wt1`, { cwd: parentRepoDir })

    // Registry only contains parent-repo
    const registryData = [
      { path: fs.realpathSync.native(parentRepoDir), indexedAt: 100 }
    ]
    fs.writeFileSync(
      path.join(homeDir, '.gitnexus', 'registry.json'),
      JSON.stringify(registryData)
    )

    mockConfig = {
      toolEnv: { HOME: homeDir }
    }

    invalidateRepoBindings()
  })

  afterEach(() => {
    try {
      execSync(`git worktree remove --force "${nestedWorktreeDir}"`, { cwd: parentRepoDir })
    } catch {}
    fs.rmSync(tmpRoot, { recursive: true, force: true })
    invalidateRepoBindings()
  })

  it('Nested worktree resolution correctly identifies linkedWorktree and does not conflate paths', async () => {
    const res = await resolveCodeIntelRepo(nestedWorktreeDir, mockConfig)
    expect(res.linkedWorktree).toBe(true)
    expect(res.toplevel).toBe(fs.realpathSync.native(nestedWorktreeDir))
    // Registry path points to main worktree since nested worktree has not been separately indexed
    expect(res.gitNexusRegistryPath).toBe(fs.realpathSync.native(parentRepoDir))
    expect(res.worktreeMismatch).toBe(true)
  })

  it('Handles duplicate registry entries by picking latest indexedAt and emitting warning', () => {
    const warnSpy = vi.fn()
    const log = { warn: warnSpy }
    const realParent = fs.realpathSync.native(parentRepoDir)

    const entries = [
      { path: realParent, indexedAt: 50 },
      { path: realParent, indexedAt: 200 },
      { path: realParent, indexedAt: 100 }
    ]

    const matched = findRegistryEntry(entries, realParent, log)
    expect(matched).toBeDefined()
    expect(matched?.indexedAt).toBe(200)
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('registry_duplicate_path'))
  })

  it('Rejects corrupt or non-array registry with CODEINTEL_TOOL_FAILED reason="registry_unreadable"', () => {
    fs.writeFileSync(path.join(homeDir, '.gitnexus', 'registry.json'), 'CORRUPT_JSON_DATA{')

    expect(() => {
      readGitNexusRegistry(mockConfig)
    }).toThrowError(CodeIntelError)

    try {
      readGitNexusRegistry(mockConfig)
    } catch (err: any) {
      expect(err.code).toBe('CODEINTEL_TOOL_FAILED')
      expect(err.data?.reason).toBe('registry_unreadable')
    }
  })

  it('Does not falsely match similar repo names (e.g., /repo vs /repo-extra)', () => {
    const realParent = fs.realpathSync.native(parentRepoDir)
    const similarDir = realParent + '-extra'

    const entries = [{ path: realParent, indexedAt: 100 }]
    const matched = findRegistryEntry(entries, similarDir)
    expect(matched).toBeUndefined()
  })

  it('TestToolStderrNeverForwarded: redacts home directory, other repos in Available list, and caps at 2KiB', () => {
    const rawStderr = `Error: repository not found at /users/secret-user/repo
Available repositories: secret-proj-alpha, secret-proj-beta, orca-internal
Stack trace at /users/secret-user/.nvm/versions/node/v20/bin/gitnexus`

    const tail = tailForStderr(rawStderr, { home: '/users/secret-user' })

    expect(tail).not.toContain('/users/secret-user')
    expect(tail).toContain('~')
    expect(tail).not.toContain('secret-proj-alpha')
    expect(tail).not.toContain('secret-proj-beta')
    expect(tail).toContain('Available: <REDACTED>')

    // Also test truncation for stderrs exceeding 2KiB
    const longStderr = `Error at /users/secret-user/repo: Available: foo\n` + 'x'.repeat(3000)
    const longTail = tailForStderr(longStderr, { home: '/users/secret-user' })
    expect(Buffer.byteLength(longTail, 'utf8')).toBeLessThanOrEqual(2048 + 3)
  })
})
