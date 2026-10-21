/**
 * What a reviewer sees of the drawings of the Outcome Review of a finished Request, in both themes (no AI, reads only):
 *
 *   ORCA_E2E_REVIEW_TITLE='[TMS] CI-CD-KIT (BL-E07)' python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real review-diagrams
 *
 * The Request must have a review with its diagrams drawn (the builtin engine draws them when the context run ends). The spec opens
 * the Outcome review tab, scrolls to each diagram and checks that it is a drawing in an empty-sandbox frame, not a table.
 */
import { expect, test } from '@playwright/test'
import { openRequestsPage, selectRequestRow } from './live-request-ui-steps'
import { openOutcomeReviewTab } from './live-outcome-review-steps'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'
import { dismissFirstRunTip, signInThroughTheForm } from './live-ui-session'

const title = process.env.ORCA_E2E_REVIEW_TITLE ?? ''

test.describe('the drawings of the Outcome Review (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')
  test.skip(
    title === '',
    'set ORCA_E2E_REVIEW_TITLE to the title of a Request whose review has drawings'
  )

  test('each drawn section shows a sandboxed drawing, in the light and the dark theme', async ({
    page
  }, testInfo) => {
    await signInThroughTheForm(page, loadLiveStackConfig())
    await dismissFirstRunTip(page)
    await openRequestsPage(page)
    await selectRequestRow(page, title)
    await openOutcomeReviewTab(page)
    const frames = page.getByTestId('review-artifact-iframe')
    let drawn = 0
    let firstDrawn = ''
    // Why: the diff shows one section at a time, so each drawn section is opened in turn.
    for (const section of ['Architecture', 'Flow', 'Behavior', 'Data']) {
      await page
        .getByTestId('outcome-system-diff')
        .getByRole('tab', { name: new RegExp(`^${section}`) })
        .click()
      // Why: the drawing is read after the tab opens; give it a moment before deciding the section has none.
      await frames
        .first()
        .waitFor({ timeout: 6_000 })
        .catch(() => undefined)
      await page.screenshot({ path: testInfo.outputPath(`tab-${section.toLowerCase()}.png`) })
      if ((await frames.count()) === 0) {
        continue
      }
      const frame = frames.first()
      await frame.scrollIntoViewIfNeeded()
      await expect(frame).toHaveAttribute('sandbox', '')
      const svg = page.frameLocator('[data-testid="review-artifact-iframe"]').first().locator('svg')
      await expect(svg).toBeVisible()
      expect(await svg.locator('text').count(), `the ${section} drawing has text`).toBeGreaterThan(
        0
      )
      await frame.screenshot({ path: testInfo.outputPath(`${section.toLowerCase()}-light.png`) })
      drawn += 1
      firstDrawn ||= section
    }
    expect(drawn, 'the review shows at least one drawing').toBeGreaterThan(0)
    await page
      .getByTestId('outcome-system-diff')
      .getByRole('tab', { name: new RegExp(`^${firstDrawn}`) })
      .click()
    await expect(page.getByTestId('review-artifact-notice')).toHaveCount(0)
    await frames.first().waitFor({ timeout: 15_000 })
    await page.emulateMedia({ colorScheme: 'dark' })
    await page.evaluate(() => document.documentElement.classList.add('dark'))
    await frames.first().scrollIntoViewIfNeeded()
    await frames
      .first()
      .screenshot({ path: testInfo.outputPath(`${firstDrawn.toLowerCase()}-dark.png`) })
    await page.screenshot({ path: testInfo.outputPath('review-tab.png'), fullPage: false })
  })
})
