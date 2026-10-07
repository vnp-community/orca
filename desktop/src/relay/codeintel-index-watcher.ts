import fs from 'fs'
import path from 'path'
import { emitCodeIntelNotification } from './codeintel-notification-sink'
import { probeIndexBasis } from './codeintel-index-basis-probe'
import { classifyIndexBasis } from './codeintel-index-basis'
import { getHeadCommit } from './codeintel-head-commit'

export class CodeIntelIndexWatcher {
  private workspaceRoot: string
  private metaJsonPath: string
  private headPath: string
  private watchers: fs.FSWatcher[] = []
  private debounceTimer: NodeJS.Timeout | null = null

  constructor(workspaceRoot: string, gitNexusRegistryPath: string) {
    this.workspaceRoot = workspaceRoot
    this.metaJsonPath = path.join(gitNexusRegistryPath, 'meta.json')
    this.headPath = path.join(workspaceRoot, '.git', 'HEAD')
  }

  start() {
    this.watchFile(this.metaJsonPath, 'meta.json', 'gitnexus')
    this.watchFile(this.headPath, 'head', 'git')
  }

  stop() {
    for (const w of this.watchers) w.close()
    this.watchers = []
    if (this.debounceTimer) clearTimeout(this.debounceTimer)
  }

  private watchFile(filePath: string, reason: string, tool: string) {
    if (!fs.existsSync(filePath)) return
    try {
      const w = fs.watch(filePath, (eventType) => {
        if (eventType === 'change') this.scheduleNotify(reason, tool)
      })
      this.watchers.push(w)
    } catch {
      // Ignore watch setup error to keep stdio clean
    }
  }

  private scheduleNotify(reason: string, tool: string) {
    if (this.debounceTimer) clearTimeout(this.debounceTimer)
    this.debounceTimer = setTimeout(() => {
      this.notify(reason, tool)
    }, 1000)
  }

  private async notify(reason: string, tool: string) {
    let indexScope = 'unknown'
    let mergeBase = null

    try {
      const probeRes = await probeIndexBasis(this.workspaceRoot, { baseRef: 'origin/HEAD' })
      const basis = classifyIndexBasis({
        tool: 'gitnexus',
        toolUsable: true,
        indexExists: true,
        rootMatches: true,
        indexedCommit: probeRes.headCommit,
        indexedAtMs: Date.now(),
        headCommit: probeRes.headCommit,
        headCommitTimeMs: probeRes.headCommitTimeMs,
        mergeBase: probeRes.mergeBase,
        mergeBaseCommitTimeMs: probeRes.mergeBaseCommitTimeMs,
        dirtySinceIndex: false,
        pendingChanges: null
      })
      indexScope = basis.indexScope
      mergeBase = probeRes.mergeBase
    } catch {
      // Ignore probe error to keep stdio clean
    }

    const payload: any = { reason, tool, workspaceRoot: this.workspaceRoot, indexScope }
    if (mergeBase) payload.mergeBase = mergeBase
    
    emitCodeIntelNotification('codeintel.indexChanged', payload)
  }
}

const activeWatchers = new Map<string, CodeIntelIndexWatcher>()

import { readRuntimeSwitches } from './codeintel/runtime-switches'

export function enableWatch(binding: any): void {
  if (readRuntimeSwitches().codeintelDisabled) return
  const root = binding.toplevel || binding.workspaceRoot
  if (activeWatchers.has(root)) return
  const watcher = new CodeIntelIndexWatcher(root, binding.gitNexusRegistryPath || root)
  watcher.start()
  activeWatchers.set(root, watcher)
}

export function disableWatch(workspaceRoot: string): void {
  const watcher = activeWatchers.get(workspaceRoot)
  if (watcher) {
    watcher.stop()
    activeWatchers.delete(workspaceRoot)
  }
}

export function cleanupCodeIntelWatchers(): void {
  for (const watcher of activeWatchers.values()) {
    watcher.stop()
  }
  activeWatchers.clear()
}

export function getWatchingRoots(): string[] {
  return Array.from(activeWatchers.keys())
}

