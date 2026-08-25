/* eslint-disable max-lines -- Why: TASK-227 covers 20 re-exported git RPC
   handlers end-to-end against real git repos; splitting per-handler would
   scatter the shared repo/remote fixture without a meaningful boundary. */
import { describe, expect, it, beforeEach, afterEach } from 'vitest'
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { execFileSync } from 'node:child_process'
import type WebSocket from 'ws'
import { gitInit, gitCommit } from './git-handler-test-setup'
import {
  handleGitStatus,
  handleGitDiff,
  handleGitCommit,
  handleGitCheckout,
  handleGitLocalBranches,
  handleGitAbortRebase,
  handleGitAbortMerge,
  handleGitConflictOperation,
  handleGitDiscard,
  handleGitBulkDiscard,
  handleGitStage,
  handleGitUnstage,
  handleGitBulkStage,
  handleGitBulkUnstage
} from './agent-git-handler-local-ops'
import {
  handleGitPush,
  handleGitPull,
  handleGitFastForward,
  handleGitRebaseFromBase,
  handleGitFetch,
  handleGitUpstreamStatus
} from './agent-git-handler-remote-ops'

type JsonRpcSuccess = { jsonrpc: '2.0'; id: unknown; result: unknown }
type JsonRpcErr = { jsonrpc: '2.0'; id: unknown; error: { code: number; message: string } }

function expectSuccess<T = unknown>(response: object): T {
  const r = response as JsonRpcSuccess | JsonRpcErr
  if ('error' in r) {
    throw new Error(`expected success, got error: ${r.error.message}`)
  }
  return r.result as T
}

function git(cwd: string, args: string[]): string {
  return execFileSync('git', args, { cwd, stdio: 'pipe', encoding: 'utf-8' })
}

function currentBranch(cwd: string): string {
  return git(cwd, ['rev-parse', '--abbrev-ref', 'HEAD']).trim()
}

function configureIdentity(cwd: string): void {
  git(cwd, ['config', 'user.email', 'test@test.com'])
  git(cwd, ['config', 'user.name', 'Test'])
}

const fakeWs = {} as WebSocket

describe('agent-git-handler-extended — TASK-227 re-exports', () => {
  let repoDir: string
  let remoteDir: string
  let mainBranch: string
  const cleanupDirs: string[] = []

  beforeEach(async () => {
    repoDir = mkdtempSync(path.join(tmpdir(), 'agent-git-ext-repo-'))
    remoteDir = mkdtempSync(path.join(tmpdir(), 'agent-git-ext-remote-'))
    git(remoteDir, ['init', '--bare'])

    gitInit(repoDir)
    writeFileSync(path.join(repoDir, 'README.md'), 'hello\n')
    gitCommit(repoDir, 'initial commit')
    mainBranch = currentBranch(repoDir)
    git(repoDir, ['remote', 'add', 'origin', remoteDir])
    git(repoDir, ['push', '-u', 'origin', mainBranch])
  })

  afterEach(async () => {
    await fs.rm(repoDir, { recursive: true, force: true })
    await fs.rm(remoteDir, { recursive: true, force: true })
    for (const dir of cleanupDirs.splice(0)) {
      await fs.rm(dir, { recursive: true, force: true })
    }
  })

  function makeClone(prefix: string): string {
    const cloneDir = mkdtempSync(path.join(tmpdir(), prefix))
    execFileSync('git', ['clone', remoteDir, cloneDir], { stdio: 'pipe' })
    configureIdentity(cloneDir)
    cleanupDirs.push(cloneDir)
    return cloneDir
  }

  it('git.status reports untracked and modified files', async () => {
    await fs.writeFile(path.join(repoDir, 'untracked.txt'), 'x')
    await fs.writeFile(path.join(repoDir, 'README.md'), 'changed\n')

    const result = await expectSuccess<{ entries: { path: string }[] }>(
      await handleGitStatus(1, { worktreePath: repoDir })
    )
    const paths = result.entries.map((e) => e.path)
    expect(paths).toContain('untracked.txt')
    expect(paths).toContain('README.md')
  })

  it('git.stage / git.diff / git.commit / git.unstage round-trip', async () => {
    await fs.writeFile(path.join(repoDir, 'staged.txt'), 'staged content\n')
    await expectSuccess(await handleGitStage(2, { worktreePath: repoDir, filePath: 'staged.txt' }))

    const diffResult = await expectSuccess<{ kind: string; modifiedContent: string }>(
      await handleGitDiff(3, { worktreePath: repoDir, filePath: 'staged.txt', staged: true })
    )
    expect(diffResult.kind).toBe('text')
    expect(diffResult.modifiedContent).toContain('staged content')

    await expectSuccess(await handleGitUnstage(4, { worktreePath: repoDir, filePath: 'staged.txt' }))
    const statusAfterUnstage = await expectSuccess<{
      entries: { path: string; area: string }[]
    }>(await handleGitStatus(5, { worktreePath: repoDir }))
    expect(
      statusAfterUnstage.entries.some((e) => e.path === 'staged.txt' && e.area === 'untracked')
    ).toBe(true)

    await expectSuccess(await handleGitStage(6, { worktreePath: repoDir, filePath: 'staged.txt' }))
    const commitResult = await expectSuccess<{ success: boolean }>(
      await handleGitCommit(7, { worktreePath: repoDir, message: 'add staged.txt' }, fakeWs)
    )
    expect(commitResult.success).toBe(true)
    expect(git(repoDir, ['log', '-1', '--pretty=%s']).trim()).toBe('add staged.txt')
  })

  it('git.bulkStage / git.bulkUnstage stage and unstage multiple files', async () => {
    await fs.writeFile(path.join(repoDir, 'a.txt'), 'a')
    await fs.writeFile(path.join(repoDir, 'b.txt'), 'b')
    await expectSuccess(
      await handleGitBulkStage(8, { worktreePath: repoDir, filePaths: ['a.txt', 'b.txt'] })
    )
    expect(git(repoDir, ['diff', '--cached', '--name-only']).trim().split('\n').sort()).toEqual([
      'a.txt',
      'b.txt'
    ])

    await expectSuccess(
      await handleGitBulkUnstage(9, { worktreePath: repoDir, filePaths: ['a.txt', 'b.txt'] })
    )
    expect(git(repoDir, ['diff', '--cached', '--name-only']).trim()).toBe('')
  })

  it('git.discard restores a tracked file and removes an untracked one', async () => {
    await fs.writeFile(path.join(repoDir, 'README.md'), 'dirty\n')
    await expectSuccess(await handleGitDiscard(10, { worktreePath: repoDir, filePath: 'README.md' }))
    expect(await fs.readFile(path.join(repoDir, 'README.md'), 'utf-8')).toBe('hello\n')

    await fs.writeFile(path.join(repoDir, 'untracked.txt'), 'x')
    await expectSuccess(
      await handleGitDiscard(11, { worktreePath: repoDir, filePath: 'untracked.txt' })
    )
    await expect(fs.access(path.join(repoDir, 'untracked.txt'))).rejects.toThrow()
  })

  it('git.bulkDiscard restores multiple tracked files', async () => {
    await fs.writeFile(path.join(repoDir, 'second.txt'), 'orig\n')
    gitCommit(repoDir, 'add second.txt')
    await fs.writeFile(path.join(repoDir, 'README.md'), 'dirty2\n')
    await fs.writeFile(path.join(repoDir, 'second.txt'), 'dirty\n')
    await expectSuccess(
      await handleGitBulkDiscard(12, {
        worktreePath: repoDir,
        filePaths: ['README.md', 'second.txt']
      })
    )
    expect(await fs.readFile(path.join(repoDir, 'second.txt'), 'utf-8')).toBe('orig\n')
    expect(await fs.readFile(path.join(repoDir, 'README.md'), 'utf-8')).toBe('hello\n')
  })

  it('git.checkout / git.localBranches switch and list branches', async () => {
    git(repoDir, ['branch', 'feature'])
    const branches = await expectSuccess<{ current: string | null; branches: string[] }>(
      await handleGitLocalBranches(13, { worktreePath: repoDir })
    )
    expect(branches.branches).toContain('feature')
    expect(branches.current).toBe(mainBranch)

    const checkoutResult = await expectSuccess<{ ok: true; branch: string }>(
      await handleGitCheckout(14, { worktreePath: repoDir, branch: 'feature' })
    )
    expect(checkoutResult.branch).toBe('feature')
    expect(currentBranch(repoDir)).toBe('feature')
  })

  it('git.conflictOperation / git.abortMerge detect and abort a real merge conflict', async () => {
    git(repoDir, ['checkout', '-b', 'conflict-branch'])
    await fs.writeFile(path.join(repoDir, 'README.md'), 'branch change\n')
    gitCommit(repoDir, 'branch change')
    git(repoDir, ['checkout', mainBranch])
    await fs.writeFile(path.join(repoDir, 'README.md'), 'main change\n')
    gitCommit(repoDir, 'main change')

    try {
      git(repoDir, ['merge', 'conflict-branch'])
    } catch {
      // expected — merge conflict
    }

    const detected = await expectSuccess<string>(
      await handleGitConflictOperation(15, { worktreePath: repoDir })
    )
    expect(detected).toBe('merge')

    await expectSuccess(await handleGitAbortMerge(16, { worktreePath: repoDir }))
    const afterAbort = await expectSuccess<string>(
      await handleGitConflictOperation(17, { worktreePath: repoDir })
    )
    expect(afterAbort).toBe('unknown')
  })

  it('git.abortRebase aborts a real rebase conflict', async () => {
    git(repoDir, ['checkout', '-b', 'rebase-branch'])
    await fs.writeFile(path.join(repoDir, 'README.md'), 'rebase branch change\n')
    gitCommit(repoDir, 'rebase branch change')
    git(repoDir, ['checkout', mainBranch])
    await fs.writeFile(path.join(repoDir, 'README.md'), 'rebase main change\n')
    gitCommit(repoDir, 'rebase main change')
    git(repoDir, ['checkout', 'rebase-branch'])

    try {
      git(repoDir, ['rebase', mainBranch])
    } catch {
      // expected — rebase conflict
    }

    await expectSuccess(await handleGitAbortRebase(18, { worktreePath: repoDir }))
    const status = await expectSuccess<string>(
      await handleGitConflictOperation(19, { worktreePath: repoDir })
    )
    expect(status).toBe('unknown')
  })

  it('git.push publishes local commits to the configured remote', async () => {
    await fs.writeFile(path.join(repoDir, 'pushed.txt'), 'x\n')
    gitCommit(repoDir, 'add pushed.txt')
    await expectSuccess(await handleGitPush(20, { worktreePath: repoDir }))

    const cloneDir = makeClone('agent-git-ext-clone-')
    const files = await fs.readdir(cloneDir)
    expect(files).toContain('pushed.txt')
  })

  it('git.fetch / git.pull / git.fastForward bring in remote commits', async () => {
    const cloneDir = makeClone('agent-git-ext-clone2-')
    await fs.writeFile(path.join(cloneDir, 'from-clone.txt'), 'x\n')
    gitCommit(cloneDir, 'add from-clone.txt')
    git(cloneDir, ['push', 'origin', mainBranch])

    await expectSuccess(await handleGitFetch(21, { worktreePath: repoDir }))
    const remoteHead = git(repoDir, ['rev-parse', `origin/${mainBranch}`]).trim()
    const cloneHead = git(cloneDir, ['rev-parse', 'HEAD']).trim()
    expect(remoteHead).toBe(cloneHead)

    await expectSuccess(await handleGitPull(22, { worktreePath: repoDir }))
    expect(await fs.readFile(path.join(repoDir, 'from-clone.txt'), 'utf-8')).toBe('x\n')

    await fs.writeFile(path.join(cloneDir, 'from-clone-2.txt'), 'y\n')
    gitCommit(cloneDir, 'add from-clone-2.txt')
    git(cloneDir, ['push', 'origin', mainBranch])
    await expectSuccess(await handleGitFastForward(23, { worktreePath: repoDir }))
    expect(await fs.readFile(path.join(repoDir, 'from-clone-2.txt'), 'utf-8')).toBe('y\n')
  })

  it('git.rebaseFromBase replays local commits onto the fetched remote base', async () => {
    const cloneDir = makeClone('agent-git-ext-clone3-')
    await fs.writeFile(path.join(cloneDir, 'remote-only.txt'), 'x\n')
    gitCommit(cloneDir, 'remote-only commit')
    git(cloneDir, ['push', 'origin', mainBranch])

    git(repoDir, ['fetch', 'origin'])
    await fs.writeFile(path.join(repoDir, 'local-only.txt'), 'y\n')
    gitCommit(repoDir, 'local-only commit')

    await expectSuccess(
      await handleGitRebaseFromBase(24, { worktreePath: repoDir, baseRef: `origin/${mainBranch}` })
    )
    expect(await fs.readFile(path.join(repoDir, 'remote-only.txt'), 'utf-8')).toBe('x\n')
    expect(await fs.readFile(path.join(repoDir, 'local-only.txt'), 'utf-8')).toBe('y\n')
  })

  it('git.upstreamStatus reports ahead/behind against the configured upstream', async () => {
    const before = await expectSuccess<{ hasUpstream: boolean; ahead: number; behind: number }>(
      await handleGitUpstreamStatus(25, { worktreePath: repoDir })
    )
    expect(before.hasUpstream).toBe(true)
    expect(before.ahead).toBe(0)
    expect(before.behind).toBe(0)

    await fs.writeFile(path.join(repoDir, 'ahead.txt'), 'x\n')
    gitCommit(repoDir, 'ahead commit')
    const after = await expectSuccess<{ ahead: number; behind: number }>(
      await handleGitUpstreamStatus(26, { worktreePath: repoDir })
    )
    expect(after.ahead).toBe(1)
    expect(after.behind).toBe(0)
  })
})
