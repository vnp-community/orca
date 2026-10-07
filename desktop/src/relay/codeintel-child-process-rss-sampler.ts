import fs from 'fs/promises'
import { execFile } from 'child_process'
import util from 'util'

const execFileAsync = util.promisify(execFile)

export interface RssSamplerOptions {
  pid: number
  intervalMs?: number
  platform?: NodeJS.Platform
  readProcStatus?: (pid: number) => Promise<string>
  runPs?: (pid: number) => Promise<string>
}

export interface RssSampler {
  stop: () => Promise<number | undefined>
}

export function parseLinuxProcStatus(content: string): number | undefined {
  const hwmMatch = content.match(/^VmHWM:\s+(\d+)\s+kB/m)
  if (hwmMatch) {
    return parseInt(hwmMatch[1], 10)
  }
  const rssMatch = content.match(/^VmRSS:\s+(\d+)\s+kB/m)
  if (rssMatch) {
    return parseInt(rssMatch[1], 10)
  }
  return undefined
}

export function parseMacPsOutput(content: string): number | undefined {
  const trimmed = content.trim()
  if (!trimmed) return undefined
  const parsed = parseInt(trimmed, 10)
  return isNaN(parsed) ? undefined : parsed
}

export function createRssSampler(opts: RssSamplerOptions): RssSampler {
  const platform = opts.platform ?? process.platform
  const intervalMs = opts.intervalMs ?? (platform === 'darwin' ? 200 : 100)
  
  if (platform !== 'linux' && platform !== 'darwin') {
    return {
      stop: async () => undefined
    }
  }

  const readProcStatus = opts.readProcStatus ?? (async (pid: number) => {
    return await fs.readFile(`/proc/${pid}/status`, 'utf-8')
  })

  const runPs = opts.runPs ?? (async (pid: number) => {
    const { stdout } = await execFileAsync('ps', ['-o', 'rss=', '-p', String(pid)])
    return stdout
  })

  let peakRss: number | undefined
  let running = true
  let timer: NodeJS.Timeout | undefined

  const takeSample = async () => {
    try {
      if (platform === 'linux') {
        const content = await readProcStatus(opts.pid)
        const rss = parseLinuxProcStatus(content)
        if (rss !== undefined) {
          peakRss = Math.max(peakRss ?? 0, rss)
        }
      } else if (platform === 'darwin') {
        const content = await runPs(opts.pid)
        const rss = parseMacPsOutput(content)
        if (rss !== undefined) {
          peakRss = Math.max(peakRss ?? 0, rss)
        }
      }
    } catch (e) {
      // Swallow errors (e.g. process exited)
    }
  }

  const loop = async () => {
    if (!running) return
    await takeSample()
    if (running) {
      timer = setTimeout(loop, intervalMs)
    }
  }

  // Start sampling loop
  timer = setTimeout(loop, intervalMs)

  return {
    stop: async () => {
      running = false
      if (timer) {
        clearTimeout(timer)
      }
      // Take one final sample just in case
      await takeSample()
      return peakRss
    }
  }
}
