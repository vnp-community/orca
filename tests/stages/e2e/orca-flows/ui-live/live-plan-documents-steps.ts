/**
 * Steps of the plan-documents spec (CR-REQ-045): the "Proposal" view of a native plan, the "Documents" view of an OpenSpec
 * change (read from rows planted by plan_documents_seed.py, because the fake claude cannot author an OpenSpec change), and the
 * real OpenSpec case that is only run on request.
 *
 * The seed is BE-REQ-TASK-045-09's. This file assumes its command line: `python3 plan_documents_seed.py seed <request-id>`
 * prints one JSON line (`secret`: the planted fake secret, `omittedPath`: the file kept only as a placeholder) and
 * `python3 plan_documents_seed.py clean <request-id>` removes what it planted. Absent fields only skip the check that needs them.
 */
import { execFileSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, type Locator, type Page } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import type { PlanCalls } from './live-plan-revision-steps'
import { detailPane, openDetailTab, reloadAndReopen } from './live-request-ui-steps'
import { UI_REFRESH_GRACE_MS } from './live-status-sync'

export const SEED_SCRIPT = resolve(__dirname, '../plan_documents_seed.py')
/** Machine markers, key shapes: nothing that looks like a credential may reach the screen. */
export const SECRET_SHAPES =
  /AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|\bsk-[A-Za-z0-9]{20,}/

const planToggle = (page: Page, name: 'Documents' | 'Proposal'): Locator =>
  detailPane(page).getByRole('radio', { name, exact: true })
const documentTabs = (page: Page): Locator => detailPane(page).getByTestId('plan-document-tabs')

export function seedAvailable(): boolean {
  return existsSync(SEED_SCRIPT)
}

/** The Proposal view of a native plan: summary, notes and every task with what the tree leaves out. Not Documents. */
export async function expectNativeProposal(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await f.step(
    'a native plan offers Proposal, not Documents, and it shows summary, notes and tasks',
    async () => {
      await openDetailTab(f.page, 'Plan')
      await expect(planToggle(f.page, 'Proposal')).toBeVisible({ timeout: UI_REFRESH_GRACE_MS })
      await expect(
        planToggle(f.page, 'Documents'),
        'no Documents entry on the native engine'
      ).toHaveCount(0)
      await planToggle(f.page, 'Proposal').click()
      const view = detailPane(f.page).getByTestId('plan-proposal-view')
      await expect(view).toBeVisible({ timeout: UI_REFRESH_GRACE_MS })
      await expect(view.getByRole('heading').first(), 'the proposal has a title').not.toBeEmpty()
      await expect(view.getByTestId('proposal-notes'), 'the notes are shown').toBeVisible()
      const task = view.getByTestId('proposal-task-1')
      await expect(task, 'the first task is shown with its number').toContainText('1.')
      await expect(
        view.getByTestId('proposal-task-2'),
        'a later task says what it waits for'
      ).toContainText(/Depends on #1/)
      await expect(view.getByTestId('proposal-depends').first()).toBeVisible()
      const answer = await f.rpc.call<{
        proposal?: {
          title?: string
          tasks?: { promptTemplate?: string }[]
          phases?: { tasks?: { promptTemplate?: string }[] }[]
        }
      }>('request.planProposal', { id: r.requestId })
      // Why: the prompt is shown only for a task that has one, and the fake claude's tasks carry none.
      const hasPrompt = [
        ...(answer.proposal?.tasks ?? []),
        ...(answer.proposal?.phases ?? []).flatMap((phase) => phase.tasks ?? [])
      ].some((task) => (task.promptTemplate ?? '').trim() !== '')
      const promptEntry = view.getByText('Agent prompt')
      await (hasPrompt
        ? expect(promptEntry.first(), 'the prompt is there, closed').toBeVisible()
        : expect(promptEntry, 'no task has a prompt to show').toHaveCount(0))
      await expect(view).toContainText(answer.proposal?.title ?? '')
      expect(
        await view.getByRole('button', { name: /edit|save|run/i }).count(),
        'the proposal is read only'
      ).toBe(0)
      await f.shots.take('plan-proposal-native')
    }
  )
}

export type SeedFacts = { secret?: string; omittedPath?: string; omittedSize?: number }

/** Plants an OpenSpec change on a Request that is awaiting plan approval; returns what the seed says it planted. */
export function plantOpenspecChange(requestId: string): SeedFacts {
  const out = execFileSync('python3', [SEED_SCRIPT, 'seed', requestId], {
    encoding: 'utf8',
    timeout: 60_000
  })
  const last = out.trim().split('\n').at(-1) ?? ''
  try {
    return JSON.parse(last) as SeedFacts
  } catch {
    return {}
  }
}

export function removePlantedChange(requestId: string): void {
  execFileSync('python3', [SEED_SCRIPT, 'clean', requestId], { encoding: 'utf8', timeout: 60_000 })
}

/** Opens a document tab by its visible name and returns the body locator. */
async function openDocument(f: LiveFlow, name: string | RegExp): Promise<Locator> {
  await documentTabs(f.page).getByRole('tab', { name }).click()
  return detailPane(f.page).getByRole('tabpanel')
}

/**
 * The Documents view of a planted change: the tabs in reading order, a document read only when its tab is opened, tasks.md
 * rendered with check boxes and as source with its machine marker, a file too large for display, the checks strip, and no secret.
 */
export async function expectOpenspecDocuments(
  f: LiveFlow,
  r: RequestUnderTest,
  facts: SeedFacts,
  calls: PlanCalls
): Promise<void> {
  await f.step('an OpenSpec plan offers Documents with the change in reading order', async () => {
    await reloadAndReopen(f.page, r.title)
    await openDetailTab(f.page, 'Plan')
    await expect(planToggle(f.page, 'Documents')).toBeVisible({ timeout: UI_REFRESH_GRACE_MS })
    await expect(
      planToggle(f.page, 'Proposal'),
      'no Proposal entry on the OpenSpec engine'
    ).toHaveCount(0)
    const readsBefore = calls.count('request.planBundleFile')
    await planToggle(f.page, 'Documents').click()
    await expect(detailPane(f.page).getByTestId('plan-documents-view')).toBeVisible({
      timeout: UI_REFRESH_GRACE_MS
    })
    const names = await documentTabs(f.page).getByRole('tab').allInnerTexts()
    const order = ['Proposal', 'Design', 'Tasks'].map((n) =>
      names.findIndex((t) => t.startsWith(n))
    )
    expect(
      order.every((i) => i >= 0),
      `the change has proposal, design and tasks (tabs: ${names.join(' | ')})`
    ).toBe(true)
    expect(order[0] < order[1] && order[1] < order[2], 'proposal, design, then tasks').toBe(true)
    expect(names.length, 'a spec tab sits between design and tasks').toBeGreaterThanOrEqual(4)
    await expect
      .poll(() => calls.count('request.planBundleFile') - readsBefore, {
        message: 'only the open document is read'
      })
      .toBe(1)
    await f.shots.take('plan-documents-openspec')

    await openDocument(f, /^Tasks/)
    const checkboxes = detailPane(f.page).getByRole('checkbox')
    await expect(
      checkboxes.first(),
      'the tasks are rendered as a list with check boxes'
    ).toBeVisible()
    await expect(checkboxes.first()).toBeDisabled()
    expect(
      calls.count('request.planBundleFile') - readsBefore,
      'one more read for the second tab'
    ).toBe(2)
    await detailPane(f.page).getByRole('radio', { name: 'Source' }).click()
    await expect(detailPane(f.page).getByTestId('plan-document-source')).toContainText(
      'orca:begin plan'
    )
    await detailPane(f.page).getByRole('radio', { name: 'Rendered' }).click()
    await expect(detailPane(f.page)).not.toContainText('orca:begin plan')
    await openDocument(f, /^Proposal/)
    expect(
      calls.count('request.planBundleFile') - readsBefore,
      'a tab opened again is not read again'
    ).toBe(2)
  })
  await f.step(
    'a file too large to display says so with its size, and no secret is on screen',
    async () => {
      if (facts.omittedPath) {
        await openDocument(
          f,
          new RegExp(escape(facts.omittedPath.replace(/^specs\/|\/spec\.md$/g, '')))
        )
        await expect(detailPane(f.page).getByTestId('plan-document-omitted')).toContainText(
          /too large to display \(\d[\d.]* (B|KB|MB)\)/
        )
      } else {
        f.testInfo.annotations.push({
          type: 'note',
          description: 'SEED-OMITTED-UNKNOWN: the seed named no omitted file'
        })
      }
      for (const name of await documentTabs(f.page).getByRole('tab').allInnerTexts()) {
        await documentTabs(f.page)
          .getByRole('tab', { name: new RegExp(`^${escape(name.split('\n')[0])}`) })
          .click()
        const text = await detailPane(f.page).innerText()
        expect(text, `no credential shape in ${name}`).not.toMatch(SECRET_SHAPES)
        if (facts.secret) {
          expect(text, `the planted secret is masked in ${name}`).not.toContain(facts.secret)
        }
      }
      await f.shots.take('plan-documents-secrets-masked')
    }
  )
  await f.step(
    'the checks strip shows what Orca checked and labels the agent report as the agent’s own',
    async () => {
      const list = detailPane(f.page).getByTestId('plan-checks-list')
      // Why: the strip opens by itself when a check needs review, and the planted change carries warnings (no knowledge, skills unverified).
      if (!(await list.isVisible().catch(() => false))) {
        await detailPane(f.page)
          .getByRole('button', { name: /^Checks/ })
          .click()
      }
      await expect(list).toContainText('STRUCTURE_OK')
      const disclaimer = detailPane(f.page).getByTestId('plan-knowledge-disclaimer')
      if (await disclaimer.isVisible().catch(() => false)) {
        await expect(disclaimer).toContainText('Orca did not verify it')
      }
    }
  )
}

function escape(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
