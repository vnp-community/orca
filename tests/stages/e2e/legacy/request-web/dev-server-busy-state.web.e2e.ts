/**
 * E2E for the dev server busy badge (TASK-FE-E2E-012 F4) against the web SPA with a mocked gateway:
 * `devServer.list` gives the first state, `devServer.subscribeStatus` pushes snapshots that overwrite it.
 *
 *   npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web \
 *     tests/stages/e2e/legacy/request-web/dev-server-busy-state.web.e2e.ts
 */
import { expect, test, type Page } from '@playwright/test'
import {
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  type FakeRequestBackend
} from './support/mock-request-ws'

type Live = { connected: boolean; busy: boolean; activeExecutions: number }

function backendWith(live: Live): FakeRequestBackend {
  const server = () => ({
    id: 'ds-1',
    name: 'Mac Studio',
    connectionType: 'direct-websocket',
    status: live.connected ? 'connected' : 'disconnected',
    platform: 'darwin',
    arch: 'arm64',
    nodeVersion: 'v22.1.0',
    lastConnectedAt: 1,
    lastError: null,
    workspaceDir: null,
    addedAt: 1,
    capabilities: [],
    approvalStatus: 'approved',
    healthStatus: 'healthy',
    activeExecutions: live.connected ? live.activeExecutions : 0,
    busy: live.connected && live.busy
  })
  return createFakeRequestBackend({ 'devServer.list': () => [server()] })
}

async function openDevServers(page: Page, backend: FakeRequestBackend): Promise<void> {
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  await expect.poll(() => page.evaluate(() => '__store' in window)).toBe(true)
  await page.evaluate(() => {
    const store = (
      window as unknown as {
        __store: { getState: () => Record<string, (...a: unknown[]) => void> }
      }
    ).__store
    store.getState().openSettingsTarget({ pane: 'servers', repoId: null })
    store.getState().openSettingsPage()
  })
  await page.getByRole('button', { name: 'Dev Servers', exact: true }).click()
}

const card = (page: Page) =>
  page.locator('[data-testid="dev-server-card"][data-dev-server-id="ds-1"]')

test.describe('Dev server busy state (mocked gateway)', () => {
  test('a: idle at first, busy while a run is going, idle again, and gone when the agent disconnects', async ({
    page
  }) => {
    const live: Live = { connected: true, busy: false, activeExecutions: 0 }
    const backend = backendWith(live)
    await openDevServers(page, backend)

    await expect(card(page)).toBeVisible()
    await expect(card(page).getByTestId('dev-server-platform')).toHaveText('macOS · arm64')
    await expect(card(page).getByTestId('dev-server-busy')).toHaveText('Idle')
    await expect.poll(() => backend.devServerStatus.openCount()).toBe(1)

    Object.assign(live, { busy: true, activeExecutions: 2 })
    backend.devServerStatus.push({
      devServerId: 'ds-1',
      connected: true,
      activeExecutions: 2,
      busy: true
    })
    await expect(card(page).getByTestId('dev-server-busy')).toHaveText('Busy (2 running)')
    await expect(card(page).getByTestId('dev-server-busy')).toHaveAttribute('data-busy', 'true')
    // The 3 s status poll refreshes the other fields but must not undo what the push said.
    await page.waitForTimeout(3_500)
    await expect(card(page).getByTestId('dev-server-busy')).toHaveText('Busy (2 running)')

    Object.assign(live, { busy: false, activeExecutions: 0 })
    backend.devServerStatus.push({
      devServerId: 'ds-1',
      connected: true,
      activeExecutions: 0,
      busy: false
    })
    await expect(card(page).getByTestId('dev-server-busy')).toHaveText('Idle')

    live.connected = false
    backend.devServerStatus.push({
      devServerId: 'ds-1',
      connected: false,
      activeExecutions: 0,
      busy: false
    })
    await expect(card(page).getByTestId('dev-server-busy')).toHaveCount(0)
  })

  test('b: a stream the server closes is opened again and the next snapshot shows', async ({
    page
  }) => {
    const backend = backendWith({ connected: true, busy: false, activeExecutions: 0 })
    await openDevServers(page, backend)
    await expect.poll(() => backend.devServerStatus.openCount()).toBe(1)
    backend.devServerStatus.end()
    await expect
      .poll(() => backend.callsTo('devServer.subscribeStatus').length, { timeout: 8_000 })
      .toBe(2)
    backend.devServerStatus.push({
      devServerId: 'ds-1',
      connected: true,
      activeExecutions: 1,
      busy: true
    })
    await expect(card(page).getByTestId('dev-server-busy')).toHaveText('Busy (1 running)')
  })

  test('c: an older gateway (no devServer.subscribeStatus, no busy field) shows the card without a busy badge', async ({
    page
  }) => {
    const backend = createFakeRequestBackend({
      'devServer.list': () => [
        {
          id: 'ds-1',
          name: 'Old',
          connectionType: 'direct-websocket',
          status: 'connected',
          platform: null,
          arch: null,
          nodeVersion: null,
          lastConnectedAt: 1,
          lastError: null,
          workspaceDir: null,
          addedAt: 1,
          capabilities: [],
          approvalStatus: 'approved'
        }
      ]
    })
    await openDevServers(page, backend)
    await expect(card(page)).toBeVisible()
    await expect(card(page).getByTestId('dev-server-busy')).toHaveCount(0)
    await expect(card(page).getByTestId('dev-server-platform')).toHaveCount(0)
  })
})
