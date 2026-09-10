import { describe, expect, it, afterEach } from 'vitest'
import { mkdtempSync, rmSync, existsSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import { parseRelayDaemonArgs, runRelayDaemon } from './relay-daemon'
import { defaultRelayPidFilePath } from './relay-daemon-pidfile'

describe('parseRelayDaemonArgs', () => {
  it('defaults to daemon=false, port=0, and the default pid file path', () => {
    const args = parseRelayDaemonArgs(['node', 'relay-daemon.js'])
    expect(args.daemon).toBe(false)
    expect(args.port).toBe(0)
    expect(args.pidFile).toBe(defaultRelayPidFilePath())
  })

  it('recognizes --daemon', () => {
    const args = parseRelayDaemonArgs(['node', 'relay-daemon.js', '--daemon'])
    expect(args.daemon).toBe(true)
  })

  it('parses --port', () => {
    const args = parseRelayDaemonArgs(['node', 'relay-daemon.js', '--daemon', '--port', '8080'])
    expect(args.port).toBe(8080)
  })

  it('ignores a non-numeric --port value', () => {
    const args = parseRelayDaemonArgs(['node', 'relay-daemon.js', '--port', 'nope'])
    expect(args.port).toBe(0)
  })

  it('parses --pid-file', () => {
    const args = parseRelayDaemonArgs(['node', 'relay-daemon.js', '--pid-file', '/tmp/custom.pid'])
    expect(args.pidFile).toBe('/tmp/custom.pid')
  })

  it('parses all three flags together, in any order', () => {
    const args = parseRelayDaemonArgs([
      'node',
      'relay-daemon.js',
      '--pid-file',
      '/tmp/custom.pid',
      '--daemon',
      '--port',
      '9090'
    ])
    expect(args).toEqual({ daemon: true, port: 9090, pidFile: '/tmp/custom.pid' })
  })
})

describe('runRelayDaemon', () => {
  let dir: string
  let stop: (() => void) | null = null

  afterEach(() => {
    stop?.()
    stop = null
    if (dir) {
      rmSync(dir, { recursive: true, force: true })
    }
  })

  it('writes the pid file and starts a real, reachable health server', async () => {
    dir = mkdtempSync(join(tmpdir(), 'relay-daemon-test-'))
    const pidFile = join(dir, 'relay-daemon.pid')

    const result = await runRelayDaemon({ daemon: true, port: 0, pidFile })
    stop = result.stop

    expect(result.port).toBeGreaterThan(0)
    expect(existsSync(pidFile)).toBe(true)
    expect(readFileSync(pidFile, 'utf8')).toBe(String(process.pid))

    const res = await fetch(`http://127.0.0.1:${result.port}/health`)
    expect(res.status).toBe(200)
    const body = (await res.json()) as { status: string; pid: number }
    expect(body.status).toBe('ok')
    expect(body.pid).toBe(process.pid)
  })

  it('stop() removes the pid file and stops the health server', async () => {
    dir = mkdtempSync(join(tmpdir(), 'relay-daemon-test-'))
    const pidFile = join(dir, 'relay-daemon.pid')

    const result = await runRelayDaemon({ daemon: true, port: 0, pidFile })
    expect(existsSync(pidFile)).toBe(true)

    result.stop()
    stop = null

    expect(existsSync(pidFile)).toBe(false)
    await expect(fetch(`http://127.0.0.1:${result.port}/health`)).rejects.toThrow()
  })
})
