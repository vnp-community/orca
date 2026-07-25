/**
 * HTTP Server — serves static web bundle + Express auth/admin routes
 *
 * Architecture:
 * - Express app handles API routes: /auth/*, /admin/api/*
 * - Node.js raw HTTP handles static file serving (streaming, efficient)
 * - Both are composed into one Node.js http.Server
 *
 * @module server/http-server
 */

import { createServer, type Server } from 'node:http'
import { createReadStream, existsSync, statSync } from 'node:fs'
import { join, extname } from 'node:path'
import express from 'express'
import cookieParser from 'cookie-parser'
import type { Request, Response, NextFunction } from 'express'
import type { HealthChecker } from '../main/db/health'
import type { AuthManager } from '../main/auth/auth-manager'
import type { ISyncDatabase } from '../main/db/types'
import { createAuthMiddleware } from '../main/auth/auth-middleware'
import { createAuthRouter } from '../main/auth/auth-router'
import { createAdminRouter }    from '../main/admin/admin-router'
import { AdminUserHandlers }    from '../main/admin/admin-user-handlers'
import { AdminSessionHandlers } from '../main/admin/admin-session-handlers'
import { AdminStatsHandler }    from '../main/admin/admin-stats-handler'
import { AdminAuditHandlers }   from '../main/admin/admin-audit-handlers'
import { AuditLogger }          from '../main/admin/audit-logger'
import { ensureFirstAdminUser } from '../main/admin/first-run-setup'

const MIME_TYPES: Record<string, string> = {
  '.html': 'text/html; charset=utf-8',
  '.js':   'application/javascript; charset=utf-8',
  '.mjs':  'application/javascript; charset=utf-8',
  '.css':  'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png':  'image/png',
  '.jpg':  'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif':  'image/gif',
  '.svg':  'image/svg+xml',
  '.ico':  'image/x-icon',
  '.woff':  'font/woff',
  '.woff2': 'font/woff2',
  '.ttf':   'font/ttf',
  '.map':   'application/json',
  '.txt':   'text/plain; charset=utf-8',
  '.webp':  'image/webp',
  '.avif':  'image/avif'
}

export interface HttpServerOptions {
  /** If provided, /health/* routes are exposed before static file serving */
  dbMonitor?: HealthChecker
  /** If provided, /auth/* and /admin/api/* Express routes are mounted */
  authManager?: AuthManager
  /** Sync DB instance for admin stats + audit logger. Required when authManager is set. */
  db?: ISyncDatabase
}

/**
 * Start an HTTP server that serves static web bundle files + optional auth API.
 *
 * @param port     - Port to listen on. Use 0 for OS-assigned port.
 * @param webRoot  - Directory containing the built web bundle (e.g., out/web/)
 * @param options  - Optional configuration (health monitor, auth manager, etc.)
 * @returns Promise resolving to the HTTP Server instance
 */
export async function startHttpServer(
  port: number,
  webRoot: string,
  options: HttpServerOptions = {}
): Promise<Server> {
  const normalizedRoot = webRoot.replace(/\/$/, '')

  // ── Express app for API routes ─────────────────────────────────────────────
  const app = express()
  app.use(express.json())
  app.use(cookieParser())

  // Auth middleware — populates req.orcaSession on every request
  if (options.authManager) {
    app.use(createAuthMiddleware(options.authManager))
    app.use('/auth', createAuthRouter(options.authManager))
    console.log('[HttpServer] Auth routes mounted at /auth')

    // ── Admin Panel ─────────────────────────────────────────────────────────
    // Use options.db or fall back to the AuthManager's internal db reference
    const adminDb = (options.db ?? (options.authManager as unknown as { db: ISyncDatabase }).db) as ISyncDatabase
    if (adminDb) {
      const auditLogger  = new AuditLogger(adminDb)
      const adminRouter  = createAdminRouter({
        userHandlers:    new AdminUserHandlers({
          userStore:    options.authManager.userStore,
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        sessionHandlers: new AdminSessionHandlers({
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        statsHandler:  new AdminStatsHandler(adminDb),
        auditHandlers: new AdminAuditHandlers(auditLogger)
      })

      app.use('/admin/api', adminRouter)
      console.log('[HttpServer] Admin routes mounted at /admin/api')

      // First-run: seed admin user if none exists
      await ensureFirstAdminUser(adminDb, options.authManager.userStore)

      // Audit server start event
      auditLogger.log({ action: 'server.start' })
    } else {
      console.warn('[HttpServer] No db provided — admin routes skipped')
    }
  }

  // Convert Express error handler
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  app.use((err: Error, _req: Request, res: Response, _next: NextFunction) => {
    console.error('[HttpServer] Unhandled error:', err.message)
    res.status(500).json({ error: 'internal_error', message: err.message })
  })

  // ── Lazily create health endpoint handler ──────────────────────────────────
  let healthHandler: ((req: import('node:http').IncomingMessage, res: import('node:http').ServerResponse) => void) | null = null
  if (options.dbMonitor) {
    const { createHealthEndpoint } = await import('./health-endpoint')
    healthHandler = createHealthEndpoint(options.dbMonitor, { includePoolStats: true })
  }

  // ── Compose: Express API + Health + Static files ───────────────────────────
  const server = createServer((req, res) => {
    const path = req.url ?? '/'

    // 1. /health/* → raw health handler
    if (healthHandler && path.startsWith('/health')) {
      void healthHandler(req, res)
      return
    }

    // 2. /auth/* or /admin/api/* → Express app
    if (path.startsWith('/auth') || path.startsWith('/admin')) {
      app(req, res)
      return
    }

    // 3. Everything else → static file handler
    handleStaticRequest(req, res, normalizedRoot)
  })

  return new Promise((resolve, reject) => {
    server.on('error', reject)
    server.listen(port, '0.0.0.0', () => {
      const addr = server.address()
      const actualPort = typeof addr === 'object' && addr ? addr.port : port
      console.log(`[HttpServer] Serving ${normalizedRoot} on :${actualPort}`)
      if (options.dbMonitor) {
        console.log('[HttpServer] Health endpoints: /health, /health/ready, /health/metrics')
      }
      if (options.authManager) {
        console.log('[HttpServer] Auth endpoints: POST /auth/local, POST /auth/logout, GET /auth/me')
      }
      resolve(server)
    })
  })
}

function handleStaticRequest(
  req: import('node:http').IncomingMessage,
  res: import('node:http').ServerResponse,
  webRoot: string
): void {
  const rawUrl = req.url ?? '/'

  let decodedPath: string
  try {
    decodedPath = decodeURIComponent(rawUrl.split('?')[0]!)
  } catch {
    res.writeHead(400, { 'Content-Type': 'text/plain' })
    res.end('Bad Request')
    return
  }

  const normalizedPath = decodedPath === '/' ? '/web-index.html' : decodedPath
  const filePath = join(webRoot, normalizedPath)

  if (!filePath.startsWith(webRoot + '/') && filePath !== webRoot) {
    res.writeHead(403, { 'Content-Type': 'text/plain' })
    res.end('Forbidden')
    return
  }

  if (existsSync(filePath) && statSync(filePath).isFile()) {
    serveFile(res, filePath)
    return
  }

  const ext = extname(decodedPath)
  if (!ext || ext === '') {
    const hasDotSegments = decodedPath.includes('..') || decodedPath.includes('./')
    if (hasDotSegments) {
      res.writeHead(404, { 'Content-Type': 'text/plain' })
      res.end('Not Found')
      return
    }
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

function serveFile(res: import('node:http').ServerResponse, filePath: string): void {
  const ext      = extname(filePath).toLowerCase()
  const mimeType = MIME_TYPES[ext] ?? 'application/octet-stream'
  const isHtml   = ext === '.html'
  const stat     = statSync(filePath)

  res.writeHead(200, {
    'Content-Type':  mimeType,
    'Content-Length': stat.size,
    'Cache-Control': isHtml ? 'no-cache, no-store, must-revalidate' : 'public, max-age=86400'
  })

  createReadStream(filePath).pipe(res)
}
