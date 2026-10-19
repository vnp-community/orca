/**
 * Steps of the starting-branch spec (CR-REQ-040): the "Start from" field of the Create request dialog and the Overview row, driven
 * with roles and the data-testids the components ship (start-from-field, request-base-ref-row, base-ref-edit, base-ref-lock).
 * Texts are the en locale. The gateway client only reads; every change of the branch goes through the screen.
 */
import { expect, type Locator, type Page } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import { findRequestByTitle, requestState } from './live-gateway-crosschecks'
import { detailHeader, detailPane } from './live-request-ui-steps'

export const startFromField = (page: Page): Locator =>
  page.getByRole('dialog', { name: 'Create request' }).getByTestId('start-from-field')
export const startFromTrigger = (page: Page): Locator =>
  startFromField(page).getByRole('button', { name: /^Starting branch:/ })
export const baseRefRow = (page: Page): Locator =>
  detailPane(page).getByTestId('request-base-ref-row')

/** Opens the dialog and fills everything except the starting branch. The field shows up once the project's repo is known. */
export async function openCreateDialog(
  page: Page,
  input: { title: string; body: string; projectName: string }
): Promise<Locator> {
  await page.getByRole('button', { name: 'Create request' }).click()
  const dialog = page.getByRole('dialog', { name: 'Create request' })
  await dialog.getByLabel('Title').fill(input.title)
  await dialog.getByLabel('Description').fill(input.body)
  await dialog.getByRole('combobox', { name: /project/i }).click()
  await page.getByRole('option', { name: input.projectName, exact: true }).click()
  return dialog
}

/** Picks a branch from the search list of the field's popover (two characters are enough for the host to answer). */
export async function chooseStartingBranch(page: Page, ref: string): Promise<void> {
  await startFromTrigger(page).click()
  await page.getByRole('combobox', { name: 'Search branches' }).fill(ref.slice(0, 3))
  await page.getByRole('option', { name: ref, exact: true }).click()
  await expect(startFromTrigger(page)).toContainText(ref)
}

/** Types a branch name by hand: the way to name one the search does not list. */
export async function typeStartingBranch(page: Page, ref: string): Promise<void> {
  await startFromTrigger(page).click()
  await page.getByRole('button', { name: 'Type a branch name instead' }).click()
  await page.getByLabel('Branch or tag name').fill(ref)
  await page.getByRole('button', { name: 'Use this branch' }).click()
  await expect(startFromTrigger(page)).toContainText(ref)
}

/** Clicks Create and returns the new Request once the service lists it and the detail pane shows it. */
export async function submitCreateDialog(f: LiveFlow, title: string): Promise<string> {
  const dialog = f.page.getByRole('dialog', { name: 'Create request' })
  await dialog.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(dialog).toBeHidden({ timeout: 30_000 })
  await expect(detailHeader(f.page)).toContainText(title)
  await expect
    .poll(async () => (await findRequestByTitle(f.rpc, f.project.id, title))?.id ?? '', {
      timeout: 15_000
    })
    .not.toBe('')
  return (await findRequestByTitle(f.rpc, f.project.id, title))!.id
}

/** The branch the service stores for the Request ('' = follow the repository default). */
export async function storedBaseRef(f: LiveFlow, requestId: string): Promise<string | undefined> {
  return (await requestState(f.rpc, requestId)).baseRef as string | undefined
}

/**
 * Changes the branch with the pen on the Overview row: a ref from the search, or the "Default branch" row when `ref` is null.
 * Waits for the service to hold the new value, then for the row to say it.
 */
export async function changeBranchOnOverview(
  f: LiveFlow,
  requestId: string,
  ref: string | null
): Promise<void> {
  const { page } = f
  await baseRefRow(page).getByTestId('base-ref-edit').click()
  if (ref === null) {
    await page.getByRole('option', { name: /^Default branch/ }).click()
  } else {
    await page.getByRole('combobox', { name: 'Search branches' }).fill(ref.slice(0, 3))
    await page.getByRole('option', { name: ref, exact: true }).click()
  }
  await expect
    .poll(() => storedBaseRef(f, requestId), { timeout: 20_000, message: 'the service stores it' })
    .toBe(ref ?? '')
  await expect(baseRefRow(page)).toContainText(
    ref === null ? 'Worktrees start from the default branch' : `Worktrees start from ${ref}`
  )
}

/** The lock the row shows once nothing may change the branch any more, with its tooltip. */
export async function expectRowLocked(page: Page, reasonText: RegExp): Promise<void> {
  const row = baseRefRow(page)
  const lock = row.getByTestId('base-ref-lock')
  await expect(lock).toBeVisible({ timeout: 30_000 })
  await expect(lock).toHaveText(/Locked/)
  await expect(row.getByTestId('base-ref-edit'), 'no pen on a locked row').toHaveCount(0)
  await lock.hover()
  await expect(page.getByRole('tooltip').first()).toContainText(reasonText)
}
