/**
 * Steps of the automatic Plan spec (CR-REQ-044 / FE-REQ-TASK-044-04): a `bug` on a LedgerLite repo up to the approval of its
 * Analysis, after which Orca writes the Plan by itself. Texts and test ids are the ones the components ship (en locale).
 *
 * Failures of the Plan run are made with the fake claude's marker file `fail_plan_next` (`<count>:<error|invalid>`), which
 * needs the dev server on this machine: run_ui_flow.py passes the stub's state directory as ORCA_E2E_FAKE_CLAUDE_STATE_DIR.
 */
import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, type Locator, type Page } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import {
  answerClarificationIfAsked,
  createClassifyAndConfirm,
  type RequestUnderTest
} from './live-flow-steps'
import { waitForApiStatus } from './live-gateway-crosschecks'
import {
  chooseAndApproveSolutionOption,
  detailPane,
  openDetailTab,
  solutionApproveButton
} from './live-request-ui-steps'
import { dismissFirstRunTip } from './live-ui-session'

export const planWriting = (page: Page): Locator =>
  detailPane(page).getByTestId('plan-auto-writing')
export const planGenerate = (page: Page): Locator => detailPane(page).getByTestId('plan-generate')
export const planFailedCard = (page: Page): Locator =>
  detailPane(page).getByTestId('plan-run-failed')
export const planRetry = (page: Page): Locator => detailPane(page).getByTestId('plan-run-retry')
/** The approval bar of a Plan that exists: the sign that the Plan was written and saved. */
export const planApproveButton = (page: Page): Locator =>
  detailPane(page).getByTestId('plan-approve')

const AUTO_START_GRACE_MS = 20_000

type StoredEngineSettings = {
  engine?: string
  openspecMinVersion?: string
  solutionAgent?: boolean
  executionMode?: string
  planAutoGenerate?: boolean
  version: number
}

/**
 * Test setup (not a product step): the project's switch for writing the Plan on its own, through the admin channel. Everything
 * else in the stored settings is written back as read. Returns the value it replaced, so the spec can put it back.
 */
export async function setPlanAutoGenerate(f: LiveFlow, enabled: boolean): Promise<boolean> {
  const current = await f.rpc.call<StoredEngineSettings>('request.engineSettings', {
    projectId: f.project.id
  })
  const before = current.planAutoGenerate ?? true
  const next = await f.rpc.call<StoredEngineSettings>('request.engineSettingsSet', {
    projectId: f.project.id,
    engine: current.engine ?? 'native',
    openspecMinVersion: current.openspecMinVersion ?? '',
    solutionAgent: current.solutionAgent ?? false,
    executionMode: current.executionMode ?? 'step_by_step',
    planAutoGenerate: enabled,
    expectedVersion: current.version
  })
  expect(next.planAutoGenerate, 'the project switch was stored').toBe(enabled)
  return before
}

export type PlanAutoView = {
  enabled?: boolean
  state?: string
  attempt?: number
  maxAttempts?: number
  reasonCode?: string
}
export type PlanProposalAnswer = {
  runId?: string
  status?: string
  errorCode?: string
  autoPlan?: PlanAutoView
}

/** What the gateway says about the latest Plan run; `null` when the Request has none (the service answers "run not found"). */
export async function readPlanProposal(
  f: LiveFlow,
  r: RequestUnderTest
): Promise<PlanProposalAnswer | null> {
  try {
    return await f.rpc.call<PlanProposalAnswer>('request.planProposal', { id: r.requestId })
  } catch (e) {
    if (/RUN_NOT_FOUND/.test(String(e))) {
      return null
    }
    throw e
  }
}

export type BrowserCalls = {
  /** How many times this window sent the method to the gateway. */
  count: (method: string) => number
}

/**
 * Counts the RPCs the browser sends. The socket of a signed-in page predates the listener, so the page is reloaded once and the
 * new socket is the one observed; a page that has not loaded anything yet needs no reload.
 */
export async function watchBrowserCalls(
  page: Page,
  options: { reload?: boolean } = {}
): Promise<BrowserCalls> {
  const sent = new Map<string, number>()
  page.on('websocket', (ws) => {
    ws.on('framesent', (frame) => {
      for (const line of String(frame.payload).split('\n')) {
        try {
          const method = (JSON.parse(line) as { method?: string }).method
          if (method) {
            sent.set(method, (sent.get(method) ?? 0) + 1)
          }
        } catch {
          // not a JSON frame
        }
      }
    })
  })
  if (options.reload ?? true) {
    await page.reload({ waitUntil: 'domcontentloaded' })
    await dismissFirstRunTip(page)
  }
  return { count: (method) => sent.get(method) ?? 0 }
}

export type PlanFailureKind = 'error' | 'invalid'

/** The directory the fake claude keeps its state in; empty when the runner did not say (then the spec cannot make it fail). */
export function fakeClaudeStateDir(): string {
  return process.env.ORCA_E2E_FAKE_CLAUDE_STATE_DIR ?? ''
}

/** The next `count` Plan prompts fail the given way. `invalid` needs 2: the native engine corrects once before it gives up. */
export function armPlanFailure(count: number, kind: PlanFailureKind): void {
  const dir = fakeClaudeStateDir()
  mkdirSync(dir, { recursive: true })
  writeFileSync(join(dir, 'fail_plan_next'), `${count}:${kind}`)
}

export function disarmPlanFailure(): void {
  const dir = fakeClaudeStateDir()
  if (dir) {
    rmSync(join(dir, 'fail_plan_next'), { force: true })
  }
}

/** A `bug` (size S, native) confirmed in the card, with its Clarification answered when one is asked. */
export async function reachAnalysisApproval(
  f: LiveFlow,
  titleWords: string
): Promise<{ r: RequestUnderTest; asked: boolean }> {
  const r = await createClassifyAndConfirm(f, {
    titleWords,
    // "crash" is a bug cue for the fake claude.
    body: 'Saving the settings page crashes with an error when the badge colour is empty. It worked last week.',
    typeLabel: 'Bug',
    typeKey: 'bug',
    size: 'S'
  })
  return { r, asked: await answerClarificationIfAsked(f, r) }
}

/**
 * Approves the Analysis on the Analysis tab and stops there. The Plan is Orca's to write, so unlike approveAnalysis this does
 * not wait for the status `planning`, which a fast stub can pass through between two polls.
 */
export async function approveAnalysisAndLeavePlanToOrca(
  f: LiveFlow,
  r: RequestUnderTest,
  asked: boolean
): Promise<void> {
  await f.step('the analysis is ready on the Analysis tab and is approved', async () => {
    const { page } = f
    await openDetailTab(page, 'Analysis')
    const start = detailPane(page).getByTestId('solution-generate')
    const ready = solutionApproveButton(page)
    await expect(
      start.or(ready).or(detailPane(page).getByTestId('solution-generation-pending')).first()
    ).toBeVisible({ timeout: 30_000 })
    // Why: after a Clarification the analysis starts by itself; the prompt can show for a moment before the run row exists.
    const startedByItself =
      asked &&
      (await start
        .waitFor({ state: 'hidden', timeout: AUTO_START_GRACE_MS })
        .then(() => true)
        .catch(() => false))
    if (!startedByItself && (await start.isVisible())) {
      await start.click()
    }
    await expect(ready).toBeVisible({ timeout: f.budgets.solutionMs })
    expect(
      await detailPane(page).getByTestId('solution-blocking-question').count(),
      'the fake analysis of a bug has no blocking question'
    ).toBe(0)
    await f.shots.take('analysis-ready')
    await chooseAndApproveSolutionOption(page)
    await waitForApiStatus(f.rpc, r.requestId, ['planning', 'awaiting_plan_approval'], 30_000)
  })
}

export type PlanWatch = {
  sawWriting: boolean
  sawGenerate: boolean
  sawFailedCard: boolean
  /** The "Attempt n of m" lines seen while Orca was writing. */
  attempts: string[]
}

/**
 * Looks at the Plan tab every 200 ms until `until` is visible (or the budget runs out) and records what it showed on the way.
 * The point is the absence of the Generate button while Orca works, which a single assertion at one moment cannot show.
 */
export async function watchPlanTab(
  f: LiveFlow,
  until: Locator,
  budgetMs: number
): Promise<PlanWatch> {
  const watch: PlanWatch = {
    sawWriting: false,
    sawGenerate: false,
    sawFailedCard: false,
    attempts: []
  }
  const deadline = Date.now() + budgetMs
  while (Date.now() < deadline) {
    if (await until.isVisible().catch(() => false)) {
      return watch
    }
    if (
      await planWriting(f.page)
        .isVisible()
        .catch(() => false)
    ) {
      if (!watch.sawWriting) {
        await f.shots.take('plan-auto-writing')
      }
      watch.sawWriting = true
      const attempt = await detailPane(f.page)
        .getByTestId('plan-auto-attempt')
        .innerText({ timeout: 200 })
        .catch(() => '')
      if (attempt && !watch.attempts.includes(attempt)) {
        watch.attempts.push(attempt)
      }
    }
    watch.sawGenerate ||= await planGenerate(f.page)
      .isVisible()
      .catch(() => false)
    watch.sawFailedCard ||= await planFailedCard(f.page)
      .isVisible()
      .catch(() => false)
    await f.page.waitForTimeout(200)
  }
  throw new Error(
    `the Plan tab showed neither the expected element nor a Plan within ${budgetMs}ms (writing seen: ${watch.sawWriting}, Generate seen: ${watch.sawGenerate}, failed card seen: ${watch.sawFailedCard})`
  )
}
