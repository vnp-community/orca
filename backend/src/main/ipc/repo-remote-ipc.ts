// ─── repo-remote-ipc.ts ───────────────────────────────────────────────────────
// IPC handlers for remote repository operations (Phase 2: SOL-004-005-006 §C.3).
// Provides: repo.listRemoteDirectory, repo.addRemote, repo.cloneRemote, repo.scanRemote
// Registered in server-bootstrap.ts after DevServerManager is initialized.

import { ipcMain } from 'electron'
import { basename } from 'node:path'
import { randomUUID } from 'node:crypto'
import type { DevServerManager } from '../dev-server/dev-server-manager'
import type { Store } from '../persistence'
import type { Repo } from '../../shared/types'

// Why local (not imported): this used to come from the dead vendored
// backend/src/relay/fs-handler-directory-browse.ts (deleted — see
// specs/agent/api/gaps-and-findings.md #9), whose only live reference
// anywhere in backend/src/main was this type, used purely as a return-type
// annotation for the agent's 'fs.listDirectory' RPC result shape.
type DirectoryEntry = {
  name: string
  path: string
  isDirectory: boolean
  /** true if the directory contains a `.git` subfolder */
  isGitRepo: boolean
}

// ── Channel names ──────────────────────────────────────────────────────────────

const REPO_REMOTE_IPC_CHANNELS = [
  'repo.listRemoteDirectory',
  'repo.addRemote',
  'repo.cloneRemote',
  'repo.scanRemote'
] as const

// ── Registration ──────────────────────────────────────────────────────────────

export function registerRepoRemoteIpcHandlers(
  devServerManager: DevServerManager,
  store: Store
): void {
  // Idempotent: remove stale handlers before re-registering.
  for (const channel of REPO_REMOTE_IPC_CHANNELS) {
    ipcMain.removeHandler(channel)
  }

  // ── repo.listRemoteDirectory ─────────────────────────────────────────────────
  // Forward a directory listing request to the remote relay's fs.listDirectory.

  ipcMain.handle(
    'repo.listRemoteDirectory',
    async (
      _event,
      params: { devServerId: string; path: string; includeGitStatus?: boolean }
    ): Promise<{ entries: DirectoryEntry[]; platform: NodeJS.Platform }> => {
      const relay = devServerManager.getRelay(params.devServerId)
      if (!relay) {throw new Error(`Dev server '${params.devServerId}' not connected`)}
      return relay.call<{ entries: DirectoryEntry[]; platform: NodeJS.Platform }>(
        'fs.listDirectory',
        { path: params.path, includeGitStatus: params.includeGitStatus ?? false }
      )
    }
  )

  // ── repo.addRemote ────────────────────────────────────────────────────────────
  // Register an existing remote directory as a repo in the local store.
  // Why: we persist the devServerId so the runtime can route SSH commands to
  // the correct relay without re-resolving the connection on every command.

  ipcMain.handle(
    'repo.addRemote',
    async (
      _event,
      params: { devServerId: string; path: string; name?: string }
    ): Promise<Repo> => {
      const relay = devServerManager.getRelay(params.devServerId)
      if (!relay) {throw new Error(`Dev server '${params.devServerId}' not connected`)}

      // Validate the path exists on the remote before persisting.
      const statResult = await relay.call<{ exists: boolean; isDirectory?: boolean }>(
        'fs.stat',
        { path: params.path }
      )
      if (!statResult.exists) {
        throw new Error(`Path does not exist on dev server: ${params.path}`)
      }

      const devServer = devServerManager.get(params.devServerId)!
      const repo: Repo = {
        id: randomUUID(),
        path: params.path,
        displayName: params.name ?? basename(params.path),
        badgeColor: '#6366f1',
        addedAt: Date.now(),
        kind: 'git',
        // Link the repo to the dev server and its SSH target.
        connectionId: devServer.sshTargetId ?? null,
        devServerId: params.devServerId
      }
      store.addRepo(repo)
      return repo
    }
  )

  // ── repo.cloneRemote ──────────────────────────────────────────────────────────
  // Clone a git repo on the remote dev server and add it to the local store.
  // Why: running git on the relay avoids tunnelling git traffic through the host;
  // the relay has direct internet access on the dev server.

  ipcMain.handle(
    'repo.cloneRemote',
    async (
      _event,
      params: { devServerId: string; url: string; targetDir?: string }
    ): Promise<{ repoId: string; path: string }> => {
      const relay = devServerManager.getRelay(params.devServerId)
      if (!relay) {throw new Error(`Dev server '${params.devServerId}' not connected`)}

      const devServer = devServerManager.get(params.devServerId)!
      const workspaceDir = devServer.workspaceDir ?? '~/orca/workspaces'
      // Derive repo name from the last path component, strip .git suffix.
      const repoName = params.url.split('/').pop()?.replace(/\.git$/, '') ?? 'repo'
      const targetPath = params.targetDir ?? `${workspaceDir}/${repoName}`

      // Delegate the actual clone to the relay's git.clone handler.
      await relay.call<{ path: string }>('git.clone', {
        url: params.url,
        targetPath
      })

      // Persist the newly cloned repo in the local store.
      const repo: Repo = {
        id: randomUUID(),
        path: targetPath,
        displayName: repoName,
        badgeColor: '#6366f1',
        addedAt: Date.now(),
        kind: 'git',
        connectionId: devServer.sshTargetId ?? null,
        devServerId: params.devServerId
      }
      store.addRepo(repo)
      return { repoId: repo.id, path: targetPath }
    }
  )

  // ── repo.scanRemote ───────────────────────────────────────────────────────────
  // Scan a root directory on the dev server and return only git repositories.
  // Why: lets the onboarding wizard present the user's existing remote projects
  // without needing them to type paths manually.

  ipcMain.handle(
    'repo.scanRemote',
    async (
      _event,
      params: { devServerId: string; rootPath: string }
    ): Promise<{ path: string; name: string }[]> => {
      const relay = devServerManager.getRelay(params.devServerId)
      if (!relay) {throw new Error(`Dev server '${params.devServerId}' not connected`)}

      const { entries } = await relay.call<{ entries: DirectoryEntry[]; platform: NodeJS.Platform }>(
        'fs.listDirectory',
        { path: params.rootPath, includeGitStatus: true }
      )

      return entries
        .filter((e) => e.isGitRepo)
        .map((e) => ({ path: e.path, name: basename(e.path) }))
    }
  )
}
