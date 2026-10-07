import { describe, it, expect, vi } from 'vitest'
import { killProcessTree } from './quality-process-tree-kill'
import * as cp from 'child_process'
import fs from 'fs'
import path from 'path'
import os from 'os'

vi.mock('child_process', async (importOriginal) => {
  const actual = await importOriginal<any>()
  return { ...actual, execFile: vi.fn() }
})

describe('quality-process-tree-kill', () => {
  it('refuses pid <= 1', async () => {
    await expect(killProcessTree(1)).rejects.toThrow(/Refusing to kill pid <= 1/)
    await expect(killProcessTree(-100)).rejects.toThrow(/Refusing to kill pid <= 1/)
  })

  it('handles ESRCH at start on POSIX', async () => {
    // A random non-existent pid
    const res = await killProcessTree(99999, { platform: 'linux' })
    expect(res.gone).toBe(true)
  })

  it('calls taskkill on win32', async () => {
    vi.mocked(cp.execFile).mockImplementation((cmd, args, opts, cb) => {
      ;(cb as any)(null, { stdout: '', stderr: '' })
      return {} as any
    })
    
    const res = await killProcessTree(12345, { platform: 'win32' })
    expect(res.gone).toBe(true)
    expect(cp.execFile).toHaveBeenCalledWith('taskkill', ['/pid', '12345', '/T', '/F'], { windowsHide: true }, expect.any(Function))
  })

  it('returns orphanSuspected on taskkill failure', async () => {
    vi.mocked(cp.execFile).mockImplementation((cmd, args, opts, cb) => {
      ;(cb as any)(new Error('failed'), { stdout: '', stderr: '' })
      return {} as any
    })
    
    const res = await killProcessTree(12345, { platform: 'win32' })
    expect(res.gone).toBe(false)
    expect(res.orphanSuspected).toBe(true)
  })

  it.skipIf(process.platform === 'win32')('kills real POSIX process ignoring SIGTERM', async () => {
    const tmpScript = path.join(os.tmpdir(), `test-script-${Date.now()}.js`)
    fs.writeFileSync(tmpScript, `
      process.on('SIGTERM', () => { /* ignore */ })
      setInterval(() => {}, 100)
    `)

    const child = cp.spawn(process.execPath, [tmpScript], { detached: true })
    const pid = child.pid!
    
    // Give it a moment to start
    await new Promise(r => setTimeout(r, 200))

    const res = await killProcessTree(pid, { platform: 'linux', termGraceMs: 200, verifyTimeoutMs: 1000 })
    expect(res.gone).toBe(true)

    // Wait for the process to actually exit if it hasn't
    await new Promise(r => setTimeout(r, 100))
    expect(() => process.kill(pid, 0)).toThrow(/ESRCH/)

    fs.unlinkSync(tmpScript)
  })
})
