/**
 * Steps of the analysis-revision spec (CR-REQ-041): the Analysis tab's "Request changes" panel, the version list, the read-only
 * old versions and the Activity row. The fake claude answers the same document whatever the feedback says, so the checks are
 * about structure: how many versions exist, their statuses, the new Approval, the feedback each version carries.
 */
import { expect, type Locator } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { pendingApprovals, requestState } from './live-gateway-crosschecks'
import { readPanelRecorder, startPanelRecorder } from './live-analysis-revision-panel-recorder'
import { detailPane, openDetailTab, solutionApproveButton } from './live-request-ui-steps'
import { expectScreenFollowsService, UI_REFRESH_GRACE_MS } from './live-status-sync'

const AUTO_START_GRACE_MS = 20_000
const ANALYSIS_SUBJECTS = new Set(['solution', 'findings', 'answer'])

export type VersionRow = {
  id: string
  status: string
  seq?: number
  revisionFeedback?: string
  generatedAt?: string
}

export const revisionPanel = (f: LiveFlow): Locator =>
  detailPane(f.page).getByTestId('analysis-revision-panel')
export const versionButtons = (f: LiveFlow): Locator =>
  detailPane(f.page).locator('[data-testid="solution-version-list"] li')

/** Waits until the Analysis tab shows the analysis awaiting approval. It is never approved here. */
export async function reachAnalysisReady(
  f: LiveFlow,
  r: RequestUnderTest,
  autoStarted: boolean
): Promise<void> {
  await f.step('the analysis reaches the Analysis tab and waits for approval', async () => {
    const pane = detailPane(f.page)
    await openDetailTab(f.page, 'Analysis')
    const start = pane.getByTestId('solution-generate')
    const ready = solutionApproveButton(f.page)
    await expect(
      start.or(ready).or(pane.getByTestId('solution-generation-pending')).first()
    ).toBeVisible({ timeout: 30_000 })
    if (await start.isVisible()) {
      const startedByItself =
        autoStarted &&
        (await expect(start)
          .toBeHidden({ timeout: AUTO_START_GRACE_MS })
          .then(
            () => true,
            () => false
          ))
      if (!startedByItself) {
        await start.click()
      }
    }
    const failed = pane.getByTestId('solution-generation-failed')
    await expect(ready.or(failed).first()).toBeVisible({ timeout: f.budgets.solutionMs })
    if (await failed.isVisible()) {
      throw new Error(
        `the analysis run failed on screen (${(await failed.innerText()).replace(/\s+/g, ' ').slice(0, 200)})`
      )
    }
    await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'ANALYSIS-READY',
      apiStatus: ['awaiting_analysis_approval'],
      chip: /Awaiting Analysis Approval/i,
      apiWaitMs: f.budgets.solutionMs
    })
    await expect(revisionPanel(f), 'the revision panel is offered').toBeVisible({
      timeout: UI_REFRESH_GRACE_MS
    })
    await f.shots.take('analysis-ready')
  })
}

/** The versions of the Request's analysis, oldest first, as the service lists them. */
export async function listVersions(f: LiveFlow, requestId: string): Promise<VersionRow[]> {
  const res = await f.rpc.call<{ solutions?: VersionRow[] }>('solution.list', { requestId })
  return [...(res.solutions ?? [])].sort(
    (a, b) =>
      (a.seq ?? 0) - (b.seq ?? 0) || (a.generatedAt ?? '').localeCompare(b.generatedAt ?? '')
  )
}

/** Ids of the Approvals that decide the analysis and are still open. */
export async function openAnalysisApprovals(f: LiveFlow, requestId: string): Promise<string[]> {
  return (await pendingApprovals(f.rpc, requestId))
    .filter((a) => ANALYSIS_SUBJECTS.has(a.subjectType))
    .map((a) => a.id)
}

/**
 * Types `text` in the panel and presses "Revise analysis". Checks the busy line, then waits for version `version` and its new
 * Approval in the service and on the screen. Returns the id of the new Approval.
 */
export async function reviseAnalysis(
  f: LiveFlow,
  r: RequestUnderTest,
  o: { text: string; version: number; previousApprovals: string[] }
): Promise<string> {
  let approvalId = ''
  await f.step(
    `request changes: "Analyzing version ${o.version}…", then version ${o.version}`,
    async () => {
      await openDetailTab(f.page, 'Analysis')
      const input = revisionPanel(f).getByTestId('analysis-revision-input')
      await input.fill(o.text)
      await expect(revisionPanel(f).getByTestId('analysis-revision-count')).toContainText(
        `${[...o.text].length} / 2000`
      )
      await f.shots.take(`revision-${o.version}-typed`)
      await startPanelRecorder(f.page)
      await revisionPanel(f).getByTestId('analysis-revision-submit').click()

      await expect
        .poll(async () => (await listVersions(f, r.requestId)).length, {
          timeout: f.budgets.solutionMs,
          message: `the service holds ${o.version} versions`
        })
        .toBe(o.version)
      await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
        requestId: r.requestId,
        title: r.title,
        step: `REVISION-${o.version}`,
        apiStatus: ['awaiting_analysis_approval'],
        chip: /Awaiting Analysis Approval/i,
        apiWaitMs: f.budgets.solutionMs
      })

      const log = await readPanelRecorder(f.page)
      expect(
        log.map((s) => s.status),
        `the panel said which version is being written while it worked (saw ${JSON.stringify(log.map((x) => x.status))})`
      ).toContain(`Analyzing version ${o.version}…`)

      const versions = await listVersions(f, r.requestId)
      const latest = versions.at(-1)!
      expect(latest.revisionFeedback, 'the new version carries the request that produced it').toBe(
        o.text
      )
      expect(latest.status, 'the new version waits for a decision').toMatch(/^(ready|proposed)$/)
      expect(
        versions.slice(0, -1).map((v) => v.status),
        'every earlier version is superseded'
      ).toEqual(versions.slice(0, -1).map(() => 'superseded'))
      if (latest.seq !== undefined) {
        expect(
          versions.map((v) => v.seq),
          'versions are numbered in order'
        ).toEqual(versions.map((_, i) => i + 1))
      }

      await expect
        .poll(async () => (await openAnalysisApprovals(f, r.requestId)).length, {
          timeout: 30_000,
          message: 'one Approval is open for the new version'
        })
        .toBe(1)
      const [open] = await openAnalysisApprovals(f, r.requestId)
      expect(
        o.previousApprovals,
        'the Approval is a new one, not the old one reopened'
      ).not.toContain(open)
      approvalId = open

      await expect(versionButtons(f), 'the list shows every version').toHaveCount(o.version, {
        timeout: UI_REFRESH_GRACE_MS
      })
      await expect(
        detailPane(f.page).getByRole('button', { name: new RegExp(`^Version ${o.version}\\b`) }),
        'the screen follows to the new version'
      ).toHaveAttribute('aria-pressed', 'true')
      await expect(
        revisionPanel(f).getByTestId('analysis-revision-input'),
        'the box is emptied'
      ).toHaveValue('')
      expect((await requestState(f.rpc, r.requestId)).status).toBe('awaiting_analysis_approval')
      await f.shots.take(`revision-${o.version}-done`)
    }
  )
  return approvalId
}

/** The text of the request that produced the version on screen; opens the collapsible only when it is closed. */
async function expectAskedFor(f: LiveFlow, text: string): Promise<void> {
  const pane = detailPane(f.page)
  const shown = pane.getByTestId('solution-revision-feedback-text')
  if (!(await shown.isVisible())) {
    await pane.getByTestId('solution-revision-feedback').getByRole('button').click()
  }
  await expect(shown).toHaveText(text)
}

/**
 * Older versions are read-only and show what was asked for them; "Back to current" returns to the version to decide on.
 * `asked[i]` is the request that produced version i + 2 (the first proposal had none); the last one is the current version.
 */
export async function expectOlderVersionsReadOnly(f: LiveFlow, asked: string[]): Promise<void> {
  await f.step('an older version is read-only and shows what was asked for it', async () => {
    const pane = detailPane(f.page)
    const version = (n: number): Locator =>
      pane.getByRole('button', { name: new RegExp(`^Version ${n}\\b`) })
    await version(1).click()
    await expect(pane.getByTestId('solution-older-version-banner')).toBeVisible()
    await expect(revisionPanel(f), 'no revision box on an old version').toHaveCount(0)
    await expect(solutionApproveButton(f.page), 'no approve on an old version').toHaveCount(0)
    await expect(
      pane.getByTestId('solution-revision-feedback'),
      'the first proposal had no request'
    ).toHaveCount(0)
    await f.shots.take('older-version-1')
    for (const [i, text] of asked.slice(0, -1).entries()) {
      await version(i + 2).click()
      await expect(pane.getByTestId('solution-older-version-banner')).toBeVisible()
      await expectAskedFor(f, text)
    }
    await pane.getByTestId('solution-back-to-current').click()
    await expect(pane.getByTestId('solution-older-version-banner')).toHaveCount(0)
    await expect(revisionPanel(f), 'the box is back on the current version').toBeVisible()
    await expect(solutionApproveButton(f.page)).toBeVisible()
    await expectAskedFor(f, asked.at(-1)!)
    await f.shots.take('current-version-asked-for')
  })
}
