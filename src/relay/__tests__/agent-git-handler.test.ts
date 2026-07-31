// src/relay/__tests__/agent-git-handler.test.ts
import { describe, it, expect, vi } from 'vitest'
import { tmpdir } from 'node:os'
import {
  validateGitArgs,
  GitValidationError,
  handleGitExec,
  handleGitPrCreate,
  handleGitWorktreeList,
  handleGitWorktreeAdd,
  handleGitWorktreeRemove,
} from '../agent-git-handler'
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

// ─── handleGitPrCreate — validation ──────────────────────────────────────────
describe('handleGitPrCreate — validation', () => {
  it('returns InvalidParams (-32602) when title is missing', async () => {
    const resp = await handleGitPrCreate(1, { base: 'main' }, mockConfig, mockLog) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('title')
  })

  it('returns InvalidParams for title containing shell metachar &', async () => {
    const resp = await handleGitPrCreate(1,
      { title: 'feat & evil', base: 'main', cwd: tmpdir(), userId: 'user-1' },
      mockConfig, mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('Unsafe')
  })

  it('returns InvalidParams for base containing shell metachar ;', async () => {
    const resp = await handleGitPrCreate(1,
      { title: 'clean title', base: 'main;rm -rf /', cwd: tmpdir(), userId: 'user-1' },
      mockConfig, mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for title containing $', async () => {
    const resp = await handleGitPrCreate(1,
      { title: 'feat: $HOME injection', base: 'main', cwd: tmpdir(), userId: 'u1' },
      mockConfig, mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for title containing backtick', async () => {
    const resp = await handleGitPrCreate(1,
      { title: 'feat: `whoami`', base: 'main', cwd: tmpdir(), userId: 'u1' },
      mockConfig, mockLog
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns error (not crash) when gh not installed — non-git cwd', async () => {
    const resp = await handleGitPrCreate(1,
      { title: 'Valid Title', base: 'main', cwd: tmpdir(), userId: 'user-test-1' },
      mockConfig, mockLog
    ) as any
    // Will fail because gh CLI not configured in test env — but must not crash
    expect(resp).toBeDefined()
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error ?? resp.result).toBeDefined()
  })
})

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
