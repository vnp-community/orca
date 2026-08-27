import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { execFileSync } from 'node:child_process'
import { handleGitClone } from './agent-git-clone-handler'

const TEST_GIT_USER_EMAIL = 'test@example.com'
const TEST_GIT_USER_NAME = 'Test User'

function gitInit(dir: string): void {
  execFileSync('git', ['init'], { cwd: dir, stdio: 'pipe' })
  execFileSync('git', ['config', 'user.email', TEST_GIT_USER_EMAIL], { cwd: dir, stdio: 'pipe' })
  execFileSync('git', ['config', 'user.name', TEST_GIT_USER_NAME], { cwd: dir, stdio: 'pipe' })
}

function gitCommit(dir: string, message: string): void {
  execFileSync('git', ['add', '.'], { cwd: dir, stdio: 'pipe' })
  execFileSync('git', ['commit', '-m', message, '--allow-empty'], { cwd: dir, stdio: 'pipe' })
}

// Why: agent-git-clone-handler.ts is Part A's (direct-websocket/relay-websocket)
// implementation of git.clone, mirroring GitHandler.cloneSimple() (Part B) —
// see specs/agent/api/gaps-and-findings.md #5.
describe('handleGitClone', () => {
  let tmpDir: string
  let notify: ReturnType<typeof vi.fn>

  beforeEach(() => {
    tmpDir = mkdtempSync(path.join(tmpdir(), 'agent-git-clone-'))
    notify = vi.fn()
  })

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true })
  })

  it('clones a local repo via the { url, targetPath } shape', async () => {
    const sourceDir = path.join(tmpDir, 'source')
    mkdirSync(sourceDir)
    gitInit(sourceDir)
    writeFileSync(path.join(sourceDir, 'file.txt'), 'hello\n')
    gitCommit(sourceDir, 'initial')

    const targetPath = path.join(tmpDir, 'cloned')
    const response = (await handleGitClone(
      1,
      { url: sourceDir, targetPath },
      notify
    )) as { result?: { path: string } }

    expect(response.result).toEqual({ path: targetPath })
    expect(await fs.readFile(path.join(targetPath, 'file.txt'), 'utf-8')).toBe('hello\n')
    expect(notify).toHaveBeenCalledWith('git.clone.output', expect.objectContaining({ data: expect.any(String) }))
  })

  it('rejects a leading "-" in url (argv injection guard)', async () => {
    const response = (await handleGitClone(
      1,
      { url: '--upload-pack=evil', targetPath: path.join(tmpDir, 'x') },
      notify
    )) as { error?: { message: string } }

    expect(response.error?.message).toContain('invalid url')
  })

  it('rejects a leading "-" in targetPath (argv injection guard)', async () => {
    const response = (await handleGitClone(
      1,
      { url: 'https://example.com/repo.git', targetPath: '-evil' },
      notify
    )) as { error?: { message: string } }

    expect(response.error?.message).toContain('invalid targetPath')
  })

  it('rejects a missing url/targetPath', async () => {
    const response = (await handleGitClone(1, {}, notify)) as { error?: { message: string } }
    expect(response.error?.message).toContain('invalid url')
  })

  it('reports failure for a clone of a non-existent source', async () => {
    const response = (await handleGitClone(
      1,
      { url: path.join(tmpDir, 'does-not-exist'), targetPath: path.join(tmpDir, 'out') },
      notify
    )) as { error?: { message: string } }

    expect(response.error).toBeDefined()
  })
})
