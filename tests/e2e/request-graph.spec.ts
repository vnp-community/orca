/**
 * E2E for the Request graph (CR-REQ-032).
 *
 * Skipped: needs request-service and impact-service (CR-REQ-030) or a mocked WS that
 * serves `impact.graph`. Not run in this environment.
 */

import { test } from './helpers/orca-app'

const SKIP_REASON = 'Needs request-service + impact.graph or a mocked WS; not run yet'

test.describe('Request graph', () => {
  test.skip(true, SKIP_REASON)

  test('a: runtime without impact.* still opens "View graph"; backend lens chips disabled; flow lens shown', async () => {})
  test('b: impact.graph with 120 nodes defaults to the list; "/" opens search; Enter selects a node', async () => {})
  test('c: Plan tab Tree | Graph toggle switches the view', async () => {})
})
