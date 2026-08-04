// src/relay/external-api-connector.ts
// External API connectors for Orca Dev Agent v5.0.
//
// Design principles:
//   - CLI-based: gh (GitHub CLI) and glab (GitLab CLI) — NOT SDK
//   - Per-user isolation: GH_CONFIG_DIR / GLAB_CONFIG_DIR per userId
//   - No shell injection: spawn() with array args, shell: false
//   - Metachar validation on all user input
//   - Timeout mandatory: 30s default
//   - Idempotency: github.pr.create checks existing PR first
//   - Auth never through Gateway: tokens stay on dev server filesystem

import { spawn } from 'node:child_process'
import { homedir } from 'node:os'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const apiTracer = createTracer('agent:ext-api')

// ─── Security ─────────────────────────────────────────────────────────────────

const SHELL_METACHARACTERS = /[&|;$`<>\\!]/

// ─── Shared executor ──────────────────────────────────────────────────────────

interface ExecResult {
  stdout:   string
  stderr:   string
  exitCode: number
}

export function execFileCaptured(
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

export function buildGhEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  return {
    ...baseEnv,
    GH_CONFIG_DIR:          `${homedir()}/.config/gh/${userId}/`,
    GH_NO_UPDATE_NOTIFIER:  '1',
    GH_PROMPT_DISABLED:     '1',
  }
}

export function buildGlabEnv(userId: string, baseEnv: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
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
  url:    string
  number: number
  title:  string
  state:  string
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
  const span   = apiTracer.start({ method: 'github.pr.create', title: title.slice(0, 40), base })

  if (!title) {
    span.fail('missing title', { method: 'github.pr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(base)) {
    span.fail('unsafe characters in params', { method: 'github.pr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in PR params' } }
  }

  const env = buildGhEnv(userId, config.toolEnv)

  // Idempotency: check if PR already exists for current branch
  const currentBranch = await getCurrentBranch(cwd, env)
  if (currentBranch) {
    const existing = await checkExistingPr(cwd, currentBranch, env)
    if (existing) {
      log.info(`github.pr.create: PR already exists #${existing.number} → ${existing.url}`)
      span.ok({ prNumber: existing.number, url: existing.url, alreadyExisted: true })
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
      span.fail(result.stderr || 'gh pr create failed', { method: 'github.pr.create', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'gh pr create failed' } }
    }
    const parsed = JSON.parse(result.stdout) as GitHubPrResult
    log.info(`github.pr.create: PR #${parsed.number} → ${parsed.url}`)
    span.ok({ prNumber: parsed.number, url: parsed.url })
    return { jsonrpc: '2.0', id, result: parsed }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { method: 'github.pr.create' })
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
  const span     = apiTracer.start({ method: 'github.pr.merge', prNumber: prNumber || '(empty)', mergeMethod: method })

  if (!prNumber) {
    span.fail('missing prNumber', { method: 'github.pr.merge' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: prNumber' } }
  }

  const mergeFlag = method === 'rebase' ? '--rebase' : method === 'merge' ? '--merge' : '--squash'
  const ghArgs = ['pr', 'merge', prNumber, mergeFlag, '--auto']
  const env = buildGhEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'gh pr merge failed', { prNumber, exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'gh pr merge failed' } }
    }
    log.info(`github.pr.merge: PR #${prNumber} merged`)
    span.ok({ prNumber, method })
    return { jsonrpc: '2.0', id, result: { ok: true, stdout: result.stdout } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { prNumber, method })
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
  const span   = apiTracer.start({ method: 'github.issue.list', state, limit })

  const env = buildGhEnv(userId, config.toolEnv)
  const ghArgs = ['issue', 'list', '--json', 'number,title,state,url', '--limit', String(limit), '--state', state]

  try {
    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'gh issue list failed', { method: 'github.issue.list', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr } }
    }
    const issues = JSON.parse(result.stdout) as unknown[]
    log.info(`github.issue.list: ${issues.length} issues`)
    span.ok({ total: issues.length })
    return { jsonrpc: '2.0', id, result: { issues, total: issues.length } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { method: 'github.issue.list' })
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
  const span   = apiTracer.start({ method: 'github.issue.create', title: title.slice(0, 40) })

  if (!title) {
    span.fail('missing title', { method: 'github.issue.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title)) {
    span.fail('unsafe characters in params', { method: 'github.issue.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in issue title' } }
  }

  const env = buildGhEnv(userId, config.toolEnv)
  const ghArgs = ['issue', 'create', '--title', title, '--body', body, '--json', 'number,url,title']

  try {
    const result = await execFileCaptured('gh', ghArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'gh issue create failed', { method: 'github.issue.create', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr } }
    }
    const parsed = JSON.parse(result.stdout) as { number: number; url: string; title: string }
    log.info(`github.issue.create: issue #${parsed.number} → ${parsed.url}`)
    span.ok({ issueNumber: parsed.number, url: parsed.url })
    return { jsonrpc: '2.0', id, result: parsed }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { method: 'github.issue.create' })
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
  const span = apiTracer.start({ method: 'github.auth.status', cli: 'gh' })

  span.step('exec', { cli: 'gh' })
  try {
    const result = await execFileCaptured('gh', ['auth', 'status'], {
      cwd: config.workDir, env, timeout: 10_000,
    })
    const ok = result.exitCode === 0
    log.info(`github.auth.status: userId=${userId} ok=${ok}`)
    if (ok) {
      span.ok({ cli: 'gh', authenticated: ok })
    } else {
      span.fail(result.stderr || 'gh auth status non-zero exit', { cli: 'gh', exitCode: result.exitCode, authenticated: false })
    }
    return { jsonrpc: '2.0', id, result: { ok, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { cli: 'gh' })
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
  const span = apiTracer.start({ method: 'gitlab.mr.create', title: title.slice(0, 40), targetBranch })

  if (!title) {
    span.fail('missing title', { method: 'gitlab.mr.create' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: title' } }
  }
  if (SHELL_METACHARACTERS.test(title) || SHELL_METACHARACTERS.test(targetBranch)) {
    span.fail('unsafe characters in params', { method: 'gitlab.mr.create' })
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
      span.fail(result.stderr || 'glab mr create failed', { method: 'gitlab.mr.create', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr || 'glab mr create failed' } }
    }
    const url = result.stdout.trim().split('\n').find((l: string) => l.startsWith('https://')) ?? result.stdout.trim()
    log.info(`gitlab.mr.create: MR → ${url}`)
    span.ok({ url })
    return { jsonrpc: '2.0', id, result: { url, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { method: 'gitlab.mr.create' })
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
  const span   = apiTracer.start({ method: 'gitlab.mr.list', state })

  const env = buildGlabEnv(userId, config.toolEnv)
  const glabArgs = ['mr', 'list', '--state', state, '--output', 'json']

  try {
    const result = await execFileCaptured('glab', glabArgs, { cwd, env, timeout: 30_000 })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'glab mr list failed', { method: 'gitlab.mr.list', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr } }
    }
    const mrs = JSON.parse(result.stdout) as unknown[]
    log.info(`gitlab.mr.list: ${mrs.length} MRs state=${state}`)
    span.ok({ total: mrs.length })
    return { jsonrpc: '2.0', id, result: { mrs, total: mrs.length } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { method: 'gitlab.mr.list' })
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
  const span   = apiTracer.start({ method: 'gitlab.pipeline.status' })

  const env = buildGlabEnv(userId, config.toolEnv)

  try {
    const result = await execFileCaptured('glab', ['pipeline', 'status', '--output', 'json'], {
      cwd, env, timeout: 30_000,
    })
    if (result.exitCode !== 0) {
      span.fail(result.stderr || 'glab pipeline status failed', { method: 'gitlab.pipeline.status', exitCode: result.exitCode })
      return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: result.stderr } }
    }
    const status = JSON.parse(result.stdout) as unknown
    log.info(`gitlab.pipeline.status: ok`)
    span.ok({})
    return { jsonrpc: '2.0', id, result: { status, raw: result.stdout } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { method: 'gitlab.pipeline.status' })
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
  const span = apiTracer.start({ method: 'gitlab.auth.status', cli: 'glab' })

  span.step('exec', { cli: 'glab' })
  try {
    const result = await execFileCaptured('glab', ['auth', 'status'], {
      cwd: config.workDir, env, timeout: 10_000,
    })
    const ok = result.exitCode === 0
    log.info(`gitlab.auth.status: userId=${userId} ok=${ok}`)
    if (ok) {
      span.ok({ cli: 'glab', authenticated: ok })
    } else {
      span.fail(result.stderr || 'glab auth status non-zero exit', { cli: 'glab', exitCode: result.exitCode, authenticated: false })
    }
    return { jsonrpc: '2.0', id, result: { ok, stdout: result.stdout, stderr: result.stderr } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(err, { cli: 'glab' })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
