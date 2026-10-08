import { expect, test, type Page } from '@playwright/test'
import { createFakeMcpBackend, mockOrcaApp, mockUser } from './support/mock-orca-ws'
import { devStackUrl } from './support/mcp-dev-backend'

// Mocked-WebSocket spec: always runs unless pointed at a real stack (then @dev-stack specs cover it).
test.skip(
  !!devStackUrl,
  'consent is covered with a mocked backend; dev stack has no seeded request'
)

const REQ_ID = '0b2f6c1e-1111-4222-8333-444455556666'
const BASE = 'http://127.0.0.1:5174'

async function open(page: Page, user: ReturnType<typeof mockUser> | null = mockUser()) {
  const backend = createFakeMcpBackend()
  backend.setRedirectUrl('https://client.example/cb?code=abc&state=xyz')
  backend.addConsent({
    requestId: REQ_ID,
    clientId: 'c1',
    clientName: 'Claude Code',
    redirectHost: 'client.example',
    scopes: [
      { id: 'orca:read', label: 'Read', description: 'Read data', risk: 'read' },
      { id: 'orca:write', label: 'Write', description: 'Change data', risk: 'write_reversible' },
      { id: 'orca:exec', label: 'Run', description: 'Run commands', risk: 'exec' }
    ],
    alreadyGranted: [],
    tenant: { id: 't', name: 'Acme' },
    isNewClient: true,
    registeredViaDcr: true,
    expiresAt: new Date(Date.now() + 600_000).toISOString()
  })
  const ws = await mockOrcaApp(page, { backend, user, baseURL: BASE })
  await page.route('https://client.example/**', (r) =>
    r.fulfill({ contentType: 'text/html', body: '<h1>client callback</h1>' })
  )
  return { backend, ws }
}

test('shows client, redirect host and pre-selected scopes without the full URL', async ({
  page
}) => {
  await open(page)
  await page.goto(`/oauth/consent?request_id=${REQ_ID}`)
  await expect(page.getByText('Authorize Claude Code')).toBeVisible()
  await expect(page.getByText(/client\.example/).first()).toBeVisible()
  await expect(page.getByText(/cb\?code/)).toHaveCount(0)
  await expect(page.getByRole('checkbox', { name: /Read/ })).toHaveAttribute('aria-checked', 'true')
  await expect(page.getByRole('checkbox', { name: /Run commands/ })).toHaveAttribute(
    'aria-checked',
    'false'
  )
  await expect(page.getByRole('note')).toContainText('registered itself')
})

test('approve with a narrowed scope list navigates to the redirect url', async ({ page }) => {
  const { backend } = await open(page)
  await page.goto(`/oauth/consent?request_id=${REQ_ID}`)
  await page.getByRole('checkbox', { name: /Write/ }).click()
  await page.getByRole('button', { name: 'Allow access' }).click()
  await page.waitForURL('https://client.example/cb?code=abc&state=xyz')
  expect(backend.decisions).toEqual([
    { requestId: REQ_ID, decision: 'approve', scopes: ['orca:read'] }
  ])
})

test('deny sends decision=deny and still redirects', async ({ page }) => {
  const { backend } = await open(page)
  await page.goto(`/oauth/consent?request_id=${REQ_ID}`)
  await page.getByRole('button', { name: 'Deny' }).click()
  await page.waitForURL(/client\.example/)
  expect(backend.decisions[0]).toMatchObject({ decision: 'deny', scopes: [] })
})

test('expired request shows an error and no Allow button', async ({ page }) => {
  const { backend } = await open(page)
  // React StrictMode runs the load effect twice in dev, so queue the failure for both calls.
  for (let i = 0; i < 2; i++) {
    backend.failNext('mcp.consent.get', new Error('MCP_CONSENT_EXPIRED: expired'))
  }
  await page.goto(`/oauth/consent?request_id=${REQ_ID}`)
  await expect(page.getByText(/has expired/)).toBeVisible()
  await expect(page.getByRole('button', { name: 'Allow access' })).toHaveCount(0)
})

test('a hostile redirect from the server is never followed', async ({ page }) => {
  const { backend } = await open(page)
  backend.setRedirectUrl('javascript:alert(1)')
  await page.goto(`/oauth/consent?request_id=${REQ_ID}`)
  await page.getByRole('button', { name: 'Allow access' }).click()
  await expect(page.getByText('Something went wrong.')).toBeVisible()
  expect(page.url()).toContain('/oauth/consent')
})

test('signed-out visitors never reach the consent request', async ({ page }) => {
  const { backend } = await open(page, null)
  await page.goto(`/oauth/consent?request_id=${REQ_ID}`)
  // /auth/me is 401, so the login page is rendered and consent.get must never be called.
  await expect(page.locator('input[type="password"]')).toBeVisible()
  await expect(page.getByText('Authorize Claude Code')).toHaveCount(0)
  expect(backend.calls.map((c) => c.method)).not.toContain('mcp.consent.get')
})
