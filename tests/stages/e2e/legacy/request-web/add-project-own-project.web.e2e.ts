/**
 * Add Project creates a PROJECT of its own for every repo and refuses a folder that is already
 * registered (SOL-FE-E2E-005 / TASK-FE-E2E-023, 025), against the web SPA with a mocked gateway.
 * The gateway payloads are the contract: project.create {name, devServerId, repoPath, visibility}.
 *
 *   npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web \
 *     tests/stages/e2e/legacy/request-web/add-project-own-project.web.e2e.ts
 */
import { expect, test, type Locator, type Page } from '@playwright/test'
import { RpcFailure } from './support/mock-request-ws'
import {
  devServer,
  MAC,
  openAddProject,
  start,
  type World
} from './support/mock-project-host-gateway'

async function goToHostPath(page: Page, path: string): Promise<Locator> {
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('button', { name: /Browse folder/ }).click()
  await expect(page.getByText('Browse host filesystem')).toBeVisible()
  const filter = page.getByPlaceholder(/Type to filter or enter a path/)
  await filter.fill(`${path}/`)
  await expect(page.getByText('.git', { exact: true })).toBeVisible()
  await filter.press('Enter')
  await page.getByRole('button', { name: 'Select folder' }).click()
  await expect(page.getByPlaceholder('/home/user/project')).toHaveValue(path)
  return dialog
}

test.describe('Add Project: one project per repo (mocked gateway)', () => {
  test('a: the project name defaults to the folder name and can be edited', async ({ page }) => {
    const world: World = { servers: [devServer('ds-1', MAC)], projects: [], repos: [] }
    const backend = await start(page, world)
    await openAddProject(page)
    const dialog = await goToHostPath(page, '/srv')

    const name = dialog.getByTestId('add-project-name')
    await expect(name).toHaveValue('srv')
    await name.fill('Billing service')
    await dialog.getByRole('button', { name: 'Add Git Project' }).click()

    await expect.poll(() => backend.callsTo('project.create').length).toBe(1)
    expect(backend.callsTo('project.create')[0]).toEqual({
      name: 'Billing service',
      devServerId: 'ds-1',
      repoPath: '/srv',
      visibility: 'private'
    })
    // The repo came with project.create: no second write, and no reuse of a host-wide project.
    expect(backend.callsTo('repo.add')).toEqual([])
  })

  test('b: a second folder gets its own project even though a project exists on that host', async ({
    page
  }) => {
    const world: World = {
      servers: [devServer('ds-1', MAC)],
      projects: [{ id: 'proj-demo', name: 'Demo (this Mac)', devServerId: 'ds-1', createdAt: 1 }],
      repos: [
        {
          id: 'repo-demo',
          projectId: 'proj-demo',
          url: '/srv/other',
          displayName: 'other',
          devServerId: 'ds-1',
          position: 0
        }
      ]
    }
    const backend = await start(page, world)
    await openAddProject(page)
    const dialog = await goToHostPath(page, '/srv')
    await dialog.getByRole('button', { name: 'Add Git Project' }).click()

    await expect.poll(() => backend.callsTo('project.create').length).toBe(1)
    expect(backend.callsTo('project.create')[0]).toMatchObject({ name: 'srv', repoPath: '/srv' })
    expect(world.projects).toHaveLength(2)
    // Nothing was added into the existing project.
    expect(world.repos.filter((repo) => repo.projectId === 'proj-demo')).toHaveLength(1)
  })
})

test.describe('Add Project: duplicate repo is blocked (mocked gateway)', () => {
  const registered = (): World => ({
    servers: [devServer('ds-1', MAC)],
    projects: [{ id: 'proj-demo', name: 'Demo (this Mac)', devServerId: 'ds-1', createdAt: 1 }],
    repos: [
      {
        id: 'repo-demo',
        projectId: 'proj-demo',
        url: '/srv',
        displayName: 'srv',
        devServerId: 'ds-1',
        position: 0
      }
    ]
  })

  test('c: the same host and path is refused with the project that holds it', async ({ page }) => {
    const backend = await start(page, registered())
    await openAddProject(page)
    const dialog = await goToHostPath(page, '/srv')
    await dialog.getByRole('button', { name: 'Add Git Project' }).click()

    const notice = dialog.getByTestId('add-project-duplicate')
    await expect(notice).toContainText('Demo (this Mac)')
    await expect(dialog.getByTestId('add-project-open-existing')).toBeVisible()
    expect(backend.callsTo('project.create')).toEqual([])
    expect(backend.callsTo('repo.add')).toEqual([])

    // Editing the path is a new question: the notice goes away.
    await page.getByPlaceholder('/home/user/project').fill('/srv/another')
    await expect(dialog.getByTestId('add-project-duplicate')).toHaveCount(0)
  })

  test('d: "Open that project" closes the dialog without adding anything', async ({ page }) => {
    const backend = await start(page, registered())
    await openAddProject(page)
    const dialog = await goToHostPath(page, '/srv')
    await dialog.getByRole('button', { name: 'Add Git Project' }).click()
    await dialog.getByTestId('add-project-open-existing').click()

    await expect(page.getByRole('dialog')).toHaveCount(0)
    expect(backend.callsTo('project.create')).toEqual([])
  })

  test('f: the same folder typed with a trailing slash is the same folder', async ({ page }) => {
    const backend = await start(page, registered())
    await openAddProject(page)
    const dialog = await goToHostPath(page, '/srv')
    await page.getByPlaceholder('/home/user/project').fill('/srv/')
    await dialog.getByRole('button', { name: 'Add Git Project' }).click()

    await expect(dialog.getByTestId('add-project-duplicate')).toContainText('Demo (this Mac)')
    expect(backend.callsTo('project.create')).toEqual([])
    expect(backend.callsTo('repo.add')).toEqual([])
  })

  test('e: a backend duplicate (another tab won the race) becomes the same notice', async ({
    page
  }) => {
    const world: World = { servers: [devServer('ds-1', MAC)], projects: [], repos: [] }
    const backend = await start(page, world)
    backend.on('project.create', () => {
      throw new RpcFailure(
        'internal',
        'PROJECT_REPO_ALREADY_REGISTERED: repo /srv on dev server ds-1 is already registered in project "Created elsewhere" (p-9)'
      )
    })
    await openAddProject(page)
    const dialog = await goToHostPath(page, '/srv')
    await dialog.getByRole('button', { name: 'Add Git Project' }).click()

    await expect(page.getByText('Project already added').first()).toBeVisible()
    await expect(page.getByText(/Created elsewhere/).first()).toBeVisible()
    expect(backend.callsTo('repo.add')).toEqual([])
  })
})
