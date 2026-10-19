/**
 * Holds back the gateway's push frames (request events) for one page, so the screen keeps showing a state the service has
 * already left: the way to make a reviewer act on a stale screen without racing the event. RPC answers pass through, and the
 * 15 s poll still refreshes the screen, so a hold is only meant to last a few seconds. Same technique as
 * tests/e2e/workflow-web/support/workflow-socket-fault-injection.ts.
 */
import type { Page } from '@playwright/test'

export class PushFrameGate {
  private held = false

  /** Must be installed before the page opens its socket (before the first goto). */
  static async install(page: Page): Promise<PushFrameGate> {
    const gate = new PushFrameGate()
    await page.routeWebSocket(/\/ws(\?|$)/, (client) => {
      const server = client.connectToServer()
      client.onMessage((message) => server.send(message))
      server.onMessage((message) => {
        if (typeof message === 'string' && gate.held) {
          const kept = message.split('\n').filter((line) => !line.includes('"streaming":true'))
          if (kept.join('').trim() === '') {
            return
          }
          client.send(kept.join('\n'))
          return
        }
        client.send(message)
      })
    })
    return gate
  }

  hold(): void {
    this.held = true
  }

  release(): void {
    this.held = false
  }
}
