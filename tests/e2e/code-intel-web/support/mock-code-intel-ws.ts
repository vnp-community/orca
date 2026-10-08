import type { Page, Route } from '@playwright/test'
import {
  createFakeCodeIntelBackend,
  type FakeCodeIntelBackend
} from '../../../../frontend/src/renderer/src/test-support/code-intel-fake-backend'
import { BOOT_CHANNEL_STUBS } from '../../mcp-web/support/mock-orca-boot-channels'

export { createFakeCodeIntelBackend, type FakeCodeIntelBackend }

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
 * Mocks /auth/* and answers the session-dialect WebSocket from the in-memory fake code-intel
 * backend. Wire: request {id, authToken, method, params}; reply {id, ok, result|error, _meta};
 * `codeIntel.subscribe` acks `null` then streams frames carrying `event` (no channel name).
 */
export async function mockCodeIntelApp(
  page: Page,
  opts: { backend: FakeCodeIntelBackend; user: MockUser | null }
): Promise<{ wsMethods: string[] }> {
  const wsMethods: string[] = []
  await page.route('**/auth/me', (r) =>
    opts.user ? json(r, opts.user) : json(r, { error: 'unauthenticated' }, 401)
  )
  await page.route('**/auth/config', (r) => json(r, { providers: [], localEnabled: true }))
  await page.routeWebSocket(/\/ws$/, (ws) => {
    const send = (m: unknown): void => ws.send(JSON.stringify(m))
    ws.onMessage(async (raw) => {
      for (const line of String(raw).split('\n').filter(Boolean)) {
        let msg: { id?: string; method?: string; params?: unknown }
        try {
          msg = JSON.parse(line)
        } catch {
          continue
        }
        if (!msg.id || !msg.method) {
          continue
        }
        wsMethods.push(msg.method)
        const meta = { runtimeId: 'backend-go' }
        if (msg.method === 'codeIntel.subscribe') {
          send({ id: msg.id, ok: true, result: null, _meta: meta })
          opts.backend.subscribe({
            onEvent: (frame) =>
              send({ id: msg.id, ok: true, result: frame, streaming: true, _meta: meta }),
            onClose: () => send({ id: msg.id, type: 'end' })
          })
          continue
        }
        if (!msg.method.startsWith('codeIntel.') && msg.method in BOOT_CHANNEL_STUBS) {
          send({ id: msg.id, ok: true, result: BOOT_CHANNEL_STUBS[msg.method], _meta: meta })
          continue
        }
        const res = await opts.backend.callEnvelope(msg.method, msg.params)
        send(
          res.ok
            ? { id: msg.id, ok: true, result: res.result, _meta: meta }
            : { id: msg.id, ok: false, error: res.error, _meta: { runtimeId: null } }
        )
      }
    })
  })
  return { wsMethods }
}
