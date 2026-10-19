/**
 * Records what the revision panel showed while a click was being handled. The "Analyzing version n..." line and a refused
 * revision's error are on screen for a short while only (the next refresh removes the panel or the line), so polling the
 * page would miss them; a MutationObserver sees every render.
 */
import type { Page } from '@playwright/test'

export type PanelSample = { status: string; error: string; input: string }

type RecorderWindow = {
  __revisionLog?: PanelSample[]
  __revisionObserver?: MutationObserver
}

/** Starts a fresh log; the page must not navigate afterwards, or the observer is gone with it. */
export async function startPanelRecorder(page: Page): Promise<void> {
  await page.evaluate(() => {
    const w = window as unknown as RecorderWindow
    w.__revisionObserver?.disconnect()
    const log: PanelSample[] = []
    w.__revisionLog = log
    const text = (id: string): string =>
      document.querySelector(`[data-testid="${id}"]`)?.textContent?.trim() ?? ''
    const read = (): void => {
      const field = document.querySelector<HTMLTextAreaElement>(
        '[data-testid="analysis-revision-input"]'
      )
      const sample = {
        status: text('analysis-revision-status'),
        error: text('analysis-revision-error'),
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
    w.__revisionObserver = observer
    read()
  })
}

export async function readPanelRecorder(page: Page): Promise<PanelSample[]> {
  return page.evaluate(() => (window as unknown as RecorderWindow).__revisionLog ?? [])
}
