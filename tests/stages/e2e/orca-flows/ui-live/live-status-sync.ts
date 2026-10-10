/**
 * "The screen shows what the service says." The service answer is authoritative; the screen gets a grace period (the SPA
 * refreshes a Request by event or by a 15 s poll). Past it, the stale screen is recorded as a gap and the page reloaded.
 */
import { expect, type Page } from '@playwright/test'
import type { GatewayRpcClient } from './gateway-rpc-client'
import { requestState, waitForApiStatus } from './live-gateway-crosschecks'
import type { GapReporter } from './live-gap-reporter'
import { detailHeader, reloadAndReopen } from './live-request-ui-steps'

export const UI_REFRESH_GRACE_MS = 45_000

export type StatusSync = {
  requestId: string
  title: string
  step: string
  apiStatus: string[]
  chip: RegExp
  apiWaitMs: number
}

/** Waits for the service status, then for the status chip. Returns how long the screen lagged behind the service. */
export async function expectScreenFollowsService(
  page: Page,
  rpc: GatewayRpcClient,
  gaps: GapReporter,
  sync: StatusSync
): Promise<number> {
  await waitForApiStatus(rpc, sync.requestId, sync.apiStatus, sync.apiWaitMs)
  const reachedAt = Date.now()
  const onScreen = await expect(detailHeader(page))
    .toContainText(sync.chip, { timeout: UI_REFRESH_GRACE_MS })
    .then(() => true)
    .catch(() => false)
  const lag = Date.now() - reachedAt
  if (!onScreen) {
    const shown = (
      await detailHeader(page)
        .innerText()
        .catch(() => '')
    )
      .replace(/\s+/g, ' ')
      .slice(0, 120)
    const now = await requestState(rpc, sync.requestId)
    gaps.record(
      `UI-STALE-${sync.step}`,
      `service is ${now.status} but the detail header still shows "${shown}" after ${UI_REFRESH_GRACE_MS / 1000}s; needed a reload`
    )
    await reloadAndReopen(page, sync.title)
    await expect(detailHeader(page)).toContainText(sync.chip, { timeout: 15_000 })
  }
  return lag
}
