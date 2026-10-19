// src/relay/agent-git-worktree-handler.ts
// git.worktree.list/add/remove — split out of agent-git-handler.ts (which
// crossed the repo's oxlint max-lines budget) to keep both files under it.
// Pure mechanical extraction: bodies unchanged, still delegates worktree
// add/remove to agent-git-handler.ts's handleGitExec for the actual git
// invocation, and reuses its resumeFrom/SHELL_METACHARACTERS helpers.

import * as path from 'node:path'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { resolveWorktreeStartPoint, WorktreeBaseRefError } from '../shared/worktree-base-ref'
import { Tracers } from '../shared/trace/tracers'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { handleGitExec, resumeFrom, SHELL_METACHARACTERS } from './agent-git-handler'
import { assertWorktreeOnBranch } from './worktree-add-branch-guard'
import { createWorktreeStartPointProbe } from './worktree-start-point-git-probe'

// Why: handleGitExec's allowlist has no symbolic-ref, and probes need the exit
// code on the thrown error to tell "ref missing" (1) from a broken git.
async function runGitForStdout(config: AgentConfig, args: string[], cwd: string): Promise<string> {
  const { execFile } = await import('node:child_process')
  const { promisify } = await import('node:util')
  const { stdout } = await promisify(execFile)('git', args, {
    cwd,
    env: { ...config.toolEnv },
    timeout: 10_000
  })
  return stdout
}

function gitExecSucceeded(result: object): boolean {
  if ('error' in result) {
    return false
  }
  return (result as { result?: { exitCode?: number } }).result?.exitCode === 0
}

// ─── git.worktree.list ────────────────────────────────────────────────────────

/**
 * List git worktrees — returns structured WorktreeInfo[] (WT-Issue-4).
 * Parses `git worktree list --porcelain` output for typed response.
 */
export async function handleGitWorktreeList(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const cwd = typeof params.cwd === 'string' ? params.cwd : config.workDir
  try {
    const { execFile } = await import('node:child_process')
    const { promisify } = await import('node:util')
    const { parseWorktreePorcelain } = await import('./git-handler')
    const execAsync = promisify(execFile)
    const { stdout } = await execAsync('git', ['worktree', 'list', '--porcelain'], {
      cwd,
      timeout: 10_000
    })
    const worktrees = parseWorktreePorcelain(stdout)
    log.info(`git.worktree.list: cwd=${cwd} count=${worktrees.length}`)
    return { jsonrpc: '2.0', id, result: { worktrees } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.ServerError, message: `git.worktree.list failed: ${msg}` }
    }
  }
}

// ─── git.worktree.add ─────────────────────────────────────────────────────────

export async function handleGitWorktreeAdd(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const worktreePath = typeof params.path === 'string' ? params.path.trim() : ''
  const branch = typeof params.branch === 'string' ? params.branch.trim() : ''
  const createBranch = params.createBranch === true
  const baseRef = typeof params.baseRef === 'string' ? params.baseRef.trim() : ''
  const cwd = typeof params.cwd === 'string' ? params.cwd : config.workDir

  const span = Tracers.worktreeCreate.start({ path: worktreePath, branch, cwd }, resumeFrom(params))

  if (!worktreePath || !branch) {
    span.fail('missing required params', { path: worktreePath, branch })
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: 'Missing required params: path, branch'
      }
    }
  }
  if (SHELL_METACHARACTERS.test(worktreePath) || SHELL_METACHARACTERS.test(branch)) {
    span.fail('unsafe characters in params', { path: worktreePath, branch })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in worktree params' }
    }
  }

  // Why: `-b -x` / `-b --detach` would make git read the branch as a flag.
  if (branch.startsWith('-')) {
    span.fail('branch starts with "-"', { branch })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Branch name must not start with "-"' }
    }
  }

  // WT-Issue-1: Security validation — prevent path traversal
  try {
    const { validateWorktreePath } = await import('./git-handler')
    span.step('validate-path', { path: worktreePath })
    validateWorktreePath(['worktree', 'add', worktreePath], cwd)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg, { path: worktreePath })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: msg } }
  }

  // baseRef is an optional start-point for the new branch (git-gateway-
  // service's CreateWorktreeInput.BaseRef) — only meaningful when creating a
  // branch; omitted, `git worktree add -b` defaults to cwd's HEAD.
  const gitCwd = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  let startPoint = ''
  if (createBranch && baseRef) {
    try {
      span.step('resolve-start-point', { baseRef })
      startPoint = await resolveWorktreeStartPoint(
        baseRef,
        createWorktreeStartPointProbe((probeArgs) => runGitForStdout(config, probeArgs, gitCwd))
      )
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      span.fail(msg, { baseRef })
      if (err instanceof WorktreeBaseRefError && err.code === 'NOT_FOUND') {
        // Why: same shape as git's own failure, which git-gateway-service classifies
        // (isBaseRefNotFoundErr) into WORKTREE_BASE_REF_NOT_FOUND.
        return {
          jsonrpc: '2.0',
          id,
          result: { stdout: '', stderr: `fatal: ${msg}`, exitCode: 128 }
        }
      }
      const code =
        err instanceof WorktreeBaseRefError
          ? AgentErrorCode.InvalidParams
          : AgentErrorCode.ServerError
      return { jsonrpc: '2.0', id, error: { code, message: msg } }
    }
  }

  const args = createBranch
    ? startPoint
      ? ['worktree', 'add', '-b', branch, worktreePath, startPoint]
      : ['worktree', 'add', '-b', branch, worktreePath]
    : ['worktree', 'add', worktreePath, branch]

  span.step('git-worktree-add-exec', { branch })
  const result = await handleGitExec(
    id,
    { args, cwd: params.cwd, timeout: 15_000, _trace: { id: span.id } },
    config,
    log
  )
  if (!gitExecSucceeded(result)) {
    const message =
      'error' in result
        ? (result as { error: { message: string } }).error.message
        : 'git exit non-zero'
    span.fail(message, { path: worktreePath })
    return result
  }

  try {
    await assertWorktreeOnBranch(
      (guardArgs, guardCwd) => runGitForStdout(config, guardArgs, guardCwd),
      gitCwd,
      path.resolve(gitCwd, worktreePath),
      branch
    )
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg, { path: worktreePath, branch })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
  span.ok({ path: worktreePath, branch })
  return result
}

// ─── git.worktree.remove ──────────────────────────────────────────────────────

export async function handleGitWorktreeRemove(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const path = typeof params.path === 'string' ? params.path.trim() : ''
  const force = params.force === true

  const span = Tracers.worktreeDelete.start({ path, force }, resumeFrom(params))

  if (!path) {
    span.fail('missing required param: path')
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
  }
  if (SHELL_METACHARACTERS.test(path)) {
    span.fail('unsafe characters in path', { path })
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Unsafe characters in path' }
    }
  }

  const args = ['worktree', 'remove', path]
  if (force) {
    args.push('--force')
  }

  span.step('git-worktree-remove-exec', { force })
  const result = await handleGitExec(
    id,
    { args, cwd: params.cwd, timeout: 15_000, _trace: { id: span.id } },
    config,
    log
  )
  if (result && typeof result === 'object' && 'error' in result) {
    span.fail((result as { error: { message: string } }).error.message, { path })
  } else {
    span.ok({ path, force })
  }
  return result
}
