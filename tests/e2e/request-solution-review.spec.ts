/**
 * E2E for reviewing a Request Solution (FE-REQ-TASK-020-05).
 *
 * NOT YET RUN: needs the Electron app plus request-service (CR-REQ-007/008/009)
 * or a mocked request.* WS channel. Skipped until one of those is available.
 */

import { test } from './helpers/orca-app'

test.describe('Request solution review', () => {
  test.skip(true, 'Requires request-service backend (CR-REQ-007/008/009) or a mocked WS; not run in CI yet')

  test('change_request with two options shows cards and a comparison table', async () => {})

  test('choose then approve calls solution.choose before approval.approve and moves to planning', async () => {})

  test('reject is blocked for a short reason and sends approval.reject with a valid comment', async () => {})

  test('Mod+Enter submits the reject dialog (Meta on Mac, Control elsewhere)', async () => {})

  test('hotfix shows no decision bar', async () => {})
})
