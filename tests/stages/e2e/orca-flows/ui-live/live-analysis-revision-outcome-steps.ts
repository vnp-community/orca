/**
 * What a revision leaves behind and how it fails (CR-REQ-041): the Activity row that never carries the reviewer's text, the
 * refusal of a text that is too long, and a revision sent from a screen that is out of date. In every refusal the typed text
 * stays and no version is made.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { readPanelRecorder, startPanelRecorder } from './live-analysis-revision-panel-recorder'
import { listVersions, revisionPanel } from './live-analysis-revision-steps'
import { requestState } from './live-gateway-crosschecks'
import { PushFrameGate } from './live-push-frame-gate'
import {
  detailHeader,
  detailPane,
  openDetailTab,
  openRequestsPage,
  selectRequestRow
} from './live-request-ui-steps'
import { UI_REFRESH_GRACE_MS } from './live-status-sync'
import { dismissFirstRunTip } from './live-ui-session'

const REVISION_LINE = 'Requested changes to the analysis'
const MAX_FEEDBACK = 2000

/** History shows one line per revision and none of the reviewers' words, on screen or in the service's answer. */
export async function expectRevisionActivity(
  f: LiveFlow,
  r: RequestUnderTest,
  o: { lines: number; secrets: string[] }
): Promise<void> {
  await f.step(`History has ${o.lines} "${REVISION_LINE}" line(s) and never the text`, async () => {
    await openDetailTab(f.page, 'History')
    const pane = detailPane(f.page)
    await expect(
      pane.getByTestId('activity-entry').filter({ hasText: REVISION_LINE }),
      'one line per revision on the screen'
    ).toHaveCount(o.lines, { timeout: 30_000 })
    for (const secret of o.secrets) {
      await expect(pane, 'the text is not on the History tab').not.toContainText(secret)
    }
    await f.shots.take('history-revision-lines')

    const feed = await f.rpc.call<{ entries?: { kind: string }[] }>('request.activity', {
      id: r.requestId,
      pageSize: 100
    })
    const entries = feed.entries ?? []
    expect(entries.filter((e) => e.kind === 'analysis.revision_requested').length).toBe(o.lines)
    const wire = JSON.stringify(entries)
    for (const secret of o.secrets) {
      expect(wire, 'the service does not send the text in the activity feed').not.toContain(secret)
    }
  })
}

/** A text over the limit is refused on the spot: the box keeps it, the service hears nothing. */
export async function expectTooLongKeepsText(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await f.step('a text over 2000 characters is refused and kept in the box', async () => {
    await openDetailTab(f.page, 'Analysis')
    const before = (await listVersions(f, r.requestId)).length
    const long = 'x'.repeat(MAX_FEEDBACK + 1)
    const input = revisionPanel(f).getByTestId('analysis-revision-input')
    await input.fill(long)
    await revisionPanel(f).getByTestId('analysis-revision-submit').click()
    await expect(revisionPanel(f).getByTestId('analysis-revision-error')).toContainText(
      `${MAX_FEEDBACK} characters or fewer`
    )
    await expect(input, 'the typed text is still there').toHaveValue(long)
    await f.shots.take('revision-too-long')
    expect((await listVersions(f, r.requestId)).length, 'no version was made').toBe(before)
    await input.fill('')
  })
}

/**
 * The Request is cancelled while another window still shows its analysis awaiting approval. Sending a revision from that
 * window is refused with a message that says so; the typed text is in the box when the message shows, and nothing is created.
 * The second window's push events are held, so the refusal is not raced by the refresh the cancel would cause.
 */
export async function expectRevisionAfterRequestMovedOn(
  f: LiveFlow,
  r: RequestUnderTest,
  text: string
): Promise<void> {
  await f.step(
    'revising from a screen that is out of date is refused and keeps the text',
    async () => {
      const before = (await listVersions(f, r.requestId)).length
      const stale = await f.page.context().newPage()
      try {
        const gate = await PushFrameGate.install(stale)
        await stale.goto('/')
        await dismissFirstRunTip(stale)
        await openRequestsPage(stale)
        await selectRequestRow(stale, r.title)
        await openDetailTab(stale, 'Analysis')
        const panel = detailPane(stale).getByTestId('analysis-revision-panel')
        await expect(panel).toBeVisible({ timeout: UI_REFRESH_GRACE_MS })
        await panel.getByTestId('analysis-revision-input').fill(text)
        await startPanelRecorder(stale)

        gate.hold()
        const now = await requestState(f.rpc, r.requestId)
        await f.rpc.call('request.cancel', {
          id: r.requestId,
          reason: `${f.cfg.runPrefix}: the analysis spec moves the Request on`,
          expectedVersion: now.version
        })
        await panel.getByTestId('analysis-revision-submit').click()

        await expect
          .poll(async () => (await readPanelRecorder(stale)).some((s) => s.error !== ''), {
            timeout: 15_000,
            message: 'the panel showed an error'
          })
          .toBe(true)
        const refusal = (await readPanelRecorder(stale)).find((s) => s.error !== '')!
        expect(refusal.error).toMatch(/already revising this analysis or has decided/)
        expect(refusal.input, 'the text was still in the box when the refusal showed').toBe(text)
        gate.release()
        await stale.screenshot({
          path: f.testInfo.outputPath('revision-after-request-moved-on.png')
        })
        await expect(
          detailHeader(stale),
          'the screen then learns the Request was cancelled'
        ).toContainText(/Cancelled/i, { timeout: UI_REFRESH_GRACE_MS })
      } finally {
        await stale.close()
      }
      expect((await listVersions(f, r.requestId)).length, 'no version was made').toBe(before)
      expect((await requestState(f.rpc, r.requestId)).status).toBe('cancelled')
    }
  )
}
