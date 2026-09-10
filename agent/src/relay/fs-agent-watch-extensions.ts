// src/relay/fs-agent-watch-extensions.ts
// Watch RPC handlers for Orca Dev Agent v5.0: fs.watch, fs.unwatch, cleanupAgentWatches.
// Split out of fs-agent-extensions.ts to stay under the repo's 300-line max-lines ratchet.

import { readdir, stat } from 'node:fs/promises'
import { watch as fsWatchSync, type FSWatcher, type Dirent } from 'node:fs'
import { join, isAbsolute, relative } from 'node:path'
import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
// Why: same high-churn exclusion list @parcel/watcher subscriptions use
// (main/ipc cluster) — reused here as a plain directory-name filter so the
// Linux per-directory polyfill doesn't crawl node_modules/.git/dist/etc.
import { WATCHER_IGNORE_DIRS } from '../main/ipc/filesystem-watcher-ignore'

// ─── fs.watch / fs.unwatch ──────────────────────────────────────────────────
// Pushes `fs.changed` notifications for a watched path. Refcounted per path:
// multiple Orca-side callers (different user processes sharing this Dev
// Server) can watch the same path without one's unwatch tearing it down for
// the others.

type AgentWatchEntry = {
  close: () => void
  refCount: number
}

const AGENT_WATCH_MAP = new Map<string, AgentWatchEntry>()

const MAX_LINUX_WATCH_DIRS = 4000

/**
 * Recursively fs.watch() every subdirectory under rootAbsPath, skipping
 * WATCHER_IGNORE_DIRS entries. Node's fs.watch(recursive:false) only reports
 * events for directories it was given directly — a freshly created
 * subdirectory needs its own watcher, so handleLinuxWatchEvent extends the
 * set dynamically instead of walking once at setup time.
 */
async function watchDirLinux(
  rootAbsPath: string,
  dirAbsPath: string,
  watchers: Map<string, FSWatcher>,
  notify: (method: string, params: Record<string, unknown>) => void
): Promise<void> {
  if (watchers.has(dirAbsPath) || watchers.size >= MAX_LINUX_WATCH_DIRS) {
    return
  }
  let watcher: FSWatcher
  try {
    watcher = fsWatchSync(dirAbsPath, { recursive: false }, (eventType, filename) => {
      handleLinuxWatchEvent(rootAbsPath, dirAbsPath, eventType, filename, watchers, notify)
    })
  } catch {
    return // dir vanished between readdir and watch — not fatal, skip it
  }
  watcher.on('error', () => {
    watchers.delete(dirAbsPath)
  })
  watchers.set(dirAbsPath, watcher)

  let entries: Dirent[]
  try {
    entries = await readdir(dirAbsPath, { withFileTypes: true })
  } catch {
    return
  }
  for (const entry of entries) {
    if (!entry.isDirectory() || WATCHER_IGNORE_DIRS.includes(entry.name)) {
      continue
    }
    await watchDirLinux(rootAbsPath, join(dirAbsPath, entry.name), watchers, notify)
  }
}

function handleLinuxWatchEvent(
  rootAbsPath: string,
  dirAbsPath: string,
  eventType: string,
  filename: string | null,
  watchers: Map<string, FSWatcher>,
  notify: (method: string, params: Record<string, unknown>) => void
): void {
  const changedAbsPath = filename ? join(dirAbsPath, filename) : dirAbsPath
  const relFromRoot = relative(rootAbsPath, changedAbsPath) || '.'
  // Why: keep the wire shape identical to the macOS/Windows native-recursive
  // branch — filename is root-relative there too, so the client's fs.changed
  // handler doesn't need a platform switch.
  notify('fs.changed', { path: rootAbsPath, eventType, filename: relFromRoot })

  stat(changedAbsPath).then(
    (st) => {
      if (st.isDirectory() && !WATCHER_IGNORE_DIRS.includes(filename ?? '')) {
        void watchDirLinux(rootAbsPath, changedAbsPath, watchers, notify)
      }
    },
    () => {
      // ENOENT: removed. If changedAbsPath was itself a watched directory,
      // drop its watcher so a later re-create under the same name re-walks
      // cleanly instead of reusing a dead descriptor.
      const removed = watchers.get(changedAbsPath)
      if (removed) {
        removed.close()
        watchers.delete(changedAbsPath)
      }
    }
  )
}

export async function handleFsWatch(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  notify: (method: string, params: Record<string, unknown>) => void
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  if (!rawPath) {
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.InvalidParams, message: 'Missing required param: path' }
    }
  }
  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  const existing = AGENT_WATCH_MAP.get(absPath)
  if (existing) {
    existing.refCount++
    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
  }

  try {
    if (process.platform === 'linux') {
      // Why: Node's recursive:true is silently ignored on Linux (Node docs).
      // Polyfill by fs.watch-ing every subdirectory individually — see
      // watchDirLinux/handleLinuxWatchEvent. Zero new deps, stays inside the
      // existing single-file esbuild bundle (agent/build.mjs has no pipeline
      // for a second child-process entry, which the @parcel/watcher cluster
      // used by relay.ts's SSH-relay branch would require).
      const watchers = new Map<string, FSWatcher>()
      await watchDirLinux(absPath, absPath, watchers, notify)
      AGENT_WATCH_MAP.set(absPath, {
        close: () => {
          for (const w of watchers.values()) {
            w.close()
          }
        },
        refCount: 1
      })
      return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
    }

    const watcher = fsWatchSync(absPath, { recursive: true }, (eventType, filename) => {
      notify('fs.changed', { path: absPath, eventType, filename: filename ?? null })
    })
    watcher.on('error', (err: Error) => {
      notify('fs.changed', {
        path: absPath,
        eventType: 'error',
        filename: null,
        error: err.message
      })
      AGENT_WATCH_MAP.delete(absPath)
    })
    AGENT_WATCH_MAP.set(absPath, { close: () => watcher.close(), refCount: 1 })
    return { jsonrpc: '2.0', id, result: { ok: true, path: absPath } }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    return {
      jsonrpc: '2.0',
      id,
      error: { code: AgentErrorCode.ServerError, message: `fs.watch failed: ${msg}` }
    }
  }
}

export async function handleFsUnwatch(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const rawPath = typeof params.path === 'string' ? params.path : ''
  const absPath = isAbsolute(rawPath) ? rawPath : join(config.workDir, rawPath)

  const entry = AGENT_WATCH_MAP.get(absPath)
  if (entry) {
    entry.refCount--
    if (entry.refCount <= 0) {
      entry.close()
      AGENT_WATCH_MAP.delete(absPath)
    }
  }
  return { jsonrpc: '2.0', id, result: { ok: true } }
}

/**
 * cleanupAgentWatches — Close all active fs watchers.
 * Must be called on session termination (agent-session.ts stop()), mirroring
 * cleanupAgentPtys — otherwise watchers outlive a dropped WebSocket.
 */
export function cleanupAgentWatches(): void {
  for (const [path, entry] of AGENT_WATCH_MAP.entries()) {
    try {
      entry.close()
    } catch {
      // best effort
    }
    AGENT_WATCH_MAP.delete(path)
  }
}
