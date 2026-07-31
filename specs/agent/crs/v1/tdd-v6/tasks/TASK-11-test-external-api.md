# TASK-11: Write Tests — external-api-connector

> ✅ **STATUS: DONE** — Completed 2026-07-30T18:13
> 📝 **Result:** 39/39 tests pass — NEW file created: `buildGhEnv` (6), `buildGlabEnv` (5), `execFileCaptured` (6), `handleGitHubPrCreate` (6), `handleGitHubPrMerge` (3), `handleGitHubIssueList` (2), `handleGitHubIssueCreate` (3), `handleGitHubAuthStatus` (1), `handleGitLabMrCreate` (4), `handleGitLabMrList` (1), `handleGitLabPipelineStatus` (1), `handleGitLabAuthStatus` (1).
**Phase:** 5
**File:** `src/relay/__tests__/external-api-connector.test.ts` (NEW FILE)
**Operation:** CREATE
**Depends on:** TASK-05 phải hoàn thành

---

## Test File

Tạo `src/relay/__tests__/external-api-connector.test.ts`:

```typescript
// src/relay/__tests__/external-api-connector.test.ts

import { describe, it, expect } from 'vitest'
import { tmpdir } from 'node:os'

import {
  handleGitHubPrCreate,
  handleGitHubPrMerge,
  handleGitHubIssueList,
  handleGitHubIssueCreate,
  handleGitHubAuthStatus,
  handleGitLabMrCreate,
  handleGitLabMrList,
  handleGitLabPipelineStatus,
  handleGitLabAuthStatus,
  buildGhEnv,
  buildGlabEnv,
  execFileCaptured,
} from '../external-api-connector'
import type { AgentConfig } from '../agent-config'
import type { AgentLogger } from '../agent-logger'

// ─── Fixtures ─────────────────────────────────────────────────────────────────

const MOCK_CONFIG = {
  devServerId:   'test-server',
  agentToken:    '',
  workDir:       tmpdir(),
  credentialDir: tmpdir(),
  toolPath:      '/usr/bin:/usr/local/bin:/bin',
  toolEnv:       { PATH: '/usr/bin:/bin' },
} as unknown as AgentConfig

const MOCK_LOG: AgentLogger = {
  info:  () => {},
  warn:  () => {},
  error: () => {},
  debug: () => {},
}

// ─── buildGhEnv ───────────────────────────────────────────────────────────────

describe('buildGhEnv', () => {
  it('sets GH_CONFIG_DIR per userId', () => {
    const env = buildGhEnv('user-abc', {})
    expect(env.GH_CONFIG_DIR).toContain('user-abc')
    expect(env.GH_CONFIG_DIR).toContain('.config/gh/')
  })

  it('different users get different GH_CONFIG_DIR', () => {
    const env1 = buildGhEnv('alice', {})
    const env2 = buildGhEnv('bob', {})
    expect(env1.GH_CONFIG_DIR).not.toBe(env2.GH_CONFIG_DIR)
  })

  it('sets GH_NO_UPDATE_NOTIFIER=1', () => {
    const env = buildGhEnv('u1', {})
    expect(env.GH_NO_UPDATE_NOTIFIER).toBe('1')
  })

  it('sets GH_PROMPT_DISABLED=1', () => {
    const env = buildGhEnv('u1', {})
    expect(env.GH_PROMPT_DISABLED).toBe('1')
  })

  it('inherits baseEnv properties', () => {
    const env = buildGhEnv('u1', { CUSTOM_VAR: 'custom-value', PATH: '/usr/bin' })
    expect(env.CUSTOM_VAR).toBe('custom-value')
    expect(env.PATH).toBe('/usr/bin')
  })
})

// ─── buildGlabEnv ─────────────────────────────────────────────────────────────

describe('buildGlabEnv', () => {
  it('sets GLAB_CONFIG_DIR per userId', () => {
    const env = buildGlabEnv('user-xyz', {})
    expect(env.GLAB_CONFIG_DIR).toContain('user-xyz')
    expect(env.GLAB_CONFIG_DIR).toContain('.config/glab-cli/')
  })

  it('different users get different GLAB_CONFIG_DIR', () => {
    const env1 = buildGlabEnv('alice', {})
    const env2 = buildGlabEnv('bob', {})
    expect(env1.GLAB_CONFIG_DIR).not.toBe(env2.GLAB_CONFIG_DIR)
  })

  it('sets CI=1 for non-interactive mode', () => {
    const env = buildGlabEnv('u1', {})
    expect(env.CI).toBe('1')
  })

  it('sets NO_COLOR=1', () => {
    const env = buildGlabEnv('u1', {})
    expect(env.NO_COLOR).toBe('1')
  })
})

// ─── SHELL_METACHARACTERS validation ─────────────────────────────────────────

describe('handleGitHubPrCreate — security validation', () => {
  it('returns InvalidParams for missing title', async () => {
    const res = await handleGitHubPrCreate(1, {
      body: 'body', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for empty title', async () => {
    const res = await handleGitHubPrCreate(1, {
      title: '   ', body: 'body', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('rejects semicolon in title', async () => {
    const res = await handleGitHubPrCreate(1, {
      title: 'Fix bug; rm -rf /', body: '', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('rejects backtick in base branch', async () => {
    const res = await handleGitHubPrCreate(1, {
      title: 'Safe title', body: '', base: 'main`whoami`', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('rejects pipe in title', async () => {
    const res = await handleGitHubPrCreate(1, {
      title: 'Fix | cat /etc/passwd', body: '', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('rejects dollar sign in title', async () => {
    const res = await handleGitHubPrCreate(1, {
      title: 'Fix $HOME exploit', body: '', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

describe('handleGitLabMrCreate — security validation', () => {
  it('returns InvalidParams for missing title', async () => {
    const res = await handleGitLabMrCreate(1, {
      description: 'desc', targetBranch: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('rejects pipe in title', async () => {
    const res = await handleGitLabMrCreate(1, {
      title: 'Fix | cat /etc/shadow', description: '', targetBranch: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('rejects metachar in targetBranch', async () => {
    const res = await handleGitLabMrCreate(1, {
      title: 'Safe title', description: '', targetBranch: 'main;rm -rf /', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

describe('handleGitHubIssueCreate — security validation', () => {
  it('returns InvalidParams for missing title', async () => {
    const res = await handleGitHubIssueCreate(1, {
      body: 'body', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('rejects metachar in issue title', async () => {
    const res = await handleGitHubIssueCreate(1, {
      title: 'Bug; curl evil.com', body: '', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

describe('handleGitHubPrMerge — validation', () => {
  it('returns InvalidParams for missing prNumber', async () => {
    const res = await handleGitHubPrMerge(1, {
      cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

// ─── execFileCaptured — timeout ───────────────────────────────────────────────

describe('execFileCaptured', () => {
  it('returns exitCode 124 and stderr on timeout', async () => {
    const result = await execFileCaptured('sleep', ['100'], {
      cwd: tmpdir(), env: process.env, timeout: 100,
    })
    expect(result.exitCode).toBe(124)
    expect(result.stderr).toContain('Timeout')
  })

  it('captures stdout for successful command', async () => {
    const result = await execFileCaptured('echo', ['hello world'], {
      cwd: tmpdir(), env: process.env, timeout: 5_000,
    })
    expect(result.exitCode).toBe(0)
    expect(result.stdout.trim()).toBe('hello world')
  })

  it('returns non-zero exitCode for failing command', async () => {
    const result = await execFileCaptured('ls', ['/path/that/does/not/exist/xyz'], {
      cwd: tmpdir(), env: process.env, timeout: 5_000,
    })
    expect(result.exitCode).not.toBe(0)
  })

  it('captures stderr for failing command', async () => {
    const result = await execFileCaptured('ls', ['/path/that/does/not/exist/xyz'], {
      cwd: tmpdir(), env: process.env, timeout: 5_000,
    })
    expect(result.stderr.length).toBeGreaterThan(0)
  })

  it('returns error message for non-existent binary', async () => {
    const result = await execFileCaptured('nonexistent-binary-xyz-abc', [], {
      cwd: tmpdir(), env: process.env, timeout: 5_000,
    })
    expect(result.exitCode).not.toBe(0)
    expect(result.stderr.length).toBeGreaterThan(0)
  })
})
```

---

## Verify

```bash
pnpm test src/relay/__tests__/external-api-connector.test.ts
# Expected: ≥ 25 tests pass
# Note: github.auth.status và gitlab.auth.status tests cần gh/glab binary
```

---

## Done criteria

- [ ] `buildGhEnv` — 5 tests (isolation, notify, prompt, inherit)
- [ ] `buildGlabEnv` — 4 tests
- [ ] `handleGitHubPrCreate` validation — 6 tests
- [ ] `handleGitLabMrCreate` validation — 3 tests
- [ ] `handleGitHubIssueCreate` validation — 2 tests
- [ ] `handleGitHubPrMerge` validation — 1 test
- [ ] `execFileCaptured` — 5 tests (timeout, stdout, stderr, exitCode, ENOENT)
- [ ] Tất cả ≥ 25 tests pass
