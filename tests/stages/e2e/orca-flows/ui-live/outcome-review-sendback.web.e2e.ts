/**
 * A demanding reviewer sends an after-code finding back as a child Request, through the screen, on a Request that already finished
 * (no AI of the review runs; creating the child makes the AI classify it, a few cents, and the child is cancelled at the end):
 *
 *   ORCA_E2E_CICDKIT_REQUEST_ID=<uuid> python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real ci-cd-kit-sendback
 */
import { expect, test } from '@playwright/test'
import { runCicdKitFlow } from './ci-cd-kit-harness'
import { sendBackAsDemandingReviewer } from './demanding-user-steps'
import { enableReview } from './ci-cd-kit-review-steps'
import { openRequestsPage, selectRequestRow } from './live-request-ui-steps'
import { isLiveRun } from './live-stack-config'

const requestId = process.env.ORCA_E2E_CICDKIT_REQUEST_ID ?? ''

test.describe('sending a finding back as a child Request (real stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')
  test.skip(requestId === '', 'set ORCA_E2E_CICDKIT_REQUEST_ID to the finished Request')

  test('the dialog is prefilled, the child is created, the parent does not move', async ({
    page,
    context
  }, testInfo) => {
    await runCicdKitFlow({ page, context, testInfo }, async (f) => {
      const got = await f.rpc.call<{ request?: { title?: string; status?: string } }>(
        'request.get',
        { id: requestId }
      )
      const title = got.request?.title ?? ''
      expect(got.request?.status).toBe('completed')
      const putBack = await enableReview(f)
      try {
        await openRequestsPage(f.page)
        await selectRequestRow(f.page, title)
        await sendBackAsDemandingReviewer(f, { requestId, title })
      } finally {
        await putBack().catch(() => undefined)
      }
    })
  })
})
