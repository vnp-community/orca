/**
 * A repo saved without a dev server is flagged in the sidebar and re-bound only when the user asks
 * (SOL-FE-E2E-004 / TASK-FE-E2E-019), against the web SPA with a mocked gateway.
 *
 *   npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web \
 *     tests/stages/e2e/legacy/request-web/repo-host-rebind.web.e2e.ts
 */
import { expect, test } from '@playwright/test'
import { devServer, MAC, start, type World } from './support/mock-project-host-gateway'

test.describe('Repo host display and re-binding (mocked gateway)', () => {
  test('g: a repo saved without a host is flagged, and "Choose dev server" re-binds it on request', async ({
    page
  }) => {
    const world: World = {
      servers: [devServer('ds-1', MAC)],
      projects: [{ id: 'proj-old', name: 'My Repos', devServerId: '', createdAt: 1 }],
      repos: [
        {
          id: 'repo-old',
          projectId: 'proj-old',
          url: '/Users/me/codex',
          displayName: 'codex',
          devServerId: '',
          position: 0
        }
      ]
    }
    const backend = await start(page, world)

    const unbound = page.getByTestId('repo-host-unbound')
    await expect(unbound).toBeVisible()
    // Old repos are never migrated on their own.
    expect(backend.callsTo('repo.rebindDevServer')).toEqual([])

    // "+" on an unbound repo asks for the host instead of opening a composer that cannot work.
    await page.locator('[data-repo-header-id]').filter({ hasText: 'codex' }).first().hover()
    await page.getByRole('button', { name: 'Create new worktree for codex', exact: true }).click()
    const picker = page.getByTestId('repo-host-picker')
    await expect(picker).toBeVisible()

    await picker.getByTestId('repo-host-picker-select').click()
    await page.getByRole('option', { name: MAC }).click()
    await picker.getByTestId('repo-host-picker-confirm').click()

    await expect.poll(() => backend.callsTo('repo.rebindDevServer').length).toBe(1)
    expect(backend.callsTo('repo.rebindDevServer')[0]).toEqual({
      repoId: 'repo-old',
      newDevServerId: 'ds-1'
    })
    await expect(picker).toHaveCount(0)
    await expect(unbound).toHaveCount(0)
    await expect(page.getByTestId('repo-host-label').first()).toHaveText(MAC)
  })

  test('h: a re-bind the backend refuses is explained in the dialog and nothing changes', async ({
    page
  }) => {
    const world: World = {
      servers: [devServer('ds-1', MAC)],
      projects: [{ id: 'proj-old', name: 'My Repos', devServerId: '', createdAt: 1 }],
      repos: [
        {
          id: 'repo-old',
          projectId: 'proj-old',
          url: '/Users/me/codex',
          displayName: 'codex',
          devServerId: '',
          position: 0
        }
      ]
    }
    const backend = await start(page, world)
    backend.on('repo.rebindDevServer', () => {
      throw new Error('PROJECT_HAS_ACTIVE_WORKFLOWS: 1 workflow is running')
    })

    await page.getByTestId('repo-host-unbound').click()
    const picker = page.getByTestId('repo-host-picker')
    await picker.getByTestId('repo-host-picker-select').click()
    await page.getByRole('option', { name: MAC }).click()
    await picker.getByTestId('repo-host-picker-confirm').click()

    await expect(picker.getByTestId('repo-host-picker-error')).toContainText(
      'Stop the tasks running on this project'
    )
    await expect(picker).toBeVisible()
    await expect(page.getByTestId('repo-host-unbound')).toBeVisible()
  })

  test('i: a repo with no host of its own shows the project host as inherited', async ({
    page
  }) => {
    const world: World = {
      servers: [devServer('ds-1', MAC)],
      projects: [{ id: 'proj-old', name: 'My Repos', devServerId: 'ds-1', createdAt: 1 }],
      repos: [
        {
          id: 'repo-old',
          projectId: 'proj-old',
          url: '/Users/me/codex',
          displayName: 'codex',
          devServerId: '',
          position: 0
        }
      ]
    }
    await start(page, world)

    const label = page.getByTestId('repo-host-label').first()
    await expect(label).toHaveText(MAC)
    await expect(label).toHaveAttribute('data-host-inherited', 'true')
    await expect(page.getByTestId('repo-host-unbound')).toHaveCount(0)
  })
})
