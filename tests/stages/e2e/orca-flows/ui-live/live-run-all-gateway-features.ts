/**
 * Sits between the SPA and the real stack's WebSocket and removes `approval.runAll` from the `features` of the
 * `request.flowStatus` answer, which is exactly what a gateway older than CR-REQ-043 sends (it would silently drop the
 * `executionMode` of an approval). Everything else is forwarded untouched. Same technique as live-base-branch-gateway-features.ts.
 */
import type { Page } from '@playwright/test'

export const RUN_ALL_FEATURE = 'approval.runAll'

/** One answer frame, or several newline-separated ones; lines that are not a flowStatus answer are returned as they came. */
export function withoutRunAllFeature(raw: string): string {
  if (!raw.includes(RUN_ALL_FEATURE)) {
    return raw
  }
  return raw
    .split('\n')
    .map((line) => {
      if (!line.includes(RUN_ALL_FEATURE)) {
        return line
      }
      try {
        const frame = JSON.parse(line) as { result?: { enabled?: unknown; features?: unknown } }
        const result = frame.result
        if (typeof result?.enabled !== 'boolean' || !Array.isArray(result.features)) {
          return line
        }
        result.features = result.features.filter((f) => f !== RUN_ALL_FEATURE)
        return JSON.stringify(frame)
      } catch {
        return line
      }
    })
    .join('\n')
}

export type FeatureStripper = {
  /** Zero would mean the SPA never saw the older gateway and the spec proves nothing. */
  strippedAnswers: () => number
}

/** Must be installed before the page loads the app: the flow status is asked once, when the SPA starts. */
export async function advertiseNoRunAllFeature(page: Page): Promise<FeatureStripper> {
  let stripped = 0
  await page.routeWebSocket(/\/ws(\?|$)/, (client) => {
    const server = client.connectToServer()
    client.onMessage((message) => server.send(message))
    server.onMessage((message) => {
      if (typeof message !== 'string') {
        client.send(message)
        return
      }
      const rewritten = withoutRunAllFeature(message)
      if (rewritten !== message) {
        stripped++
      }
      client.send(rewritten)
    })
  })
  return { strippedAnswers: () => stripped }
}
