import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { executeStep } from './quality-run-step-executor'
import { PlannedStep } from './quality-run-types'
import fs from 'fs'
import path from 'path'
import os from 'os'

describe('quality-run-step-executor', () => {
  let tmpRoot: string

  beforeEach(() => {
    tmpRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-quality-test-'))
  })

  afterEach(() => {
    fs.rmSync(tmpRoot, { recursive: true, force: true })
  })

  const makeStep = (script: string, maxOutputBytes = 1024 * 1024, timeoutMs = 5000): PlannedStep => {
    const scriptPath = path.join(tmpRoot, 'script.js')
    fs.writeFileSync(scriptPath, script)
    return {
      id: 'step1',
      toolPath: process.execPath,
      args: [scriptPath],
      cwd: tmpRoot,
      env: {},
      timeoutMs,
      maxOutputBytes,
      heavy: false,
      parser: 'foo'
    }
  }

  it('handles success', async () => {
    const step = makeStep(`console.log('hello stdout'); console.error('hello stderr'); process.exit(0)`)
    const ac = new AbortController()
    
    const res = await executeStep(step, { runDir: path.join(tmpRoot, 'run'), signal: ac.signal })
    
    expect(res.kind).toBe('success')
    expect(res.exitCode).toBe(0)
    expect(res.stdoutTail.trim()).toBe('hello stdout')
    expect(res.stderrTail.trim()).toBe('hello stderr')

    const statOut = fs.statSync(path.join(tmpRoot, 'run', 'step1.stdout'))
    // expect mode 0600. on windows it might be different, but let's check basic
    if (process.platform !== 'win32') {
      expect(statOut.mode & 0o777).toBe(0o600)
    }
  })

  it('handles failed_exit', async () => {
    const step = makeStep(`process.exit(3)`)
    const ac = new AbortController()
    
    const res = await executeStep(step, { runDir: path.join(tmpRoot, 'run'), signal: ac.signal })
    
    expect(res.kind).toBe('failed_exit')
    expect(res.exitCode).toBe(3)
  })

  it('handles timeout', async () => {
    // We make a script that sleeps forever. We set timeoutMs=700 (since interval is 500ms, it will catch it at 1000ms maybe or earlier if elapsed >= 500).
    const step = makeStep(`setInterval(() => {}, 1000)`, 1024, 700)
    const ac = new AbortController()
    
    const res = await executeStep(step, { runDir: path.join(tmpRoot, 'run'), signal: ac.signal })
    
    expect(res.kind).toBe('timeout')
    expect(res.exitCode).toBe(null) // process was killed
  })

  it('handles output_too_large', async () => {
    // Print a lot of text quickly
    const step = makeStep(`
      for (let i = 0; i < 1000; i++) {
        console.log('X'.repeat(1024))
      }
      setInterval(() => {}, 1000)
    `, 100000)
    const ac = new AbortController()
    
    const res = await executeStep(step, { runDir: path.join(tmpRoot, 'run'), signal: ac.signal })
    
    expect(res.kind).toBe('output_too_large')
  })

  it('handles cancelled', async () => {
    const step = makeStep(`setInterval(() => {}, 1000)`)
    const ac = new AbortController()
    
    const p = executeStep(step, { runDir: path.join(tmpRoot, 'run'), signal: ac.signal })
    
    setTimeout(() => ac.abort(), 200)
    const res = await p
    
    expect(res.kind).toBe('cancelled')
  })

  it('handles spawn_error', async () => {
    const step = makeStep(``)
    step.toolPath = '/does/not/exist/bin'
    const ac = new AbortController()
    
    const res = await executeStep(step, { runDir: path.join(tmpRoot, 'run'), signal: ac.signal })
    
    expect(res.kind).toBe('spawn_error')
  })

  it('handles heavyGate acquire', async () => {
    const step = makeStep(`process.exit(0)`)
    step.heavy = true
    const ac = new AbortController()

    let releaseCalled = false
    const heavyGate = {
      acquire: async () => {
        return () => { releaseCalled = true }
      }
    }
    
    const res = await executeStep(step, { runDir: path.join(tmpRoot, 'run'), signal: ac.signal, heavyGate })
    
    expect(res.kind).toBe('success')
    expect(releaseCalled).toBe(true)
  })

  it('handles gate_timeout', async () => {
    const step = makeStep(``)
    step.heavy = true
    const ac = new AbortController()

    const heavyGate = {
      acquire: async () => {
        throw new Error('timeout limit reached')
      }
    }
    
    const res = await executeStep(step, { runDir: path.join(tmpRoot, 'run'), signal: ac.signal, heavyGate })
    
    expect(res.kind).toBe('gate_timeout')
  })
})
