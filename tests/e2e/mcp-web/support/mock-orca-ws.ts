import type { Page, Route } from '@playwright/test'
import {
  createFakeMcpBackend,
  type FakeMcpBackend
} from '../../../../frontend/src/renderer/src/test-support/mcp-fake-backend'
import { BOOT_CHANNEL_STUBS } from './mock-orca-boot-channels'

export { createFakeMcpBackend, type FakeMcpBackend }

export type MockUser = {
  id: string
  email: string
  name: string
  role: 'admin' | 'developer'
  provider: 'none'
}

export const mockUser = (role: MockUser['role'] = 'developer'): MockUser => ({
  id: 'u1',
  email: 'user@example.com',
  name: 'Test User',
  role,
  provider: 'none'
})

const json = (route: Route, body: unknown, status = 200): Promise<void> =>
  route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })

/**
 * Serves the SPA for /oauth/consent (what nginx must do in production), mocks /auth/*, and answers
 * the session-dialect WebSocket from the shared in-memory fake MCP backend. Channels the fake does
 * not implement fail with "is not yet implemented", like the real gateway.
 * Wire format: request {id, authToken, method, params}; reply {id, ok, result|error, _meta}.
 */
export async function mockOrcaApp(
  page: Page,
  opts: {
    backend: FakeMcpBackend
    user: MockUser | null
    baseURL: string
    /** Answer the SPA's boot channels so startup hydration completes (cold-start specs). */
    bootChannels?: boolean
  }
): Promise<{ wsMethods: string[] }> {
  const wsMethods: string[] = []
  await page.route('**/auth/me', (r) =>
    opts.user ? json(r, opts.user) : json(r, { error: 'unauthenticated' }, 401)
  )
  await page.route('**/auth/config', (r) => json(r, { providers: [], localEnabled: true }))
  await page.route('**/oauth/consent*', async (route) => {
    const res = await route.fetch({ url: `${opts.baseURL}/web-index.html` })
    await route.fulfill({ response: res })
  })
  await page.routeWebSocket(/\/ws$/, (ws) => {
    const send = (m: unknown): void => ws.send(JSON.stringify(m))
    ws.onMessage(async (raw) => {
      for (const line of String(raw).split('\n').filter(Boolean)) {
        let msg: { id?: string; method?: string; params?: unknown; type?: string }
        try {
          msg = JSON.parse(line)
        } catch {
          continue
        }
        if (!msg.id || !msg.method) {
          continue // legacy connectivity client frames
        }
        wsMethods.push(msg.method)
        const meta = { runtimeId: 'backend-go' }
        if (msg.method === 'mcp.events.subscribe') {
          send({ id: msg.id, ok: true, result: null, _meta: meta })
          opts.backend.bridge.subscribeEvents((event) =>
            send({ id: msg.id, ok: true, result: event, streaming: true, _meta: meta })
          )
          continue
        }
        if (
          opts.bootChannels &&
          !msg.method.startsWith('mcp.') &&
          msg.method in BOOT_CHANNEL_STUBS
        ) {
          send({ id: msg.id, ok: true, result: BOOT_CHANNEL_STUBS[msg.method], _meta: meta })
          continue
        }
        try {
          const result = await opts.backend.bridge.call(
            msg.method as never,
            ...((msg.params && Object.keys(msg.params).length ? [msg.params] : []) as never[])
          )
          send({ id: msg.id, ok: true, result, _meta: meta })
        } catch (e) {
          send({
            id: msg.id,
            ok: false,
            error: { code: 'internal', message: (e as Error).message },
            _meta: { runtimeId: null }
          })
        }
      }
    })
  })
  return { wsMethods }
}
