import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { execFileSync } from 'node:child_process'
import { handleGitInit } from './agent-git-init-handler'

// Why: agent-git-init-handler.ts is Part A's (direct-websocket/relay-websocket)
// implementation of git.init — added for the "Initialize as Git repo" feature
// (a repo added to Orca whose folder isn't a git repository yet). Mirrors
// agent-git-clone-handler.test.ts's shape/fixtures exactly.
describe('handleGitInit', () => {
  let tmpDir: string
  let notify: (method: string, params: Record<string, unknown>) => void

  beforeEach(() => {
    tmpDir = mkdtempSync(path.join(tmpdir(), 'agent-git-init-'))
    notify = vi.fn()
  })

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true })
  })

  it('initializes a git repo at an existing non-git folder', async () => {
    const destPath = path.join(tmpDir, 'existing-folder')
    await fs.mkdir(destPath, { recursive: true })
    await fs.writeFile(path.join(destPath, 'README.md'), '# hi\n')

    const response = (await handleGitInit(1, { destPath }, notify)) as {
      result?: { path: string; defaultBranch: string; remoteAdded: boolean }
    }

    expect(response.result?.path).toBe(destPath)
    expect(response.result?.remoteAdded).toBe(false)
    // .git directory now exists
    const stat = await fs.stat(path.join(destPath, '.git'))
    expect(stat.isDirectory()).toBe(true)
    expect(notify).toHaveBeenCalledWith(
      'git.init.output',
      expect.objectContaining({ data: expect.any(String) })
    )
  })

  it('creates destPath if it does not exist yet (mkdir -p semantics)', async () => {
    const destPath = path.join(tmpDir, 'nested', 'new-folder')
    const response = (await handleGitInit(1, { destPath }, notify)) as { result?: { path: string } }
    expect(response.result?.path).toBe(destPath)
    const stat = await fs.stat(path.join(destPath, '.git'))
    expect(stat.isDirectory()).toBe(true)
  })

  it('is idempotent against an already-initialized git repo', async () => {
    const destPath = path.join(tmpDir, 'already-a-repo')
    await fs.mkdir(destPath, { recursive: true })
    execFileSync('git', ['init'], { cwd: destPath, stdio: 'pipe' })

    const response = (await handleGitInit(1, { destPath }, notify)) as { error?: unknown }
    expect(response.error).toBeUndefined()
  })

  it('respects an explicit defaultBranch', async () => {
    const destPath = path.join(tmpDir, 'branch-folder')
    await fs.mkdir(destPath, { recursive: true })

    const response = (await handleGitInit(1, { destPath, defaultBranch: 'trunk' }, notify)) as {
      result?: { defaultBranch: string }
    }

    expect(response.result?.defaultBranch).toBe('trunk')
    const branch = execFileSync('git', ['symbolic-ref', '--short', 'HEAD'], {
      cwd: destPath,
      encoding: 'utf-8'
    }).trim()
    expect(branch).toBe('trunk')
  })

  it('adds a remote in the same call when remoteUrl is given', async () => {
    const destPath = path.join(tmpDir, 'with-remote')
    await fs.mkdir(destPath, { recursive: true })

    const response = (await handleGitInit(
      1,
      { destPath, remoteUrl: 'https://example.com/org/repo.git' },
      notify
    )) as { result?: { remoteAdded: boolean } }

    expect(response.result?.remoteAdded).toBe(true)
    const remoteUrl = execFileSync('git', ['remote', 'get-url', 'origin'], {
      cwd: destPath,
      encoding: 'utf-8'
    }).trim()
    expect(remoteUrl).toBe('https://example.com/org/repo.git')
  })

  it('uses a custom remoteName when given', async () => {
    const destPath = path.join(tmpDir, 'with-custom-remote')
    await fs.mkdir(destPath, { recursive: true })

    await handleGitInit(
      1,
      { destPath, remoteUrl: 'https://example.com/org/repo.git', remoteName: 'upstream' },
      notify
    )

    const remoteUrl = execFileSync('git', ['remote', 'get-url', 'upstream'], {
      cwd: destPath,
      encoding: 'utf-8'
    }).trim()
    expect(remoteUrl).toBe('https://example.com/org/repo.git')
  })

  it('rejects a missing destPath', async () => {
    const response = (await handleGitInit(1, {}, notify)) as { error?: { message: string } }
    expect(response.error?.message).toContain('invalid destPath')
  })

  it('rejects a leading "-" in destPath (argv injection guard)', async () => {
    const response = (await handleGitInit(1, { destPath: '-evil' }, notify)) as {
      error?: { message: string }
    }
    expect(response.error?.message).toContain('invalid destPath')
  })

  it('rejects a leading "-" in defaultBranch (argv injection guard)', async () => {
    const destPath = path.join(tmpDir, 'evil-branch')
    const response = (await handleGitInit(
      1,
      { destPath, defaultBranch: '--upload-pack=evil' },
      notify
    )) as { error?: { message: string } }
    expect(response.error?.message).toContain('invalid defaultBranch')
  })

  it('rejects a leading "-" in remoteUrl (argv injection guard)', async () => {
    const destPath = path.join(tmpDir, 'evil-remote')
    const response = (await handleGitInit(
      1,
      { destPath, remoteUrl: '--upload-pack=evil' },
      notify
    )) as { error?: { message: string } }
    expect(response.error?.message).toContain('invalid remoteUrl')
  })
})
