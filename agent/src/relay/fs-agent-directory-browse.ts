// src/relay/fs-agent-directory-browse.ts
// Part A (direct-websocket/relay-websocket) implementation of
// `fs.listDirectory`.
//
// Why this file exists: previously only existed on Part B
// (fs-handler-directory-browse.ts's FsDirectoryBrowserHandler, used by
// relay.ts/relay-ssh) — backend/src/main/ipc/repo-remote-ipc.ts's
// repo.listRemoteDirectory/repo.scanRemote reach the agent via a raw
// relay.call() dispatched against whichever connection type the Dev Server
// actually uses, so Part A's absence broke this for direct-websocket/
// relay-websocket (the default mode). See
// specs/agent/api/gaps-and-findings.md #5 /
// specs/agent/api/compliance-audit-2026-08-15.md.
//
// Identical logic to fs-handler-directory-browse.ts (Part B) — that file
// has no RelayDispatcher-specific state beyond its constructor's
// registration call, so this is a near-verbatim port, not a redesign.

import { readdir, stat } from 'node:fs/promises'
import { join } from 'node:path'
import { AgentErrorCode } from '../shared/agent-wire-protocol'

export type DirectoryEntry = {
  name: string
  path: string
  isDirectory: boolean
  /** true if the directory contains a `.git` subfolder */
  isGitRepo: boolean
}

async function isGitRepo(dirPath: string): Promise<boolean> {
  try {
    await stat(join(dirPath, '.git'))
    return true
  } catch {
    return false
  }
}

export async function handleFsListDirectory(
  id: string | number | null,
  params: Record<string, unknown>
): Promise<object> {
  const dirPath = typeof params.path === 'string' ? params.path : ''
  const includeGitStatus = params.includeGitStatus === true

  if (!dirPath) {
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
  }

  let entries: DirectoryEntry[]
  try {
    const items = await readdir(dirPath, { withFileTypes: true })
    entries = await Promise.all(
      items
        .filter((item) => item.isDirectory())
        .map(async (item) => {
          const fullPath = join(dirPath, item.name)
          return {
            name: item.name,
            path: fullPath,
            isDirectory: true,
            isGitRepo: includeGitStatus ? await isGitRepo(fullPath) : false
          }
        })
    )
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0', id,
      error: { code: AgentErrorCode.ServerError, message: `Cannot list directory ${dirPath}: ${msg}` }
    }
  }

  return { jsonrpc: '2.0', id, result: { entries, platform: process.platform } }
}
