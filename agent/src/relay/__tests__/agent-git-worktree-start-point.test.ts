import { execFile } from 'node:child_process'
import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { promisify } from 'node:util'
import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'
import { handleGitWorktreeAdd } from '../agent-git-worktree-handler'
import type { GitExec } from '../git-handler-ops'
import { addWorktreeOp } from '../git-handler-worktree-ops'
import { assertWorktreeOnBranch } from '../worktree-add-branch-guard'
import {
  createStartPointFixture,
  type StartPointFixture
} from './worktree-start-point-repo-fixture'

type Reply = {
  result?: { stdout: string; stderr: string; exitCode: number }
  error?: { code: number; message: string }
}

const log: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }
let fx: StartPointFixture
let config: AgentConfig
let counter = 0

beforeAll(() => {
  fx = createStartPointFixture()
  config = {
    mode: 'direct-websocket',
    orcaUrl: '',
    orcaHttpUrl: '',
    apiSecret: '',
    agentToken: '',
    agentPort: 6799,
    devServerId: 'test',
    logLevel: 'info',
    workDir: fx.clone,
    toolPath: '',
    toolEnv: fx.env,
    credentialDir: join(fx.root, 'creds'),
    tlsRejectUnauthorized: true
  }
})
afterAll(() => fx?.cleanup())

async function addTaskWorktree(baseRef: string | undefined, branch = `task/${++counter}`) {
  const path = join(fx.root, `wt-${counter}`)
  const reply = (await handleGitWorktreeAdd(
    1,
    { path, branch, createBranch: true, cwd: fx.clone, ...(baseRef ? { baseRef } : {}) },
    config,
    log
  )) as Reply
  return { reply, path, branch }
}

function expectOnNewBranchAt(path: string, branch: string, sha: string): void {
  expect(fx.git(path, 'symbolic-ref', '--short', 'HEAD')).toBe(branch)
  expect(fx.git(path, 'rev-parse', 'HEAD')).toBe(sha)
}

describe('handleGitWorktreeAdd start point resolution (real git)', () => {
  it('reproduces the original bug with plain git, and the handler does not', async () => {
    const plainPath = join(fx.root, 'wt-plain-git')
    fx.git(fx.clone, 'worktree', 'add', '-b', 'task/plain', plainPath, 'release/y')
    // Bare `release/y` (remote-only) makes git check out a fresh LOCAL release/y
    // and ignore -b: the exact failure the resolver exists to prevent.
    expect(fx.git(plainPath, 'symbolic-ref', '--short', 'HEAD')).not.toBe('task/plain')
    fx.git(fx.clone, 'worktree', 'remove', '--force', plainPath)
    fx.git(fx.clone, 'branch', '-D', 'release/y')
    expect(fx.git(fx.clone, 'branch', '--list', 'task/plain')).toBe('')

    const { reply, path, branch } = await addTaskWorktree('release/y')
    expect(reply.result?.exitCode).toBe(0)
    expectOnNewBranchAt(path, branch, fx.sha['origin:release/y'])
    expect(fx.git(fx.clone, 'branch', '--list', 'release/y')).toBe('')
  })

  it.each([
    ['a local branch', 'release/x', 'local:release/x'],
    ['a remote-only branch (bare name)', 'release/y', 'origin:release/y'],
    ['a remote-only branch (remote/name)', 'origin/release/y', 'origin:release/y'],
    ['a tag', 'v1', 'tag:v1'],
    ['a local branch shadowing a remote one', 'release/dup', 'local:release/dup'],
    ['the remote copy of a shadowed branch', 'origin/release/dup', 'origin:release/dup'],
    [
      'a branch on a single non-origin remote via remote/name',
      'upstream/feature/z',
      'upstream:feature/z'
    ],
    ['an already qualified ref', 'refs/remotes/origin/release/y', 'origin:release/y']
  ])('starts the new branch from %s while another branch is checked out', async (_, ref, key) => {
    const { reply, path, branch } = await addTaskWorktree(ref)
    expect(reply.result?.exitCode).toBe(0)
    expectOnNewBranchAt(path, branch, fx.sha[key])
    expect(fx.git(fx.clone, 'symbolic-ref', '--short', 'HEAD')).toBe('other')
  })

  it('accepts full and abbreviated commit ids', async () => {
    const full = fx.sha['tag:v1']
    for (const ref of [full, full.slice(0, 10)]) {
      const { reply, path, branch } = await addTaskWorktree(ref)
      expect(reply.result?.exitCode).toBe(0)
      expectOnNewBranchAt(path, branch, full)
    }
  })

  it('falls back to the current HEAD when no base ref is given', async () => {
    const { reply, path, branch } = await addTaskWorktree(undefined)
    expect(reply.result?.exitCode).toBe(0)
    expectOnNewBranchAt(path, branch, fx.sha['local:other'])
  })

  it('refuses a name that exists on several remotes instead of guessing', async () => {
    const { reply, path } = await addTaskWorktree('feature/z')
    expect(reply.error?.code).toBe(-32602)
    expect(reply.error?.message).toMatch(/ambiguous.*fork, upstream|ambiguous.*upstream, fork/)
    expect(existsSync(path)).toBe(false)
  })

  it('rejects a base ref that would be read as a git flag', async () => {
    const { reply, path } = await addTaskWorktree('--detach')
    expect(reply.error?.code).toBe(-32602)
    expect(existsSync(path)).toBe(false)
  })

  it('rejects a branch name that would be read as a git flag', async () => {
    const { reply, path } = await addTaskWorktree('release/x', '-x')
    expect(reply.error?.code).toBe(-32602)
    expect(existsSync(path)).toBe(false)
  })

  it('reports an unknown base ref the way git-gateway-service classifies it', async () => {
    const { reply, path } = await addTaskWorktree('release/nope')
    expect(reply.result?.exitCode).toBe(128)
    expect(reply.result?.stderr).toContain('invalid reference')
    expect(existsSync(path)).toBe(false)
  })

  it('does not create the worktree when git itself is unusable for probing', async () => {
    const reply = (await handleGitWorktreeAdd(
      1,
      {
        path: join(fx.root, 'wt-not-a-repo'),
        branch: 'task/x',
        createBranch: true,
        baseRef: 'main',
        cwd: fx.root
      },
      { ...config, workDir: fx.root },
      log
    )) as Reply
    // Not-a-repo (exit 128) must surface as an error, never as "ref not found".
    expect(reply.error?.message ?? reply.result?.stderr ?? '').not.toContain('invalid reference')
  })
})

describe('assertWorktreeOnBranch (real git)', () => {
  const run = async (args: string[], cwd: string): Promise<string> =>
    (await promisify(execFile)('git', args, { cwd, env: fx.env })).stdout

  it('passes for a worktree on the requested branch', async () => {
    const path = join(fx.root, 'wt-guard-ok')
    fx.git(fx.clone, 'worktree', 'add', '-q', '-b', 'task/guard-ok', path, 'main')
    await expect(
      assertWorktreeOnBranch(run, fx.clone, path, 'task/guard-ok')
    ).resolves.toBeUndefined()
  })

  it('removes a worktree that ended up on another branch and reports it', async () => {
    const path = join(fx.root, 'wt-guard-wrong')
    fx.git(fx.clone, 'worktree', 'add', '-q', '-b', 'task/guard-other', path, 'main')
    await expect(assertWorktreeOnBranch(run, fx.clone, path, 'task/guard-want')).rejects.toThrow(
      /instead of the requested branch "task\/guard-want"; the worktree was removed/
    )
    expect(existsSync(path)).toBe(false)
  })
})

describe('addWorktreeOp start point resolution (real git, SSH path)', () => {
  const git: GitExec = async (args, cwd) => {
    const { stdout, stderr } = await promisify(execFile)('git', args, { cwd, env: fx.env })
    return { stdout, stderr }
  }

  it('bases a task branch on a remote-only release branch without checking it out', async () => {
    const targetDir = join(fx.root, 'wt-ssh')
    await addWorktreeOp(git, {
      repoPath: fx.clone,
      branchName: 'task/ssh',
      targetDir,
      base: 'release/y'
    })
    expectOnNewBranchAt(targetDir, 'task/ssh', fx.sha['origin:release/y'])
    expect(fx.git(fx.clone, 'branch', '--list', 'release/y')).toBe('')
  })

  it('fails with the ambiguity error and creates nothing', async () => {
    const targetDir = join(fx.root, 'wt-ssh-ambiguous')
    await expect(
      addWorktreeOp(git, {
        repoPath: fx.clone,
        branchName: 'task/ssh-ambiguous',
        targetDir,
        base: 'feature/z'
      })
    ).rejects.toThrow(/ambiguous/)
    expect(existsSync(targetDir)).toBe(false)
  })
})
