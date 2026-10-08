/**
 * Electron placeholder for the Request Plan tab (CR-REQ-021).
 *
 * The real scenarios (a)-(e) run in the web SPA against a mocked gateway WebSocket:
 * tests/e2e/request-web/request-plan-tree.web.e2e.ts (playwright.web.config.ts).
 * The Electron app has no request-service wiring in the default e2e environment.
 */

import { test } from './helpers/orca-app'

test.describe('Request plan tree (Electron)', () => {
  test.skip(
    true,
    'Covered by tests/e2e/request-web/request-plan-tree.web.e2e.ts; Electron has no request-service here'
  )
  test('see request-web/request-plan-tree.web.e2e.ts', async () => {})
})
