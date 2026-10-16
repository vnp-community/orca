/**
 * pty-agent-cwd.ts — which directory a new agent-mode PTY opens in.
 * Split out of pty-agent-bridge.ts so each file stays under the max-lines budget.
 */

import { statSync } from 'node:fs'
import { homedir } from 'node:os'
import { resolve } from 'node:path'

/** The directory to open, or null when `raw` is unusable (empty, NUL byte, missing, not a directory). */
export function usableCwd(raw: string): string | null {
  if (!raw || raw.includes('\0')) {
    return null
  }
  const resolved = resolve(raw)
  try {
    const stat = statSync(resolved)
    return stat.isDirectory() ? resolved : null
  } catch {
    return null
  }
}

export function safeCwd(raw: string): string {
  return usableCwd(raw) ?? homedir()
}
