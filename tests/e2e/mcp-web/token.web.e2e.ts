import { expect, test } from '@playwright/test'
import { createFakeMcpBackend, mockOrcaApp, mockUser } from './support/mock-orca-ws'
import { SPA_URL, bootApp, openMcpTab } from './support/mcp-app-navigation'
import {
  callMcp,
  devStackUrl,
  loginViaApi,
  devCredentials,
  DEV_STACK_SKIP_REASON
} from './support/mcp-dev-backend'

test.describe('PAT lifecycle (mocked WebSocket backend)', () => {
  test.skip(!!devStackUrl, 'mocked variant; the @dev-stack variant below runs instead')

  test('secret is shown once, never persisted, and revoke stops the PAT', async ({ page }) => {
    const backend = createFakeMcpBackend({ role: 'user' })
    await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL })
    await bootApp(page, backend)
    await openMcpTab(page, 'Access tokens')

    await page.getByRole('button', { name: 'Create token' }).first().click()
    await page.getByLabel('Name').fill('ci')
    await page.getByRole('button', { name: 'Create token' }).last().click()
    await expect(page.getByText('Copy your token now')).toBeVisible()

    const [secret] = backend.allSecrets()
    expect(secret).toMatch(/^orca_pat_/)
    await page.getByRole('button', { name: 'Show' }).click()
    await expect(page.getByTestId('mcp-token-secret')).toHaveText(secret)
    expect(backend.patAccepted(secret)).toBe(true)

    await page.getByRole('checkbox', { name: 'I have saved this token' }).click()
    await page.getByRole('button', { name: 'Done' }).click()
    await expect(page.getByText('Copy your token now')).toHaveCount(0)
    await expect(page.getByTestId('mcp-token-secret')).toHaveCount(0)
    expect(await page.content()).not.toContain(secret)

    // Not in any browser storage, and gone after a reload; only the metadata row comes back.
    const storage = await page.evaluate(() =>
      JSON.stringify([{ ...localStorage }, { ...sessionStorage }])
    )
    expect(storage).not.toContain(secret)
    await page.reload()
    await openMcpTab(page, 'Access tokens')
    await expect(page.getByRole('cell', { name: 'ci', exact: true })).toBeVisible()
    expect(await page.content()).not.toContain(secret)

    await page.getByRole('button', { name: 'Revoke token ci' }).click()
    await page.getByRole('button', { name: 'Revoke token', exact: true }).click()
    await expect(page.getByText('Revoked', { exact: true })).toBeVisible()
    expect(backend.patAccepted(secret)).toBe(false)
  })

  test('a lifetime above the cap is rejected with the cap shown', async ({ page }) => {
    const backend = createFakeMcpBackend({ role: 'user' })
    await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL })
    await bootApp(page, backend)
    await openMcpTab(page, 'Access tokens')
    await page.getByRole('button', { name: 'Create token' }).first().click()
    // The cap drops server-side after the page loaded server.info.
    backend.setInfo({ maxTokenDays: 7 })
    await page.getByLabel('Name').fill('ci')
    await page.getByRole('button', { name: 'Create token' }).last().click()
    await expect(page.getByText(/Maximum lifetime is \d+ days\./)).toBeVisible()
    expect(backend.allSecrets()).toHaveLength(0)
  })
})

test.describe('@dev-stack PAT lifecycle', () => {
  test.skip(!devStackUrl, DEV_STACK_SKIP_REASON)

  test('create -> use over /mcp -> revoke -> 401', async ({ page }) => {
    await loginViaApi(page.request, devCredentials.user)
    await page.goto('/')
    await openMcpTab(page, 'Access tokens')
    await page.getByRole('button', { name: 'Create token' }).first().click()
    await page.getByLabel('Name').fill(`e2e-${Date.now()}`)
    await page.getByRole('button', { name: 'Create token' }).last().click()
    await page.getByRole('button', { name: 'Show' }).click()
    const secret = (await page.getByTestId('mcp-token-secret').textContent()) ?? ''
    expect(secret.length).toBeGreaterThan(10)
    expect((await callMcp(page.request, secret, 'tools/list', {})).status).toBe(200)
    await page.getByRole('checkbox', { name: 'I have saved this token' }).click()
    await page.getByRole('button', { name: 'Done' }).click()
    await page.getByRole('button', { name: /^Revoke token e2e-/ }).click()
    await page.getByRole('button', { name: 'Revoke token', exact: true }).click()
    await expect
      .poll(async () => (await callMcp(page.request, secret, 'tools/list', {})).status, {
        timeout: 90_000
      })
      .toBe(401)
  })
})
