/**
 * Request steps shared by the deepseek-harness specs: Create request in the dialog (project choice, the note naming the repo and
 * host, what the SPA sends), and the Requests page shell (list, j/k, tabs, pending count, Escape). All through the UI; the gateway
 * is read-only evidence.
 */
import { expect, type Locator, type Page } from '@playwright/test'
import type { DeepseekRun } from './deepseek-flow-harness'
import { DEEPSEEK_REPO } from './deepseek-repo-fixture'
import { callsTo } from './live-add-project-own-project-steps'
import type { LiveFlow } from './live-flow-harness'
import { requestState } from './live-gateway-crosschecks'
import { openRequestsPage } from './live-request-ui-steps'

/** Create request for the run's project: the note names the repo and its host, and the SPA sends a manual request with no source. */
export async function createDeepseekRequest(
  f: LiveFlow,
  run: DeepseekRun,
  title: string,
  body: string
): Promise<void> {
  const { page } = f
  const from = run.tap.calls.length
  await page.getByRole('button', { name: 'Create request' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Title').fill(title)
  await dialog.getByLabel('Description').fill(body)
  await dialog.getByRole('combobox', { name: /project/i }).click()
  await page.getByRole('option', { name: f.project.name, exact: true }).click()
  const note = dialog.getByTestId('create-request-repo-note')
  await expect(note, 'the note names the repo, its path and the host').toContainText(
    // Why: Add Project names the repo after the project, so renaming the project in the dialog renames the repo label too.
    `repo ${f.project.name} (${DEEPSEEK_REPO}) on ${f.cfg.devServerName}`,
    { timeout: 15_000 }
  )
  await f.shots.take('create-request-dialog')
  await dialog.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(dialog).toBeHidden()
  const sent = callsTo(run.tap, 'request.create', from)
  expect(sent, 'the SPA sent one request.create').toHaveLength(1)
  expect(sent[0].params, 'request.create carries the project, title and body').toMatchObject({
    projectId: f.project.id,
    title,
    body
  })
  expect(sent[0].params, 'a manual request sends no source').not.toHaveProperty('source')
}

const listbox = (page: Page): Locator => page.getByRole('listbox', { name: 'Requests' })

async function selectedOption(page: Page): Promise<string> {
  const selected = listbox(page).locator('[role="option"][aria-selected="true"]')
  await expect(selected, 'exactly one row is selected').toHaveCount(1)
  // Why: a row ends with its age ("18 seconds ago"), which changes between two reads of the same row.
  return (await selected.innerText())
    .replace(/\s+/g, ' ')
    .replace(/\s*\d+ (second|minute|hour)s? ago\s*$/, '')
    .trim()
}

/** The Requests page shell on real data: the rows, j/k, the pending count, Approvals/Backlog tabs, Escape closes the page. */
export async function expectRequestsShell(f: LiveFlow, titles: string[]): Promise<void> {
  const { page } = f
  await f.step('the Requests page: list, j/k, pending count, tabs, Escape', async () => {
    await openRequestsPage(page)
    const list = listbox(page)
    for (const title of titles) {
      await expect(
        list.getByRole('option', {
          name: new RegExp(title.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
        }),
        `the list has "${title}"`
      ).toHaveCount(1)
    }
    await list.focus()
    await page.keyboard.press('j')
    const first = await selectedOption(page)
    await page.keyboard.press('j')
    const second = await selectedOption(page)
    expect(second, 'j moves the selection to another row').not.toBe(first)
    await page.keyboard.press('k')
    expect(await selectedOption(page), 'k moves it back').toBe(first)
    await expect(
      page.getByLabel(/\d+ pending approvals?/).first(),
      'the sidebar shows the pending approvals'
    ).toBeVisible({ timeout: 45_000 })
    for (const tab of [/Approvals/, 'Backlog', 'Requests'] as const) {
      await page.getByRole('tab', { name: tab }).click()
      await expect(page.getByRole('tab', { name: tab })).toHaveAttribute('aria-selected', 'true')
    }
    await f.shots.take('requests-shell')
  })
}

/** Escape closes the Requests page and returns to the app; the request stays what it was. */
export async function expectEscapeClosesRequests(f: LiveFlow, requestId: string): Promise<void> {
  await f.step('Escape closes the Requests page', async () => {
    const before = (await requestState(f.rpc, requestId)).status
    await f.page
      .locator('body')
      .click({ position: { x: 5, y: 5 } })
      .catch(() => {})
    await f.page.keyboard.press('Escape')
    await expect(f.page.getByRole('heading', { level: 1, name: 'Requests' })).toHaveCount(0)
    expect((await requestState(f.rpc, requestId)).status, 'closing the page changes nothing').toBe(
      before
    )
  })
}
