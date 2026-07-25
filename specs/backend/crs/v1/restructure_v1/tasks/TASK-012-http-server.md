# TASK-012: Tạo `src/server/http-server.ts`

**Source:** SOL-BE-004  
**Phase:** 2 | **Effort:** S (45–60 min)  
**Depends on:** TASK-011

---

## Objective

Tạo `src/server/http-server.ts` — HTTP server serve static web bundle (`out/web/`) và hỗ trợ SPA routing (mọi path không có extension đều trả về `web-index.html`).

---

## File to create

### `src/server/http-server.ts`

```typescript
import { createServer, IncomingMessage, ServerResponse, Server } from 'node:http'
import { createReadStream, existsSync, statSync } from 'node:fs'
import { join, extname } from 'node:path'

const MIME_TYPES: Record<string, string> = {
  '.html':  'text/html; charset=utf-8',
  '.js':    'application/javascript; charset=utf-8',
  '.mjs':   'application/javascript; charset=utf-8',
  '.css':   'text/css; charset=utf-8',
  '.json':  'application/json; charset=utf-8',
  '.png':   'image/png',
  '.jpg':   'image/jpeg',
  '.jpeg':  'image/jpeg',
  '.gif':   'image/gif',
  '.svg':   'image/svg+xml',
  '.ico':   'image/x-icon',
  '.woff':  'font/woff',
  '.woff2': 'font/woff2',
  '.ttf':   'font/ttf',
  '.map':   'application/json',
  '.txt':   'text/plain; charset=utf-8',
}

/**
 * Start an HTTP server that serves static web bundle files.
 *
 * Features:
 * - Serves files from webRoot directory
 * - Correct MIME types for all asset types
 * - SPA fallback: unknown paths → web-index.html
 * - Cache headers for assets (1 day), no-cache for HTML
 *
 * @param port - Port to listen on. Use 0 for OS-assigned port.
 * @param webRoot - Directory containing the built web bundle (e.g., out/web/)
 * @returns Promise resolving to the HTTP Server instance
 */
export async function startHttpServer(
  port: number,
  webRoot: string
): Promise<Server> {
  const server = createServer((req: IncomingMessage, res: ServerResponse) => {
    handleRequest(req, res, webRoot)
  })

  return new Promise((resolve, reject) => {
    server.on('error', reject)
    server.listen(port, '0.0.0.0', () => {
      const addr = server.address()
      const actualPort = typeof addr === 'object' && addr ? addr.port : port
      console.log(`[HttpServer] Serving ${webRoot} on :${actualPort}`)
      resolve(server)
    })
  })
}

function handleRequest(
  req: IncomingMessage,
  res: ServerResponse,
  webRoot: string
): void {
  const urlPath = req.url?.split('?')[0] ?? '/'
  const normalizedPath = urlPath === '/' ? '/web-index.html' : urlPath

  const filePath = join(webRoot, normalizedPath)

  // Security: prevent path traversal
  if (!filePath.startsWith(webRoot)) {
    res.writeHead(403, { 'Content-Type': 'text/plain' })
    res.end('Forbidden')
    return
  }

  if (existsSync(filePath) && statSync(filePath).isFile()) {
    serveFile(res, filePath)
    return
  }

  // SPA fallback: serve index for paths without file extension
  const ext = extname(urlPath)
  if (!ext || ext === '') {
    const indexPath = join(webRoot, 'web-index.html')
    if (existsSync(indexPath)) {
      serveFile(res, indexPath)
    } else {
      res.writeHead(404, { 'Content-Type': 'text/plain' })
      res.end('Not Found: web-index.html missing from ' + webRoot)
    }
    return
  }

  res.writeHead(404, { 'Content-Type': 'text/plain' })
  res.end('Not Found')
}

function serveFile(res: ServerResponse, filePath: string): void {
  const ext = extname(filePath).toLowerCase()
  const mimeType = MIME_TYPES[ext] ?? 'application/octet-stream'
  const isHtml = ext === '.html'

  const stat = statSync(filePath)

  res.writeHead(200, {
    'Content-Type': mimeType,
    'Content-Length': stat.size,
    'Cache-Control': isHtml
      ? 'no-cache, no-store, must-revalidate'
      : 'public, max-age=86400',
  })

  createReadStream(filePath).pipe(res)
}
```

---

## Test file

### `src/server/__tests__/http-server.test.ts`

```typescript
import { describe, it, expect, afterEach, beforeAll } from 'vitest'
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
  writeFileSync(join(webRoot, 'web-index.html'), '<html><body><div id="root"></div></body></html>')
  writeFileSync(join(webRoot, 'style.css'), 'body { margin: 0; }')
  writeFileSync(join(webRoot, 'app.js'), 'console.log("app")')
  writeFileSync(join(webRoot, 'data.json'), '{"key":"value"}')

  server = await startHttpServer(0, webRoot)
  const port = (server.address() as any).port
  baseUrl = `http://127.0.0.1:${port}`
})

afterEach(() => {})

// Note: server closed at test suite end via afterAll if needed
// For simplicity in CI, we leave it open as Node cleanup handles it

describe('HTTP Static Server', () => {
  describe('GET /', () => {
    it('returns 200 with web-index.html content', async () => {
      const res = await fetch(`${baseUrl}/`)
      expect(res.status).toBe(200)
      const body = await res.text()
      expect(body).toContain('<div id="root">')
    })

    it('returns text/html content-type', async () => {
      const res = await fetch(`${baseUrl}/`)
      expect(res.headers.get('content-type')).toContain('text/html')
    })

    it('returns no-cache headers for HTML', async () => {
      const res = await fetch(`${baseUrl}/`)
      expect(res.headers.get('cache-control')).toContain('no-cache')
    })
  })

  describe('Static assets', () => {
    it('serves .css with correct MIME type', async () => {
      const res = await fetch(`${baseUrl}/style.css`)
      expect(res.status).toBe(200)
      expect(res.headers.get('content-type')).toContain('text/css')
    })

    it('serves .js with correct MIME type', async () => {
      const res = await fetch(`${baseUrl}/app.js`)
      expect(res.status).toBe(200)
      expect(res.headers.get('content-type')).toContain('javascript')
    })

    it('serves .json with correct MIME type', async () => {
      const res = await fetch(`${baseUrl}/data.json`)
      expect(res.status).toBe(200)
      expect(res.headers.get('content-type')).toContain('json')
    })

    it('returns 1-day cache for assets', async () => {
      const res = await fetch(`${baseUrl}/style.css`)
      const cc = res.headers.get('cache-control') ?? ''
      expect(cc).toContain('max-age=86400')
    })
  })

  describe('SPA routing', () => {
    it('returns web-index.html for unknown routes', async () => {
      const res = await fetch(`${baseUrl}/some/deep/route`)
      expect(res.status).toBe(200)
      const body = await res.text()
      expect(body).toContain('<div id="root">')
    })

    it('returns web-index.html for /settings', async () => {
      const res = await fetch(`${baseUrl}/settings`)
      expect(res.status).toBe(200)
    })
  })

  describe('404 cases', () => {
    it('returns 404 for missing asset files (.js, .png, etc)', async () => {
      const res = await fetch(`${baseUrl}/missing-file.js`)
      expect(res.status).toBe(404)
    })
  })

  describe('Security', () => {
    it('blocks path traversal attempts', async () => {
      const res = await fetch(`${baseUrl}/../../../etc/passwd`)
      // Should return 403 or 404, not 200
      expect([403, 404]).toContain(res.status)
    })
  })

  describe('When web-index.html missing', () => {
    it('returns 404 for / when webRoot is empty', async () => {
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
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep "http-server" | head -10
npx vitest run src/server/__tests__/http-server.test.ts
```

Expected: **12+ tests pass**, 0 errors.

---

## Done criteria

- [x] `src/server/http-server.ts` tạo thành công
- [x] `startHttpServer(0, webRoot)` trả về Server resolve khi listen
- [x] `/` → `web-index.html`
- [x] `/some/route` → `web-index.html` (SPA fallback)
- [x] `/file.css` → correct MIME type
- [x] Path traversal `/../` → 403 hoặc 404
- [x] Empty webRoot → 404
- [x] 12+ tests pass
