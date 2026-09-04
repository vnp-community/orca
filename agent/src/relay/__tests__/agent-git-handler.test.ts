// src/relay/__tests__/agent-git-handler.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { tmpdir } from 'node:os'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { execSync } from 'node:child_process'
import {
  validateGitArgs,
  GitValidationError,
  handleGitExec,
  handleGitWorktreeList,
  handleGitWorktreeAdd,
  handleGitWorktreeRemove,
  handleGitBaseRefDefault,
  handleGitSearchRefs,
} from '../agent-git-handler'
import { setConnectionGitIdentity } from '../git-identity-registry'
import { registerTraceSink, type TraceEvent } from '../../shared/trace'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

const mockConfig: AgentConfig = {
  mode: 'direct-websocket',
  orcaUrl: '',
  agentToken: '',
  agentPort: 6799,
  devServerId: 'test',
  logLevel: 'info',
  workDir: '/tmp',
  toolPath: '/usr/bin',
  toolEnv: { PATH: '/usr/bin:/usr/local/bin' },
  credentialDir: '/tmp/.creds',
  tlsRejectUnauthorized: true,
}

const mockLog: AgentLogger = { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() }

// ─── validateGitArgs — allowed subcommands ────────────────────────────────────
describe('validateGitArgs — allowed subcommands', () => {
  it.each([
    'status', 'diff', 'log', 'commit', 'push', 'pull', 'fetch',
    'branch', 'checkout', 'merge', 'rebase', 'stash', 'add', 'restore',
  ])('allows "%s"', (cmd) => {
    expect(() => validateGitArgs([cmd])).not.toThrow()
  })

  it('allows multi-arg commands with valid flags', () => {
    expect(() => validateGitArgs(['log', '--oneline', '-10'])).not.toThrow()
  })

  it('allows "origin/main" (forward slash in arg is OK)', () => {
    expect(() => validateGitArgs(['diff', 'origin/main'])).not.toThrow()
  })

  it('allows "rev-parse" (hyphenated subcommand)', () => {
    expect(() => validateGitArgs(['rev-parse', 'HEAD'])).not.toThrow()
  })
})

// ─── validateGitArgs — disallowed subcommands ─────────────────────────────────
describe('validateGitArgs — disallowed subcommands', () => {
  it('throws GIT_NO_SUBCOMMAND on empty args', () => {
    let err: GitValidationError | null = null
    try { validateGitArgs([]) } catch (e) { err = e as GitValidationError }
    expect(err).toBeInstanceOf(GitValidationError)
    expect(err!.code).toBe('GIT_NO_SUBCOMMAND')
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for "clean"', () => {
    let err: GitValidationError | null = null
    try { validateGitArgs(['clean', '-fd']) } catch (e) { err = e as GitValidationError }
    expect(err!.code).toBe('GIT_DISALLOWED_SUBCOMMAND')
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for "bisect"', () => {
    let err: GitValidationError | null = null
    try { validateGitArgs(['bisect']) } catch (e) { err = e as GitValidationError }
    expect(err!.code).toBe('GIT_DISALLOWED_SUBCOMMAND')
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for "gc"', () => {
    expect(() => validateGitArgs(['gc'])).toThrow(GitValidationError)
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for "init"', () => {
    expect(() => validateGitArgs(['init'])).toThrow(GitValidationError)
  })
})

// ─── validateGitArgs — shell metacharacter checks ────────────────────────────
describe('validateGitArgs — shell metacharacter rejection', () => {
  it.each(['&', '|', ';', '$', '`', '<', '>', '!'])(
    'rejects arg containing "%s"', (char) => {
      let err: GitValidationError | null = null
      try { validateGitArgs(['log', `--format=${char}evil`]) } catch (e) { err = e as GitValidationError }
      expect(err!.code).toBe('GIT_SHELL_METACHARACTER_IN_ARG')
    }
  )

  it('rejects backslash in arg', () => {
    expect(() => validateGitArgs(['log', '--format=\\n'])).toThrow(GitValidationError)
  })

  it('allows args with no metacharacters', () => {
    expect(() => validateGitArgs(['log', '--format=%H', '--no-walk'])).not.toThrow()
  })
})

// ─── handleGitExec — validation rejection ────────────────────────────────────
describe('handleGitExec — validation errors', () => {
  it('returns InvalidParams (-32602) for empty args', async () => {
    const resp = await handleGitExec(1, { args: [] }, mockConfig, mockLog) as any
    expect(resp.error).toBeDefined()
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for disallowed subcommand', async () => {
    const resp = await handleGitExec(1, { args: ['clean', '-fd'] }, mockConfig, mockLog) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for metacharacter in arg', async () => {
    const resp = await handleGitExec(1, { args: ['log', '--format=$HOME'] }, mockConfig, mockLog) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('does NOT crash on invalid args — always returns object', async () => {
    const resp = await handleGitExec(99, { args: [] }, mockConfig, mockLog)
    expect(typeof resp).toBe('object')
    expect(resp).not.toBeNull()
  })
})

// ─── validateGitArgs — worktree subcommand ────────────────────────────────────
describe('validateGitArgs — worktree subcommand', () => {
  it('allows "worktree list"', () => {
    expect(() => validateGitArgs(['worktree', 'list'])).not.toThrow()
  })

  it('allows "worktree add" with path and branch', () => {
    expect(() => validateGitArgs(['worktree', 'add', '/tmp/wt', 'my-feature'])).not.toThrow()
  })

  it('allows "worktree remove" with path', () => {
    expect(() => validateGitArgs(['worktree', 'remove', '/tmp/wt'])).not.toThrow()
  })
})

// ─── handleGitExec — integration ──────────────────────────────────────────────
describe('handleGitExec — integration', () => {
  it('returns a defined response for "git status" in /tmp (non-git dir)', async () => {
    const resp = await handleGitExec(1, { args: ['status'], cwd: '/tmp' }, mockConfig, mockLog) as any
    expect(resp).toBeDefined()
    // non-git dir returns error (exit 128) or result — both valid
    if (resp.result !== undefined) {
      expect(typeof resp.result.exitCode).toBe('number')
    } else {
      expect(resp.error.code).toBeDefined()
    }
  })

  it('result has jsonrpc 2.0 format', async () => {
    const resp = await handleGitExec(1, { args: ['status'] }, mockConfig, mockLog) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.id).toBe(1)
  })
})

// handleGitPrCreate was removed (BUG-AG-HLD-004) — 'git.pr.create' now routes
// to handleGitHubPrCreate (external-api-connector.ts), which has its own
// coverage in external-api-connector.test.ts, including idempotency checks.

// ─── handleGitWorktreeList — validation ───────────────────────────────────────
describe('handleGitWorktreeList', () => {
  it('returns defined response without crashing for any cwd', async () => {
    const resp = await handleGitWorktreeList(1, { cwd: tmpdir() }, mockConfig, mockLog) as any
    expect(resp).toBeDefined()
    expect(resp.jsonrpc).toBe('2.0')
  })

  it('result or error is always set', async () => {
    const resp = await handleGitWorktreeList(1, {}, mockConfig, mockLog) as any
    expect(resp.result ?? resp.error).toBeDefined()
  })
})

// ─── handleGitBaseRefDefault / handleGitSearchRefs — backfilled gap
// (RelayExecutor.BaseRefDefault/SearchRefs have always called these method
// names; the agent never had a handler for either — see agent-git-handler.ts's
// doc comment on these two handlers) ──────────────────────────────────────────
describe('handleGitBaseRefDefault', () => {
  let repoDir: string

  beforeEach(() => {
    repoDir = mkdtempSync(join(tmpdir(), 'base-ref-default-test-'))
    execSync('git init -q', { cwd: repoDir })
    // No real remote needed to exercise this — refs/remotes/origin/HEAD is
    // just a ref, settable directly, matching what a real `git clone` leaves
    // behind without this test needing a live remote to fetch from.
    execSync('git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/main', { cwd: repoDir })
  })

  afterEach(() => {
    rmSync(repoDir, { recursive: true, force: true })
  })

  it('resolves the short branch name from refs/remotes/origin/HEAD', async () => {
    const resp = await handleGitBaseRefDefault(1, { repoPath: repoDir }, mockConfig, mockLog) as any
    expect(resp.result?.ref).toBe('main')
  })

  it('returns an error, not a crash, when there is no origin/HEAD ref', async () => {
    const bareDir = mkdtempSync(join(tmpdir(), 'base-ref-default-bare-'))
    execSync('git init -q', { cwd: bareDir })
    try {
      const resp = await handleGitBaseRefDefault(1, { repoPath: bareDir }, mockConfig, mockLog) as any
      expect(resp.error).toBeDefined()
    } finally {
      rmSync(bareDir, { recursive: true, force: true })
    }
  })
})

describe('handleGitSearchRefs', () => {
  let repoDir: string

  beforeEach(() => {
    repoDir = mkdtempSync(join(tmpdir(), 'search-refs-test-'))
    execSync('git init -q -b main', { cwd: repoDir })
    execSync('git commit --allow-empty -q -m init', { cwd: repoDir })
    execSync('git branch feature/login', { cwd: repoDir })
    execSync('git branch feature/logout', { cwd: repoDir })
  })

  afterEach(() => {
    rmSync(repoDir, { recursive: true, force: true })
  })

  it('returns every ref when query is empty', async () => {
    const resp = await handleGitSearchRefs(1, { repoPath: repoDir, query: '' }, mockConfig, mockLog) as any
    expect(resp.result?.refs).toEqual(expect.arrayContaining(['main', 'feature/login', 'feature/logout']))
  })

  it('substring-filters refs by query', async () => {
    const resp = await handleGitSearchRefs(1, { repoPath: repoDir, query: 'log' }, mockConfig, mockLog) as any
    expect(resp.result?.refs.sort()).toEqual(['feature/login', 'feature/logout'])
  })
})

// ─── handleGitWorktreeAdd — validation ───────────────────────────────────────
describe('handleGitWorktreeAdd — validation', () => {
  it('returns InvalidParams when path is missing', async () => {
    const resp = await handleGitWorktreeAdd(1, { branch: 'feature' }, mockConfig, mockLog) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('path')
  })

  it('returns InvalidParams when branch is missing', async () => {
    const resp = await handleGitWorktreeAdd(1, { path: '/tmp/wt' }, mockConfig, mockLog) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('branch')
  })

  it('returns InvalidParams for path with shell metachar', async () => {
    const resp = await handleGitWorktreeAdd(1,
      { path: '/tmp/wt;rm -rf /', branch: 'feature' },
      mockConfig, mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('Unsafe')
  })

  it('returns InvalidParams for branch with $', async () => {
    const resp = await handleGitWorktreeAdd(1,
      { path: '/tmp/wt', branch: '$HOME/evil' },
      mockConfig, mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns defined response for valid params (no crash even without git repo)', async () => {
    const resp = await handleGitWorktreeAdd(1,
      { path: '/tmp/wt-test', branch: 'feature-xyz', cwd: tmpdir() },
      mockConfig, mockLog
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.result ?? resp.error).toBeDefined()
  })
})

// ─── handleGitWorktreeRemove — validation ─────────────────────────────────────
describe('handleGitWorktreeRemove — validation', () => {
  it('returns InvalidParams when path is missing', async () => {
    const resp = await handleGitWorktreeRemove(1, {}, mockConfig, mockLog) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('path')
  })

  it('returns InvalidParams for path containing |', async () => {
    const resp = await handleGitWorktreeRemove(1,
      { path: '/tmp/wt|evil' },
      mockConfig, mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('Unsafe')
  })

  it('passes force flag to git command (no validation error)', async () => {
    const resp = await handleGitWorktreeRemove(1,
      { path: '/tmp/nonexistent-wt', force: true, cwd: tmpdir() },
      mockConfig, mockLog
    ) as any
    // Should fail at git execution (wt doesn't exist) but not at validation
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.result ?? resp.error).toBeDefined()
  })

  it('response always has jsonrpc 2.0 format', async () => {
    const resp = await handleGitWorktreeRemove(1,
      { path: '/tmp/wt-remove-test', cwd: tmpdir() },
      mockConfig, mockLog
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.id).toBe(1)
  })
})

// ─── worktree:create/worktree:delete tracing (CR-TRACE-001) ──────────────────
describe('agent-git-handler — worktree tracing', () => {
  let events: TraceEvent[]
  let unregister: () => void
  let repoDir: string

  beforeEach(() => {
    events = []
    unregister = registerTraceSink((e) => events.push(e))
    // Real git repo — matches this file's existing "no spawn mock, run real
    // git" convention (see handleGitExec/handleGitWorktreeList integration tests above).
    repoDir = mkdtempSync(join(tmpdir(), 'wt-trace-test-'))
    execSync('git init -q', { cwd: repoDir })
    execSync('git config user.email test@example.com', { cwd: repoDir })
    execSync('git config user.name Test', { cwd: repoDir })
    execSync('git commit -q --allow-empty -m init', { cwd: repoDir })
  })
  afterEach(() => {
    unregister()
    rmSync(repoDir, { recursive: true, force: true })
  })

  it('handleGitWorktreeAdd emits worktree:create span with ok() on success', async () => {
    const wtPath = join(repoDir, 'wt1')
    const resp = await handleGitWorktreeAdd(
      1, { path: wtPath, branch: 'feature/x', createBranch: true, cwd: repoDir }, mockConfig, mockLog
    ) as any
    expect(resp.result).toBeDefined()
    const ok = events.find(e => e.flow === 'worktree:create' && e.level === 'ok')
    expect(ok).toBeDefined()
    expect(ok?.fields.path).toBe(wtPath)
  })

  it('handleGitWorktreeAdd emits fail() when path/branch missing', async () => {
    await handleGitWorktreeAdd(1, {}, mockConfig, mockLog)
    const fail = events.find(e => e.flow === 'worktree:create' && e.level === 'fail')
    expect(fail).toBeDefined()
  })

  it('resumes span id from params._trace.id when present', async () => {
    await handleGitWorktreeAdd(1, { path: '/tmp/wt1', branch: 'x', _trace: { id: 'abc123' } }, mockConfig, mockLog)
    const start = events.find(e => e.flow === 'worktree:create' && e.level === 'start')
    expect(start?.id).toBe('abc123')
  })

  it('generates a new span id when params._trace is absent (backward-compat)', async () => {
    await handleGitWorktreeAdd(1, { path: '/tmp/wt1', branch: 'x' }, mockConfig, mockLog)
    const start = events.find(e => e.flow === 'worktree:create' && e.level === 'start')
    expect(start?.id).toBeDefined()
    expect(start?.id).not.toBe('abc123')
  })

  it('handleGitWorktreeRemove emits worktree:delete span, forwards id to nested agent:git span', async () => {
    const wtPath = join(repoDir, 'wt2')
    await handleGitWorktreeAdd(1, { path: wtPath, branch: 'feature/y', createBranch: true, cwd: repoDir }, mockConfig, mockLog)
    events = []

    await handleGitWorktreeRemove(2, { path: wtPath, cwd: repoDir, _trace: { id: 'xyz789' } }, mockConfig, mockLog)
    const deleteStart = events.find(e => e.flow === 'worktree:delete' && e.level === 'start')
    const gitStart     = events.find(e => e.flow === 'agent:git' && e.level === 'start')
    expect(deleteStart?.id).toBe('xyz789')
    expect(gitStart?.id).toBe('xyz789') // nối tiếp id xuống agent:git qua _trace forward
  })

  // git-gateway-service's CreateWorktreeInput.BaseRef was previously silently
  // ignored (no wire param for it at all) — this covers the new optional
  // start-point support: the new branch is created FROM baseRef, not cwd's
  // current HEAD.
  it('handleGitWorktreeAdd branches from baseRef when provided, not cwd HEAD', async () => {
    // Advance repoDir's HEAD past baseCommit — the new worktree's branch
    // should still point at baseCommit, not this later commit.
    const baseCommit = execSync('git rev-parse HEAD', { cwd: repoDir }).toString().trim()
    execSync('git commit -q --allow-empty -m later', { cwd: repoDir })

    const wtPath = join(repoDir, 'wt-baseref')
    const resp = await handleGitWorktreeAdd(
      1,
      { path: wtPath, branch: 'feature/from-base', createBranch: true, cwd: repoDir, baseRef: baseCommit },
      mockConfig, mockLog
    ) as any
    expect(resp.result).toBeDefined()

    const wtHead = execSync('git rev-parse HEAD', { cwd: wtPath }).toString().trim()
    expect(wtHead).toBe(baseCommit)
  })

  it('handleGitWorktreeAdd omits baseRef arg when not provided (defaults to cwd HEAD)', async () => {
    const wtPath = join(repoDir, 'wt-no-baseref')
    const resp = await handleGitWorktreeAdd(
      1,
      { path: wtPath, branch: 'feature/no-base', createBranch: true, cwd: repoDir },
      mockConfig, mockLog
    ) as any
    expect(resp.result).toBeDefined()

    const repoHead = execSync('git rev-parse HEAD', { cwd: repoDir }).toString().trim()
    const wtHead = execSync('git rev-parse HEAD', { cwd: wtPath }).toString().trim()
    expect(wtHead).toBe(repoHead)
  })
})

// ─── git.exec 'commit' — per-connection identity (BUG-AG-HLD-003 parity) ─────
// Why: previously handleGitExec had no way to apply preflight.setGitIdentity's
// stored identity at all — it always used config.toolEnv verbatim. See
// specs/agent/api/gaps-and-findings.md #5.
describe('agent-git-handler — commit identity injection', () => {
  let repoDir: string

  beforeEach(() => {
    repoDir = mkdtempSync(join(tmpdir(), 'commit-identity-test-'))
    execSync('git init -q', { cwd: repoDir })
    // Deliberately different from the per-connection identity below, so a
    // passing test proves the per-connection value won, not the ambient one.
    execSync('git config user.email repo-default@example.com', { cwd: repoDir })
    execSync('git config user.name "Repo Default"', { cwd: repoDir })
  })
  afterEach(() => {
    rmSync(repoDir, { recursive: true, force: true })
  })

  it('uses the per-connection identity set via preflight.setGitIdentity, not the repo default', async () => {
    const ws = {} as never
    setConnectionGitIdentity(ws, { name: 'Ada Lovelace', email: 'ada@example.com' })

    const resp = await handleGitExec(
      1, { args: ['commit', '--allow-empty', '-m', 'test'], cwd: repoDir }, mockConfig, mockLog, ws
    ) as any
    expect(resp.result.exitCode).toBe(0)

    const author = execSync("git log -1 --format='%an|%ae'", { cwd: repoDir }).toString().trim()
    expect(author).toBe('Ada Lovelace|ada@example.com')
  })

  it('falls back to the ambient env when no ws is passed (backward-compatible)', async () => {
    const resp = await handleGitExec(
      1, { args: ['commit', '--allow-empty', '-m', 'test'], cwd: repoDir }, mockConfig, mockLog
    ) as any
    expect(resp.result.exitCode).toBe(0)

    const author = execSync("git log -1 --format='%an|%ae'", { cwd: repoDir }).toString().trim()
    expect(author).toBe('Repo Default|repo-default@example.com')
  })

  it('does not leak one connection identity into a commit made without ws', async () => {
    const otherWs = {} as never
    setConnectionGitIdentity(otherWs, { name: 'Someone Else', email: 'else@example.com' })

    const resp = await handleGitExec(
      1, { args: ['commit', '--allow-empty', '-m', 'test'], cwd: repoDir }, mockConfig, mockLog
    ) as any
    expect(resp.result.exitCode).toBe(0)

    const author = execSync("git log -1 --format='%an|%ae'", { cwd: repoDir }).toString().trim()
    expect(author).not.toContain('Someone Else')
  })
})
