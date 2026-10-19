/**
 * The screens a user has for watching an agent work on a Request task, read-only (TASK-FE-E2E-011): Settings > Dev Servers
 * (is the machine online), the task sheet opened from the Plan tab (status, "Agent running", tabs), and the worktree's Git
 * tab ("Open in Git"). Nothing here clicks anything that starts, changes or cancels work: only navigation and tab switches.
 */
import { expect, type Locator, type Page } from '@playwright/test'
import { linesAddedByRun } from './live-agent-session-judge'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { dismissFirstRunTip, requestsNavButton } from './live-ui-session'

/** Every tab of the task sheet (TaskDetail.tsx); "Result" exists only when the runtime serves `execution.get`. */
export const TASK_SHEET_TABS = ['Details', 'Subtasks', 'AI Agent', 'Comments', 'Access', 'Result']

/** Text of an element that may not exist: absent means '' at once (a bare innerText() would wait the 30 s action timeout). */
async function textIfPresent(target: Locator): Promise<string> {
  return (await target.count()) > 0
    ? (
        (await target
          .first()
          .innerText({ timeout: 2_000 })
          .catch(() => '')) || ''
      ).trim()
    : ''
}

const escapeRegExp = (text: string): string => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

/** A second tab on Settings > Dev Servers, which keeps polling the dev server list by itself (web-preload-api.ts, every 3 s). */
export async function openDevServersPane(devPage: Page): Promise<void> {
  await devPage.goto('/')
  await expect(requestsNavButton(devPage)).toBeVisible({ timeout: 45_000 })
  await dismissFirstRunTip(devPage)
  await devPage.getByRole('button', { name: 'Settings' }).first().click()
  // Why: Settings opens on "Remote Orca Servers"; the dev servers have their own entry in the settings sidebar.
  await devPage.getByRole('button', { name: 'Dev Servers', exact: true }).first().click()
  await expect(devPage.locator('.dev-server-card').first()).toBeVisible({ timeout: 30_000 })
}

function devServerCard(devPage: Page, name: string): Locator {
  return devPage.locator('.dev-server-card').filter({
    has: devPage.locator('.dev-server-card__name', {
      hasText: new RegExp(`^${escapeRegExp(name)}$`)
    })
  })
}

export type DevServerCardReading = {
  /** Badge label: "connected", "disconnected", or the OS name; "no card" when the card is missing. */
  badge: string
  /** `data-busy` of `dev-server-busy`: "true" | "false", or "" when the card shows no busy state at all. */
  busy: string
  activeExecutions: string
  /** Text of `dev-server-platform` ("macOS · arm64"), "" when absent. */
  platform: string
}

/** What the dev server card shows: status badge, busy/idle chip and platform. */
export async function readDevServerCard(
  devPage: Page,
  name: string
): Promise<DevServerCardReading> {
  const card = devServerCard(devPage, name)
  const label = await card
    .locator('.dev-server-badge')
    .first()
    .getAttribute('aria-label', { timeout: 10_000 })
    .catch(() => null)
  const busy = card.getByTestId('dev-server-busy')
  const hasBusy = (await busy.count()) > 0
  return {
    badge: label?.replace(/^Dev server status:\s*/, '') ?? 'no card',
    busy: hasBusy ? ((await busy.getAttribute('data-busy')) ?? '') : '',
    activeExecutions: hasBusy ? ((await busy.getAttribute('data-active-executions')) ?? '') : '',
    platform: await textIfPresent(card.getByTestId('dev-server-platform'))
  }
}

/** True for the labels the badge uses for a reachable server: "connected", or the OS name once the platform is known. */
export const isOnlineLabel = (label: string): boolean =>
  /^(connected|macOS|Windows|Linux|FreeBSD|OpenBSD)$/.test(label)

export const taskSheet = (page: Page): Locator =>
  page.getByRole('dialog').filter({ has: page.getByTestId('task-detail') })

/** Request > Plan tab > the task's row > the task sheet: the way a user gets from a Request to one of its tasks. */
export async function openTaskSheetFromPlan(page: Page, taskId: string): Promise<boolean> {
  await openDetailTab(page, 'Plan')
  const row = detailPane(page).getByTestId(`plan-task-row-${taskId}`)
  const found = await row
    .waitFor({ state: 'visible', timeout: 20_000 })
    .then(() => true)
    .catch(() => false)
  if (!found) {
    return false
  }
  await row.click()
  await expect(taskSheet(page)).toBeVisible({ timeout: 20_000 })
  return true
}

export async function closeTaskSheet(page: Page): Promise<void> {
  if (await taskSheet(page).isVisible()) {
    // Why: Escape also reaches the Requests page, which then drops the selected Request; the sheet's own close button does not.
    await taskSheet(page).getByRole('button', { name: 'Close' }).first().click()
    await expect(taskSheet(page)).toBeHidden({ timeout: 10_000 })
  }
}

export type AgentActivityReading = {
  /** `data-state` of the panel: running | finished | idle; "absent" when the sheet has no panel. */
  state: string
  live: boolean
  lines: { kind: string; text: string }[]
}

/** The "Agent activity" panel of the task sheet (TaskAgentActivityPanel): its state and the lines it shows. */
export async function readAgentActivity(page: Page): Promise<AgentActivityReading> {
  const panel = taskSheet(page).getByTestId('task-agent-activity')
  if ((await panel.count()) === 0) {
    return { state: 'absent', live: false, lines: [] }
  }
  const rows = panel.getByTestId('task-agent-activity-line')
  const lines: AgentActivityReading['lines'] = []
  for (let i = 0; i < (await rows.count()); i++) {
    lines.push({
      kind: (await rows.nth(i).getAttribute('data-kind')) ?? '',
      text: (await rows.nth(i).innerText()).replace(/\s+/g, ' ').trim()
    })
  }
  return {
    state: (await panel.getAttribute('data-state')) ?? '',
    live: (await panel.getByTestId('task-agent-activity-live').count()) > 0,
    lines
  }
}

export type TaskSheetReading = {
  /** "Status: in_progress" line (useTaskActivity polls task.get every 4 s). */
  statusLine: string
  /** Text of the Execute button: "Agent running..." while the task is in_progress. */
  runButton: string
  engine: string
  tabsPresent: string[]
  /** Visible text per tab, as a user would read it. */
  textByTab: Record<string, string>
}

/** Reads the task sheet and walks its tabs. Only tab triggers are clicked; no button inside a tab is ever pressed. */
export async function readTaskSheet(page: Page, allTabs: boolean): Promise<TaskSheetReading> {
  const sheet = taskSheet(page)
  const reading: TaskSheetReading = {
    statusLine: await textIfPresent(sheet.getByTestId('task-live-status')),
    runButton: await textIfPresent(sheet.getByTestId('run-agent-btn')),
    engine: await textIfPresent(sheet.getByTestId('execution-engine-badge')),
    tabsPresent: [],
    textByTab: {}
  }
  for (const name of allTabs ? TASK_SHEET_TABS : ['Details']) {
    const tab = sheet.getByRole('tab', { name, exact: true })
    if ((await tab.count()) === 0) {
      continue
    }
    await tab.click()
    await page.waitForTimeout(250)
    reading.tabsPresent.push(name)
    reading.textByTab[name] = await sheet.innerText()
  }
  return reading
}

/** Texts of the Request's own tabs, for the "is there a trace of what the agent did" check. */
export async function readRequestTabs(page: Page): Promise<Record<string, string>> {
  const out: Record<string, string> = {}
  for (const name of ['Overview', 'Analysis', 'Plan', 'History', 'Related'] as const) {
    const tab = detailPane(page).getByRole('tab', { name })
    if ((await tab.count()) === 0) {
      continue
    }
    await tab.click()
    await page.waitForTimeout(400)
    out[name] = await detailPane(page).innerText()
  }
  return out
}

/**
 * "Open in Git" from the task sheet (TaskDetail.tsx): the product's way to see the files an agent changed. It must land on the
 * task worktree's Source Control list (`source-control-entry` rows). Returns the text of those rows, or null when the sheet has
 * no such button.
 */
export async function openTaskWorktreeInGit(
  page: Page,
  waitForText: RegExp
): Promise<string | null> {
  const button = taskSheet(page).getByTestId('open-in-git-btn')
  if ((await button.count()) === 0) {
    return null
  }
  await button.click()
  const entries = page.getByTestId('source-control-entry')
  const rowsText = async (): Promise<string> => {
    const texts: string[] = []
    for (let i = 0; i < (await entries.count()); i++) {
      texts.push(
        await entries
          .nth(i)
          .innerText({ timeout: 2_000 })
          .catch(() => '')
      )
    }
    return texts.join('\n')
  }
  await expect
    .poll(async () => waitForText.test(await rowsText()), {
      timeout: 30_000,
      message: 'the Source Control list of the task worktree shows the changed files'
    })
    .toBe(true)
    .catch(() => {})
  const changes = await rowsText()
  if (changes) {
    return changes
  }
  // Why: an agent that committed its work leaves the Changes list empty; the commits are under "COMMITS".
  const commits = page.getByText('COMMITS', { exact: true }).first()
  const before = await page.locator('body').innerText()
  if (await commits.isVisible()) {
    await commits.click()
    await page.waitForTimeout(2_500)
  }
  return linesAddedByRun(before, await page.locator('body').innerText())
}

/** Git calls the page makes over its socket (method, a short view of the params and of the reply), to explain an empty Source Control. */
export function recordGitRpc(page: Page): { calls: string[] } {
  const rec = { calls: [] as string[] }
  page.on('websocket', (ws) => {
    const pending = new Map<string, string>()
    ws.on('framesent', (f) => {
      try {
        const m = JSON.parse(String(f.payload)) as {
          id?: string
          method?: string
          params?: unknown
        }
        if (m.id && m.method && /^(git|worktree)\./.test(m.method)) {
          pending.set(m.id, `${m.method} ${JSON.stringify(m.params).slice(0, 160)}`)
        }
      } catch {
        // not a JSON frame
      }
    })
    ws.on('framereceived', (f) => {
      try {
        const m = JSON.parse(String(f.payload)) as {
          id?: string
          ok?: boolean
          result?: unknown
          error?: { code?: string }
        }
        const call = m.id ? pending.get(m.id) : undefined
        if (call && m.id) {
          pending.delete(m.id)
          rec.calls.push(
            `${call} -> ${m.ok === false ? `ERR ${m.error?.code ?? ''}` : JSON.stringify(m.result).slice(0, 200)}`
          )
          rec.calls.splice(0, Math.max(0, rec.calls.length - 400))
        }
      } catch {
        // not a JSON frame
      }
    })
  })
  return rec
}

/**
 * Information only (G5d/G5e backlog): opens the calc.py row of Source Control and reports whether a diff showing the new function
 * appears, and whether the page logged a render crash meanwhile. Never fails the run.
 */
export async function peekDiffOfChangedFile(
  page: Page,
  fn: string
): Promise<{ shown: boolean; crashed: boolean; text: string }> {
  const row = page
    .getByTestId('source-control-entry')
    .filter({ hasText: /calc\.py/ })
    .first()
  if ((await row.count()) === 0) {
    return { shown: false, crashed: false, text: 'no calc.py row to open' }
  }
  let crashed = false
  const onConsole = (m: { text: () => string }): void => {
    crashed ||= /render crash contained by boundary|diff-content-signature/.test(m.text())
  }
  page.on('console', onConsole)
  const before = await page.locator('body').innerText()
  await row.click({ timeout: 5_000 }).catch(() => {})
  const shown = await expect
    .poll(
      async () =>
        new RegExp(fn).test(linesAddedByRun(before, await page.locator('body').innerText())),
      {
        timeout: 10_000
      }
    )
    .toBe(true)
    .then(
      () => true,
      () => false
    )
  page.off('console', onConsole)
  return {
    shown,
    crashed,
    text: shown
      ? `a diff showing ${fn} appeared`
      : `no diff text mentioning ${fn} within 10s${crashed ? '; the page logged a render crash' : ''}`
  }
}
