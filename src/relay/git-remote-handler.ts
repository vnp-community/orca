/**
 * git-remote-handler.ts — Relay-side remote git operations (TASK-044)
 *
 * Separate module from git-handler.ts to handle new write operations:
 * status, diff, add, restore, commit, push, pull, fetch, branch, checkout,
 * merge, rebase, stash, log, worktree, tag, show, rev-parse.
 *
 * Security rules:
 * - Only ALLOWED_GIT_SUBCOMMANDS whitelist
 * - Shell metacharacters (&|;$`) forbidden in args
 * - execFile() NOT exec() (no shell injection)
 * - maxBuffer: 10MB, default timeout: 30s
 *
 * @module relay/git-remote-handler
 */

import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)

// ── Security constants ─────────────────────────────────────────────────────────

export const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag', 'show', 'rev-parse',
])

const SHELL_METACHARACTERS = /[&|;$`]/

const MAX_BUFFER_BYTES = 10 * 1024 * 1024
const DEFAULT_TIMEOUT_MS = 30_000

// ── Validation ─────────────────────────────────────────────────────────────────

export function validateGitArgs(args: string[]): void {
  if (args.length === 0) {
    throw new Error('GIT_NO_SUBCOMMAND')
  }
  const subcommand = args[0]
  if (!ALLOWED_GIT_SUBCOMMANDS.has(subcommand!)) {
    throw new Error(`GIT_DISALLOWED_SUBCOMMAND: ${subcommand}`)
  }
  for (const arg of args) {
    if (SHELL_METACHARACTERS.test(arg)) {
      throw new Error(`GIT_SHELL_METACHARACTER_IN_ARG: ${arg}`)
    }
  }
}

// ── Types ──────────────────────────────────────────────────────────────────────

export interface GitExecResult {
  stdout: string
  stderr: string
  exitCode: number
}

// ── Handlers ──────────────────────────────────────────────────────────────────

export const gitRemoteHandlers = {
  'git.exec': async (params: {
    cwd?: string
    args: string[]
    timeout?: number
  }): Promise<GitExecResult> => {
    validateGitArgs(params.args)

    try {
      const { stdout, stderr } = await execFileAsync('git', params.args, {
        cwd: params.cwd ?? process.cwd(),
        maxBuffer: MAX_BUFFER_BYTES,
        timeout: params.timeout ?? DEFAULT_TIMEOUT_MS,
      })
      return { stdout, stderr, exitCode: 0 }
    } catch (err) {
      const e = err as NodeJS.ErrnoException & { stdout?: string; stderr?: string; code?: number | string }
      return {
        stdout: e.stdout ?? '',
        stderr: e.stderr ?? e.message,
        exitCode: typeof e.code === 'number' ? e.code : 1,
      }
    }
  },

  'git.execStream': async (params: {
    cwd?: string
    args: string[]
  }): Promise<GitExecResult> => {
    return gitRemoteHandlers['git.exec']({ cwd: params.cwd, args: params.args })
  },
}
