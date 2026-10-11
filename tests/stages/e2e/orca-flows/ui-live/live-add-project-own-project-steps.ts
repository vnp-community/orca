/**
 * UI steps and evidence of the add-project spec (TASK-FE-E2E-027): Add Project for a folder, the "already added" block, picking the
 * project in Create request, and reading the answer of the Analysis tab. Everything the user does goes through the UI; the gateway
 * is read-only evidence, except the one deliberate write that probes the backend duplicate guard (see the spec).
 */
import { expect, type Locator, type Page } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import {
  browseAndSelectFolder,
  chooseHostInDialog,
  clickAddGitProject,
  openAddProjectDialog
} from './live-add-project-repro-steps'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { expectScreenFollowsService } from './live-status-sync'

/** What the SPA sent over its WebSocket (method + params only; the frame's auth token is dropped). */
export type SentRpc = { method: string; params: Record<string, unknown> }

export function tapSentRpc(page: Page): { calls: SentRpc[] } {
  const tap = { calls: [] as SentRpc[] }
  page.on('websocket', (ws) => {
    ws.on('framesent', (frame) => {
      try {
        const m = JSON.parse(String(frame.payload)) as {
          method?: string
          params?: SentRpc['params']
        }
        if (m.method) {
          tap.calls.push({ method: m.method, params: m.params ?? {} })
        }
      } catch {
        // not a JSON frame
      }
    })
  })
  return tap
}

export const callsTo = (tap: { calls: SentRpc[] }, method: string, from = 0): SentRpc[] =>
  tap.calls.slice(from).filter((c) => c.method === method)

export const sidebarRepoHeader = (page: Page, repoName: string): Locator =>
  page.locator('[data-repo-header-id]').filter({ hasText: repoName }).first()

/** "Open that project" opened THAT project: the active worktree row of the sidebar belongs to its repo. */
export async function expectActiveWorktreeOf(
  page: Page,
  registry: { projects: { id: string; repos: { id: string }[] }[] },
  projectId: string
): Promise<void> {
  const active = page.locator('[role="option"][aria-current="page"][data-worktree-id]')
  const repoId = registry.projects.find((p) => p.id === projectId)?.repos[0]?.id ?? ''
  await expect(active, 'a worktree row is active after "Open that project"').toHaveCount(1, {
    timeout: 15_000
  })
  const id = (await active.getAttribute('data-worktree-id')) ?? ''
  console.log(
    `[addproject] active worktree after "Open that project": ${id} (repo of the project: ${repoId})`
  )
  expect(id, 'the active worktree belongs to the repo of the project that was refused').toContain(
    repoId
  )
}

/** Opens the Add Project dialog, picks the host and the folder (Browse folder), and stops on the "Open host project" step. */
export async function openAddProjectOnFolder(
  page: Page,
  hostName: string,
  folderPath: string
): Promise<Locator> {
  await openAddProjectDialog(page)
  await chooseHostInDialog(page, hostName)
  await browseAndSelectFolder(page, folderPath)
  return page.getByRole('dialog')
}

/** Add Project for a folder, all the way through: the project name proposed is the folder name; ends with the dialog closed. */
export async function addProjectThroughTheUi(
  page: Page,
  hostName: string,
  folder: { path: string; name: string }
): Promise<void> {
  const dialog = await openAddProjectOnFolder(page, hostName, folder.path)
  await expect(
    dialog.getByTestId('add-project-name'),
    'the project name defaults to the folder name'
  ).toHaveValue(folder.name)
  await clickAddGitProject(page, folder.name)
}

/** Tries to add a folder that is already registered; returns the block notice (the dialog stays open). */
export async function addAlreadyRegisteredFolder(
  page: Page,
  hostName: string,
  folderPath: string,
  typedPath?: string
): Promise<{ dialog: Locator; notice: Locator }> {
  const dialog = await openAddProjectOnFolder(page, hostName, folderPath)
  if (typedPath) {
    // Why: a user can type the path with a trailing slash; it is the same folder and must be refused the same way.
    await page.getByPlaceholder('/home/user/project').fill(typedPath)
  }
  await dialog.getByRole('button', { name: 'Add Git Project' }).click()
  const notice = dialog.getByTestId('add-project-duplicate')
  await expect(notice, 'Add Project blocks a folder that is already added').toBeVisible({
    timeout: 30_000
  })
  return { dialog, notice }
}

/** Create request: the project chosen in the dialog decides the repo note; returns the note text of each project picked. */
export async function createRequestInProject(
  page: Page,
  input: { title: string; body: string; decoyProject: string; project: string }
): Promise<{ decoyNote: string; note: string }> {
  await page.getByRole('button', { name: 'Create request' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Title').fill(input.title)
  await dialog.getByLabel('Description').fill(input.body)
  const note = dialog.getByTestId('create-request-repo-note')
  const pick = async (project: string): Promise<string> => {
    await dialog.getByRole('combobox', { name: /project/i }).click()
    await page.getByRole('option', { name: project, exact: true }).click()
    await expect(note).toContainText(project, { timeout: 15_000 })
    return (await note.innerText()).replace(/\s+/g, ' ').trim()
  }
  // Why: picking another project first shows that the note follows the choice, instead of staying on the first project.
  const decoyNote = await pick(input.decoyProject)
  const chosen = await pick(input.project)
  await dialog.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(dialog).toBeHidden()
  return { decoyNote, note: chosen }
}

/**
 * Opens the Analysis tab of a `question` Request and waits for the answer (never approves it). Returns the text of the answer view
 * and of its citations as shown on screen.
 */
export async function readAnswerOnScreen(
  f: LiveFlow,
  r: RequestUnderTest
): Promise<{ view: string; citations: string[]; confidence: string }> {
  const { rpc, page, gaps, shots } = f
  await f.step('the Analysis tab shows the answer (not approved)', async () => {
    await openDetailTab(page, 'Analysis')
    const pane = detailPane(page)
    const start = pane.getByTestId('solution-generate')
    const view = pane.getByTestId('answer-view')
    const failed = pane.getByTestId('solution-generation-failed')
    await expect(
      start.or(view).or(pane.getByTestId('solution-generation-pending')).or(failed).first()
    ).toBeVisible({ timeout: 30_000 })
    if (await start.isVisible()) {
      const byItself = await expect(start)
        .toBeHidden({ timeout: 20_000 })
        .then(
          () => true,
          () => false
        )
      if (!byItself) {
        gaps.record(
          'ANALYSIS-NOT-AUTO-STARTED',
          '20s after the Clarification the Request is analyzing with no run; "Generate solution" was pressed instead'
        )
        await start.click()
      }
    }
    await expect(view.or(failed).first()).toBeVisible({ timeout: f.budgets.solutionMs })
    if (await failed.isVisible()) {
      await shots.take('analysis-failed')
      throw new Error(
        `the analysis run failed on screen: ${(await failed.innerText()).replace(/\s+/g, ' ').slice(0, 200)}`
      )
    }
    await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-ANSWER-READY',
      apiStatus: ['awaiting_analysis_approval'],
      chip: /Awaiting Analysis Approval/i,
      apiWaitMs: 60_000
    })
    await openDetailTab(page, 'Analysis')
    await expect(view).toBeVisible()
  })
  const view = detailPane(f.page).getByTestId('answer-view')
  const items = await view.getByTestId('answer-citations').locator('li').allInnerTexts()
  const confidence = await view
    .getByTestId('answer-confidence')
    .innerText()
    .catch(() => '')
  return {
    view: (await view.innerText()).replace(/\s+/g, ' ').trim(),
    citations: items.map((t) => t.replace(/\s+/g, ' ').trim()),
    confidence: confidence.replace(/\s+/g, ' ').trim()
  }
}
