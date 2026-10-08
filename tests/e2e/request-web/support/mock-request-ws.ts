import { expect, type Page, type Route } from '@playwright/test'
import { BOOT_CHANNEL_STUBS } from '../../mcp-web/support/mock-orca-boot-channels'

export const SPA_URL = 'http://127.0.0.1:5174'

type Params = Record<string, unknown>
type Handler = (params: Params) => unknown
export type RpcCall = { method: string; params: Params }

/** A thrown RpcFailure becomes a WS error frame `{code, message}` (gateway shape, CONTRACT 5). */
export class RpcFailure extends Error {
  constructor(
    readonly code: string,
    message: string
  ) {
    super(message)
  }
}

export type FakeRequestBackend = {
  calls: RpcCall[]
  callsTo: (method: string) => Params[]
  /** Replace or add one channel. */
  on: (method: string, handler: Handler) => void
  /** Push a `request.event` frame to every open `request.subscribe` stream. */
  emit: (event: Record<string, unknown>) => void
  handle: (method: string, params: Params) => Promise<unknown>
  has: (method: string) => boolean
  subscribers: Set<(event: unknown) => void>
}

const now = '2026-10-08T09:00:00Z'

/** CONTRACT RequestView with sensible defaults for the fields the UI reads. */
export function requestView(over: Params = {}): Params {
  return {
    id: 'req-1',
    projectId: 'proj-1',
    number: 1,
    title: 'Request',
    body: 'Body',
    type: 'change_request',
    status: 'awaiting_type_confirmation',
    urgency: 'normal',
    typeSource: 'ai',
    confidence: 0.82,
    classificationReason: 'Mentions a behaviour change',
    version: 3,
    createdAt: now,
    updatedAt: now,
    ...over
  }
}

export function approvalView(over: Params = {}): Params {
  return {
    id: 'ap-1',
    requestId: 'req-1',
    subjectType: 'solution',
    subjectId: 'sol-1',
    subjectDigest: 'digest-1',
    status: 'pending',
    version: 1,
    createdAt: now,
    updatedAt: now,
    ...over
  }
}

/**
 * In-memory request-service behind the gateway. Only channels a spec relies on are modelled;
 * anything else fails with `method_not_found`, which the UI must treat as "unsupported".
 */
export function createFakeRequestBackend(
  handlers: Record<string, Handler> = {}
): FakeRequestBackend {
  const calls: RpcCall[] = []
  const subscribers = new Set<(event: unknown) => void>()
  const table: Record<string, Handler> = {
    'request.flowStatus': () => ({ enabled: true }),
    'request.list': () => ({ requests: [], nextPageToken: null }),
    'request.typeHistory': () => ({ changes: [] }),
    'approval.listPending': () => ({ approvals: [], nextPageToken: null }),
    'approval.list': () => ({ approvals: [], nextPageToken: null }),
    'solution.list': () => ({ solutions: [], runs: [], nextPageToken: null }),
    'backlog.requests': () => ({ items: [], nextPageToken: null }),
    'backlog.tasks': () => ({ items: [], nextPageToken: null }),
    'backlog.execute': () => ({ items: [], nextPageToken: null }),
    ...handlers
  }
  return {
    calls,
    subscribers,
    callsTo: (method) => calls.filter((c) => c.method === method).map((c) => c.params),
    on: (method, handler) => {
      table[method] = handler
    },
    emit: (event) => subscribers.forEach((s) => s(event)),
    has: (method) => method in table,
    handle: async (method, params) => {
      calls.push({ method, params })
      const handler = table[method]
      if (!handler) {
        throw new RpcFailure('method_not_found', `${method} is not yet implemented`)
      }
      return handler(params)
    }
  }
}

const json = (route: Route, body: unknown, status = 200): Promise<void> =>
  route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })

/**
 * Mocks /auth/* and answers the session-dialect WebSocket: boot channels from the shared stubs,
 * request-flow channels from the fake. Wire: {id, method, params} -> {id, ok, result|error, _meta}.
 */
export async function mockRequestApp(page: Page, backend: FakeRequestBackend): Promise<void> {
  await page.route('**/auth/me', (r) =>
    json(r, {
      id: 'u1',
      email: 'user@example.com',
      name: 'Test User',
      role: 'developer',
      provider: 'none'
    })
  )
  await page.route('**/auth/config', (r) => json(r, { providers: [], localEnabled: true }))
  await page.routeWebSocket(/\/ws$/, (ws) => {
    const send = (m: unknown): void => ws.send(JSON.stringify(m))
    ws.onMessage(async (raw) => {
      for (const line of String(raw).split('\n').filter(Boolean)) {
        let msg: { id?: string; method?: string; params?: Params }
        try {
          msg = JSON.parse(line)
        } catch {
          continue
        }
        if (!msg.id || !msg.method) {
          continue
        }
        const meta = { runtimeId: 'backend-go' }
        const id = msg.id
        if (msg.method === 'request.subscribe') {
          backend.calls.push({ method: msg.method, params: msg.params ?? {} })
          send({ id, ok: true, result: null, _meta: meta })
          backend.subscribers.add((event) =>
            send({ id, ok: true, result: event, streaming: true, _meta: meta })
          )
          continue
        }
        // A spec may override a boot channel (e.g. project.list) by registering it on the fake.
        if (msg.method in BOOT_CHANNEL_STUBS && !backend.has(msg.method)) {
          send({ id, ok: true, result: BOOT_CHANNEL_STUBS[msg.method], _meta: meta })
          continue
        }
        try {
          const result = await backend.handle(msg.method, msg.params ?? {})
          send({ id, ok: true, result, _meta: meta })
        } catch (e) {
          const code = e instanceof RpcFailure ? e.code : 'internal'
          send({
            id,
            ok: false,
            error: { code, message: (e as Error).message },
            _meta: { runtimeId: null }
          })
        }
      }
    })
  })
}

/** Boots the SPA; returns once the shell is interactive. */
export async function bootRequestApp(page: Page): Promise<void> {
  await page.goto('/web-index.html')
  const settings = page.getByRole('button', { name: 'Settings' }).first()
  // Why: a shared Vite dev server can be mid-HMR when the page loads; one reload recovers it.
  if (!(await settings.isVisible({ timeout: 15_000 }).catch(() => false))) {
    await page.reload()
  }
  await expect(settings).toBeVisible({ timeout: 20_000 })
  // Boot stubs close onboarding, but the first-run feature tip may still sit on top.
  await page
    .getByRole('button', { name: 'Got it' })
    .click({ timeout: 2000 })
    .catch(() => {})
}

/** Opens the Requests page through the sidebar entry, as a user would. */
export async function openRequestsFromSidebar(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Requests', exact: true }).first().click()
  await expect(page.getByRole('tab', { name: 'Requests' })).toBeVisible()
}

/** Same entry point screens use (`openRequestPage()`), driven through the dev-exposed store. */
export async function openRequestPageViaStore(
  page: Page,
  target: { section: 'requests' | 'approvals' | 'backlog'; requestId?: string; focus?: string }
): Promise<void> {
  await expect.poll(() => page.evaluate(() => '__store' in window)).toBe(true)
  await page.evaluate((t) => {
    const store = (
      window as unknown as {
        __store: { getState: () => Record<string, (...a: unknown[]) => void> }
      }
    ).__store
    store.getState().setActiveView('requests')
    store.getState().setRequestPageData(t)
  }, target)
}
