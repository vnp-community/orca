/**
 * The real OpenSpec case of the plan-documents spec (CR-REQ-045, type C): scenario 3 of the LedgerLite fixture ("charge a fee on
 * transfers", a change request) started from release/1.x, with the real Claude. One generated plan costs about 7 to 13 USD and a
 * revision about the same, so the spec runs this only with ORCA_E2E_OPENSPEC=1. Not run by whoever wrote it.
 */
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect } from '@playwright/test'
import { LEDGERLITE_FIXTURE_DIR, LEDGERLITE_RELEASE_BRANCH } from './fixture-repo-ledgerlite'
import { approveAnalysis } from './live-analysis-approval-steps'
import {
  chooseStartingBranch,
  openCreateDialog,
  submitCreateDialog
} from './live-base-branch-steps'
import { configureProject } from './live-continuous-execution-steps'
import type { LiveFlow } from './live-flow-harness'
import {
  answerClarificationIfAsked,
  cancelThroughTheUi,
  createClassifyAndConfirm
} from './live-flow-steps'
import { waitForApiStatus } from './live-gateway-crosschecks'
import { SECRET_SHAPES } from './live-plan-documents-steps'
import { openPlanApprovals, revisePlan } from './live-plan-revision-steps'
import { detailPane, openDetailTab } from './live-request-ui-steps'

const PLAN_WAIT_MS = 70 * 60_000

/** The body of scenario 3, read from the fixture's own document so the two cannot drift. */
function scenarioThreeBody(): string {
  const doc = readFileSync(join(LEDGERLITE_FIXTURE_DIR, 'docs', 'e2e-scenarios.md'), 'utf8')
  const section = doc.slice(doc.indexOf('## 3. Change request'))
  const body = /```text\n([\s\S]*?)\n```/.exec(section)?.[1]
  if (!body) {
    throw new Error('scenario 3 of docs/e2e-scenarios.md has no ```text body to paste')
  }
  return body
}

type FileRow = { path: string; change?: string }

export async function expectRealOpenspecPlan(f: LiveFlow): Promise<void> {
  const before = await configureProject(f, {
    engine: 'openspec',
    solutionAgent: true,
    executionMode: 'step_by_step'
  })
  try {
    const body = scenarioThreeBody()
    const r = await createClassifyAndConfirm(f, {
      titleWords: 'charge a fee on transfers',
      body,
      typeLabel: 'Change request',
      typeKey: 'change_request',
      size: 'M',
      create: async (title) => {
        await openCreateDialog(f.page, { title, body, projectName: f.project.name })
        await chooseStartingBranch(f.page, LEDGERLITE_RELEASE_BRANCH)
        await submitCreateDialog(f, title)
      }
    })
    const asked = await answerClarificationIfAsked(f, r)
    await approveAnalysis(f, r, {
      autoStarted: asked,
      next: { api: 'planning', chip: /Planning/i }
    })
    await f.step('the OpenSpec plan is written and Documents shows the whole change', async () => {
      await openDetailTab(f.page, 'Plan')
      await waitForApiStatus(f.rpc, r.requestId, ['awaiting_plan_approval'], PLAN_WAIT_MS)
      await detailPane(f.page).getByRole('radio', { name: 'Documents', exact: true }).click()
      const tabs = detailPane(f.page).getByTestId('plan-document-tabs').getByRole('tab')
      for (const name of [/^Proposal/, /^Design/, /^Tasks/]) {
        await expect(tabs.filter({ hasText: name })).toHaveCount(1, { timeout: 60_000 })
      }
      expect(
        await tabs.count(),
        'at least one spec delta between design and tasks'
      ).toBeGreaterThanOrEqual(4)
      for (const name of await tabs.allInnerTexts()) {
        await tabs
          .filter({ hasText: name.split('\n')[0] })
          .first()
          .click()
        expect(await detailPane(f.page).innerText(), `no credential in ${name}`).not.toMatch(
          SECRET_SHAPES
        )
      }
      await f.shots.take('plan-documents-openspec-real')
    })
    const approval1 = await openPlanApprovals(f, r.requestId)
    await revisePlan(f, r, {
      text: 'Add a task for the fee rounding tests.',
      version: 2,
      previousApprovals: approval1
    })
    await f.step('version 2 changed some files and left the others as they were', async () => {
      const second = await f.rpc.call<{ bundle?: { files?: FileRow[] } }>('request.planProposal', {
        id: r.requestId,
        revision: 2
      })
      const changes = (second.bundle?.files ?? []).map((x) => x.change ?? 'unchanged')
      expect(changes, 'at least one document changed').toContain('changed')
      expect(changes, 'the revision did not rewrite every document').toContain('unchanged')
      await f.shots.take('plan-documents-openspec-real-revised')
    })
    await cancelThroughTheUi(f, r, /Awaiting Plan Approval/i)
  } finally {
    await configureProject(f, before).catch(() => undefined)
  }
}
