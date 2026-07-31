# TDD-AG-13: External API Connectors — GitHub & GitLab (v5.0)

**Document:** TDD-AG-13 (NEW — v5.0)
**Version:** 1.0
**Date:** 2026-07-30
**Domain:** External API connectors (GitHub via `gh` CLI, GitLab via `glab` CLI)
**Feature:** F06, F30, F39
**ADR:** ADR-012
**HLD Ref:** C3.12, §12 (dev-server-architecture.md)
**Related TDD:** TDD-AG-10 (git handler), TDD-AG-09 (credential store)

> **Status: 🚧 In-Progress** — v5.0 new module

---

## 1. Vai trò trong hệ thống

`ExternalApiConnector` layer là **bridge** giữa Dev Server Agent và GitHub/GitLab APIs:

- GitHub operations: PR create/view/merge, issue CRUD, repo ops (via `gh` CLI)
- GitLab operations: MR create/view/list, issue CRUD, pipeline status (via `glab` CLI)
- Per-user auth isolation via `GH_CONFIG_DIR` / `GLAB_CONFIG_DIR`
- **Không** dùng GitHub SDK hoặc direct HTTPS calls — chỉ dùng official CLI tools
- CLI tool errors được forward về Gateway trong chuẩn JSON-RPC error format

---

## 2. Source File

```
src/relay/
└── external-api-connector.ts      ← [NEW v5.0] GitHub + GitLab connectors
```

> Distinct từ `agent-git-handler.ts` (git exec/stream) và `preflight-handler.ts` (đã có).

---

## 3. GitHub Connector

### 3.1 Types

```typescript
// src/relay/external-api-connector.ts

export interface GitHubPrCreateParams {
  readonly title:    string
  readonly body:     string
  readonly base:     string    // target branch (default: 'main')
  readonly cwd:      string    // git repo directory on dev server
  readonly userId:   string    // for GH_CONFIG_DIR isolation
  readonly draft?:   boolean
  readonly labels?:  string[]
}

export interface GitHubPrResult {
  readonly url:      string
  readonly number:   number
  readonly title:    string
  readonly state:    'open' | 'closed' | 'merged'
}

export interface GitHubIssueCreateParams {
  readonly title:    string
  readonly body:     string
  readonly labels?:  string[]
  readonly cwd:      string
  readonly userId:   string
}
```

### 3.2 buildGhEnv() — per-user isolation

```typescript
// src/relay/external-api-connector.ts

function buildGhEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  const home = homedir()
  return {
    ...baseEnv,
    GH_CONFIG_DIR: `${home}/.config/gh/${userId}/`,
    // Disable gh auto-updating (can cause hangs)
    GH_NO_UPDATE_NOTIFIER: '1',
    // Force non-interactive mode
    GH_PROMPT_DISABLED: '1',
  }
}

function buildGlabEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  const home = homedir()
  return {
    ...baseEnv,
    GLAB_CONFIG_DIR: `${home}/.config/glab-cli/${userId}/`,
    NO_COLOR: '1',         // disable color codes in output
    CI: '1',               // force non-interactive
  }
}
```

### 3.3 handleGitHubPrCreate()

```typescript
// src/relay/external-api-connector.ts

export async function handleGitHubPrCreate(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const title  = typeof params.title  === 'string' ? params.title.trim()  : ''
  const body   = typeof params.body   === 'string' ? params.body           : ''
  const base   = typeof params.base   === 'string' ? params.base.trim()   : 'main'
  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId          : ''
  const draft  = params.draft === true

  if (!title) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }

  // Security: validate metacharacters
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in PR params' } }
  }

  // Check idempotency: does PR already exist for current branch?
  const currentBranch = await getCurrentBranch(cwd, config.toolEnv)
  if (currentBranch) {
    const existing = await checkExistingPr(cwd, currentBranch, userId, config.toolEnv, log)
    if (existing) {
      log.info(`github.pr.create: PR already exists → ${existing.url}`)
      return { jsonrpc: '2.0', id, result: { ...existing, alreadyExisted: true } }
    }
  }

  const ghArgs = [
    'pr', 'create',
    '--title', title,
    '--body',  body,
    '--base',  base,
    '--json',  'url,number,title,state',
  ]
  if (draft) ghArgs.push('--draft')

  const env = buildGhEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })

    if (result.exitCode !== 0) {
      log.error(`github.pr.create failed: ${result.stderr}`)
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'gh pr create failed', details: result.stderr } }
    }

    const parsed = JSON.parse(result.stdout) as GitHubPrResult
    log.info(`github.pr.create: PR #${parsed.number} → ${parsed.url}`)
    return { jsonrpc: '2.0', id, result: parsed }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

### 3.4 All GitHub RPC handlers

```typescript
// Thêm vào src/relay/external-api-connector.ts:

// github.pr.view — view PR metadata
export async function handleGitHubPrView(id, params, config, log): Promise<object>

// github.pr.merge — squash merge
export async function handleGitHubPrMerge(id, params, config, log): Promise<object>

// github.issue.list — list open issues
export async function handleGitHubIssueList(id, params, config, log): Promise<object>

// github.issue.create — create issue
export async function handleGitHubIssueCreate(id, params, config, log): Promise<object>

// github.auth.status — verify auth (used by preflight)
export async function handleGitHubAuthStatus(id, params, config, log): Promise<object>
```

---

## 4. GitLab Connector

### 4.1 handleGitLabMrCreate()

```typescript
// src/relay/external-api-connector.ts

export async function handleGitLabMrCreate(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const title        = typeof params.title       === 'string' ? params.title.trim()        : ''
  const description  = typeof params.description === 'string' ? params.description          : ''
  const targetBranch = typeof params.targetBranch === 'string' ? params.targetBranch.trim() : 'main'
  const cwd          = typeof params.cwd         === 'string' ? params.cwd                  : config.workDir
  const userId       = typeof params.userId      === 'string' ? params.userId               : ''

  if (!title) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }

  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(targetBranch)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in MR params' } }
  }

  const glabArgs = [
    'mr', 'create',
    '--title',          title,
    '--description',    description,
    '--target-branch',  targetBranch,
    '--yes',            // non-interactive
  ]

  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })

    if (result.exitCode !== 0) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'glab mr create failed' } }
    }

    // glab outputs MR URL to stdout
    const url = result.stdout.trim().split('\n').find(l => l.startsWith('https://')) ?? result.stdout.trim()
    log.info(`gitlab.mr.create: MR → ${url}`)
    return { jsonrpc: '2.0', id, result: { url, stdout: result.stdout, stderr: result.stderr } }

  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
```

### 4.2 All GitLab RPC handlers

```typescript
// gitlab.mr.list, gitlab.mr.view, gitlab.issue.create, gitlab.pipeline.status, gitlab.auth.status
```

---

## 5. Shared Utilities

```typescript
// src/relay/external-api-connector.ts

const SHELL_METACHARACTERS = /[&|;$`<>\\!]/

interface ExecResult {
  stdout: string
  stderr: string
  exitCode: number
}

function execFileCaptured(
  binary: string,
  args: string[],
  opts: { cwd: string; env: NodeJS.ProcessEnv; timeout: number }
): Promise<ExecResult> {
  return new Promise((resolve) => {
    const child = spawn(binary, args, {
      cwd: opts.cwd,
      env: opts.env,
      stdio: ['pipe', 'pipe', 'pipe'],
      shell: false,   // MANDATORY: no shell injection
    })

    const stdout: string[] = []
    const stderr: string[] = []

    const timer = setTimeout(() => {
      child.kill('SIGTERM')
      resolve({ stdout: '', stderr: `Timeout after ${opts.timeout}ms`, exitCode: 124 })
    }, opts.timeout)

    child.stdout?.on('data', (b: Buffer) => stdout.push(b.toString()))
    child.stderr?.on('data', (b: Buffer) => stderr.push(b.toString()))

    child.on('close', (code) => {
      clearTimeout(timer)
      resolve({ stdout: stdout.join(''), stderr: stderr.join(''), exitCode: code ?? 0 })
    })

    child.on('error', (err) => {
      clearTimeout(timer)
      resolve({ stdout: '', stderr: err.message, exitCode: 1 })
    })

    child.stdin?.end()
  })
}

// Get current git branch
async function getCurrentBranch(cwd: string, env: NodeJS.ProcessEnv): Promise<string | null> {
  const result = await execFileCaptured('git', ['rev-parse', '--abbrev-ref', 'HEAD'], { cwd, env, timeout: 5_000 })
  return result.exitCode === 0 ? result.stdout.trim() : null
}

// Check if PR already exists for branch
async function checkExistingPr(
  cwd: string, branch: string, userId: string, env: NodeJS.ProcessEnv, log: AgentLogger
): Promise<GitHubPrResult | null> {
  const result = await execFileCaptured('gh', [
    'pr', 'list', '--head', branch, '--json', 'url,number,title,state', '--limit', '1'
  ], { cwd, env: buildGhEnv(userId, env), timeout: 15_000 })

  if (result.exitCode !== 0 || !result.stdout.trim()) return null
  try {
    const prs = JSON.parse(result.stdout) as GitHubPrResult[]
    return prs[0] ?? null
  } catch {
    return null
  }
}
```

---

## 6. RPC Method Registration

```typescript
// src/relay/agent-rpc-dispatch.ts (extend) — thêm sau existing cases:

// ── v5.0: github.pr.create ───────────────────────────────────────────────
case 'github.pr.create': {
  try {
    const { handleGitHubPrCreate } = await import('./external-api-connector')
    return (await handleGitHubPrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.create unavailable: ${err instanceof Error ? err.message : String(err)}`)
  }
}

case 'github.pr.merge': {
  try {
    const { handleGitHubPrMerge } = await import('./external-api-connector')
    return (await handleGitHubPrMerge(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.merge unavailable: ${err instanceof Error ? err.message : String(err)}`)
  }
}

case 'github.issue.list': {
  try {
    const { handleGitHubIssueList } = await import('./external-api-connector')
    return (await handleGitHubIssueList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    return makeError(rpc.id, AgentErrorCode.ServerError, `github.issue.list unavailable: ${err instanceof Error ? err.message : String(err)}`)
  }
}

case 'github.issue.create': {
  try {
    const { handleGitHubIssueCreate } = await import('./external-api-connector')
    return (await handleGitHubIssueCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    return makeError(rpc.id, AgentErrorCode.ServerError, `github.issue.create unavailable: ${err instanceof Error ? err.message : String(err)}`)
  }
}

case 'gitlab.mr.create': {
  try {
    const { handleGitLabMrCreate } = await import('./external-api-connector')
    return (await handleGitLabMrCreate(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.create unavailable: ${err instanceof Error ? err.message : String(err)}`)
  }
}

case 'gitlab.pipeline.status': {
  try {
    const { handleGitLabPipelineStatus } = await import('./external-api-connector')
    return (await handleGitLabPipelineStatus(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
  } catch (err: unknown) {
    return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.pipeline.status unavailable: ${err instanceof Error ? err.message : String(err)}`)
  }
}
```

---

## 7. Tests

```typescript
// src/relay/__tests__/external-api-connector.test.ts

describe('SHELL_METACHARACTERS validation', () => {
  it('rejects semicolon in PR title', async () => {
    const res = await handleGitHubPrCreate(null, {
      title: 'Fix bug; rm -rf /', body: 'body', base: 'main', cwd: '/tmp', userId: 'u1'
    }, config, log) as { error: { code: number } }
    expect(res.error.code).toBe(-32602)
  })

  it('rejects backtick in base branch', async () => {
    const res = await handleGitHubPrCreate(null, {
      title: 'Safe title', body: 'body', base: 'main`whoami`', cwd: '/tmp', userId: 'u1'
    }, config, log) as { error: { code: number } }
    expect(res.error.code).toBe(-32602)
  })

  it('rejects pipe in MR title', async () => {
    const res = await handleGitLabMrCreate(null, {
      title: 'Fix | cat /etc/passwd', body: 'body', targetBranch: 'main', cwd: '/tmp', userId: 'u1'
    }, config, log) as { error: { code: number } }
    expect(res.error.code).toBe(-32602)
  })
})

describe('buildGhEnv', () => {
  it('sets GH_CONFIG_DIR per userId', () => {
    const env = buildGhEnv('user-abc', {})
    expect(env.GH_CONFIG_DIR).toContain('user-abc')
    expect(env.GH_CONFIG_DIR).toContain('.config/gh/')
  })

  it('different userIds get different GH_CONFIG_DIR', () => {
    const env1 = buildGhEnv('alice', {})
    const env2 = buildGhEnv('bob', {})
    expect(env1.GH_CONFIG_DIR).not.toBe(env2.GH_CONFIG_DIR)
  })

  it('sets GH_NO_UPDATE_NOTIFIER=1', () => {
    const env = buildGhEnv('user-abc', {})
    expect(env.GH_NO_UPDATE_NOTIFIER).toBe('1')
  })
})

describe('buildGlabEnv', () => {
  it('sets GLAB_CONFIG_DIR per userId', () => {
    const env = buildGlabEnv('user-xyz', {})
    expect(env.GLAB_CONFIG_DIR).toContain('user-xyz')
    expect(env.GLAB_CONFIG_DIR).toContain('.config/glab-cli/')
  })

  it('sets CI=1 for non-interactive mode', () => {
    const env = buildGlabEnv('user-xyz', {})
    expect(env.CI).toBe('1')
  })
})

describe('handleGitHubPrCreate — validation', () => {
  it('returns InvalidParams for missing title', async () => {
    const res = await handleGitHubPrCreate(null, {
      body: 'body', base: 'main', cwd: '/tmp', userId: 'u1'
    }, config, log) as { error: { code: number } }
    expect(res.error.code).toBe(-32602)
  })

  it('returns InvalidParams for empty title', async () => {
    const res = await handleGitHubPrCreate(null, {
      title: '   ', body: 'body', base: 'main', cwd: '/tmp', userId: 'u1'
    }, config, log) as { error: { code: number } }
    expect(res.error.code).toBe(-32602)
  })
})

describe('handleGitLabMrCreate — validation', () => {
  it('returns InvalidParams for missing title', async () => {
    const res = await handleGitLabMrCreate(null, {
      description: 'desc', targetBranch: 'main', cwd: '/tmp', userId: 'u1'
    }, config, log) as { error: { code: number } }
    expect(res.error.code).toBe(-32602)
  })
})

describe('execFileCaptured — timeout', () => {
  it('returns exitCode 124 and stderr on timeout', async () => {
    const result = await execFileCaptured('sleep', ['100'], {
      cwd: '/tmp', env: process.env, timeout: 100
    })
    expect(result.exitCode).toBe(124)
    expect(result.stderr).toContain('Timeout')
  })
})
```

**Target: ≥ 20 tests**

---

## 8. Design Principles (từ HLD §12.5)

| Principle | Implementation |
|-----------|---------------|
| CLI-based, not SDK | `execFile('gh', ...)` không phải `@octokit/rest` |
| Per-user isolation | `buildGhEnv(userId)` / `buildGlabEnv(userId)` per call |
| No shell injection | `shell: false` trong tất cả spawn, args là array |
| Metachar validation | `SHELL_METACHARACTERS.test()` trước khi pass vào CLI |
| Timeout mandatory | 30s default cho tất cả external calls |
| Idempotency | `checkExistingPr()` trước khi tạo PR mới |
| Error forwarding | `stderr` từ CLI forward về Gateway trong `error.details` |
| Auth isolation | GitHub/GitLab tokens KHÔNG qua Gateway |

---

## 9. Implementation Checklist

- [ ] `src/relay/external-api-connector.ts` — tạo file mới
- [ ] `buildGhEnv(userId)` + `buildGlabEnv(userId)` helpers
- [ ] `execFileCaptured()` helper (shell: false, timeout, capture stdout/stderr)
- [ ] `handleGitHubPrCreate()` — với idempotency check
- [ ] `handleGitHubPrMerge()` — squash merge
- [ ] `handleGitHubIssueList()` — list issues JSON
- [ ] `handleGitHubIssueCreate()` — create issue
- [ ] `handleGitHubAuthStatus()` — preflight check
- [ ] `handleGitLabMrCreate()` — create MR
- [ ] `handleGitLabMrList()` / `handleGitLabMrView()`
- [ ] `handleGitLabPipelineStatus()` — CI status
- [ ] `handleGitLabAuthStatus()` — preflight check
- [ ] `src/relay/agent-rpc-dispatch.ts` — thêm 6 routes mới
- [ ] `src/relay/__tests__/external-api-connector.test.ts` — tạo test file
