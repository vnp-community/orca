/**
 * Records what the plan revision panel showed while a click was being handled. "Writing version n…" and a refused revision's
 * error are on screen for a short while only (the next refresh removes the line or the panel), so polling the page would miss
 * them; a MutationObserver sees every render. Same technique as live-analysis-revision-panel-recorder.ts, on the plan's test ids.
 */
import type { Page } from '@playwright/test'

export type PlanPanelSample = { status: string; error: string; input: string }

type RecorderWindow = {
  __planRevisionLog?: PlanPanelSample[]
  __planRevisionObserver?: MutationObserver
}

/** Starts a fresh log; the page must not navigate afterwards, or the observer is gone with it. */
export async function startPlanPanelRecorder(page: Page): Promise<void> {
  await page.evaluate(() => {
    const w = window as unknown as RecorderWindow
    w.__planRevisionObserver?.disconnect()
    const log: PlanPanelSample[] = []
    w.__planRevisionLog = log
    const text = (id: string): string =>
      document.querySelector(`[data-testid="${id}"]`)?.textContent?.trim() ?? ''
    const read = (): void => {
      const field = document.querySelector<HTMLTextAreaElement>(
        '[data-testid="plan-revision-input"]'
      )
      const sample = {
        status: text('plan-revision-status'),
        error: text('plan-revision-error'),
        input: field?.value ?? ''
      }
      const last = log.at(-1)
      if (!last || last.status !== sample.status || last.error !== sample.error) {
        log.push(sample)
      }
    }
    const observer = new MutationObserver(read)
    observer.observe(document.body, {
      subtree: true,
      childList: true,
      characterData: true,
      attributes: true
    })
    w.__planRevisionObserver = observer
    read()
  })
}

export async function readPlanPanelRecorder(page: Page): Promise<PlanPanelSample[]> {
  return page.evaluate(() => (window as unknown as RecorderWindow).__planRevisionLog ?? [])
}
