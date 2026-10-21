/**
 * Outcome Review steps of the real ci-cd-kit flow (FE-OR-T332, ORV-20): the same Request is reviewed twice, with the REAL Claude, once
 * at the Plan (design phase, no code) and once after "Approve and run all steps" (delivered phase, decisions). Opt-in because the agent
 * reads cost money: ORCA_E2E_CICDKIT_REVIEW=1.
 *
 * Nothing here fails on what the agent concluded, only on what the screen must never do (invent a verdict, show a raw code, offer a
 * decision before the code exists). What the agent and Orca found goes into `ReviewFacts` for the report.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { reloadAndReopen } from './live-request-ui-steps'
import {
  detailTabNames,
  openOutcomeReviewTab,
  outcomeReviewTab,
  reviewInvite,
  reviewTab,
  setOutcomeReview
} from './live-outcome-review-steps'

const MINUTE = 60_000
type Raw = Record<string, unknown>

export const reviewEnabled = (): boolean => process.env.ORCA_E2E_CICDKIT_REVIEW === '1'

export type ReviewFacts = {
  phase: string
  state: string
  items: number
  needsDecision: number
  advisory: number
  diagrams: string[]
  banners: string[]
  kg: string
  runs: string[]
  decisionsMade: string[]
  approvedText: string
}

const arr = (v: unknown): Raw[] => (Array.isArray(v) ? (v as Raw[]) : [])
const str = (v: unknown): string => (typeof v === 'string' ? v : '')

async function readReview(f: LiveFlow, r: RequestUnderTest): Promise<Raw | null> {
  const got = await f.rpc.call<Raw>('request.review.get', { id: r.requestId })
  const review = got.review as Raw | null | undefined
  return review && str(review.reviewId) !== '' ? review : null
}

async function readRuns(f: LiveFlow, r: RequestUnderTest): Promise<Raw[]> {
  return arr((await f.rpc.call<Raw>('request.review.runs', { id: r.requestId })).runs)
}

const active = (run: Raw): boolean => ['queued', 'running'].includes(str(run.status))

/** Waits until at least `minRuns` runs exist and none is going, or the budget ends. Returns the runs seen. */
async function waitForRuns(
  f: LiveFlow,
  r: RequestUnderTest,
  label: string,
  ms: number,
  minRuns = 1
): Promise<Raw[]> {
  const until = Date.now() + ms
  while (Date.now() < until) {
    const runs = await readRuns(f, r)
    if (runs.length >= minRuns && !runs.some(active)) {
      return runs
    }
    await f.page.waitForTimeout(10_000)
  }
  throw new Error(`${label}: the review runs did not finish within ${Math.round(ms / MINUTE)} min`)
}

const describeRun = (run: Raw): string =>
  `${str(run.kind)}/${str(run.scope)}: ${str(run.status)}${str(run.errorCode) ? ` (${str(run.errorCode)})` : ''}${
    str(run.costActual) ? ` cost ${str(run.costActual)}` : ''
  }`

async function factsOf(
  f: LiveFlow,
  r: RequestUnderTest,
  decisionsMade: string[]
): Promise<ReviewFacts> {
  const review = await readReview(f, r)
  const model = (review?.model ?? {}) as Raw
  const items = arr(model.items)
  const banners = await f.page
    .locator('[data-testid^="outcome-banner-"]')
    .evaluateAll((els) => els.map((e) => e.getAttribute('data-testid') ?? ''))
  return {
    phase: str(review?.phase),
    state: str(review?.state),
    items: items.length,
    needsDecision: items.filter((i) => i.needsDecision === true && i.advisory !== true).length,
    advisory: items.filter((i) => i.advisory === true).length,
    diagrams: arr(model.diagrams).map(
      (d) => `${str(d.section)}/${str(d.variant)}:${str(d.status)}`
    ),
    banners,
    kg: str(((model.kg ?? {}) as Raw).snapshot) || 'no snapshot',
    runs: (await readRuns(f, r)).map(describeRun),
    decisionsMade,
    approvedText: ''
  }
}

/** Turns the review on for the owner's project for this run; returns what puts it back. */
export async function enableReview(f: LiveFlow): Promise<() => Promise<void>> {
  const before = await setOutcomeReview(f, 'manual')
  return async () => {
    await setOutcomeReview(f, before === 'off' || before === 'manual' ? before : 'default')
  }
}

/** At the Plan: the tab follows Execution, opening it only reads, the agent's reading shows a design review with no decisions. */
export async function reviewTheDesign(f: LiveFlow, r: RequestUnderTest): Promise<ReviewFacts> {
  await f.step('design review: the tab follows Execution and opening it only reads', async () => {
    await reloadAndReopen(f.page, r.title)
    const names = await detailTabNames(f.page)
    const at = names.findIndex((n) => n.startsWith('Execution'))
    expect(at, 'the Request has an Execution tab').toBeGreaterThan(-1)
    expect(names[at + 1]).toBe('Outcome review')
    await openOutcomeReviewTab(f.page)
    // Why: the invitation sits inside the same `outcome-review-tab` wrapper as a review, so only the invite id tells them apart.
    await expect(reviewInvite(f.page)).toHaveAttribute('data-kind', 'design')
    await f.shots.take('review-design-invitation')
  })

  await f.step("design review: the agent's reading (real Claude)", async () => {
    await f.page.getByTestId('outcome-review-generate').click()
    await expect
      .poll(async () => (await readRuns(f, r)).length, {
        timeout: 60_000,
        message: 'the click started no run'
      })
      .toBeGreaterThan(0)
    const runs = await waitForRuns(f, r, 'design', 25 * MINUTE)
    for (const run of runs) {
      expect(
        str(run.errorCode),
        `no raw code is expected to hide a failure: ${describeRun(run)}`
      ).not.toBe('REQUEST_INTERNAL')
    }
    await reloadAndReopen(f.page, r.title)
    await outcomeReviewTab(f.page).click()
    await expect(reviewTab(f.page)).toBeVisible({ timeout: 60_000 })
    await expect(reviewInvite(f.page)).toHaveCount(0)
    await f.shots.take('review-design-ready')
  })

  await f.step('design review: design phase, nothing to decide, after-code locked', async () => {
    await expect(reviewTab(f.page)).toHaveAttribute('data-phase', 'design')
    await expect(f.page.getByTestId('outcome-decision-bar')).toHaveCount(0)
    await expect(f.page.getByTestId('outcome-approve')).toHaveCount(0)
    const modes = f.page.getByTestId('outcome-mode-switch')
    await expect(modes.locator('[data-mode="after_code"]')).toHaveAttribute('aria-disabled', 'true')
    expect(await f.page.locator('body').innerText()).not.toMatch(/REQUEST_REVIEW_[A-Z_]+/)
    expect(await f.page.locator('body').innerText()).not.toMatch(/safe to release/i)
    await f.shots.take('review-design-phase')
  })
  return factsOf(f, r, [])
}

/** After the code: a delivered review, the decisions, and an approval that says it is not a release sign-off. */
export async function reviewTheDelivery(f: LiveFlow, r: RequestUnderTest): Promise<ReviewFacts> {
  const decisionsMade: string[] = []
  await f.step('delivered review: the agent reads design and code (real Claude)', async () => {
    await reloadAndReopen(f.page, r.title)
    await outcomeReviewTab(f.page).click()
    await expect(reviewTab(f.page).or(reviewInvite(f.page)).first()).toBeVisible({
      timeout: 60_000
    })
    await f.shots.take('review-after-execution')
    // Why: a re-run to try the decisions again must not pay for the same reading twice.
    if (
      process.env.ORCA_E2E_CICDKIT_REVIEW_KEEP === '1' &&
      (await reviewTab(f.page).getAttribute('data-phase')) === 'delivered'
    ) {
      return
    }
    const before = (await readRuns(f, r)).length
    // Why: no review yet shows the invitation; a design review shows the "Generate after-code review" banner; a review of the
    // delivered phase (facts only) is read again through the Regenerate panel.
    const banner = f.page.getByTestId('outcome-banner-generate')
    if ((await reviewInvite(f.page).count()) > 0) {
      await f.page.getByTestId('outcome-review-generate').click()
    } else if (await banner.isVisible().catch(() => false)) {
      await banner.click()
    } else {
      await f.page.getByTestId('outcome-header-regenerate').click()
      await f.page.getByLabel('What to do').selectOption('context')
      await f.page.getByLabel('Why').selectOption('other')
      await f.page.getByTestId('regenerate-submit').click()
    }
    await expect
      .poll(async () => (await readRuns(f, r)).length, {
        timeout: 60_000,
        message: 'the click started no run'
      })
      .toBeGreaterThan(before)
    await waitForRuns(f, r, 'delivered', 30 * MINUTE, before + 1)
    await reloadAndReopen(f.page, r.title)
    await outcomeReviewTab(f.page).click()
    await expect(reviewTab(f.page)).toHaveAttribute('data-phase', 'delivered', { timeout: 60_000 })
    await f.shots.take('review-delivered')
  })

  await f.step(
    'delivered review: after-code view opens and no verdict says the work is safe',
    async () => {
      const modes = f.page.getByTestId('outcome-mode-switch')
      await expect(modes.locator('[data-mode="after_code"]')).not.toHaveAttribute(
        'aria-disabled',
        'true'
      )
      await modes.locator('[data-mode="after_code"]').click()
      const text = await f.page.locator('body').innerText()
      expect(text).not.toMatch(/REQUEST_REVIEW_[A-Z_]+/)
      expect(text).not.toMatch(/safe to release|ready to ship/i)
      await f.shots.take('review-delivered-after-code')
    }
  )

  await f.step(
    'delivered review: Approve is refused while decisions are open, then each is decided',
    async () => {
      const cards = f.page.getByTestId('outcome-decision-card')
      const undecided = f.page.getByRole('button', { name: 'Accept as exception' })
      const decided = f.page.getByTestId('outcome-decision-made')
      const open = await undecided.count()
      if (open > 0) {
        await f.page.getByTestId('outcome-approve').click()
        await expect(f.page.getByTestId('outcome-approve-error')).toBeVisible()
        await f.shots.take('review-approve-refused')
      }
      for (let i = 0; i < Math.min(open, 8); i += 1) {
        const already = await decided.count()
        const card = cards.filter({ has: undecided }).first()
        await card.getByRole('button', { name: 'Accept as exception' }).click()
        await card
          .getByLabel(/Why/)
          .fill('Accepted during the BL-E07 acceptance run: reviewed by the owner.')
        await card.getByTestId('outcome-decision-confirm').click()
        await expect(decided).toHaveCount(already + 1, { timeout: 30_000 })
        decisionsMade.push(`item ${i + 1}: accepted as exception`)
      }
      await f.shots.take('review-decisions-made')
    }
  )

  let approvedText = ''
  await f.step('delivered review: approval keeps its words and survives a reload', async () => {
    if ((await f.page.getByRole('button', { name: 'Accept as exception' }).count()) > 0) {
      approvedText = 'not approved: decisions remain'
      return
    }
    await f.page.getByTestId('outcome-approve').click()
    const approved = f.page.getByTestId('outcome-approved')
    await expect(approved).toBeVisible({ timeout: 30_000 })
    await expect(approved).toContainText('This is not a release sign-off')
    await reloadAndReopen(f.page, r.title)
    await outcomeReviewTab(f.page).click()
    await expect(f.page.getByTestId('outcome-approved')).toBeVisible({ timeout: 30_000 })
    await f.shots.take('review-approved')
    approvedText = (await f.page.getByTestId('outcome-approved').innerText()).trim()
  })

  const facts = await factsOf(f, r, decisionsMade)
  facts.approvedText = approvedText
  return facts
}

export function reviewFactLines(label: string, facts: ReviewFacts | null): string[] {
  if (!facts) {
    return [`${label}: không chạy`]
  }
  return [
    `${label}: pha ${facts.phase}, trạng thái ${facts.state}, ${facts.items} mục (${facts.needsDecision} cần quyết định, ${facts.advisory} chỉ tham khảo), KG ${facts.kg}`,
    `${label}: hình ${facts.diagrams.join(', ') || 'không có'}; dải ${facts.banners.join(', ') || 'không có'}`,
    ...facts.runs.map((x) => `${label}: lượt chạy ${x}`),
    ...facts.decisionsMade.map((x) => `${label}: ${x}`),
    ...(facts.approvedText ? [`${label}: ${facts.approvedText}`] : [])
  ]
}
