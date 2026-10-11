/**
 * The task sheet of a task the real agent has just run on the user's repo: the Result tab holds what the agent printed, the idle
 * agent panel points to it, the sheet fits narrow and wide windows, and Escape closes only the sheet.
 */
import { expect } from '@playwright/test'
import { closeTaskSheet, openTaskSheetFromPlan, taskSheet } from './live-agent-session-ui'
import type { DeepseekFixture } from './deepseek-repo-fixture'
import type { LiveFlow } from './live-flow-harness'
import { detailHeader, reloadAndReopen } from './live-request-ui-steps'

const WINDOWS = [
  { name: 'wide', width: 1440 },
  { name: 'narrow', width: 700 },
  { name: 'phone', width: 390 }
]

export async function expectTaskSheetAfterRun(f: LiveFlow, taskId: string): Promise<void> {
  const { page } = f
  await f.step('the task sheet after the run: Result tab, window fit, Escape', async () => {
    const sheet = taskSheet(page)
    const original = page.viewportSize()
    try {
      for (const w of WINDOWS) {
        // Why: resizing with the sheet open reflows the page and drops it; a user opens it in the window they have.
        await page.setViewportSize({ width: w.width, height: 900 })
        expect(
          await openTaskSheetFromPlan(page, taskId),
          `${w.name} window: the Plan tab has a row for the task`
        ).toBe(true)
        await expect(sheet, `${w.name} window: the sheet is open`).toBeVisible()
        // Why: measure after the slide-in animation has finished, or the sheet is still partly outside the window.
        await sheet.evaluate((el) => Promise.all(el.getAnimations().map((a) => a.finished)))
        const fit = await sheet.evaluate((el) => {
          const box = el.getBoundingClientRect()
          return {
            overflowX: el.scrollWidth - el.clientWidth,
            right: box.right,
            vw: window.innerWidth
          }
        })
        expect(
          fit.overflowX,
          `${w.name} (${w.width}px): nothing scrolls sideways in the sheet`
        ).toBeLessThanOrEqual(1)
        expect(
          fit.right,
          `${w.name} (${w.width}px): the sheet stays inside the window`
        ).toBeLessThanOrEqual(fit.vw + 1)
        const box = (await sheet.boundingBox())!
        for (const id of ['run-agent-btn', 'open-in-git-btn', 'task-title-input']) {
          const el = sheet.getByTestId(id)
          // Why: a button that is not on the sheet (e.g. no worktree yet) is nothing to measure; boundingBox would wait 30 s for it.
          const b = (await el.count()) > 0 ? await el.first().boundingBox() : null
          if (b) {
            expect(b.x, `${w.name}: ${id} starts inside the sheet`).toBeGreaterThanOrEqual(
              box.x - 1
            )
            expect(b.x + b.width, `${w.name}: ${id} ends inside the sheet`).toBeLessThanOrEqual(
              box.x + box.width + 1
            )
          }
        }
        await expect(
          sheet.getByRole('button', { name: 'Close' }).first(),
          `${w.name}: the Close button is reachable`
        ).toBeInViewport()
        await closeTaskSheet(page)
      }
    } finally {
      if (original) {
        await page.setViewportSize(original)
      }
    }
    expect(
      await openTaskSheetFromPlan(page, taskId),
      'the sheet opens again at the usual size'
    ).toBe(true)
    const panel = sheet.getByTestId('task-agent-activity')
    await expect(panel, 'the idle agent panel is not "running" any more').not.toHaveAttribute(
      'data-state',
      'running'
    )
    const pointer = panel.getByTestId('task-agent-activity-open-result')
    await (
      (await pointer.isVisible()) ? pointer : sheet.getByRole('tab', { name: 'Result' })
    ).click()
    await expect(sheet.getByRole('tab', { name: 'Result' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    const output = sheet.getByTestId('execution-agent-output')
    await expect(output, 'the Result tab shows what the agent printed').toBeVisible({
      timeout: 30_000
    })
    expect(
      (await output.innerText()).trim().length,
      'the agent output is not empty'
    ).toBeGreaterThan(0)
    await f.shots.take('task-sheet-result')
    await page.keyboard.press('Escape')
    await expect(sheet, 'Escape closes the sheet').toHaveCount(0)
    await expect(
      detailHeader(page),
      'the Request stays selected on the Requests page'
    ).toBeVisible()
  })
}

/** The worktree of a finished task is gone from the host: "Open in Git" says so instead of staying silent, and the sheet stays usable. */
export async function expectGoneWorktreeIsExplained(
  f: LiveFlow,
  fx: DeepseekFixture,
  taskId: string,
  requestTitle: string
): Promise<void> {
  const { page } = f
  await f.step('"Open in Git" explains a task worktree that is no longer on the host', async () => {
    for (const done of fx.removeTaskWorktrees()) {
      console.log(`[deepseek] removed for the check: ${done}`)
    }
    // Why: the sidebar keeps what it listed before; a user who finds the worktree gone reloads and opens the Request again.
    await reloadAndReopen(page, requestTitle)
    expect(await openTaskSheetFromPlan(page, taskId), 'the Plan tab still has the task row').toBe(
      true
    )
    await taskSheet(page).getByTestId('open-in-git-btn').click()
    await expect(
      page.getByText(/This task's worktree (is no longer on this host|no longer exists)/)
    ).toBeVisible({ timeout: 30_000 })
    await expect(taskSheet(page).getByTestId('open-in-git-btn')).toBeEnabled()
    await f.shots.take('worktree-gone-explained')
    await closeTaskSheet(page)
  })
}
