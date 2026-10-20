/**
 * The Plan half before the run: it must appear without the Generate button (CR-REQ-044), its Documents are read and photographed
 * with no secret on them (CR-REQ-045), and its tasks are read for anything that must not run on the owner's machine.
 */
import { expect } from '@playwright/test'
import { CICDKIT_REPO } from './ci-cd-kit-backlog'
import { unsafePlanText } from './ci-cd-kit-plan-safety'
import type { Timeline } from './ci-cd-kit-report'
import type { RpcRow } from './gateway-rpc-client'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'
import {
  planApproveButton,
  planFailedCard,
  planGenerate,
  planWriting,
  readPlanProposal
} from './live-auto-plan-steps'
import { SECRET_SHAPES } from './live-plan-documents-steps'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { findSecrets, loadSecretCatalog } from './live-secret-leak-scan'
import { expectScreenFollowsService } from './live-status-sync'

const MAX_LEAF_TASKS = 16
const PLAN_TASK_TYPES = ['plan', 'phase', 'task', 'bug', 'feature', 'epic']

export type AutoPlanSeen = {
  generateSeen: boolean
  writingSeen: boolean
  attempts: string[]
  waitMs: number
}

/** Waits for the Plan Orca writes by itself; the Generate button must never show, and a failed card ends the run (no Retry spent). */
export async function waitForAutoPlan(
  f: LiveFlow,
  r: RequestUnderTest,
  timeline: Timeline,
  limitMs: number
): Promise<AutoPlanSeen> {
  const seen: AutoPlanSeen = { generateSeen: false, writingSeen: false, attempts: [], waitMs: 0 }
  await f.step('the Plan appears by itself, without a click on Generate', async () => {
    const t0 = Date.now()
    await openDetailTab(f.page, 'Plan')
    let lastState = ''
    for (;;) {
      const req = await requestState(f.rpc, r.requestId)
      if (req.status === 'awaiting_plan_approval') {
        break
      }
      if (!['planning', 'awaiting_analysis_approval'].includes(String(req.status))) {
        throw new Error(
          `the Request left planning for ${String(req.status)} while the Plan was being written`
        )
      }
      if (
        await planGenerate(f.page)
          .isVisible()
          .catch(() => false)
      ) {
        if (!seen.generateSeen) {
          await f.shots.take('plan-generate-button-shown')
        }
        seen.generateSeen = true
      }
      if (
        await planWriting(f.page)
          .isVisible()
          .catch(() => false)
      ) {
        if (!seen.writingSeen) {
          await f.shots.take('plan-auto-writing')
        }
        seen.writingSeen = true
        const attempt = await detailPane(f.page)
          .getByTestId('plan-auto-attempt')
          .innerText({ timeout: 500 })
          .catch(() => '')
        if (attempt && !seen.attempts.includes(attempt)) {
          seen.attempts.push(attempt)
        }
      }
      if (
        await planFailedCard(f.page)
          .isVisible()
          .catch(() => false)
      ) {
        await f.shots.take('plan-run-failed')
        const text = (await planFailedCard(f.page).innerText()).replace(/\s+/g, ' ').slice(0, 300)
        throw new Error(
          `PLAN-RUN-FAILED: the Plan card shows a failed run after its automatic retries (${text})`
        )
      }
      const proposal = await readPlanProposal(f, r)
      const state = `${proposal?.status ?? 'none'}/${proposal?.autoPlan?.state ?? ''}/${proposal?.autoPlan?.attempt ?? 0}`
      if (state !== lastState) {
        lastState = state
        console.log(`[ci-cd-kit] plan run ${state} at ${Math.round((Date.now() - t0) / 1000)}s`)
      }
      if (proposal?.status === 'failed' && !proposal.autoPlan?.state) {
        throw new Error(`PLAN-RUN-FAILED: the Plan run failed (${proposal.errorCode ?? 'no code'})`)
      }
      expect(
        Date.now() - t0,
        `the Plan was written within ${Math.round(limitMs / 60_000)} min`
      ).toBeLessThan(limitMs)
      await f.page.waitForTimeout(2_000)
    }
    seen.waitMs = Date.now() - t0
    await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-PLAN-AUTO',
      apiStatus: ['awaiting_plan_approval'],
      chip: /Awaiting Plan Approval/i,
      apiWaitMs: 30_000
    })
    await expect(planApproveButton(f.page)).toBeVisible({ timeout: 45_000 })
    timeline.mark('plan written')
    await f.shots.take('plan-ready')
    expect(seen.generateSeen, 'the Generate button never showed while Orca wrote the Plan').toBe(
      false
    )
  })
  return seen
}

const slug = (text: string): string =>
  text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 40)

export type DocumentsSeen = { tabs: string[]; leaks: string[] }

/** Documents view: every tab is opened and read, screenshots of proposal, design, specs and tasks, and no credential on any of them. */
export async function readPlanDocuments(f: LiveFlow): Promise<DocumentsSeen> {
  const seen: DocumentsSeen = { tabs: [], leaks: [] }
  await f.step(
    'the Plan tab Documents view: proposal, design, specs, tasks, no secret',
    async () => {
      await openDetailTab(f.page, 'Plan')
      await detailPane(f.page).getByRole('radio', { name: 'Documents', exact: true }).click()
      await expect(detailPane(f.page).getByTestId('plan-documents-view')).toBeVisible({
        timeout: 45_000
      })
      const tabs = detailPane(f.page).getByTestId('plan-document-tabs').getByRole('tab')
      const names = (await tabs.allInnerTexts()).map((t) => t.split('\n')[0].trim())
      seen.tabs = names
      for (const want of [/^Proposal/, /^Design/, /^Tasks/]) {
        expect(
          names.some((n) => want.test(n)),
          `the change has a ${want} document (tabs: ${names.join(' | ')})`
        ).toBe(true)
      }
      const catalog = loadSecretCatalog()
      for (const [i, name] of names.entries()) {
        await tabs.nth(i).click()
        await f.page.waitForTimeout(700)
        const text = await detailPane(f.page).innerText()
        if (SECRET_SHAPES.test(text)) {
          seen.leaks.push(`credential shape in ${name}`)
        }
        seen.leaks.push(...findSecrets(catalog, text).map((l) => `${l} in ${name}`))
        if (i < 10) {
          await f.shots.take(`plan-document-${slug(name)}`)
        }
      }
      expect(seen.leaks, 'no credential shape or stack secret on any document').toEqual([])
    }
  )
  return seen
}

export type PlanTaskRow = { id: string; type: string; title: string; status: string }

function stringLeaves(value: unknown, out: string[] = []): string[] {
  if (typeof value === 'string') {
    out.push(value)
  } else if (Array.isArray(value)) {
    value.forEach((v) => stringLeaves(v, out))
  } else if (value && typeof value === 'object') {
    Object.values(value).forEach((v) => stringLeaves(v, out))
  }
  return out
}

/** The tasks of the Plan as the service holds them; refuses a Plan that is large or asks for something unsafe. */
export async function reviewPlanBeforeRunAll(
  f: LiveFlow,
  r: RequestUnderTest
): Promise<PlanTaskRow[]> {
  let tasks: PlanTaskRow[] = []
  await f.step('read the Plan tasks before approving (size and safety)', async () => {
    const res = await f.rpc.call<{ tasks?: RpcRow[] }>('task.list', {
      projectId: f.project.id,
      requestId: r.requestId,
      taskTypes: PLAN_TASK_TYPES,
      pageSize: 100
    })
    const rows = res.tasks ?? []
    tasks = rows.map((t) => ({
      id: String(t.id),
      type: String(t.type ?? ''),
      title: String(t.title ?? ''),
      status: String(t.status ?? '')
    }))
    const leaves = tasks.filter((t) => !['plan', 'phase'].includes(t.type))
    console.log(
      `[ci-cd-kit] plan: ${tasks.length} rows\n${tasks.map((t) => `  ${t.type}: ${t.title.slice(0, 110)}`).join('\n')}`
    )
    expect(leaves.length, 'the Plan has working tasks').toBeGreaterThan(0)
    expect(
      leaves.length,
      `the Plan stays within ${MAX_LEAF_TASKS} working tasks`
    ).toBeLessThanOrEqual(MAX_LEAF_TASKS)
    const unsafe = rows.flatMap((row) => {
      const hits = unsafePlanText(stringLeaves(row).join('\n'), [CICDKIT_REPO])
      return hits.map((h) => `${String(row.type)} "${String(row.title).slice(0, 60)}": ${h}`)
    })
    expect(
      unsafe,
      'no Plan task asks for a push, a docker/deploy command or a path outside the repo'
    ).toEqual([])
  })
  return tasks
}
