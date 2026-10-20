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

/** The name of the repository default branch of the LedgerLite fixture: no remote, so the conventional local `main`. */
export const LEDGERLITE_DEFAULT_BRANCH = 'main'

/** The branch list popover. A closed popover also has role dialog, so the list is found by its own name. */
export const branchList = (page: Page): Locator => page.getByRole('listbox', { name: 'Branches' })

/** The unchosen button names the real default branch (as repo.baseRefDefault says) with a "default" tag, not a generic label. */
export async function expectTriggerShowsDefault(page: Page, name: string): Promise<void> {
  const trigger = startFromTrigger(page)
  // The name is cut in two spans for middle truncation, so the text reads "main" + "default" with no space between.
  await expect(trigger).toHaveText(
    new RegExp(`^${name.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&')}\\s*default$`)
  )
  await expect(trigger).not.toContainText('Default branch')
  await expect(trigger).toHaveAccessibleName(`Starting branch: Default branch (${name})`)
}

/** Opens the field's popover on the repo's branch list: no typing needed before anything is shown. */
export async function openBranchList(page: Page): Promise<Locator> {
  await startFromTrigger(page).click()
  const list = branchList(page)
  await expect(list).toBeVisible({ timeout: 30_000 })
  return list
}

/** Opens the list and checks the pinned default and every wanted local branch is in it. */
export async function expectBranchesListed(page: Page, refs: string[]): Promise<void> {
  const list = await openBranchList(page)
  await expect(
    list.getByRole('option', { name: `${LEDGERLITE_DEFAULT_BRANCH} (default)` })
  ).toBeVisible()
  for (const ref of refs) {
    await expect(
      list.getByRole('option', { name: ref, exact: true }),
      `${ref} is listed`
    ).toBeVisible()
  }
  await expect(list.getByText('Local branches')).toBeVisible()
}

/** Picks a branch from the list of the field's popover. */
export async function chooseStartingBranch(page: Page, ref: string): Promise<void> {
  const list = await openBranchList(page)
  await list.getByRole('option', { name: ref, exact: true }).click()
  await expect(startFromTrigger(page)).toContainText(ref)
}

/** Picks the pinned default row of the list: "no choice", nothing is sent as baseRef. */
export async function chooseDefaultStartingBranch(page: Page): Promise<void> {
  const list = await openBranchList(page)
  await list.getByRole('option', { name: `${LEDGERLITE_DEFAULT_BRANCH} (default)` }).click()
  await expectTriggerShowsDefault(page, LEDGERLITE_DEFAULT_BRANCH)
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
 * Changes the branch with the pen on the Overview row: a branch of the list, or the pinned default row when `ref` is null.
 * Waits for the service to hold the new value, then for the row to say it.
 */
export async function changeBranchOnOverview(
  f: LiveFlow,
  requestId: string,
  ref: string | null
): Promise<void> {
  const { page } = f
  await baseRefRow(page).getByTestId('base-ref-edit').click()
  const list = branchList(page)
  await expect(list).toBeVisible({ timeout: 30_000 })
  await (
    ref === null
      ? list.getByRole('option', { name: /\(default\)$/ })
      : list.getByRole('option', { name: ref, exact: true })
  ).click()
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
