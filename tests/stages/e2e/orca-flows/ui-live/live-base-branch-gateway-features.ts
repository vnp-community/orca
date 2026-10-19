/**
 * Sits between the SPA and the real stack's WebSocket and removes `request.baseRef` from the `features` of the
 * `request.flowStatus` answer, which is exactly what a gateway older than CR-REQ-039 sends. Everything else is forwarded
 * untouched, so the stack under test stays real. Same technique as tests/e2e/workflow-web/support/workflow-socket-fault-injection.ts.
 */
import type { Page } from '@playwright/test'

export const BASE_REF_FEATURE = 'request.baseRef'

/** One answer frame, or several newline-separated ones; lines that are not a flowStatus answer are returned as they came. */
export function withoutBaseRefFeature(raw: string): string {
  if (!raw.includes(BASE_REF_FEATURE)) {
    return raw
  }
  return raw
    .split('\n')
    .map((line) => {
      if (!line.includes(BASE_REF_FEATURE)) {
        return line
      }
      try {
        const frame = JSON.parse(line) as { result?: { enabled?: unknown; features?: unknown } }
        const result = frame.result
        if (typeof result?.enabled !== 'boolean' || !Array.isArray(result.features)) {
          return line
        }
        result.features = result.features.filter((f) => f !== BASE_REF_FEATURE)
        return JSON.stringify(frame)
      } catch {
        return line
      }
    })
    .join('\n')
}

export type FeatureStripper = {
  /** How many answers the proxy rewrote: zero would mean the SPA never saw the older gateway and the spec proves nothing. */
  strippedAnswers: () => number
}

/** Must be installed before the page loads the app: the flow status is asked once, when the SPA starts. */
export async function advertiseNoBaseRefFeature(page: Page): Promise<FeatureStripper> {
  let stripped = 0
  await page.routeWebSocket(/\/ws(\?|$)/, (client) => {
    const server = client.connectToServer()
    client.onMessage((message) => server.send(message))
    server.onMessage((message) => {
      if (typeof message !== 'string') {
        client.send(message)
        return
      }
      const rewritten = withoutBaseRefFeature(message)
      if (rewritten !== message) {
        stripped++
      }
      client.send(rewritten)
    })
  })
  return { strippedAnswers: () => stripped }
}
