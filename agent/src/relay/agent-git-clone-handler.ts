// src/relay/agent-git-clone-handler.ts
// Part A (direct-websocket/relay-websocket) implementation of `git.clone`.
//
// Why this file exists: `git.clone` previously only existed on Part B
// (git-handler.ts's GitHandler, used by relay.ts/relay-ssh) — the
// {url,targetPath} shape backend/src/main/ipc/repo-remote-ipc.ts's
// repo.cloneRemote sends. Since that caller reaches the agent via a raw
// relay.call() dispatched against whichever connection type the Dev Server
// actually uses, Part A's absence meant a direct-websocket/relay-websocket
// (the default mode) Dev Server threw MethodNotFound for this flow. See
// specs/agent/api/gaps-and-findings.md #5 /
// specs/agent/api/compliance-audit-2026-08-15.md.
//
// Mirrors GitHandler.cloneSimple()/spawnCloneSimple() (git-handler.ts, Part
// B) — same argv-injection guard (reject a leading '-' or embedded NUL in
// either url or targetPath, since these are typed as the literal argv
// elements passed to spawn(), not a shell string, but a '-'-prefixed value
// would still be parsed as a git flag) and the same pinned-locale env via
// buildRelayGitEnv(). Not calling into git-handler.ts directly — that
// class is tightly coupled to RelayDispatcher/RequestContext (Part B's
// types); this is a small enough amount of logic that duplicating it here
// (rather than threading Part B's dispatcher types into Part A) is the
// simpler, more isolated fix.

import { spawn } from 'node:child_process'
import { buildRelayGitEnv } from './relay-command-env'
import { getGitCloneFailureMessage } from '../shared/git-clone-failure-message'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'

const cloneTracer = createTracer('agent:git-clone')

type NotifyFn = (method: string, params: Record<string, unknown>) => void

function assertSafeCloneParams(url: string, targetPath: string): void {
  if (!url || url.startsWith('-') || url.includes('\0')) {
    throw new Error('git.clone: invalid url')
  }
  if (!targetPath || targetPath.startsWith('-') || targetPath.includes('\0')) {
    throw new Error('git.clone: invalid targetPath')
  }
}

function spawnClone(
  url: string,
  targetPath: string,
  notify: NotifyFn
): Promise<{ path: string }> {
  return new Promise((resolve, reject) => {
    const child = spawn('git', ['clone', '--progress', '--', url, targetPath], {
      stdio: ['ignore', 'pipe', 'pipe'],
      env: buildRelayGitEnv()
    })
    let stderr = ''
    let settled = false

    // Not typically used by `git clone`, but streamed for parity with the
    // repo.cloneRemote-facing handler this replaces.
    child.stdout.on('data', (chunk: Buffer) => {
      notify('git.clone.output', { data: chunk.toString('utf-8') })
    })
    child.stderr.on('data', (chunk: Buffer) => {
      const text = chunk.toString('utf-8')
      stderr += text
      notify('git.clone.output', { data: text })
    })
    child.on('error', (err) => {
      if (settled) {return}
      settled = true
      reject(new Error(`Failed to start git clone: ${err.message}`))
    })
    child.on('close', (code, signal) => {
      if (settled) {return}
      settled = true
      if (code === 0 && !signal) {
        resolve({ path: targetPath })
        return
      }
      reject(new Error(`Git clone failed: ${getGitCloneFailureMessage(stderr)}`))
    })
  })
}

export async function handleGitClone(
  id: string | number | null,
  params: Record<string, unknown>,
  notify: NotifyFn
): Promise<object> {
  const url = typeof params.url === 'string' ? params.url : ''
  const targetPath = typeof params.targetPath === 'string' ? params.targetPath : ''
  const span = cloneTracer.start({ method: 'git.clone' })

  try {
    assertSafeCloneParams(url, targetPath)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: msg } }
  }

  try {
    const result = await spawnClone(url, targetPath, notify)
    span.ok({ targetPath })
    return { jsonrpc: '2.0', id, result }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.ServerError, message: msg } }
  }
}
