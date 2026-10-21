/**
 * COST CLASS C (one context run of the real Claude, about 1 to 2 USD): the Outcome Review of a Request that already finished, taken
 * from backlog BL-E07 by `--real ci-cd-kit` (ORCA_E2E_CICDKIT_REQUEST_ID = its id). Reads the delivered phase with the real Claude,
 * then takes the decisions and the approval through the UI. Nothing is created in the owner's repo; the review choice of the
 * project is put back at the end.
 *
 *   ORCA_E2E_CICDKIT_REQUEST_ID=<uuid> python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real ci-cd-kit-review
 */
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import { runCicdKitFlow } from './ci-cd-kit-harness'
import {
  enableReview,
  reviewFactLines,
  reviewTheDelivery,
  type ReviewFacts
} from './ci-cd-kit-review-steps'
import { openRequestsPage, selectRequestRow } from './live-request-ui-steps'
import { isLiveRun } from './live-stack-config'

const requestId = process.env.ORCA_E2E_CICDKIT_REQUEST_ID ?? ''

test.describe('the Outcome Review of the finished BL-E07 Request (real Claude)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')
  test.skip(requestId === '', 'set ORCA_E2E_CICDKIT_REQUEST_ID to the finished Request')

  test('delivered review, decisions, approval', async ({ page, context }, testInfo) => {
    await runCicdKitFlow({ page, context, testInfo }, async (f) => {
      const got = await f.rpc.call<{ request?: { title?: string; status?: string } }>(
        'request.get',
        { id: requestId }
      )
      const title = got.request?.title ?? ''
      expect(got.request?.status, 'the Request to review has finished').toBe('completed')
      const putBack = await enableReview(f)
      let facts: ReviewFacts | null = null
      try {
        await openRequestsPage(f.page)
        await selectRequestRow(f.page, title)
        facts = await reviewTheDelivery(f, { requestId, title })
      } finally {
        await putBack().catch(() => undefined)
        const lines = reviewFactLines('sau code', facts)
        const dir = process.env.ORCA_E2E_REPORT_DIR || testInfo.outputDir
        mkdirSync(dir, { recursive: true })
        writeFileSync(
          join(dir, 'ci-cd-kit-review.vi.md'),
          `${lines.map((l) => `- ${l}`).join('\n')}\n`
        )
        console.log(`[ci-cd-kit-review]\n${lines.join('\n')}`)
      }
    })
  })
})
