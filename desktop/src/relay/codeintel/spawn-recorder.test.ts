import { describe, it, expect } from 'vitest'
import { installSpawnRecorder } from './spawn-recorder'

describe('spawn-recorder', () => {
  it('records argv and options without spawning', async () => {
    const fakeCp = {
      spawn: () => {},
      execFile: () => {},
      execFileSync: () => {},
      spawnSync: () => {},
      exec: () => {}
    }

    const recorder = installSpawnRecorder(fakeCp, { stdout: 'ok' })

    const child = (fakeCp.spawn as any)('gitnexus', ['status', '--json'], { cwd: '/repo' })
    expect(recorder.calls.length).toBe(1)
    expect(recorder.calls[0]).toEqual({
      method: 'spawn',
      file: 'gitnexus',
      argv: ['status', '--json'],
      options: { cwd: '/repo' }
    })
    expect(child).toBeDefined()
    expect(typeof child.on).toBe('function')

    fakeCp.execFileSync('codegraph', ['search', 'foo'], { timeout: 1000 })
    expect(recorder.calls.length).toBe(2)
    expect(recorder.calls[1]).toEqual({
      method: 'execFileSync',
      file: 'codegraph',
      argv: ['search', 'foo'],
      options: { timeout: 1000 }
    })

    recorder.reset()
    expect(recorder.calls.length).toBe(0)
  })
})
