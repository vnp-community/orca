// src/relay/agent-capability-probes.ts
// Host-level probes behind the agent.capabilities report: version runner, claude login state, host info.
// Split out of agent-capability-report.ts to keep that file within the line budget.

import * as os from 'node:os'
import { execFile as _execFile } from 'node:child_process'
import type { statfs as StatFs } from 'node:fs/promises'
import { promisify } from 'node:util'

const execFileAsync = promisify(_execFile)

const TOOL_TIMEOUT_MS = 3_000

export type RunVersionCommand = (
  id: string,
  args: readonly string[],
  env: NodeJS.ProcessEnv
) => Promise<{ stdout: string } | null>

export function createVersionRunner(platform: NodeJS.Platform): RunVersionCommand {
  const exec = (file: string, args: readonly string[], env: NodeJS.ProcessEnv) =>
    execFileAsync(file, [...args], {
      env,
      timeout: TOOL_TIMEOUT_MS,
      windowsHide: true,
      maxBuffer: 1024 * 1024
    })
  return async (id, args, env) => {
    try {
      return { stdout: (await exec(id, args, env)).stdout }
    } catch (err: unknown) {
      if ((err as { code?: string }).code !== 'ENOENT') {
        return { stdout: '' } // installed but timed out or errored
      }
      if (platform === 'win32') {
        try {
          return { stdout: (await exec(`${id}.cmd`, args, env)).stdout }
        } catch {
          // fall through: not installed
        }
      }
      return null // not installed
    }
  }
}

/** Only the loggedIn boolean is read, never email or org. */
export async function readClaudeLogin(
  run: RunVersionCommand,
  env: NodeJS.ProcessEnv
): Promise<'logged_in' | 'logged_out' | 'unknown'> {
  try {
    const res = await run('claude', ['auth', 'status', '--json'], env)
    if (!res) {
      return 'unknown'
    }
    const parsed = JSON.parse(res.stdout.trim()) as Record<string, unknown>
    if (typeof parsed['loggedIn'] === 'boolean') {
      return parsed['loggedIn'] ? 'logged_in' : 'logged_out'
    }
  } catch {
    // an unparsable or failed probe is "unknown", never a claim
  }
  return 'unknown'
}

export type HostInfo = {
  platform: string
  arch: string
  nodeVersion: string
  cpuCount: number
  memTotalMb: number
  memFreeMb: number
  diskFreeMb?: number
  loadAvg1: number
}

export async function buildHostInfo(
  workDir: string,
  platform: NodeJS.Platform,
  statfsDep?: typeof StatFs
): Promise<HostInfo> {
  let diskFreeMb: number | undefined
  try {
    const statfs = statfsDep ?? (await import('node:fs/promises')).statfs
    const s = await statfs(workDir)
    diskFreeMb = Math.floor((s.bavail * s.bsize) / (1024 * 1024))
  } catch {
    // omit diskFreeMb on error
  }
  return {
    platform,
    arch: os.arch(),
    nodeVersion: process.version,
    cpuCount: os.cpus().length,
    memTotalMb: Math.floor(os.totalmem() / (1024 * 1024)),
    memFreeMb: Math.floor(os.freemem() / (1024 * 1024)),
    ...(diskFreeMb !== undefined ? { diskFreeMb } : {}),
    loadAvg1: os.loadavg()[0] ?? 0
  }
}
