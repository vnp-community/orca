/**
 * What the Plan tab and the graph show for the REAL Plan the AI wrote for the user's repo, compared with the task service's own
 * list: the phases and tasks, in order, with progress; a short reject reason that stays locked; the Tree | Graph toggle; and the
 * Request graph sheet with its Flow lens. Gateway reads only.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { detailPane, openDetailTab } from './live-request-ui-steps'

const PLAN_TASK_TYPES = ['plan', 'phase', 'task', 'bug', 'feature', 'epic']

type Row = { id: string; type: string; title: string; position?: number }

async function planRows(f: LiveFlow, r: RequestUnderTest): Promise<Row[]> {
  const res = await f.rpc.call<{ tasks?: Row[] }>('task.list', {
    projectId: f.project.id,
    requestId: r.requestId,
    taskTypes: PLAN_TASK_TYPES,
    pageSize: 100
  })
  return res.tasks ?? []
}

/** The Plan tab lists the same phases and tasks as the service, in order, with a progress bar. */
export async function expectPlanTabShape(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await f.step(
    'the Plan tab shows the Plan the service holds: phases in order, tasks, progress',
    async () => {
      await openDetailTab(f.page, 'Plan')
      const rows = await planRows(f, r)
      const phases = rows.filter((t) => t.type === 'phase')
      const leaves = rows.filter((t) => !['plan', 'phase'].includes(t.type))
      const panel = detailPane(f.page)
      await expect(
        panel.getByTestId('plan-progress-value'),
        'the Plan shows its progress'
      ).toBeVisible()
      await expect(panel.getByRole('progressbar', { name: 'Plan progress' })).toBeVisible()
      if (phases.length > 0) {
        await expect(
          panel.getByText(new RegExp(`${phases.length} phases?, ${leaves.length} tasks?`)),
          'the summary counts the phases and tasks of the service'
        ).toBeVisible()
        // Why: a Phase header is a button named by the Phase title (not "Phase 1: ..."); they come top to bottom in the service's order.
        const ordered = phases.toSorted((x, y) => (x.position ?? 0) - (y.position ?? 0))
        const tops: number[] = []
        for (const ph of ordered) {
          const header = panel.getByRole('button', { name: ph.title, exact: true })
          await expect(header, `Phase "${ph.title}" is listed`).toBeVisible()
          tops.push((await header.boundingBox())?.y ?? 0)
        }
        expect(tops, 'the Phases are listed in the service order').toEqual(
          tops.toSorted((x, y) => x - y)
        )
        await expect(
          panel.locator('[data-testid^="phase-summary-"]').first(),
          'a Phase shows how many of its tasks are done'
        ).toContainText(/\d+\/\d+ tasks done/)
      }
      for (const t of leaves) {
        await expect(
          panel.getByTestId(`plan-task-row-${t.id}`),
          `row of task "${t.title.slice(0, 40)}"`
        ).toBeVisible()
      }
      await f.shots.take('plan-tab-shape')
    }
  )
}

/** Reject on the Plan: a short reason keeps the button locked; Cancel sends nothing. */
export async function expectShortRejectStaysLocked(
  f: LiveFlow,
  sentRejects: () => number
): Promise<void> {
  await f.step('a short reject reason keeps Reject locked on the Plan tab', async () => {
    const before = sentRejects()
    await openDetailTab(f.page, 'Plan')
    await detailPane(f.page).getByRole('button', { name: 'Reject' }).first().click()
    const dialog = f.page.getByRole('dialog')
    await dialog.getByRole('textbox').fill('too short')
    await expect(dialog.getByRole('button', { name: 'Reject' })).toBeDisabled()
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await expect(dialog).toHaveCount(0)
    expect(sentRejects(), 'Cancel sent no approval.reject').toBe(before)
  })
}

/** Plan tab Tree | Graph: the graph shows the Plan lens on a canvas and hides the tree; Tree brings it back. */
export async function expectTreeGraphToggle(f: LiveFlow): Promise<void> {
  await f.step('the Plan tab toggles between Tree and Graph', async () => {
    await openDetailTab(f.page, 'Plan')
    const view = f.page.getByRole('group', { name: 'Plan view' })
    await view.getByRole('radio', { name: 'Graph' }).click()
    const panel = f.page.getByTestId('graph-panel')
    await expect(panel.getByRole('radio', { name: 'Plan' })).toHaveAttribute('aria-checked', 'true')
    await expect(panel.getByTestId('graph-canvas')).toBeVisible({ timeout: 30_000 })
    await f.shots.take('plan-graph')
    await view.getByRole('radio', { name: 'Tree' }).click()
    await expect(f.page.getByTestId('graph-panel')).toHaveCount(0)
  })
}

/** "View graph": the Flow lens draws the Request's stages, List shows the same as a table; the lenses the backend cannot serve stay disabled. */
export async function expectRequestGraphSheet(f: LiveFlow): Promise<void> {
  await f.step('"View graph" opens the Request graph with its Flow lens', async () => {
    await f.page.getByRole('button', { name: 'View graph' }).click()
    const sheet = f.page.getByTestId('request-graph-sheet')
    await expect(sheet).toBeVisible()
    await expect(sheet.getByRole('radio', { name: 'Flow' })).toHaveAttribute('aria-checked', 'true')
    await expect(sheet.getByTestId('graph-canvas')).toBeVisible({ timeout: 30_000 })
    await sheet.getByRole('radio', { name: 'List' }).click()
    await expect(sheet.getByTestId('graph-list')).toContainText(/Classification|Analysis/)
    await f.shots.take('request-graph-sheet')
    await f.page.keyboard.press('Escape')
    await expect(sheet).toHaveCount(0)
  })
}
