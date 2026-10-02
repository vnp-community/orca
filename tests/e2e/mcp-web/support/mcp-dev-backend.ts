// Dev-stack mode: set MCP_E2E_BASE_URL (and credentials) to run @dev-stack specs against a real
// backend-go stack (BE-MCP-SOL-015). Without it those specs are skipped, never faked.
import type { APIRequestContext } from '@playwright/test'

export const devStackUrl = process.env.MCP_E2E_BASE_URL ?? null
export const DEV_STACK_SKIP_REASON = 'needs a real MCP dev stack: set MCP_E2E_BASE_URL'

export const devCredentials = {
  admin: {
    email: process.env.MCP_E2E_ADMIN_EMAIL ?? '',
    password: process.env.MCP_E2E_ADMIN_PASSWORD ?? ''
  },
  user: {
    email: process.env.MCP_E2E_USER_EMAIL ?? '',
    password: process.env.MCP_E2E_USER_PASSWORD ?? ''
  }
}

/** POST /auth/local; the session cookie lands in the request context's cookie jar. */
export async function loginViaApi(
  request: APIRequestContext,
  creds: { email: string; password: string }
): Promise<void> {
  const res = await request.post('/auth/local', { data: creds })
  if (!res.ok()) {
    throw new Error(`login failed: ${res.status()}`)
  }
}

/** Calls the MCP HTTP endpoint with a PAT (initialize first, as the protocol requires). */
export async function callMcp(
  request: APIRequestContext,
  pat: string,
  method: string,
  params: unknown
): Promise<{ status: number; body: unknown }> {
  const headers = {
    Authorization: `Bearer ${pat}`,
    Accept: 'application/json, text/event-stream',
    'Content-Type': 'application/json'
  }
  const init = await request.post('/mcp', {
    headers,
    data: {
      jsonrpc: '2.0',
      id: 0,
      method: 'initialize',
      params: {
        protocolVersion: '2025-06-18',
        capabilities: {},
        clientInfo: { name: 'e2e', version: '0' }
      }
    }
  })
  if (init.status() !== 200) {
    return { status: init.status(), body: null }
  }
  const sessionId = init.headers()['mcp-session-id']
  const res = await request.post('/mcp', {
    headers: { ...headers, ...(sessionId ? { 'Mcp-Session-Id': sessionId } : {}) },
    data: { jsonrpc: '2.0', id: 1, method, params }
  })
  return { status: res.status(), body: await res.text() }
}
