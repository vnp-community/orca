/**
 * Known product gaps found while driving the real UI. A gap is a place where the UI (or the stack behind it) did not do
 * what the flow needs and the spec had to continue through the gateway instead. By default a gap fails the run through a
 * soft assertion (so every later step still executes and reports); `ORCA_E2E_LIVE_GAPS=annotate` only records it.
 */
import { expect, type TestInfo } from '@playwright/test'
import type { LiveStackConfig } from './live-stack-config'

export class GapReporter {
  readonly gaps: string[] = []

  constructor(
    private readonly testInfo: TestInfo,
    private readonly mode: LiveStackConfig['gapMode']
  ) {}

  record(id: string, description: string): void {
    const text = `${id}: ${description}`
    this.gaps.push(text)
    this.testInfo.annotations.push({ type: 'product-gap', description: text })
    console.log(`[gap] ${text}`)
    if (this.mode === 'fail') {
      expect.soft(false, `product gap ${text}`).toBe(true)
    }
  }
}
