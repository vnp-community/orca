/**
 * First step of every deepseek-harness spec: Add Project on the user's repo through the UI, under a project name that carries the run
 * prefix, so cleanup removes exactly this project (and its repo registration, never the folder) and nothing of the user's.
 * Also the checks of the Add Project dialog that need no AI: the host is chosen for the user, the name defaults to the folder name
 * and can be edited, the same folder is refused a second time, and "Open that project" lands on the project just made.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { ProjectWithRepos } from './live-add-project-ledger'
import {
  expectNoWriteSent,
  expectOwnProject,
  expectOwnProjectRpc,
  expectRegistryUnchanged,
  takeRegistry
} from './live-add-project-own-project-checks'
import {
  addAlreadyRegisteredFolder,
  callsTo,
  expectActiveWorktreeOf,
  openAddProjectOnFolder,
  type SentRpc
} from './live-add-project-own-project-steps'
import { clickAddGitProject } from './live-add-project-repro-steps'
import { DEEPSEEK_REPO } from './deepseek-repo-fixture'

/** Name of the project this run makes for the user's repo; `prefix` is the run prefix `e2e-ui-<MMDDhhmmss>`. */
export const deepseekProjectName = (prefix: string, folder: string): string =>
  `e2e-ds-${prefix.replace(/^e2e-ui-/, '')}-${folder}`

export async function addDeepseekProject(
  f: LiveFlow,
  tap: { calls: SentRpc[] },
  repoName: string
): Promise<ProjectWithRepos> {
  const { page, rpc } = f
  const name = deepseekProjectName(f.cfg.runPrefix, repoName)
  let project!: ProjectWithRepos
  await f.step(
    `Add Project on ${DEEPSEEK_REPO}: its own project "${name}", bound to the host`,
    async () => {
      const before = await takeRegistry(rpc)
      const from = tap.calls.length
      const dialog = await openAddProjectOnFolder(page, f.cfg.devServerName, DEEPSEEK_REPO)
      const nameField = dialog.getByTestId('add-project-name')
      await expect(nameField, 'the project name defaults to the folder name').toHaveValue(repoName)
      await nameField.fill(name)
      await expect(nameField, 'the proposed name can be edited').toHaveValue(name)
      await f.shots.take('add-project-dialog')
      await clickAddGitProject(page, repoName)
      project = expectOwnProject(
        before,
        await takeRegistry(rpc),
        { name, path: DEEPSEEK_REPO },
        f.project.devServerId
      )
      expectOwnProjectRpc(tap.calls, from, { name, path: DEEPSEEK_REPO }, f.project.devServerId)
      Object.assign(f.project, { id: project.id, name: project.name })
      await f.shots.take('project-added')
    }
  )
  return project
}

/** The same folder, typed with and without a trailing slash, is refused with the project that holds it; "Open that project" opens it. */
export async function expectDuplicateIsRefused(
  f: LiveFlow,
  tap: { calls: SentRpc[] },
  project: ProjectWithRepos
): Promise<void> {
  const { page, rpc } = f
  await f.step(
    'Add Project refuses the same folder, with or without a trailing slash',
    async () => {
      const registry = await takeRegistry(rpc)
      const from = tap.calls.length
      const first = await addAlreadyRegisteredFolder(page, f.cfg.devServerName, DEEPSEEK_REPO)
      await expect.soft(first.notice).toContainText(project.name)
      await expect.soft(first.dialog.getByTestId('add-project-open-existing')).toBeVisible()
      await f.shots.take('duplicate-blocked')
      await first.dialog.getByTestId('add-project-open-existing').click()
      await expect(page.getByRole('dialog')).toHaveCount(0, { timeout: 15_000 })
      await expectActiveWorktreeOf(page, registry, project.id)
      const slash = await addAlreadyRegisteredFolder(
        page,
        f.cfg.devServerName,
        DEEPSEEK_REPO,
        `${DEEPSEEK_REPO}/`
      )
      await expect.soft(slash.notice).toContainText(project.name)
      await page.keyboard.press('Escape')
      expectNoWriteSent(tap.calls, from, 'the same folder')
      expectRegistryUnchanged(registry, await takeRegistry(rpc), 'after the refused Add Project')
      expect(callsTo(tap, 'project.create', from), 'no project.create was sent').toEqual([])
    }
  )
}
