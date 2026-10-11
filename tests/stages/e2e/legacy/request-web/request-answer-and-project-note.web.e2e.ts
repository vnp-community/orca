/**
 * SOL-FE-E2E-005: the Analysis tab shows the answer a `question` Request got from the AI
 * (`options.answer_markdown`, request-service answer.schema.json), and Create request says which
 * repo the Request will run on. Web SPA + mocked gateway WebSocket.
 *
 *   npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web \
 *     tests/stages/e2e/legacy/request-web/request-answer-and-project-note.web.e2e.ts
 */
import { expect, test } from '@playwright/test'
import {
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  openRequestsFromSidebar,
  requestView
} from './support/mock-request-ws'

const answerSolution = {
  id: 'sol-a',
  requestId: 'req-1',
  kind: 'answer',
  status: 'ready',
  version: 1,
  createdAt: '2026-10-11T09:00:00Z',
  options: {
    schema_version: 1,
    kind: 'answer',
    answer_markdown: 'The cache is cleared on **logout**, see the session service.',
    confidence: 0.82,
    citations: [{ ref: 'src/session/logout.ts:42', excerpt: 'cache.clear()' }],
    limitations: ['Did not read the mobile client']
  }
}

test.describe('Request answer and project repo note (mocked request-service)', () => {
  test('a: the Answer box shows answer_markdown with confidence and citations, not "No data"', async ({
    page
  }) => {
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [requestView({ type: 'question', status: 'answered' })] }),
      'request.get': () => ({ request: requestView({ type: 'question', status: 'answered' }) }),
      'solution.list': () => ({ solutions: [answerSolution], runs: [] })
    })
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-1' })
    await page.getByRole('tab', { name: 'Analysis' }).click()

    const view = page.getByTestId('answer-view')
    await expect(view.getByText('logout', { exact: true })).toBeVisible()
    await expect(view.locator('strong')).toHaveText('logout')
    await expect(view.getByTestId('answer-confidence')).toContainText('82%')
    await expect(view.getByText('src/session/logout.ts:42')).toBeVisible()
    await expect(view.getByText('cache.clear()')).toBeVisible()
    await expect(view.getByText('Did not read the mobile client')).toBeVisible()
    await expect(view.getByText('No data')).toHaveCount(0)
  })

  test('b: Create request names the repo the request runs on, and the first of several', async ({
    page
  }) => {
    const repo = (id: string, url: string, name: string): Record<string, unknown> => ({
      id,
      projectId: 'proj-1',
      url,
      displayName: name,
      position: 0,
      devServerId: ''
    })
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [requestView()] }),
      'project.list': () => [
        { id: 'proj-1', name: 'Orca' },
        { id: 'proj-2', name: 'Empty' }
      ],
      'repo.list': (params) => ({
        repos:
          params.projectId === 'proj-1'
            ? [repo('r1', '/srv/orca', 'orca'), repo('r2', '/srv/docs', 'docs')]
            : []
      })
    })
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestsFromSidebar(page)
    await page.getByRole('button', { name: 'Create request' }).click()
    const dialog = page.getByRole('dialog')

    await dialog.getByRole('combobox', { name: 'Project' }).click()
    await page.getByRole('option', { name: 'Orca' }).click()
    const note = dialog.getByTestId('create-request-repo-note')
    await expect(note).toContainText('repo orca (/srv/orca)')
    await expect(note).toContainText("first of this project's 2 repos")

    await dialog.getByRole('combobox', { name: 'Project' }).click()
    await page.getByRole('option', { name: 'Empty' }).click()
    await expect(dialog.getByTestId('create-request-repo-note')).toContainText('no repo yet')
  })
})
