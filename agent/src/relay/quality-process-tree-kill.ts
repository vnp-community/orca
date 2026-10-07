import { execFile } from 'child_process'
import { promisify } from 'util'

const execFileAsync = promisify(execFile)

export interface KillProcessTreeOptions {
  termGraceMs?: number
  verifyTimeoutMs?: number
  platform?: NodeJS.Platform
}

export async function killProcessTree(
  pid: number,
  opts: KillProcessTreeOptions = {}
): Promise<{ gone: boolean; orphanSuspected: boolean }> {
  if (pid <= 1) {
    throw new Error('Refusing to kill pid <= 1')
  }

  const platform = opts.platform || process.platform
  const termGraceMs = opts.termGraceMs ?? 5000
  const verifyTimeoutMs = opts.verifyTimeoutMs ?? 10000

  if (platform === 'win32') {
    try {
      await execFileAsync('taskkill', ['/pid', String(pid), '/T', '/F'], { windowsHide: true })
      return { gone: true, orphanSuspected: false }
    } catch (err: any) {
      if (err.message && err.message.includes('not found')) {
        return { gone: true, orphanSuspected: false }
      }
      console.error(`taskkill failed for pid ${pid}`, err)
      return { gone: false, orphanSuspected: true }
    }
  }

  // POSIX
  const checkAlive = () => {
    try {
      process.kill(-pid, 0)
      return true
    } catch (err: any) {
      if (err.code === 'ESRCH') return false
      throw err // EPERM or others
    }
  }

  if (!checkAlive()) {
    return { gone: true, orphanSuspected: false }
  }

  // Send SIGTERM
  try {
    process.kill(-pid, 'SIGTERM')
  } catch (err: any) {
    if (err.code === 'ESRCH') return { gone: true, orphanSuspected: false }
  }

  // Wait grace period
  let waited = 0
  while (waited < termGraceMs) {
    await new Promise(r => setTimeout(r, 100))
    if (!checkAlive()) return { gone: true, orphanSuspected: false }
    waited += 100
  }

  // Send SIGKILL
  try {
    process.kill(-pid, 'SIGKILL')
  } catch (err: any) {
    if (err.code === 'ESRCH') return { gone: true, orphanSuspected: false }
  }

  // Wait verify period
  waited = 0
  while (waited < verifyTimeoutMs) {
    await new Promise(r => setTimeout(r, 100))
    if (!checkAlive()) return { gone: true, orphanSuspected: false }
    waited += 100
  }

  console.warn(`killProcessTree: pid ${pid} still alive after SIGKILL + timeout`)
  return { gone: false, orphanSuspected: true }
}
