import fs from 'fs'
import path from 'path'
import os from 'os'
import { spawn } from 'child_process'
import { killProcessTree } from './quality-process-tree-kill'
import { PlannedStep, StepExecResult } from './quality-run-types'

export async function executeStep(
  step: PlannedStep,
  ctx: {
    runDir: string
    signal: AbortSignal
    heavyGate?: { acquire(signal: AbortSignal, waitMs: number): Promise<() => void> }
  }
): Promise<StepExecResult> {
  const startMs = Date.now()
  let releaseGate: (() => void) | undefined

  if (step.heavy && ctx.heavyGate) {
    try {
      releaseGate = await ctx.heavyGate.acquire(ctx.signal, 120000)
    } catch (err: any) {
      if (err.name === 'AbortError') {
        return { kind: 'cancelled', exitCode: null, stdoutTail: '', stderrTail: '', durationMs: Date.now() - startMs }
      }
      return { kind: 'gate_timeout', exitCode: null, stdoutTail: '', stderrTail: '', durationMs: Date.now() - startMs }
    }
  }

  const stdoutPath = path.join(ctx.runDir, `${step.id}.stdout`)
  const stderrPath = path.join(ctx.runDir, `${step.id}.stderr`)

  let fdOut: number | null = null
  let fdErr: number | null = null

  try {
    if (!fs.existsSync(ctx.runDir)) {
      fs.mkdirSync(ctx.runDir, { recursive: true, mode: 0o700 })
    }

    fdOut = fs.openSync(stdoutPath, 'wx', 0o600)
    fdErr = fs.openSync(stderrPath, 'wx', 0o600)

    const child = spawn(step.toolPath, step.args, {
      cwd: step.cwd,
      env: step.env,
      stdio: ['ignore', fdOut, fdErr],
      detached: process.platform !== 'win32',
      windowsHide: true,
      shell: false
    })

    if (child.pid && child.pid > 0) {
      try {
        os.setPriority(child.pid, 10)
      } catch (e) {
        // Ignore EPERM
      }
    }

    let kind: StepExecResult['kind'] = 'success'
    let exitCode: number | null = null
    let childExited = false
    let resolveExit: () => void

    const exitPromise = new Promise<void>(res => {
      resolveExit = res
    })

    child.on('error', () => {
      if (!childExited) {
        childExited = true
        kind = 'spawn_error'
        resolveExit()
      }
    })

    child.on('exit', (code, sig) => {
      if (!childExited) {
        childExited = true
        exitCode = code
        if (kind === 'success' && (code !== 0 || sig)) {
          kind = 'failed_exit'
        }
        resolveExit()
      }
    })

    if (child.pid === undefined) {
      kind = 'spawn_error'
      childExited = true
      resolveExit!()
    }

    const maxWaitMs = step.timeoutMs
    let elapsed = 0
    let monitorTimer: NodeJS.Timeout | null = null

    const checkSize = () => {
      if (childExited) return
      try {
        const stOut = fs.fstatSync(fdOut!)
        const stErr = fs.fstatSync(fdErr!)
        if (stOut.size + stErr.size > step.maxOutputBytes) {
          kind = 'output_too_large'
          resolveExit()
          return
        }
      } catch (e) {
        // ignore
      }

      elapsed += 500
      if (elapsed >= maxWaitMs) {
        kind = 'timeout'
        resolveExit()
      }
    }

    monitorTimer = setInterval(checkSize, 500)

    const abortHandler = () => {
      if (!childExited) {
        kind = 'cancelled'
        resolveExit()
      }
    }
    ctx.signal.addEventListener('abort', abortHandler)

    await exitPromise

    if (monitorTimer) clearInterval(monitorTimer)
    ctx.signal.removeEventListener('abort', abortHandler)

    if (kind !== 'success' && kind !== 'failed_exit' && kind !== 'spawn_error' && child.pid) {
      await killProcessTree(child.pid)
    }

    const durationMs = Date.now() - startMs

    const readTail = (filePath: string) => {
      try {
        const st = fs.statSync(filePath)
        const size = Math.min(st.size, 8192)
        if (size === 0) return ''
        const buf = Buffer.alloc(size)
        const fd = fs.openSync(filePath, 'r')
        fs.readSync(fd, buf, 0, size, Math.max(0, st.size - size))
        fs.closeSync(fd)
        return buf.toString('utf8')
      } catch {
        return ''
      }
    }

    return {
      kind,
      exitCode,
      stdoutTail: readTail(stdoutPath),
      stderrTail: readTail(stderrPath),
      durationMs
    }
  } catch (err: any) {
    return {
      kind: 'spawn_error',
      exitCode: null,
      stdoutTail: '',
      stderrTail: err.message,
      durationMs: Date.now() - startMs
    }
  } finally {
    if (fdOut !== null) try { fs.closeSync(fdOut) } catch (e) {}
    if (fdErr !== null) try { fs.closeSync(fdErr) } catch (e) {}
    if (releaseGate) releaseGate()
  }
}
