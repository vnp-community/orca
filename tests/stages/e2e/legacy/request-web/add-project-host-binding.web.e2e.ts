/**
 * Add Project binds the project to the dev server chosen in the dialog (SOL-FE-E2E-004 / TASK-FE-E2E-021),
 * against the web SPA with a mocked gateway. The payloads sent to the gateway are the contract:
 * repo.add (and repo.clone / repo.create) must carry devServerId, and the dialog must never fall back to a
 * host-less "local" add.
 *
 *   npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web \
 *     tests/stages/e2e/legacy/request-web/add-project-host-binding.web.e2e.ts
 */
import { expect, test } from '@playwright/test'
import {
  BOX,
  devServer,
  MAC,
  openAddProject,
  start,
  type World
} from './support/mock-project-host-gateway'

test.describe('Add Project host binding (mocked gateway)', () => {
  test('a: one connected host is chosen for the user and repo.add carries its devServerId', async ({
    page
  }) => {
    const world: World = { servers: [devServer('ds-1', MAC)], projects: [], repos: [] }
    const backend = await start(page, world)
    await openAddProject(page)

    const dialog = page.getByRole('dialog')
    await expect(dialog.getByRole('combobox')).toContainText(MAC)
    await expect(dialog.getByTestId('add-repo-host-gate')).toHaveCount(0)

    await dialog.getByRole('button', { name: /Browse folder/ }).click()
    await expect(page.getByText('Browse host filesystem')).toBeVisible()
    const filter = page.getByPlaceholder(/Type to filter or enter a path/)
    await filter.fill('/srv/')
    await expect(page.getByText('.git', { exact: true })).toBeVisible()
    await filter.press('Enter')
    await page.getByRole('button', { name: 'Select folder' }).click()
    await expect(page.getByPlaceholder('/home/user/project')).toHaveValue('/srv')
    await page.getByRole('button', { name: 'Add Git Project' }).click()

    await expect.poll(() => backend.callsTo('repo.add').length).toBe(1)
    expect(backend.callsTo('repo.add')[0]).toMatchObject({
      url: '/srv',
      devServerId: 'ds-1'
    })
    // The project holding the repo is bound to the same host (named after it).
    expect(backend.callsTo('project.create')[0]).toMatchObject({
      name: MAC,
      devServerId: 'ds-1',
      visibility: 'private'
    })
    // The nested-repo scan targets the dialog's host, not a global selection.
    expect(backend.callsTo('projectGroup.scanNested')[0]).toMatchObject({ devServerId: 'ds-1' })
  })

  test('b: two connected hosts: nothing is preselected, Browse waits for a choice', async ({
    page
  }) => {
    const world: World = {
      servers: [devServer('ds-1', MAC), devServer('ds-2', BOX)],
      projects: [],
      repos: []
    }
    const backend = await start(page, world)
    await openAddProject(page)

    const dialog = page.getByRole('dialog')
    await expect(dialog.getByRole('combobox')).toContainText('Choose a host')
    await expect(dialog.getByTestId('add-repo-host-gate')).toHaveAttribute(
      'data-gate-status',
      'choose-host'
    )
    await expect(dialog.getByRole('button', { name: /Browse folder/ })).toBeDisabled()

    await dialog.getByRole('combobox').click()
    await page.getByRole('option', { name: new RegExp(BOX) }).click()
    await expect(dialog.getByRole('combobox')).toContainText(BOX)
    await expect(dialog.getByTestId('add-repo-host-gate')).toHaveCount(0)
    await expect(dialog.getByRole('button', { name: /Browse folder/ })).toBeEnabled()
    expect(backend.callsTo('repo.add')).toEqual([])
  })

  test('c: no connected host: blocked with a way to Dev Servers, never a local add', async ({
    page
  }) => {
    const world: World = {
      servers: [devServer('ds-1', MAC, 'disconnected')],
      projects: [],
      repos: []
    }
    const backend = await start(page, world)
    await openAddProject(page)

    const dialog = page.getByRole('dialog')
    const gate = dialog.getByTestId('add-repo-host-gate')
    await expect(gate).toHaveAttribute('data-gate-status', 'no-host')
    await expect(gate).toContainText('No dev server is connected')
    await expect(dialog.getByRole('button', { name: /Browse folder/ })).toBeDisabled()

    await gate.getByRole('button', { name: 'Open Dev Servers' }).click()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(page.getByTestId('dev-server-card')).toBeVisible()
    expect(backend.callsTo('repo.add')).toEqual([])
  })

  test('d: the chosen host drops while browsing: the picker stays open and says so', async ({
    page
  }) => {
    const world: World = { servers: [devServer('ds-1', MAC)], projects: [], repos: [] }
    const backend = await start(page, world)
    await openAddProject(page)
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: /Browse folder/ }).click()
    await expect(page.getByText('Browse host filesystem')).toBeVisible()
    // The status stream must be open before a snapshot can reach the store.
    await expect.poll(() => backend.devServerStatus.openCount()).toBe(1)

    backend.devServerStatus.push({
      devServerId: 'ds-1',
      connected: false,
      activeExecutions: 0,
      busy: false
    })

    await expect(page.getByTestId('remote-file-browser-host-offline')).toBeVisible()
    // Still on the Browse step, still on the same host (not silently switched to another one).
    await expect(page.getByText('Browse host filesystem')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Select folder' })).toBeVisible()
    expect(backend.callsTo('repo.add')).toEqual([])
  })

  test('e: clone and create register the repo on the chosen host', async ({ page }) => {
    const world: World = { servers: [devServer('ds-1', MAC)], projects: [], repos: [] }
    const backend = await start(page, world)
    await openAddProject(page)
    const dialog = page.getByRole('dialog')

    await dialog.getByRole('button', { name: /Clone from URL/ }).click()
    // The labels are not bound to their inputs, so address the fields by placeholder/position.
    await dialog
      .getByPlaceholder(/github\.com\/user\/repo/)
      .fill('https://github.com/stablyai/orca.git')
    await dialog.locator('input').nth(1).fill('/srv/orca')
    await dialog.getByRole('button', { name: 'Clone', exact: true }).click()

    await expect.poll(() => backend.callsTo('repo.add').length).toBe(1)
    expect(backend.callsTo('repo.clone')[0]).toEqual({
      devServerId: 'ds-1',
      url: 'https://github.com/stablyai/orca.git',
      destPath: '/srv/orca'
    })
    expect(backend.callsTo('repo.add')[0]).toMatchObject({ url: '/srv/orca', devServerId: 'ds-1' })
  })

  test('f: create on the host registers the new repo with the same devServerId', async ({
    page
  }) => {
    const world: World = { servers: [devServer('ds-1', MAC)], projects: [], repos: [] }
    const backend = await start(page, world)
    await openAddProject(page)
    const dialog = page.getByRole('dialog')

    await dialog.getByRole('button', { name: /Create new project/ }).click()
    await dialog.getByPlaceholder(/./).first().fill('fresh')
    await dialog.locator('input').nth(1).fill('/srv')
    await dialog.getByRole('button', { name: 'Create project' }).click()

    await expect.poll(() => backend.callsTo('repo.add').length).toBe(1)
    expect(backend.callsTo('repo.create')[0]).toEqual({
      devServerId: 'ds-1',
      destPath: '/srv/fresh',
      defaultBranch: ''
    })
    expect(backend.callsTo('repo.add')[0]).toMatchObject({ url: '/srv/fresh', devServerId: 'ds-1' })
  })
})
