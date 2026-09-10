import { describe, expect, it, afterEach } from 'vitest'

import { RelayDaemonHealthServer } from './relay-daemon-health-server'

describe('RelayDaemonHealthServer', () => {
  let server: RelayDaemonHealthServer | null = null

  afterEach(() => {
    server?.stop()
    server = null
  })

  it('starts on an ephemeral port and answers GET /health with 200 + ok status', async () => {
    server = new RelayDaemonHealthServer()
    const port = await server.start()
    expect(port).toBeGreaterThan(0)
    expect(server.boundPort).toBe(port)

    const res = await fetch(`http://127.0.0.1:${port}/health`)
    expect(res.status).toBe(200)
    const body = (await res.json()) as { status: string; pid: number; uptimeMs: number }
    expect(body.status).toBe('ok')
    expect(body.pid).toBe(process.pid)
    expect(typeof body.uptimeMs).toBe('number')
  })

  it('returns 404 for any other path', async () => {
    server = new RelayDaemonHealthServer()
    const port = await server.start()

    const res = await fetch(`http://127.0.0.1:${port}/not-health`)
    expect(res.status).toBe(404)
  })

  it('returns 404 for a non-GET method on /health', async () => {
    server = new RelayDaemonHealthServer()
    const port = await server.start()

    const res = await fetch(`http://127.0.0.1:${port}/health`, { method: 'POST' })
    expect(res.status).toBe(404)
  })

  it('uses a custom buildStatus callback when provided', async () => {
    server = new RelayDaemonHealthServer({
      buildStatus: () => ({ status: 'ok', pid: 999, uptimeMs: 42 })
    })
    const port = await server.start()

    const res = await fetch(`http://127.0.0.1:${port}/health`)
    const body = (await res.json()) as { pid: number; uptimeMs: number }
    expect(body.pid).toBe(999)
    expect(body.uptimeMs).toBe(42)
  })

  it('stop() closes the listener so subsequent requests fail', async () => {
    server = new RelayDaemonHealthServer()
    const port = await server.start()
    server.stop()

    await expect(fetch(`http://127.0.0.1:${port}/health`)).rejects.toThrow()
  })

  it('rejects start() when the requested port is already in use', async () => {
    server = new RelayDaemonHealthServer()
    const port = await server.start()

    const second = new RelayDaemonHealthServer({ port })
    await expect(second.start()).rejects.toThrow()
  })
})
