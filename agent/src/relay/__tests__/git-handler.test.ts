// src/relay/__tests__/git-handler.test.ts
// TASK-16 compliance: git handler tests via agent-git-handler module
// (Implementation file is src/relay/agent-git-handler.ts — aligned with monorepo naming)
import { describe, it, expect, vi } from 'vitest'
import {
  validateGitArgs,
  GitValidationError,
  handleGitExec,
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

// ─── handleGitExec — integration (requires git) ────────────────────────────
describe('handleGitExec — integration', () => {
  it('returns git status in /tmp without crashing', async () => {
    const resp = await handleGitExec(1, { args: ['status'], cwd: '/tmp' }, mockConfig, mockLog) as any
    expect(resp).toBeDefined()
    if (resp.result !== undefined) {
      expect(typeof resp.result.exitCode).toBe('number')
    } else {
      expect(resp.error.code).toBeDefined()
    }
  })
})
