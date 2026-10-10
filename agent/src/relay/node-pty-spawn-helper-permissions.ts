import { chmodSync, statSync } from 'node:fs'
import { dirname, join } from 'node:path'

type EnsureOptions = {
  platform?: NodeJS.Platform
  arch?: string
  /** Directory of the installed node-pty package; resolved from node_modules when omitted. */
  nodePtyDir?: string | null
}

function resolveInstalledNodePtyDir(): string | null {
  try {
    return dirname(require.resolve('node-pty/package.json'))
  } catch {
    return null
  }
}

// Why: node-pty 1.x spawns every PTY through a `spawn-helper` binary; some installers (pnpm store,
// tar extraction without modes) drop its exec bit and every pty.create then fails with
// "posix_spawnp failed". Repairing at first use keeps terminals working across reinstalls.
export function ensureNodePtySpawnHelperExecutable(options: EnsureOptions = {}): string[] {
  const platform = options.platform ?? process.platform
  if (platform === 'win32') {
    return []
  }
  const nodePtyDir =
    options.nodePtyDir === undefined ? resolveInstalledNodePtyDir() : options.nodePtyDir
  if (!nodePtyDir) {
    return []
  }
  const arch = options.arch ?? process.arch
  const candidates = [
    join(nodePtyDir, 'build', 'Release', 'spawn-helper'),
    join(nodePtyDir, 'build', 'Debug', 'spawn-helper'),
    join(nodePtyDir, 'prebuilds', `${platform}-${arch}`, 'spawn-helper')
  ]
  const repaired: string[] = []
  for (const helperPath of candidates) {
    try {
      const mode = statSync(helperPath).mode
      if ((mode & 0o111) === 0o111) {
        continue
      }
      chmodSync(helperPath, mode | 0o755)
      repaired.push(helperPath)
    } catch {
      // Missing helper (other build flavor) or read-only install: node-pty reports the real error.
    }
  }
  return repaired
}
