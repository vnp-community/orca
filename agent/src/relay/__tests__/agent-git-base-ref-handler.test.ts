// src/relay/__tests__/agent-git-base-ref-handler.test.ts
// Real git repos, no mocking of git. Same table as the Go executor's
// base_ref_default_test.go so both host paths stay in agreement.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { tmpdir } from 'node:os'
import { mkdtempSync, rmSync, realpathSync } from 'node:fs'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'
import { handleGitBaseRefDefault } from '../agent-git-base-ref-handler'
import { branchFromRef } from '../agent-git-base-ref-resolver'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

type Response = {
  result?: { ref?: string }
  error?: { code: number; message: string }
}

const config = { workDir: '/tmp' } as AgentConfig
const log: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

const git = (cwd: string, ...args: string[]): string =>
  execFileSync(
    'git',
    ['-c', 'user.email=t@example.com', '-c', 'user.name=T', '-c', 'commit.gpgsign=false', ...args],
    { cwd, encoding: 'utf8' }
  )

describe('handleGitBaseRefDefault', () => {
  const dirs: string[] = []
  const tmp = (): string => {
    const dir = realpathSync(mkdtempSync(join(tmpdir(), 'base-ref-default-')))
    dirs.push(dir)
    return dir
  }
  // repo with one commit on `branch`
  const repoWithCommit = (branch = 'main'): string => {
    const dir = tmp()
    git(dir, 'init', '-q', '-b', branch)
    git(dir, 'commit', '--allow-empty', '-q', '-m', 'init')
    return dir
  }
  // bare upstream whose HEAD is defaultBranch, with extra branches pushed
  const upstream = (defaultBranch: string, ...extra: string[]): string => {
    const bare = tmp()
    git(bare, 'init', '-q', '--bare', '-b', defaultBranch)
    const seed = repoWithCommit(defaultBranch)
    git(seed, 'remote', 'add', 'origin', bare)
    git(seed, 'push', '-q', 'origin', defaultBranch)
    for (const b of extra) {
      git(seed, 'branch', b)
      git(seed, 'push', '-q', 'origin', b)
    }
    return bare
  }
  const clone = (from: string): string => {
    const dest = join(tmp(), 'clone')
    git(tmp(), 'clone', '-q', from, dest)
    return dest
  }
  const resolve = async (repoPath: string, preferredBranch?: string): Promise<Response> =>
    (await handleGitBaseRefDefault(
      1,
      preferredBranch === undefined ? { repoPath } : { repoPath, preferredBranch },
      config,
      log
    )) as Response

  beforeEach(() => {
    dirs.length = 0
  })
  afterEach(() => {
    for (const d of dirs) {
      rmSync(d, { recursive: true, force: true })
    }
  })

  it('a real clone resolves origin/HEAD', async () => {
    expect((await resolve(clone(upstream('main')))).result?.ref).toBe('main')
  })

  it('keeps every segment of a slashed origin/HEAD target', async () => {
    const repo = clone(upstream('release/1.x', 'feature/other'))
    git(repo, 'checkout', '-q', '-b', 'feature/work')
    expect((await resolve(repo)).result?.ref).toBe('release/1.x')
  })

  it('origin/HEAD wins over a checked-out feature branch', async () => {
    const repo = clone(upstream('main', 'feature/other'))
    git(repo, 'checkout', '-q', 'feature/other')
    expect((await resolve(repo)).result?.ref).toBe('main')
  })

  it('a dangling origin/HEAD falls through to a real branch', async () => {
    const repo = repoWithCommit('main')
    git(repo, 'symbolic-ref', 'refs/remotes/origin/HEAD', 'refs/remotes/origin/gone')
    expect((await resolve(repo)).result?.ref).toBe('main')
  })

  it('init + remote add uses ls-remote and keeps a slashed branch', async () => {
    const repo = tmp()
    git(repo, 'init', '-q', '-b', 'feature/work')
    git(repo, 'remote', 'add', 'origin', upstream('release/1.x'))
    expect((await resolve(repo)).result?.ref).toBe('release/1.x')
  })

  it('local-only: the configured default beats the checked-out feature branch', async () => {
    const repo = repoWithCommit('main')
    git(repo, 'checkout', '-q', '-b', 'feature/other')
    expect((await resolve(repo, 'main')).result?.ref).toBe('main')
  })

  it('local-only: a slashed configured default, with or without refs/heads/', async () => {
    const repo = repoWithCommit('main')
    git(repo, 'branch', 'release/1.x')
    git(repo, 'checkout', '-q', '-b', 'feature/other')
    expect((await resolve(repo, 'release/1.x')).result?.ref).toBe('release/1.x')
    expect((await resolve(repo, 'refs/heads/release/1.x')).result?.ref).toBe('release/1.x')
  })

  it('local-only: a configured default that is not a local branch falls back to main', async () => {
    const repo = repoWithCommit('main')
    git(repo, 'checkout', '-q', '-b', 'feature/other')
    expect((await resolve(repo, 'develop')).result?.ref).toBe('main')
  })

  it('local-only without a hint prefers main over the checked-out branch', async () => {
    const repo = repoWithCommit('main')
    git(repo, 'checkout', '-q', '-b', 'feature/other')
    expect((await resolve(repo)).result?.ref).toBe('main')
  })

  it('local-only with master but no main', async () => {
    const repo = repoWithCommit('master')
    git(repo, 'checkout', '-q', '-b', 'feature/other')
    expect((await resolve(repo)).result?.ref).toBe('master')
  })

  it('main beats master', async () => {
    const repo = repoWithCommit('main')
    git(repo, 'branch', 'master')
    git(repo, 'checkout', '-q', '-b', 'feature/other')
    expect((await resolve(repo)).result?.ref).toBe('main')
  })

  it('an option-like hint is ignored', async () => {
    expect((await resolve(repoWithCommit('main'), '--upload-pack=x')).result?.ref).toBe('main')
  })

  it('a fresh init without commits returns the current branch, never a hint that cannot exist', async () => {
    const repo = tmp()
    git(repo, 'init', '-q', '-b', 'trunk')
    expect((await resolve(repo)).result?.ref).toBe('trunk')
    expect((await resolve(repo, 'main')).result?.ref).toBe('trunk')
  })

  it('an empty remote falls back to the current branch', async () => {
    const bare = tmp()
    git(bare, 'init', '-q', '--bare', '-b', 'main')
    const repo = tmp()
    git(repo, 'init', '-q', '-b', 'main')
    git(repo, 'remote', 'add', 'origin', bare)
    expect((await resolve(repo)).result?.ref).toBe('main')
  })

  it('a detached HEAD still finds main', async () => {
    const repo = repoWithCommit('main')
    git(repo, 'checkout', '-q', '--detach')
    expect((await resolve(repo)).result?.ref).toBe('main')
  })

  it('a detached HEAD with no candidate returns an error, not a crash', async () => {
    const repo = repoWithCommit('main')
    git(repo, 'checkout', '-q', '-b', 'feature/only')
    git(repo, 'branch', '-q', '-D', 'main')
    git(repo, 'checkout', '-q', '--detach')
    expect((await resolve(repo)).error?.message).toContain('git.baseRefDefault failed')
  })

  it('a folder that is not a git repo returns an error', async () => {
    expect((await resolve(tmp())).error).toBeDefined()
  })
})

describe('branchFromRef', () => {
  it.each([
    ['refs/heads/main', 'main'],
    ['refs/heads/feature/other\n', 'feature/other'],
    ['refs/remotes/origin/main', 'main'],
    ['refs/remotes/origin/release/1.x', 'release/1.x'],
    ['refs/remotes/upstream/team/a/b', 'team/a/b'],
    ['refs/remotes/origin', ''],
    ['release/1.x', 'release/1.x'],
    ['', '']
  ])('%j -> %j', (input, want) => {
    expect(branchFromRef(input)).toBe(want)
  })
})
