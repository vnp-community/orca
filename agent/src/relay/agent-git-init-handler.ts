// src/relay/agent-git-init-handler.ts
// Part A (direct-websocket/relay-websocket) implementation of `git.init`.
//
// Mirrors agent-git-clone-handler.ts's shape exactly (same argv-injection
// guard pattern, same buildRelayGitEnv() env, same span/notify conventions)
// — added for the "Initialize as Git repo" feature: a folder that's been
// added to Orca but isn't a git repository yet (WORKTREE_DETECT_FAILED /
// "not a valid git repository") gets a button offering to run `git init`
// right there, optionally attaching a remote in the same call so the user
// doesn't need a second round trip.
//
// remote add is intentionally NOT routed through git.exec — see
// git-exec-validator.ts's REMOTE_WRITE_SUBCOMMANDS blocklist, which
// deliberately rejects `git remote add` via that generic relay (any
// write-shaped git subcommand there is opaque and unauditable). This
// handler instead runs the exact, fully-known `git remote add <name> <url>`
// argv itself, same narrow-purpose-built pattern as git.clone's `git clone
// --progress -- <url> <targetPath>`.

import { spawn } from 'node:child_process'
import { mkdir } from 'node:fs/promises'
import { buildRelayGitEnv } from './relay-command-env'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const initTracer = createTracer('agent:git-init')

type NotifyFn = (method: string, params: Record<string, unknown>) => void

function assertSafeArg(name: string, value: string): void {
  if (!value || value.startsWith('-') || value.includes('\0')) {
    throw new Error(`git.init: invalid ${name}`)
  }
}

function assertSafeInitParams(
  destPath: string,
  defaultBranch: string,
  remoteName: string,
  remoteUrl: string
): void {
  assertSafeArg('destPath', destPath)
  if (defaultBranch) {
    assertSafeArg('defaultBranch', defaultBranch)
  }
  if (remoteUrl) {
    // remoteUrl implies remoteName is required too (defaulted by the
    // caller below before this runs) — both must be argv-safe.
    assertSafeArg('remoteName', remoteName)
    assertSafeArg('remoteUrl', remoteUrl)
  }
}

function runGit(args: string[], cwd: string, notify: NotifyFn, label: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const child = spawn('git', args, {
      cwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: buildRelayGitEnv()
    })
    let stderr = ''
    let settled = false
    child.stdout.on('data', (chunk: Buffer) => {
      notify('git.init.output', { data: chunk.toString('utf-8') })
    })
    child.stderr.on('data', (chunk: Buffer) => {
      const text = chunk.toString('utf-8')
      stderr += text
      notify('git.init.output', { data: text })
    })
    child.on('error', (err) => {
      if (settled) {
        return
      }
      settled = true
      reject(new Error(`Failed to start ${label}: ${err.message}`))
    })
    child.on('close', (code, signal) => {
      if (settled) {
        return
      }
      settled = true
      if (code === 0 && !signal) {
        resolve()
        return
      }
      reject(new Error(`${label} failed: ${stderr.trim() || `exit code ${code}`}`))
    })
  })
}

function readResolvedBranch(cwd: string): Promise<string> {
  return new Promise((resolve) => {
    const child = spawn('git', ['symbolic-ref', '--short', 'HEAD'], {
      cwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: buildRelayGitEnv()
    })
    let stdout = ''
    child.stdout.on('data', (chunk: Buffer) => {
      stdout += chunk.toString('utf-8')
    })
    child.on('error', () => resolve(''))
    child.on('close', () => resolve(stdout.trim()))
  })
}

export async function handleGitInit(
  id: string | number | null,
  params: Record<string, unknown>,
  notify: NotifyFn
): Promise<object> {
  const destPath = typeof params.destPath === 'string' ? params.destPath : ''
  const defaultBranch = typeof params.defaultBranch === 'string' ? params.defaultBranch : ''
  const remoteUrl = typeof params.remoteUrl === 'string' ? params.remoteUrl : ''
  const remoteName =
    typeof params.remoteName === 'string' && params.remoteName ? params.remoteName : 'origin'
  const span = initTracer.start({ method: 'git.init' })

  try {
    assertSafeInitParams(destPath, defaultBranch, remoteName, remoteUrl)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: msg } }
  }

  try {
    // Idempotent against an already-existing folder — mkdir on an existing
    // directory is a no-op, matching localgit.Executor.InitRepo's own
    // "works in place on an existing non-git folder" behavior.
    await mkdir(destPath, { recursive: true })

    const initArgs = defaultBranch ? ['init', '-b', defaultBranch] : ['init']
    await runGit(initArgs, destPath, notify, 'git init')

    if (remoteUrl) {
      await runGit(['remote', 'add', remoteName, remoteUrl], destPath, notify, 'git remote add')
    }

    const resolvedBranch = defaultBranch || (await readResolvedBranch(destPath))
    span.ok({ destPath, resolvedBranch, remoteAdded: Boolean(remoteUrl) })
    return {
      jsonrpc: '2.0',
      id,
      result: { path: destPath, defaultBranch: resolvedBranch, remoteAdded: Boolean(remoteUrl) }
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
