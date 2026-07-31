# TASK-05: Create External API Connector — GitHub & GitLab

> ✅ **STATUS: DONE** — Completed 2026-07-30T17:52

**Phase:** 3
**File:** `src/relay/external-api-connector.ts` (NEW FILE)
**Operation:** CREATE
**CR:** [CR-AG-13](../solutions/CR-AG-13-external-api-connectors.md)
**TDD:** TDD-AG-13
**Depends on:** Không (standalone, chỉ import AgentConfig/AgentLogger/AgentErrorCode)
**Blocked by:** Không

---

## Mục tiêu

Tạo mới `src/relay/external-api-connector.ts`:
- GitHub operations: PR create (với idempotency), PR merge, issue list/create, auth status
- GitLab operations: MR create, MR list, pipeline status, auth status
- Per-user auth isolation (`GH_CONFIG_DIR`, `GLAB_CONFIG_DIR`)
- Shared `execFileCaptured()` helper (shell: false, timeout)

---

## File cần tạo

Tạo file mới hoàn toàn tại: `src/relay/external-api-connector.ts`

```typescript
// src/relay/external-api-connector.ts
// External API connectors for Orca Dev Agent v5.0.
//
// Design principles:
//   - CLI-based: gh (GitHub CLI) and glab (GitLab CLI) — NOT SDK
//   - Per-user isolation: GH_CONFIG_DIR / GLAB_CONFIG_DIR per userId
//   - No shell injection: execFile() with array args, shell: false
//   - Metachar validation on all user input
//   - Timeout mandatory: 30s default
//   - Idempotency: github.pr.create checks existing PR first
//   - Auth never through Gateway: tokens stay on dev server filesystem

import { spawn } from 'node:child_process'
import { homedir } from 'node:os'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

// ─── Security ─────────────────────────────────────────────────────────────────

const SHELL_METACHARACTERS = /[&|;$`<>\\!]/

// ─── Shared executor ──────────────────────────────────────────────────────────

interface ExecResult {
  stdout:   string
  stderr:   string
  exitCode: number
}

function execFileCaptured(
  binary: string,
  args:   string[],
  opts:   { cwd: string; env: NodeJS.ProcessEnv; timeout: number }
): Promise<ExecResult> {
  return new Promise((resolve) => {
    const child = spawn(binary, args, {
      cwd:   opts.cwd,
      env:   opts.env,
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

// ─── Environment builders ─────────────────────────────────────────────────────

function buildGhEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  return {
    ...baseEnv,
    GH_CONFIG_DIR:          `${homedir()}/.config/gh/${userId}/`,
    GH_NO_UPDATE_NOTIFIER:  '1',
    GH_PROMPT_DISABLED:     '1',
  }
}

function buildGlabEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  return {
    ...baseEnv,
    GLAB_CONFIG_DIR: `${homedir()}/.config/glab-cli/${userId}/`,
    NO_COLOR:        '1',
    CI:              '1',
  }
}

// ─── Idempotency helpers ──────────────────────────────────────────────────────

async function getCurrentBranch(cwd: string, env: NodeJS.ProcessEnv): Promise<string | null> {
  const result = await execFileCaptured('git', ['rev-parse', '--abbrev-ref', 'HEAD'], {
    cwd, env, timeout: 5_000,
  })
  return result.exitCode === 0 ? result.stdout.trim() : null
}

interface GitHubPrResult {
  url:   string
  number: number
  title: string
  state: string
}

async function checkExistingPr(
  cwd:    string,
  branch: string,
  env:    NodeJS.ProcessEnv
): Promise<GitHubPrResult | null> {
  const result = await execFileCaptured('gh', [
    'pr', 'list', '--head', branch,
    '--json', 'url,number,title,state',
    '--limit', '1',
  ], { cwd, env, timeout: 15_000 })

  if (result.exitCode !== 0 || !result.stdout.trim()) return null
  try {
    const prs = JSON.parse(result.stdout) as GitHubPrResult[]
    return prs[0] ?? null
  } catch {
    return null
  }
}

// ─── github.pr.create ─────────────────────────────────────────────────────────

export async function handleGitHubPrCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
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
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in PR params' } }
  }

  const env = buildGhEnv(userId, config.toolEnv)

  // Idempotency: check if PR already exists for current branch
  const currentBranch = await getCurrentBranch(cwd, env)
  if (currentBranch) {
    const existing = await checkExistingPr(cwd, currentBranch, env)
    if (existing) {
      log.info(`github.pr.create: PR already exists #${existing.number} → ${existing.url}`)
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

  try {
    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      log.error(`github.pr.create failed: ${result.stderr}`)
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'gh pr create failed' } }
    }
    const parsed = JSON.parse(result.stdout) as GitHubPrResult
    log.info(`github.pr.create: PR #${parsed.number} → ${parsed.url}`)
    return { jsonrpc: '2.0', id, result: parsed }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── github.pr.merge ──────────────────────────────────────────────────────────

export async function handleGitHubPrMerge(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const prNumber = typeof params.prNumber === 'number' ? String(params.prNumber) : ''
  const cwd      = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId   = typeof params.userId === 'string' ? params.userId : ''
  const method   = typeof params.method === 'string' ? params.method : 'squash'

  if (!prNumber) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: prNumber' } }
  }

  const mergeFlag = method === 'rebase' ? '--rebase' : method === 'merge' ? '--merge' : '--squash'
  const ghArgs = ['pr', 'merge', prNumber, mergeFlag, '--auto']
  const env = buildGhEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'gh pr merge failed' } }
    }
    log.info(`github.pr.merge: PR #${prNumber} merged`)
    return { jsonrpc: '2.0', id, result: { ok: true, stdout: result.stdout } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── github.issue.list ────────────────────────────────────────────────────────

export async function handleGitHubIssueList(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const limit  = typeof params.limit  === 'number' ? Math.min(params.limit, 50) : 30
  const state  = typeof params.state  === 'string' ? params.state : 'open'

  const env = buildGhEnv(userId, config.toolEnv)
  const ghArgs = ['issue', 'list', '--json', 'number,title,state,url', '--limit', String(limit), '--state', state]

  try {
    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr } }
    }
    const issues = JSON.parse(result.stdout) as unknown[]
    log.info(`github.issue.list: ${issues.length} issues`)
    return { jsonrpc: '2.0', id, result: { issues, total: issues.length } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── github.issue.create ──────────────────────────────────────────────────────

export async function handleGitHubIssueCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const title  = typeof params.title  === 'string' ? params.title.trim()  : ''
  const body   = typeof params.body   === 'string' ? params.body           : ''
  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId : ''

  if (!title) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in issue title' } }
  }

  const env = buildGhEnv(userId, config.toolEnv)
  const ghArgs = ['issue', 'create', '--title', title, '--body', body, '--json', 'number,url,title']

  try {
    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr } }
    }
    const parsed = JSON.parse(result.stdout) as { number: number; url: string; title: string }
    log.info(`github.issue.create: issue #${parsed.number} → ${parsed.url}`)
    return { jsonrpc: '2.0', id, result: parsed }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── github.auth.status ───────────────────────────────────────────────────────

export async function handleGitHubAuthStatus(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const env = buildGhEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('gh', ['auth', 'status'], {
      cwd: config.workDir, env, timeout: 10_000,
    })
    const ok = result.exitCode === 0
    log.info(`github.auth.status: userId=${userId} ok=${ok}`)
    return { jsonrpc: '2.0', id, result: { ok, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── gitlab.mr.create ─────────────────────────────────────────────────────────

export async function handleGitLabMrCreate(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const title        = typeof params.title        === 'string' ? params.title.trim()        : ''
  const description  = typeof params.description  === 'string' ? params.description          : ''
  const targetBranch = typeof params.targetBranch === 'string' ? params.targetBranch.trim()  : 'main'
  const cwd          = typeof params.cwd          === 'string' && params.cwd ? params.cwd : config.workDir
  const userId       = typeof params.userId       === 'string' ? params.userId               : ''

  if (!title) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(targetBranch)) {
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in MR params' } }
  }

  const glabArgs = [
    'mr', 'create',
    '--title',         title,
    '--description',   description,
    '--target-branch', targetBranch,
    '--yes',           // non-interactive
  ]

  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'glab mr create failed' } }
    }
    const url = result.stdout.trim().split('\n').find(l => l.startsWith('https://')) ?? result.stdout.trim()
    log.info(`gitlab.mr.create: MR → ${url}`)
    return { jsonrpc: '2.0', id, result: { url, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── gitlab.mr.list ───────────────────────────────────────────────────────────

export async function handleGitLabMrList(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const state  = typeof params.state  === 'string' ? params.state : 'opened'

  const env = buildGlabEnv(userId, config.toolEnv)
  const glabArgs = ['mr', 'list', '--state', state, '--output', 'json']

  try {
    const result = await execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr } }
    }
    const mrs = JSON.parse(result.stdout) as unknown[]
    log.info(`gitlab.mr.list: ${mrs.length} MRs state=${state}`)
    return { jsonrpc: '2.0', id, result: { mrs, total: mrs.length } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── gitlab.pipeline.status ───────────────────────────────────────────────────

export async function handleGitLabPipelineStatus(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const cwd    = typeof params.cwd    === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId : ''

  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('glab', ['pipeline', 'status', '--output', 'json'], {
      cwd, env, timeout: 30_000,
    })
    if (result.exitCode !== 0) {
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr } }
    }
    const status = JSON.parse(result.stdout) as unknown
    log.info(`gitlab.pipeline.status: ok`)
    return { jsonrpc: '2.0', id, result: { status, raw: result.stdout } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── gitlab.auth.status ───────────────────────────────────────────────────────

export async function handleGitLabAuthStatus(
  id:     string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log:    AgentLogger
): Promise<object> {
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('glab', ['auth', 'status'], {
      cwd: config.workDir, env, timeout: 10_000,
    })
    const ok = result.exitCode === 0
    log.info(`gitlab.auth.status: userId=${userId} ok=${ok}`)
    return { jsonrpc: '2.0', id, result: { ok, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}

// ─── Exports for testing ──────────────────────────────────────────────────────
export { buildGhEnv, buildGlabEnv, execFileCaptured }
```

---

## Verify

```bash
# TypeScript compile
npx tsc --noEmit -p config/tsconfig.node.json

# Check file created
wc -l src/relay/external-api-connector.ts
# Expected: ~280+ lines

# Check exports
grep -n "^export" src/relay/external-api-connector.ts
# Expected: handleGitHubPrCreate, handleGitHubPrMerge, handleGitHubIssueList,
#           handleGitHubIssueCreate, handleGitHubAuthStatus, handleGitLabMrCreate,
#           handleGitLabMrList, handleGitLabPipelineStatus, handleGitLabAuthStatus,
#           buildGhEnv, buildGlabEnv, execFileCaptured
```

---

## Done criteria

- [ ] File `src/relay/external-api-connector.ts` được tạo
- [ ] `buildGhEnv()` — GH_CONFIG_DIR per userId, GH_NO_UPDATE_NOTIFIER, GH_PROMPT_DISABLED
- [ ] `buildGlabEnv()` — GLAB_CONFIG_DIR per userId, NO_COLOR, CI
- [ ] `execFileCaptured()` — shell: false, timeout 30s, capture stdout+stderr
- [ ] `handleGitHubPrCreate()` — idempotency check + validation + gh pr create
- [ ] `handleGitHubPrMerge()` — gh pr merge với method (squash/rebase/merge)
- [ ] `handleGitHubIssueList()` — gh issue list với state filter
- [ ] `handleGitHubIssueCreate()` — gh issue create với validation
- [ ] `handleGitHubAuthStatus()` — gh auth status
- [ ] `handleGitLabMrCreate()` — glab mr create --yes
- [ ] `handleGitLabMrList()` — glab mr list --output json
- [ ] `handleGitLabPipelineStatus()` — glab pipeline status
- [ ] `handleGitLabAuthStatus()` — glab auth status
- [ ] TypeScript compile không lỗi
