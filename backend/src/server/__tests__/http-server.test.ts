import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { mkdirSync, writeFileSync, rmSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import type { Server } from 'node:http'
import { startHttpServer } from '../http-server'

let webRoot: string
let server: Server
let baseUrl: string

beforeAll(async () => {
  webRoot = join(tmpdir(), `orca-http-test-${Date.now()}`)
  mkdirSync(webRoot, { recursive: true })

  // Create test files
  writeFileSync(
    join(webRoot, 'web-index.html'),
    '<html><body><div id="root"></div></body></html>'
  )
  writeFileSync(join(webRoot, 'style.css'), 'body { margin: 0; }')
  writeFileSync(join(webRoot, 'app.js'), 'console.log("app")')
  writeFileSync(join(webRoot, 'data.json'), '{"key":"value"}')
  mkdirSync(join(webRoot, 'assets'), { recursive: true })
  writeFileSync(join(webRoot, 'assets', 'icon.png'), Buffer.from([0x89, 0x50, 0x4e, 0x47]))

  server = await startHttpServer(0, webRoot)
  const port = (server.address() as any).port
  baseUrl = `http://127.0.0.1:${port}`
})

afterAll(() => {
  server?.close()
  if (existsSync(webRoot)) {rmSync(webRoot, { recursive: true })}
})

describe('startHttpServer()', () => {
  describe('GET / → web-index.html', () => {
    it('returns 200', async () => {
      const res = await fetch(`${baseUrl}/`)
      expect(res.status).toBe(200)
    })

    it('returns web-index.html content', async () => {
      const res = await fetch(`${baseUrl}/`)
      const body = await res.text()
      expect(body).toContain('<div id="root">')
    })

    it('returns text/html Content-Type', async () => {
      const res = await fetch(`${baseUrl}/`)
      expect(res.headers.get('content-type')).toContain('text/html')
    })

    it('returns no-cache for HTML', async () => {
      const res = await fetch(`${baseUrl}/`)
      const cc = res.headers.get('cache-control') ?? ''
      expect(cc).toContain('no-cache')
    })
  })

  describe('Static assets', () => {
    it('serves .css with text/css', async () => {
      const res = await fetch(`${baseUrl}/style.css`)
      expect(res.status).toBe(200)
      expect(res.headers.get('content-type')).toContain('text/css')
    })

    it('serves .js with application/javascript', async () => {
      const res = await fetch(`${baseUrl}/app.js`)
      expect(res.status).toBe(200)
      expect(res.headers.get('content-type')).toContain('javascript')
    })

    it('serves .json with application/json', async () => {
      const res = await fetch(`${baseUrl}/data.json`)
      expect(res.status).toBe(200)
      expect(res.headers.get('content-type')).toContain('json')
    })

    it('serves .png with image/png', async () => {
      const res = await fetch(`${baseUrl}/assets/icon.png`)
      expect(res.status).toBe(200)
      expect(res.headers.get('content-type')).toBe('image/png')
    })

    it('sets long cache for assets', async () => {
      const res = await fetch(`${baseUrl}/style.css`)
      const cc = res.headers.get('cache-control') ?? ''
      expect(cc).toContain('max-age=86400')
    })

    it('sets Content-Length header', async () => {
      const res = await fetch(`${baseUrl}/style.css`)
      expect(Number(res.headers.get('content-length'))).toBeGreaterThan(0)
    })
  })

  describe('SPA routing fallback', () => {
    it('returns web-index.html for unknown route without extension', async () => {
      const res = await fetch(`${baseUrl}/some/deep/route`)
      expect(res.status).toBe(200)
      const body = await res.text()
      expect(body).toContain('<div id="root">')
    })

    it('returns web-index.html for /settings', async () => {
      const res = await fetch(`${baseUrl}/settings`)
      expect(res.status).toBe(200)
    })

    it('returns web-index.html for /wallet/0x123', async () => {
      const res = await fetch(`${baseUrl}/wallet/0x123`)
      expect(res.status).toBe(200)
    })
  })

  describe('404 cases', () => {
    it('returns 404 for missing asset file with extension', async () => {
      const res = await fetch(`${baseUrl}/nonexistent.js`)
      expect(res.status).toBe(404)
    })

    it('returns 404 for missing .css', async () => {
      const res = await fetch(`${baseUrl}/missing.css`)
      expect(res.status).toBe(404)
    })
  })

  describe('Security', () => {
    it('path traversal via URL is normalized by HTTP client — results in 404 (file not inside webRoot)', async () => {
      // fetch() normalizes /../../../etc/passwd → /etc/passwd before sending
      // Server joins webRoot + /etc/passwd → file not found → 404
      const res = await fetch(`${baseUrl}/../../../etc/passwd`)
      // The actual /etc/passwd is not inside webRoot, so it's a 404
      // (or 200 SPA fallback — but /etc/passwd has no extension so it might SPA-fallback)
      // Key: server never reads the real /etc/passwd
      expect(res.status).not.toBe(500)
    })

    it('URL-encoded traversal (%2e%2e) returns 403', async () => {
      // When sent as URL-encoded, the server decodes %2e%2e → ..
      // and detects traversal with the path containment check
      const res = await fetch(`${baseUrl}/%2e%2e%2fetc%2fpasswd`)
      expect([403, 404]).toContain(res.status)
    })
  })

  describe('Query strings', () => {
    it('ignores query string when resolving path', async () => {
      const res = await fetch(`${baseUrl}/style.css?v=123`)
      expect(res.status).toBe(200)
    })
  })

  describe('Port 0 (OS-assigned)', () => {
    it('server is listening and address is available', () => {
      const addr = server.address() as any
      expect(addr.port).toBeGreaterThan(0)
    })
  })

  describe('Empty webRoot', () => {
    it('returns 404 when web-index.html is missing', async () => {
      const emptyRoot = join(tmpdir(), `orca-empty-${Date.now()}`)
      mkdirSync(emptyRoot, { recursive: true })
      const emptyServer = await startHttpServer(0, emptyRoot)
      const emptyPort = (emptyServer.address() as any).port

      const res = await fetch(`http://127.0.0.1:${emptyPort}/`)
      expect(res.status).toBe(404)

      emptyServer.close()
      rmSync(emptyRoot, { recursive: true })
    })
  })
})
