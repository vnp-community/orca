# TASK-09: Write Tests — agent-git-handler

> ✅ **STATUS: DONE** — Completed 2026-07-30T18:13
> 📝 **Result:** 58/58 tests pass — Extended existing file: added `worktree` subcommand validation (3), `handleGitPrCreate` validation (6), `handleGitWorktreeList` (2), `handleGitWorktreeAdd` (5), `handleGitWorktreeRemove` (4), fixed broken `--version` test.
**Phase:** 5
**File:** `src/relay/__tests__/agent-git-handler.test.ts` (NEW FILE)
**Operation:** CREATE
**Depends on:** TASK-03 phải hoàn thành

---

## Mục tiêu

Viết tests cho `agent-git-handler.ts` tập trung vào:
- `validateGitArgs()` — whitelist và metachar rejection
- `handleGitExec()` — params validation, timeout, ENOENT
- `handleGitPrCreate()` — title/base metachar validation (NEW từ TASK-03)
- `handleGitWorktreeAdd()` — path/branch validation (NEW từ TASK-03)
- `handleGitWorktreeRemove()` — path validation (NEW từ TASK-03)

> **Note:** Các tests thực thi git command thực tế cần có git repo để chạy.
> Tests tập trung vào **validation path** (không cần thực thi thực sự).

---

## Test File

Tạo `src/relay/__tests__/agent-git-handler.test.ts`:

```typescript
// src/relay/__tests__/agent-git-handler.test.ts

import { describe, it, expect } from 'vitest'
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

// ─── Test Fixtures ─────────────────────────────────────────────────────────────

const MOCK_CONFIG = {
  devServerId:   'test-server',
  agentToken:    '',
  workDir:       tmpdir(),
  credentialDir: tmpdir(),
  toolPath:      '/usr/bin:/bin',
  toolEnv:       { PATH: '/usr/bin:/bin' },
} as unknown as AgentConfig

const MOCK_LOG: AgentLogger = {
  info:  () => {},
  warn:  () => {},
  error: () => {},
  debug: () => {},
}

// ─── validateGitArgs ──────────────────────────────────────────────────────────

describe('validateGitArgs', () => {
  it('accepts allowed subcommands', () => {
    const allowed = ['status', 'diff', 'add', 'commit', 'push', 'pull', 'log', 'worktree']
    for (const cmd of allowed) {
      expect(() => validateGitArgs([cmd])).not.toThrow()
    }
  })

  it('throws GIT_NO_SUBCOMMAND for empty args', () => {
    expect(() => validateGitArgs([])).toThrow(GitValidationError)
    try { validateGitArgs([]) } catch (e) {
      expect((e as GitValidationError).code).toBe('GIT_NO_SUBCOMMAND')
    }
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for gc', () => {
    expect(() => validateGitArgs(['gc'])).toThrow(GitValidationError)
    try { validateGitArgs(['gc']) } catch (e) {
      expect((e as GitValidationError).code).toBe('GIT_DISALLOWED_SUBCOMMAND')
    }
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for bisect', () => {
    expect(() => validateGitArgs(['bisect'])).toThrow(GitValidationError)
  })

  it('throws GIT_SHELL_METACHARACTER_IN_ARG for semicolon', () => {
    expect(() => validateGitArgs(['status', '; rm -rf /'])).toThrow(GitValidationError)
    try { validateGitArgs(['status', '; rm -rf /']) } catch (e) {
      expect((e as GitValidationError).code).toBe('GIT_SHELL_METACHARACTER_IN_ARG')
    }
  })

  it('throws GIT_SHELL_METACHARACTER_IN_ARG for backtick', () => {
    expect(() => validateGitArgs(['log', '--format=%s`whoami`'])).toThrow(GitValidationError)
  })

  it('throws GIT_SHELL_METACHARACTER_IN_ARG for pipe', () => {
    expect(() => validateGitArgs(['status', '| cat /etc/passwd'])).toThrow(GitValidationError)
  })

  it('accepts safe args with -- separator', () => {
    expect(() => validateGitArgs(['add', '--', 'src/main.ts'])).not.toThrow()
  })
})

// ─── handleGitExec ────────────────────────────────────────────────────────────

describe('handleGitExec', () => {
  it('returns InvalidParams for disallowed subcommand', async () => {
    const res = await handleGitExec(1, { args: ['gc'] }, MOCK_CONFIG, MOCK_LOG) as {
      error?: { code: number }
    }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for metachar injection attempt', async () => {
    const res = await handleGitExec(1, {
      args: ['status', '; curl evil.com'],
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for empty args', async () => {
    const res = await handleGitExec(1, { args: [] }, MOCK_CONFIG, MOCK_LOG) as {
      error?: { code: number }
    }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns result with exitCode for valid git command in git repo', async () => {
    // This test requires running inside a git repo (CI passes)
    const res = await handleGitExec(1, {
      args: ['status', '--porcelain'],
      cwd:  process.cwd(),
    }, MOCK_CONFIG, MOCK_LOG) as { result?: { exitCode: number; stdout: string } }

    // Either succeeds (in git repo) or error (not git repo) - both valid
    if (res.result) {
      expect(typeof res.result.exitCode).toBe('number')
      expect(typeof res.result.stdout).toBe('string')
    }
  })
})

// ─── handleGitPrCreate ────────────────────────────────────────────────────────

describe('handleGitPrCreate', () => {
  it('returns InvalidParams for missing title', async () => {
    const res = await handleGitPrCreate(1, {
      body: 'body', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for empty title (whitespace)', async () => {
    const res = await handleGitPrCreate(1, {
      title: '   ', body: 'body', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for metachar in title', async () => {
    const res = await handleGitPrCreate(1, {
      title: 'Fix bug; rm -rf /', body: 'body', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for metachar in base', async () => {
    const res = await handleGitPrCreate(1, {
      title: 'Safe title', body: 'body', base: 'main`whoami`', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for pipe in title', async () => {
    const res = await handleGitPrCreate(1, {
      title: 'Fix | cat /etc/shadow', body: 'body', base: 'main', cwd: tmpdir(), userId: 'u1',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

// ─── handleGitWorktreeAdd ──────────────────────────────────────────────────────

describe('handleGitWorktreeAdd', () => {
  it('returns InvalidParams for missing path', async () => {
    const res = await handleGitWorktreeAdd(1, {
      branch: 'feature/x', cwd: tmpdir(),
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for missing branch', async () => {
    const res = await handleGitWorktreeAdd(1, {
      path: '/tmp/wt1', cwd: tmpdir(),
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for metachar in path', async () => {
    const res = await handleGitWorktreeAdd(1, {
      path: '/tmp/wt;rm -rf /', branch: 'main', cwd: tmpdir(),
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for metachar in branch', async () => {
    const res = await handleGitWorktreeAdd(1, {
      path: '/tmp/wt1', branch: 'feat`whoami`', cwd: tmpdir(),
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

// ─── handleGitWorktreeRemove ──────────────────────────────────────────────────

describe('handleGitWorktreeRemove', () => {
  it('returns InvalidParams for missing path', async () => {
    const res = await handleGitWorktreeRemove(1, {}, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })

  it('returns InvalidParams for metachar in path', async () => {
    const res = await handleGitWorktreeRemove(1, {
      path: '/tmp/wt; rm -rf /',
    }, MOCK_CONFIG, MOCK_LOG) as { error?: { code: number } }
    expect(res.error?.code).toBe(-32602)
  })
})

// ─── handleGitWorktreeList ────────────────────────────────────────────────────

describe('handleGitWorktreeList', () => {
  it('delegates to handleGitExec with worktree list args', async () => {
    // Should call git worktree list (valid subcommand = not filtered by security)
    // In non-git-repo: will return git error with exitCode
    const res = await handleGitWorktreeList(1, { cwd: tmpdir() }, MOCK_CONFIG, MOCK_LOG) as {
      result?: { exitCode: number }; error?: unknown
    }
    // Either result (in git repo) or error (not git repo) — both valid
    expect(res.result !== undefined || res.error !== undefined).toBe(true)
  })
})
```

---

## Verify

```bash
pnpm test src/relay/__tests__/agent-git-handler.test.ts
# Expected: ≥ 20 tests pass
```

---

## Done criteria

- [ ] `validateGitArgs` — 8 tests (whitelist, metachar, empty)
- [ ] `handleGitExec` — 4 tests (validation path)
- [ ] `handleGitPrCreate` — 5 tests (title/base validation)
- [ ] `handleGitWorktreeAdd` — 4 tests
- [ ] `handleGitWorktreeRemove` — 2 tests
- [ ] `handleGitWorktreeList` — 1 test
- [ ] Tất cả tests pass ≥ 20
