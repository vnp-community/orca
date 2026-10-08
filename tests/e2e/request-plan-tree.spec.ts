/**
 * E2E for the Request Plan tab (CR-REQ-021).
 *
 * Skipped: needs request-service (CR-REQ-011/012/013) or a mocked WS that serves
 * Plan/Phase tasks and approvals. Not run in this environment.
 */

import { test } from './helpers/orca-app'

const SKIP_REASON = 'Needs request-service plan tree (CR-REQ-011/012/013) or a mocked WS; not run yet'

test.describe('Request plan tree', () => {
  test.skip(true, SKIP_REASON)

  test('a: planning change_request shows Plan > 2 Phases in order with progress', async () => {})
  test('b: Board hides Plan/Phase by default; the toggle shows them; children stay at root', async () => {})
  test('c: approving the plan calls approval.approve; a short reject reason stays locked', async () => {})
  test('d: unapproved phase locks Run; approving then Start phase calls request.startPhase', async () => {})
  test('e: runtime without request channels shows the unsupported notice, no red error', async () => {})
})
