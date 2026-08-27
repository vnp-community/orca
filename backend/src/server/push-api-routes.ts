// ─── push-api-routes.ts ───────────────────────────────────────────────────────
// HTTP route handlers for Web Push API (Phase 3 — TASK-034).
//
// Routes:
//   GET  /api/vapid-public-key  → 200 { publicKey: string }
//   POST /api/push-subscribe    → 201 { id: string }
//   POST /api/push-unsubscribe  → 204 (no body)
//
// Design: attaches a 'request' listener to the existing Node.js HTTP server
// so it shares the same port as the static file server. Routes that don't
// match are left unhandled — they fall through to other listeners.

import type { IncomingMessage, ServerResponse } from 'node:http'
import type { Server } from 'node:http'
import type { WebPushManager } from '../main/notifications/web-push-manager'

const MAX_BODY_BYTES = 64 * 1024 // 64 KB — sufficient for any push subscription JSON

/**
 * Register push notification API routes on an existing HTTP server.
 *
 * Why a listener instead of a framework: the Orca HTTP server is a plain
 * Node.js Server; attaching a listener is the simplest integration that
 * doesn't require adding a framework dependency.
 */
export function registerPushApiRoutes(server: Server, pushManager: WebPushManager): void {
  server.on('request', (req: IncomingMessage, res: ServerResponse) => {
    const url = req.url ?? ''

    // ── GET /api/vapid-public-key ──────────────────────────────────────────────
    // Returns the server's VAPID public key so the browser can subscribe.
    // The client must have this key before calling pushManager.subscribe().

    if (req.method === 'GET' && url === '/api/vapid-public-key') {
      // ADR-021 Phase 1: WebPushManager's methods are async now (see its
      // WebPushStoreDependency doc comment) — this handler itself stays a
      // plain (non-async) listener callback, so the async work runs as a
      // detached promise chain with its own .catch(), same pattern already
      // used below for the subscribe/unsubscribe routes.
      pushManager
        .getPublicKey()
        .then((publicKey) => {
          res.writeHead(200, { 'Content-Type': 'application/json' })
          res.end(JSON.stringify({ publicKey }))
        })
        .catch(() => {
          res.writeHead(500, { 'Content-Type': 'text/plain' })
          res.end('Failed to load VAPID public key')
        })
      return
    }

    // ── POST /api/push-subscribe ───────────────────────────────────────────────
    // Body: { subscription: PushSubscriptionJSON }
    // Saves (or upserts) the subscription. Returns the record ID.

    if (req.method === 'POST' && url === '/api/push-subscribe') {
      readBody(req)
        .then(async (body) => {
          const parsed = JSON.parse(body) as { subscription?: unknown }
          if (!parsed.subscription || typeof parsed.subscription !== 'object') {
            res.writeHead(400, { 'Content-Type': 'text/plain' })
            res.end('Missing subscription field')
            return
          }
          const record = await pushManager.saveSubscription(
            parsed.subscription as PushSubscriptionJSON,
            { userAgent: req.headers['user-agent'] }
          )
          res.writeHead(201, { 'Content-Type': 'application/json' })
          res.end(JSON.stringify({ id: record.id }))
        })
        .catch(() => {
          res.writeHead(400, { 'Content-Type': 'text/plain' })
          res.end('Invalid body')
        })
      return
    }

    // ── POST /api/push-unsubscribe ─────────────────────────────────────────────
    // Body: { endpoint: string }
    // Removes the subscription from the store. Idempotent — no error if not found.

    if (req.method === 'POST' && url === '/api/push-unsubscribe') {
      readBody(req)
        .then(async (body) => {
          const parsed = JSON.parse(body) as { endpoint?: unknown }
          if (typeof parsed.endpoint !== 'string') {
            res.writeHead(400, { 'Content-Type': 'text/plain' })
            res.end('Missing endpoint field')
            return
          }
          await pushManager.removeSubscription(parsed.endpoint)
          res.writeHead(204)
          res.end()
        })
        .catch(() => {
          res.writeHead(400)
          res.end()
        })
      return
    }

    // Unknown routes: no response — fall through to other listeners (e.g. static file server).
  })
}

/**
 * Read the request body as a UTF-8 string.
 * Rejects if body exceeds MAX_BODY_BYTES to prevent memory exhaustion.
 */
async function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let data = ''
    let size = 0

    req.on('data', (chunk: Buffer) => {
      size += chunk.length
      if (size > MAX_BODY_BYTES) {
        req.destroy()
        reject(new Error('Request body too large'))
        return
      }
      data += chunk.toString('utf-8')
    })
    req.on('end', () => resolve(data))
    req.on('error', reject)
  })
}
