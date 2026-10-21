/**
 * Page-level actions of the Request flow, written with roles, labels and the data-testids the components already ship
 * (request-detail-header, type-confirmation-card, clarification-panel, approval-row ...). Texts are the en locale.
 */
import { expect, type Locator, type Page } from '@playwright/test'
import { dismissFirstRunTip, requestsNavButton } from './live-ui-session'

export const detailHeader = (page: Page): Locator => page.getByTestId('request-detail-header')
export const detailPane = (page: Page): Locator => page.getByTestId('request-detail-pane')

export async function openRequestsPage(page: Page): Promise<void> {
  await dismissFirstRunTip(page)
  // Why: the SPA restores the last view, and another tab of the same browser may have left it on Settings.
  const back = page.getByRole('button', { name: 'Back to app' })
  if (await back.isVisible()) {
    await back.click()
  }
  const tab = page.getByRole('tab', { name: 'Requests' })
  // Why: the SPA restores the last view on reload, and clicking the active nav button is not needed then.
  if (!(await tab.isVisible())) {
    await requestsNavButton(page).click()
  }
  await expect(tab).toBeVisible()
}

export async function createRequestThroughTheDialog(
  page: Page,
  input: { title: string; body: string; projectName: string }
): Promise<void> {
  await page.getByRole('button', { name: 'Create request' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Title').fill(input.title)
  await dialog.getByLabel('Description').fill(input.body)
  await dialog.getByRole('combobox', { name: /project/i }).click()
  await page.getByRole('option', { name: input.projectName, exact: true }).click()
  await dialog.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(dialog).toBeHidden()
}

/** Selects the request in the list (the UI keeps it selected after Create, so this is for returning to it). */
export async function selectRequestRow(page: Page, title: string): Promise<void> {
  const row = page.getByRole('option', { name: new RegExp(escapeRegExp(title)) })
  await row.first().click({ timeout: 30_000 })
  await expect(detailHeader(page)).toContainText(title)
}

/** Full reload, then back to the same request: what a user does when the page looks stale. */
export async function reloadAndReopen(page: Page, title: string): Promise<void> {
  // Why: the SPA keeps long-lived connections open and 'load' can take very long; the shell check below is the real readiness test.
  await page.reload({ waitUntil: 'domcontentloaded' })
  await expect(
    requestsNavButton(page).or(page.getByRole('button', { name: 'Back to app' }))
  ).toBeVisible({ timeout: 45_000 })
  await dismissFirstRunTip(page)
  await openRequestsPage(page)
  await selectRequestRow(page, title)
}

export async function openDetailTab(
  page: Page,
  name: 'Overview' | 'Analysis' | 'Plan' | 'Execution' | 'History' | 'Related'
): Promise<void> {
  await detailPane(page).getByRole('tab', { name }).click()
}

/** Types an answer into every control of the open Clarification and submits. Returns false when Submit never enables. */
export async function answerClarificationInPanel(page: Page, answer: string): Promise<boolean> {
  const panel = page.getByTestId('clarification-panel')
  await expect(
    panel.getByRole('textbox').or(panel.getByRole('checkbox')).or(panel.getByRole('radio')).first()
  ).toBeVisible({
    timeout: 30_000
  })
  const boxes = panel.getByRole('textbox')
  for (let i = 0; i < (await boxes.count()); i++) {
    // Why: a single-line input can be a number field (spike time box) that rejects words; text areas take the sentence.
    const singleLine = await boxes.nth(i).evaluate((el) => el.tagName === 'INPUT')
    await boxes.nth(i).fill(singleLine ? '8' : `${answer}\n${answer} (second line)`)
  }
  const numbers = panel.getByRole('spinbutton')
  for (let i = 0; i < (await numbers.count()); i++) {
    await numbers.nth(i).fill('8')
  }
  const switches = panel.getByRole('checkbox')
  for (let i = 0; i < (await switches.count()); i++) {
    if (!(await switches.nth(i).isChecked())) {
      await switches.nth(i).check()
    }
  }
  // Why: single_choice and boolean are Radix ToggleGroups (role "group" with radio items), `radiogroup` only for native radios.
  const groups = panel.locator('[role="radiogroup"], [role="group"]').filter({
    has: page.getByRole('radio')
  })
  for (let i = 0; i < (await groups.count()); i++) {
    const radios = groups.nth(i).getByRole('radio')
    if (
      (await radios.count()) > 0 &&
      (await groups.nth(i).locator('[aria-checked="true"]').count()) === 0
    ) {
      await radios.first().click()
    }
  }
  const submit = panel.getByRole('button', { name: /submit answers/i })
  const enabled = await expect(submit)
    .toBeEnabled({ timeout: 10_000 })
    .then(() => true)
    .catch(() => false)
  if (!enabled) {
    return false
  }
  await submit.click()
  return true
}

/** The type card: set type/size explicitly (so the flow does not depend on what the AI proposed) and confirm. */
export async function confirmTypeInCard(
  page: Page,
  type: string,
  size: string,
  urgency?: string,
  reason?: string
): Promise<void> {
  const card = page.getByTestId('type-confirmation-card')
  await expect(card).toBeVisible()
  const typeBox = card.getByRole('combobox').first()
  if (!new RegExp(`^${type}\\b`, 'i').test((await typeBox.innerText()).trim())) {
    await typeBox.click()
    await page.getByRole('option', { name: new RegExp(`^${type}\\b`, 'i') }).click()
  }
  const sizeBox = card.getByRole('combobox', { name: 'Size' })
  if ((await sizeBox.innerText()).trim() !== size) {
    await sizeBox.click()
    await page.getByRole('option', { name: size, exact: true }).click()
  }
  if (urgency) {
    // Why: the backend refuses a hotfix that is not urgent (REQUEST_HOTFIX_REQUIRES_URGENT).
    const urgencyBox = card.getByRole('combobox', { name: 'Urgency' })
    if ((await urgencyBox.innerText()).trim() !== urgency) {
      await urgencyBox.click()
      await page.getByRole('option', { name: urgency, exact: true }).click()
    }
  }
  if (reason) {
    // Why: the backend requires a reason for a hotfix (REQUEST_REASON_REQUIRED).
    await card.getByPlaceholder(/Why this type/).fill(reason)
  }
  await card.getByRole('button', { name: 'Confirm type' }).click()
}

/** The approve control of the Analysis tab: "Approve this option" (solution) or "Approve" (answer, findings, diagnosis). */
export const solutionApproveButton = (page: Page): Locator =>
  detailPane(page).locator('[data-testid^="solution-approve"]')

/**
 * Picks the recommended option (or the first; kinds without options have none) and approves. Returns the chosen
 * option's testid, or '' when the analysis has no options to choose from.
 */
export async function chooseAndApproveSolutionOption(page: Page): Promise<string> {
  const pane = detailPane(page)
  await expect(solutionApproveButton(page)).toBeVisible({ timeout: 30_000 })
  const options = pane.getByRole('radio')
  let testId = ''
  if ((await options.count()) > 0) {
    const recommended = options.filter({ hasText: 'Recommended' })
    const others = options.filter({ hasNotText: 'Recommended' })
    // Why: a demanding user does not take the recommendation; choosing another option must ask for a reason of 10 characters or more.
    const against = process.env.ORCA_E2E_DEMANDING === '1' && (await others.count()) > 0
    const pick = against
      ? others.first()
      : (await recommended.count()) > 0
        ? recommended.first()
        : options.first()
    testId = (await pick.getAttribute('data-testid')) ?? ''
    await pick.click()
    await expect(pick).toHaveAttribute('aria-checked', 'true')
    if (against) {
      const approve = solutionApproveButton(page)
      await expect(approve, 'against the recommendation, Approve waits for a reason').toBeDisabled()
      await pane.getByRole('textbox').last().fill('too short')
      await expect(approve, 'nine characters are not a reason').toBeDisabled()
      await pane
        .getByRole('textbox')
        .last()
        .fill('Chống trùng ở tầng dữ liệu an toàn hơn cho 200 hàng và chạy lại nhiều lần.')
      await expect(approve).toBeEnabled()
    }
  }
  await solutionApproveButton(page).click()
  await confirmHighRiskChoiceIfAsked(page)
  return testId
}

/**
 * A high-risk option is confirmed a second time by typing its title. A careful person first types something else (the button must
 * stay off), then the title. Returns whether the dialog was there.
 */
export const uiFindings: string[] = []

export async function confirmHighRiskChoiceIfAsked(page: Page): Promise<boolean> {
  const dialog = page.getByTestId('high-risk-decision-dialog')
  // Why: the dialog opens after the choice is stored and the decisions are read again, so it is awaited, not looked at once.
  const there = await dialog.waitFor({ state: 'visible', timeout: 20_000 }).then(
    () => true,
    () => false
  )
  if (!there) {
    return false
  }
  const label = await dialog.locator('label').first().innerText()
  const title = /Type "(.*)" to confirm/.exec(label)?.[1] ?? ''
  expect(title, 'the dialog names the option to type').not.toBe('')
  if ((await dialog.getByRole('listitem').count()) === 0) {
    uiFindings.push('the high-risk dialog says "Review the reasons" but lists none')
  }
  if (title.length > 40) {
    uiFindings.push(`the high-risk dialog asks to type a ${title.length}-character title by hand`)
  }
  const input = dialog.getByRole('textbox')
  await input.fill('xác nhận')
  await expect(
    dialog.getByRole('button', { name: 'Confirm choice' }),
    'a wrong text cannot confirm'
  ).toBeDisabled()
  await input.fill(title)
  await dialog.getByRole('button', { name: 'Confirm choice' }).click()
  await expect(dialog).toBeHidden({ timeout: 30_000 })
  return true
}

/** In the approvals inbox: the row of this request whose subject label matches (e.g. "Plan"). */
export function approvalRowFor(page: Page, title: string, subject: RegExp): Locator {
  return page.getByTestId('approval-row').filter({ hasText: title }).filter({ hasText: subject })
}

export async function rejectApprovalRow(page: Page, row: Locator, reason: string): Promise<void> {
  await row.getByRole('button', { name: 'Reject' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('textbox').fill(reason)
  await dialog.getByRole('button', { name: 'Reject', exact: true }).click()
  await expect(dialog).toBeHidden({ timeout: 20_000 })
}

export async function cancelOpenRequestThroughTheUi(page: Page, reason: string): Promise<void> {
  await detailHeader(page).getByTestId('request-cancel').click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('textbox').fill(reason)
  await dialog.getByTestId('request-cancel-confirm').click()
  await expect(dialog).toBeHidden({ timeout: 20_000 })
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
