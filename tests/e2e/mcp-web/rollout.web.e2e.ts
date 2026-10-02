import { expect, test, type Page } from '@playwright/test'
import { createFakeMcpBackend, mockOrcaApp, mockUser } from './support/mock-orca-ws'
import { SPA_URL, bootApp } from './support/mcp-app-navigation'
import { devStackUrl } from './support/mcp-dev-backend'

test.skip(
  !!devStackUrl,
  'mocked variants only; two-stack rollout checks need BE-MCP-SOL-015 stacks'
)

const errorsOn = (page: Page): string[] => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  return errors
}

test('MCP disabled: no MCP entry in Settings and no MCP-related page error', async ({ page }) => {
  const backend = createFakeMcpBackend()
  backend.setInfo({ enabled: false })
  await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL })
  const errors = errorsOn(page)
  await page.goto('/web-index.html')
  await page.getByRole('button', { name: 'Settings' }).first().click()
  await expect(page.getByRole('button', { name: 'Agents' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'MCP', exact: true })).toHaveCount(0)
  expect(backend.streamCount()).toBe(0)
  expect(errors.filter((m) => /mcp/i.test(m))).toEqual([])
})

test('kill switch banner appears live without reloading and clears again', async ({ page }) => {
  const backend = createFakeMcpBackend({ role: 'user' })
  await mockOrcaApp(page, { backend, user: mockUser(), baseURL: SPA_URL })
  await bootApp(page, backend)
  await page.getByRole('button', { name: 'Settings' }).first().click()
  await page.getByRole('button', { name: 'MCP', exact: true }).click()
  await expect(page.getByText(/paused by an administrator/)).toHaveCount(0)
  backend.setKillSwitch(true, 'incident 42')
  await expect(page.getByText(/paused by an administrator/)).toBeVisible()
  await expect(page.getByText(/incident 42/)).toBeVisible()
  backend.setKillSwitch(false)
  await expect(page.getByText(/paused by an administrator/)).toHaveCount(0)
})

test('tenant turned off: admins get the enable card, regular users get no MCP entry', async ({
  page
}) => {
  const backend = createFakeMcpBackend({ role: 'admin' })
  backend.setInfo({ tenantEnabled: false })
  await mockOrcaApp(page, { backend, user: mockUser('admin'), baseURL: SPA_URL })
  await page.goto('/web-index.html')
  await page.getByRole('button', { name: 'Settings' }).first().click()
  await page.getByRole('button', { name: 'MCP', exact: true }).click()
  await expect(page.getByText('MCP is turned off for your organization')).toBeVisible()
})
