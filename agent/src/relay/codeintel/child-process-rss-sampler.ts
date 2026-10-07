import fs from 'fs'
import { execFile } from 'child_process'
import { promisify } from 'util'

const execFileAsync = promisify(execFile)

export function parseLinuxProcStatus(text: string): number | null {
  const hwmMatch = text.match(/VmHWM:\s*(\d+)\s*kB/i)
  if (hwmMatch) {
    return parseInt(hwmMatch[1], 10)
  }
  const rssMatch = text.match(/VmRSS:\s*(\d+)\s*kB/i)
  if (rssMatch) {
    return parseInt(rssMatch[1], 10)
  }
  return null
}

export function parseMacOsPsOutput(text: string): number | null {
  const trimmed = text.trim()
  if (!trimmed) return null
  const val = parseInt(trimmed, 10)
  return isNaN(val) ? null : val
}

export interface RssSamplerOptions {
  pid?: number
  intervalMs?: number
  platform?: NodeJS.Platform
  reader?: (pid: number) => Promise<number | null>
}

export interface RssSampler {
  stop: () => Promise<number | undefined>
}

export function createRssSampler(opts: RssSamplerOptions): RssSampler {
  const platform = opts.platform ?? process.platform
  const pid = opts.pid

  if (platform === 'win32' || !pid) {
    return {
      stop: async () => undefined
    }
  }

  let intervalMs = opts.intervalMs ?? 100
  if (platform === 'darwin') {
    intervalMs = Math.max(200, intervalMs)
  }

  const defaultReader = async (p: number): Promise<number | null> => {
    if (platform === 'linux') {
      try {
        const text = await fs.promises.readFile(`/proc/${p}/status`, 'utf8')
        return parseLinuxProcStatus(text)
      } catch {
        return null
      }
    } else if (platform === 'darwin') {
      try {
        const { stdout } = await execFileAsync('ps', ['-o', 'rss=', '-p', String(p)], { timeout: 2000 })
        return parseMacOsPsOutput(stdout)
      } catch {
        return null
      }
    }
    return null
  }

  const read = opts.reader ?? defaultReader

  let peakRssKb: number | null = null
  let stopped = false

  const sample = async () => {
    if (stopped) return
    try {
      const val = await read(pid)
      if (val !== null && val > 0) {
        if (peakRssKb === null || val > peakRssKb) {
          peakRssKb = val
        }
      }
    } catch {}
  }

  // Initial read
  sample().catch(() => {})

  const timer = setInterval(() => {
    sample().catch(() => {})
  }, intervalMs)

  return {
    stop: async () => {
      stopped = true
      clearInterval(timer)
      await sample().catch(() => {})
      return peakRssKb !== null ? peakRssKb : undefined
    }
  }
}
