/**
 * JsonFileUsageStatePersistence — the original `readFileSync`/`writeFileSync`
 * behavior of `ClaudeUsageStore`/`CodexUsageStore`, extracted verbatim behind
 * `UsageStatePersistence<TState>` (ADR-021). Electron desktop mode keeps
 * using this — see usage-state-persistence.ts's module doc comment.
 *
 * @module main/usage/json-file-usage-state-persistence
 */

import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs'
import { dirname } from 'node:path'
import type { UsageStatePersistence } from './usage-state-persistence'

export class JsonFileUsageStatePersistence<TState> implements UsageStatePersistence<TState> {
  constructor(private readonly filePath: () => string) {}

  async load(): Promise<TState | null> {
    const usageFile = this.filePath()
    if (!existsSync(usageFile)) {return null}
    return JSON.parse(readFileSync(usageFile, 'utf-8')) as TState
  }

  async save(state: TState): Promise<void> {
    const usageFile = this.filePath()
    const dir = dirname(usageFile)
    if (!existsSync(dir)) {mkdirSync(dir, { recursive: true })}
    // Why: scans can refresh while the app is in active use. Atomic temp-file
    // + rename so a crash or concurrent write cannot leave a truncated
    // analytics file as the common failure mode — same pattern as the main
    // Store (persistence.ts) and the original inline writeToDisk() this replaces.
    const tmpFile = `${usageFile}.${process.pid}.${Date.now()}.${Math.random().toString(16).slice(2)}.tmp`
    writeFileSync(tmpFile, JSON.stringify(state, null, 2), 'utf-8')
    renameSync(tmpFile, usageFile)
  }
}
