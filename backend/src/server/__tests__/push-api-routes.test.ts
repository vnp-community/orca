// ─── push-api-routes.test.ts ──────────────────────────────────────────────────
// Unit tests for registerPushApiRoutes — TASK-041.
// Uses Node.js http.createServer + supertest-style manual HTTP requests.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import * as http from 'node:http'
import { registerPushApiRoutes } from '../push-api-routes'
import type { WebPushSubscription } from '../../shared/types'

// ── Fake WebPushManager ────────────────────────────────────────────────────────

function makePushManager(overrides?: { publicKey?: string }) {
  const subscriptions: WebPushSubscription[] = []
  return {
    getPublicKey: vi.fn(() => overrides?.publicKey ?? 'test_vapid_public_key'),
    saveSubscription: vi.fn((sub: PushSubscriptionJSON, meta?: { userAgent?: string }) => {
      const record: WebPushSubscription = {
        id: `rec-${Math.random()}`,
        endpoint: sub.endpoint!,
        keys: sub.keys as { auth: string; p256dh: string },
        addedAt: Date.now(),
        userAgent: meta?.userAgent
      }
      subscriptions.push(record)
      return record
    }),
    removeSubscription: vi.fn((endpoint: string) => {
      const idx = subscriptions.findIndex((s) => s.endpoint === endpoint)
      if (idx >= 0) {subscriptions.splice(idx, 1)}
    }),
    subscriptions
  }
}

// ── HTTP test helper ───────────────────────────────────────────────────────────

function doRequest(
  server: http.Server,
  method: string,
  path: string,
  body?: object
): Promise<{ statusCode: number; body: string; headers: http.IncomingHttpHeaders }> {
  return new Promise((resolve, reject) => {
    const addr = server.address() as { port: number }
    const bodyStr = body ? JSON.stringify(body) : undefined
    const req = http.request(
      {
        host: '127.0.0.1',
        port: addr.port,
        path,
        method,
        headers: {
          'Content-Type': 'application/json',
          ...(bodyStr ? { 'Content-Length': Buffer.byteLength(bodyStr).toString() } : {})
        }
      },
      (res) => {
        let data = ''
        res.on('data', (c) => (data += c))
        res.on('end', () =>
          resolve({ statusCode: res.statusCode!, body: data, headers: res.headers })
        )
      }
    )
    req.on('error', reject)
    if (bodyStr) {req.write(bodyStr)}
    req.end()
  })
}

// ── Tests ──────────────────────────────────────────────────────────────────────

describe('Push API Routes', () => {
  let server: http.Server
  let pushManager: ReturnType<typeof makePushManager>

  beforeEach(
    () =>
      new Promise<void>((resolve) => {
        pushManager = makePushManager()
        server = http.createServer()
        registerPushApiRoutes(server, pushManager as never)
        server.listen(0, '127.0.0.1', resolve)
      })
  )

  afterEach(
    () =>
      new Promise<void>((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()))
      })
  )

  // ── GET /api/vapid-public-key ────────────────────────────────────────────────

  it('GET /api/vapid-public-key → 200 { publicKey: string }', async () => {
    const res = await doRequest(server, 'GET', '/api/vapid-public-key')

    expect(res.statusCode).toBe(200)
    const parsed = JSON.parse(res.body)
    expect(parsed.publicKey).toBe('test_vapid_public_key')
    expect(pushManager.getPublicKey).toHaveBeenCalledOnce()
  })

  // ── POST /api/push-subscribe ─────────────────────────────────────────────────

  it('POST /api/push-subscribe body hợp lệ → 201 { id: string }', async () => {
    const res = await doRequest(server, 'POST', '/api/push-subscribe', {
      subscription: {
        endpoint: 'https://fcm.googleapis.com/ep1',
        keys: { auth: 'auth_val', p256dh: 'p256_val' }
      }
    })

    expect(res.statusCode).toBe(201)
    const parsed = JSON.parse(res.body)
    expect(parsed.id).toBeTruthy()
    expect(pushManager.saveSubscription).toHaveBeenCalledOnce()
  })

  it('POST /api/push-subscribe deduplicate endpoint (saveSubscription gọi lại với cùng endpoint)', async () => {
    const body = {
      subscription: {
        endpoint: 'https://fcm.googleapis.com/ep-dup',
        keys: { auth: 'a', p256dh: 'b' }
      }
    }
    await doRequest(server, 'POST', '/api/push-subscribe', body)
    const res = await doRequest(server, 'POST', '/api/push-subscribe', body)

    expect(res.statusCode).toBe(201)
    // saveSubscription được gọi 2 lần — upsert là trách nhiệm của WebPushManager
    expect(pushManager.saveSubscription).toHaveBeenCalledTimes(2)
  })

  it('POST /api/push-subscribe body không hợp lệ → 400', async () => {
    const res = await doRequest(server, 'POST', '/api/push-subscribe', {
      // missing subscription field
      endpoint: 'https://fcm.googleapis.com/ep-bad'
    })
    expect(res.statusCode).toBe(400)
  })

  it('POST /api/push-subscribe JSON sai cú pháp → 400', async () => {
    // Gửi raw non-JSON string
    const result = await new Promise<{ statusCode: number }>((resolve, reject) => {
      const addr = server.address() as { port: number }
      const body = 'not-json'
      const req = http.request(
        {
          host: '127.0.0.1',
          port: addr.port,
          path: '/api/push-subscribe',
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'Content-Length': body.length.toString() }
        },
        (res) => {
          res.resume()
          resolve({ statusCode: res.statusCode! })
        }
      )
      req.on('error', reject)
      req.write(body)
      req.end()
    })
    expect(result.statusCode).toBe(400)
  })

  // ── POST /api/push-unsubscribe ───────────────────────────────────────────────

  it('POST /api/push-unsubscribe → 204', async () => {
    const res = await doRequest(server, 'POST', '/api/push-unsubscribe', {
      endpoint: 'https://fcm.googleapis.com/ep1'
    })

    expect(res.statusCode).toBe(204)
    expect(pushManager.removeSubscription).toHaveBeenCalledWith('https://fcm.googleapis.com/ep1')
  })

  it('POST /api/push-unsubscribe body không hợp lệ → 400', async () => {
    const res = await doRequest(server, 'POST', '/api/push-unsubscribe', {
      // missing endpoint
      foo: 'bar'
    })
    expect(res.statusCode).toBe(400)
  })

  // ── Unknown route ────────────────────────────────────────────────────────────

  it('Unknown route → không gửi response (pass-through: push manager không được gọi)', async () => {
    // Why: for unknown routes our handler does NOT write a response — it passes
    // through to the next listener. The simplest way to verify this is to check
    // that none of the push-manager methods were invoked.
    // We send the request with a short deadline; if the server sends a response
    // we record it, otherwise we proceed with the assertion below.
    const addr = server.address() as { port: number }

    const responseReceived = await new Promise<boolean>((resolve) => {
      const net = require('node:net') as typeof import('node:net')
      const socket = net.createConnection({ host: '127.0.0.1', port: addr.port })
      const request =
        'GET /api/unknown-route HTTP/1.1\r\n' +
        `Host: 127.0.0.1:${addr.port}\r\n` +
        'Connection: close\r\n\r\n'

      let received = false
      socket.on('data', () => { received = true })
      socket.on('error', () => resolve(false))

      socket.write(request)

      // Give the server 200 ms to respond — our handler should stay silent.
      setTimeout(() => {
        socket.destroy()
        resolve(received)
      }, 200)
    })

    // Our route handler must NOT have responded (no push-manager involvement)
    expect(pushManager.getPublicKey).not.toHaveBeenCalled()
    expect(pushManager.saveSubscription).not.toHaveBeenCalled()
    expect(pushManager.removeSubscription).not.toHaveBeenCalled()
    // No response should have been sent by our handler
    expect(responseReceived).toBe(false)
  })
})
