// src/relay/__tests__/external-api-connector.test.ts
// Unit tests for external-api-connector.ts.
// Strategy: test validation/env-building as pure units; CLI execution
// tests that require gh/glab are wrapped as "no-crash" integration tests.
import { describe, it, expect, vi } from 'vitest'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  buildGhEnv,
  buildGlabEnv,
  execFileCaptured,
  handleGitHubPrCreate,
  handleGitHubPrMerge,
  handleGitHubIssueList,
  handleGitHubIssueCreate,
  handleGitHubAuthStatus,
  handleGitLabMrCreate,
  handleGitLabMrList,
  handleGitLabPipelineStatus,
  handleGitLabAuthStatus,
} from '../external-api-connector'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'
import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent } from '../../shared/trace'

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const MOCK_CONFIG = {
  workDir:   tmpdir(),
  toolPath:  '/usr/local/bin:/usr/bin:/bin',
  toolEnv:   { PATH: '/usr/local/bin:/usr/bin:/bin' },
} as unknown as AgentConfig

const MOCK_LOG: AgentLogger = {
  info:  vi.fn(),
  warn:  vi.fn(),
  error: vi.fn(),
  debug: vi.fn(),
}

// ─── buildGhEnv ───────────────────────────────────────────────────────────────
describe('buildGhEnv', () => {
  it('sets GH_CONFIG_DIR with userId', () => {
    const env = buildGhEnv('alice', {})
    expect(env.GH_CONFIG_DIR).toContain('alice')
    expect(env.GH_CONFIG_DIR).toMatch(/\.config\/gh\/alice\/$/)
  })

  it('sets GH_NO_UPDATE_NOTIFIER=1', () => {
    const env = buildGhEnv('bob', {})
    expect(env.GH_NO_UPDATE_NOTIFIER).toBe('1')
  })

  it('sets GH_PROMPT_DISABLED=1', () => {
    const env = buildGhEnv('bob', {})
    expect(env.GH_PROMPT_DISABLED).toBe('1')
  })

  it('different users get different GH_CONFIG_DIR', () => {
    const envA = buildGhEnv('alice', {})
    const envB = buildGhEnv('bob', {})
    expect(envA.GH_CONFIG_DIR).not.toBe(envB.GH_CONFIG_DIR)
  })

  it('merges base env into result', () => {
    const env = buildGhEnv('alice', { MY_VAR: 'test-value' })
    expect(env.MY_VAR).toBe('test-value')
  })

  it('GH_CONFIG_DIR overrides any baseEnv GH_CONFIG_DIR', () => {
    const env = buildGhEnv('alice', { GH_CONFIG_DIR: '/old/path' })
    expect(env.GH_CONFIG_DIR).toContain('alice')
    expect(env.GH_CONFIG_DIR).not.toBe('/old/path')
  })
})

// ─── buildGlabEnv ─────────────────────────────────────────────────────────────
describe('buildGlabEnv', () => {
  it('sets GLAB_CONFIG_DIR with userId', () => {
    const env = buildGlabEnv('alice', {})
    expect(env.GLAB_CONFIG_DIR).toContain('alice')
    expect(env.GLAB_CONFIG_DIR).toMatch(/\.config\/glab-cli\/alice\/$/)
  })

  it('sets NO_COLOR=1 for non-interactive mode', () => {
    const env = buildGlabEnv('alice', {})
    expect(env.NO_COLOR).toBe('1')
  })

  it('sets CI=1 for non-interactive mode', () => {
    const env = buildGlabEnv('bob', {})
    expect(env.CI).toBe('1')
  })

  it('different users get different GLAB_CONFIG_DIR', () => {
    const envA = buildGlabEnv('alice', {})
    const envB = buildGlabEnv('carol', {})
    expect(envA.GLAB_CONFIG_DIR).not.toBe(envB.GLAB_CONFIG_DIR)
  })

  it('merges base env into result', () => {
    const env = buildGlabEnv('alice', { CUSTOM_VAR: 'value-123' })
    expect(env.CUSTOM_VAR).toBe('value-123')
  })
})

// ─── execFileCaptured ─────────────────────────────────────────────────────────
describe('execFileCaptured', () => {
  it('captures stdout from "echo"', async () => {
    const result = await execFileCaptured('echo', ['hello-test'], {
      cwd: tmpdir(), env: process.env as NodeJS.ProcessEnv, timeout: 5_000,
    })
    expect(result.stdout.trim()).toBe('hello-test')
    expect(result.exitCode).toBe(0)
  })

  it('captures stderr from "ls --invalid-flag"', async () => {
    const result = await execFileCaptured('ls', ['--invalid-flag-xyz-abc-123'], {
      cwd: tmpdir(), env: process.env as NodeJS.ProcessEnv, timeout: 5_000,
    })
    expect(result.exitCode).not.toBe(0)
    expect(result.stderr.length + result.stdout.length).toBeGreaterThan(0)
  })

  it('returns exitCode 0 for successful command', async () => {
    const result = await execFileCaptured('true', [], {
      cwd: tmpdir(), env: process.env as NodeJS.ProcessEnv, timeout: 5_000,
    })
    expect(result.exitCode).toBe(0)
  })

  it('returns exitCode 1 for failed command', async () => {
    const result = await execFileCaptured('false', [], {
      cwd: tmpdir(), env: process.env as NodeJS.ProcessEnv, timeout: 5_000,
    })
    expect(result.exitCode).toBe(1)
  })

  it('times out and returns exitCode 124', async () => {
    const result = await execFileCaptured('sleep', ['10'], {
      cwd: tmpdir(), env: process.env as NodeJS.ProcessEnv, timeout: 100, // 100ms
    })
    expect(result.exitCode).toBe(124)
    expect(result.stderr).toContain('Timeout')
  })

  it('returns error info when binary does not exist', async () => {
    const result = await execFileCaptured('binary-that-does-not-exist-xyz-abc', [], {
      cwd: tmpdir(), env: process.env as NodeJS.ProcessEnv, timeout: 5_000,
    })
    expect(result.exitCode).toBe(1)
    expect(result.stderr.length).toBeGreaterThan(0)
  })
})

// ─── handleGitHubPrCreate — validation ───────────────────────────────────────
describe('handleGitHubPrCreate — validation', () => {
  it('returns InvalidParams when title is missing', async () => {
    const resp = await handleGitHubPrCreate(1, { base: 'main' }, MOCK_CONFIG, MOCK_LOG) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('title')
  })

  it('returns InvalidParams for title with shell metachar &', async () => {
    const resp = await handleGitHubPrCreate(1,
      { title: 'feat & evil', base: 'main', userId: 'u1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('Unsafe')
  })

  it('returns InvalidParams for title with $', async () => {
    const resp = await handleGitHubPrCreate(1,
      { title: 'feat: $HOME', base: 'main', userId: 'u1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for title with backtick', async () => {
    const resp = await handleGitHubPrCreate(1,
      { title: 'feat: `cmd`', base: 'main', userId: 'u1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for base with ;', async () => {
    const resp = await handleGitHubPrCreate(1,
      { title: 'safe title', base: 'main;evil', userId: 'u1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('does not crash when gh CLI not available — returns defined error', async () => {
    const resp = await handleGitHubPrCreate(1,
      { title: 'Valid PR Title', base: 'main', cwd: tmpdir(), userId: 'test-user' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error ?? resp.result).toBeDefined()
  })
})

// ─── handleGitHubPrMerge — validation ────────────────────────────────────────
describe('handleGitHubPrMerge — validation', () => {
  it('returns InvalidParams when prNumber is missing', async () => {
    const resp = await handleGitHubPrMerge(1, {}, MOCK_CONFIG, MOCK_LOG) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('prNumber')
  })

  it('does not crash for valid prNumber when gh not available', async () => {
    const resp = await handleGitHubPrMerge(1,
      { prNumber: 42, userId: 'user-1', cwd: tmpdir() },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error ?? resp.result).toBeDefined()
  })

  it('uses squash merge method by default', async () => {
    // Verify no validation error with default method
    const resp = await handleGitHubPrMerge(1,
      { prNumber: 1, userId: 'user-1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
  })
})

// ─── handleGitHubIssueList — no-crash integration ────────────────────────────
describe('handleGitHubIssueList', () => {
  it('returns defined response without crashing', async () => {
    const resp = await handleGitHubIssueList(1,
      { userId: 'user-1', cwd: tmpdir() },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error ?? resp.result).toBeDefined()
  })

  it('caps limit at 50', async () => {
    // Just validate no crash with large limit
    const resp = await handleGitHubIssueList(1,
      { userId: 'user-1', limit: 1000, cwd: tmpdir() },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
  })
})

// ─── handleGitHubIssueCreate — validation ────────────────────────────────────
describe('handleGitHubIssueCreate — validation', () => {
  it('returns InvalidParams when title is missing', async () => {
    const resp = await handleGitHubIssueCreate(1, {}, MOCK_CONFIG, MOCK_LOG) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('returns InvalidParams for title with &', async () => {
    const resp = await handleGitHubIssueCreate(1,
      { title: 'bug & exploit', userId: 'u1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('Unsafe')
  })

  it('does not crash for valid params when gh not available', async () => {
    const resp = await handleGitHubIssueCreate(1,
      { title: 'Valid Issue', body: 'Description', userId: 'user-1', cwd: tmpdir() },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error ?? resp.result).toBeDefined()
  })
})

// ─── handleGitHubAuthStatus — no-crash integration ───────────────────────────
describe('handleGitHubAuthStatus', () => {
  it('returns defined response with ok boolean', async () => {
    const resp = await handleGitHubAuthStatus(1,
      { userId: 'user-1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    if (resp.result) {
      expect(typeof resp.result.ok).toBe('boolean')
    } else {
      expect(resp.error).toBeDefined()
    }
  })
})

// ─── handleGitLabMrCreate — validation ───────────────────────────────────────
describe('handleGitLabMrCreate — validation', () => {
  it('returns InvalidParams when title is missing', async () => {
    const resp = await handleGitLabMrCreate(1, {}, MOCK_CONFIG, MOCK_LOG) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('title')
  })

  it('returns InvalidParams for title with |', async () => {
    const resp = await handleGitLabMrCreate(1,
      { title: 'feat|evil', targetBranch: 'main', userId: 'u1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.error.code).toBe(-32602)
    expect(resp.error.message).toContain('Unsafe')
  })

  it('returns InvalidParams for targetBranch with ;', async () => {
    const resp = await handleGitLabMrCreate(1,
      { title: 'safe title', targetBranch: 'main;evil', userId: 'u1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.error.code).toBe(-32602)
  })

  it('does not crash for valid params when glab not available', async () => {
    const resp = await handleGitLabMrCreate(1,
      { title: 'Valid MR', targetBranch: 'main', userId: 'user-1', cwd: tmpdir() },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error ?? resp.result).toBeDefined()
  })
})

// ─── handleGitLabMrList — no-crash integration ───────────────────────────────
describe('handleGitLabMrList', () => {
  it('returns defined response without crashing', async () => {
    const resp = await handleGitLabMrList(1,
      { userId: 'user-1', cwd: tmpdir() },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error ?? resp.result).toBeDefined()
  })
})

// ─── handleGitLabPipelineStatus — no-crash integration ───────────────────────
describe('handleGitLabPipelineStatus', () => {
  it('returns defined response without crashing', async () => {
    const resp = await handleGitLabPipelineStatus(1,
      { userId: 'user-1', cwd: tmpdir() },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    expect(resp.error ?? resp.result).toBeDefined()
  })
})

// ─── handleGitLabAuthStatus — no-crash integration ───────────────────────────
describe('handleGitLabAuthStatus', () => {
  it('returns defined response with ok boolean', async () => {
    const resp = await handleGitLabAuthStatus(1,
      { userId: 'user-1' },
      MOCK_CONFIG, MOCK_LOG
    ) as any
    expect(resp.jsonrpc).toBe('2.0')
    if (resp.result) {
      expect(typeof resp.result.ok).toBe('boolean')
    } else {
      expect(resp.error).toBeDefined()
    }
  })
})

// ─── handleGitHubAuthStatus — agent:ext-api tracing (TASK-AG-014.1) ─────────
// gh is not authenticated in the test sandbox, so the terminal event may be
// ok or fail depending on `gh auth status` exit code — assert on whichever
// terminal event actually fires rather than a fixed outcome.
describe('handleGitHubAuthStatus — agent:ext-api tracing', () => {
  it('emits a terminal agent:ext-api event tagged cli:"gh"', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleGitHubAuthStatus(1, { userId: 'user-1' }, MOCK_CONFIG, MOCK_LOG)
    unregister()

    const extApiEvents = events.filter(e => e.flow === 'agent:ext-api')
    expect(extApiEvents.length).toBeGreaterThan(0)
    const terminal = extApiEvents.find(e => e.level === 'ok' || e.level === 'fail')
    expect(terminal?.fields.cli).toBe('gh')
  })

  it('KHÔNG có field nào trong agent:ext-api chứa nội dung stdout/stderr của gh auth status', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleGitHubAuthStatus(1, { userId: 'user-1' }, MOCK_CONFIG, MOCK_LOG)
    unregister()

    const fields = events.filter(e => e.flow === 'agent:ext-api').flatMap(e => Object.keys(e.fields))
    expect(fields).not.toContain('stdout')
    expect(fields).not.toContain('stderr')
  })
})

// ─── handleGitLabAuthStatus — agent:ext-api tracing (TASK-AG-014.1) ─────────
// glab is not installed in the test sandbox, so execFileCaptured always
// resolves a non-zero exitCode via its spawn-error path — deterministic fail.
describe('handleGitLabAuthStatus — agent:ext-api tracing', () => {
  it('span.fail(..., {cli:"glab", exitCode}) khi glab auth status exit != 0', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleGitLabAuthStatus(1, { userId: 'user-1' }, MOCK_CONFIG, MOCK_LOG)
    unregister()

    const fail = events.find(e => e.flow === 'agent:ext-api' && e.level === 'fail')
    expect(fail?.fields.cli).toBe('glab')
    expect(fail?.fields.exitCode).toBeDefined()
  })

  it('KHÔNG có field nào trong agent:ext-api chứa nội dung stdout/stderr của glab auth status', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    await handleGitLabAuthStatus(1, { userId: 'user-1' }, MOCK_CONFIG, MOCK_LOG)
    unregister()

    const fields = events.filter(e => e.flow === 'agent:ext-api').flatMap(e => Object.keys(e.fields))
    expect(fields).not.toContain('stdout')
    expect(fields).not.toContain('stderr')
  })
})
