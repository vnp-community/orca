/**
 * Add Project and worktrees on the user's repo deepseek-harness, through the UI, no AI (TASK-FE-E2E-028): the folder gets a project of
 * its own bound to the host, the same folder is refused (with and without a trailing slash) and "Open that project" opens it, the
 * sidebar shows the host of the repo, and a worktree made from the "+" of the project row is created next to the repo as
 * `<repo>-<name>` (the reported bug of TASK-FE-E2E-015, now on the real repo). Cleanup removes the worktree and its branch.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-ui
 */
import { existsSync } from 'node:fs'
import { expect, test } from '@playwright/test'
import { expectDuplicateIsRefused } from './deepseek-add-project-steps'
import { runDeepseekFlow } from './deepseek-flow-harness'
import { DEEPSEEK_REPO } from './deepseek-repo-fixture'
import { sidebarRepoHeader } from './live-add-project-own-project-steps'
import { createWorktreeFromProjectRow, waitForCreateOutcome } from './live-add-project-repro-steps'
import { isLiveRun } from './live-stack-config'

test.describe("Add Project and worktrees on the user's repo (TASK-FE-E2E-028)", () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('own project, duplicates refused, the host shown, a worktree created next to the repo', async ({
    page,
    context
  }, testInfo) => {
    await runDeepseekFlow(
      { page, context, testInfo, slug: 'deepseek-ui', mode: 'readonly' },
      async (f, run) => {
        await expectDuplicateIsRefused(f, run.tap, run.project)
        await f.step('the sidebar shows the host the repo is bound to', async () => {
          const header = sidebarRepoHeader(page, f.project.name)
          await expect(header).toBeVisible()
          const label = header.getByTestId('repo-host-label')
          await expect(label, 'the repo row names its host').toHaveText(f.cfg.devServerName)
          await expect(label, "the host is the repo's own, not inherited").not.toHaveAttribute(
            'data-host-inherited',
            'true'
          )
          await expect(header.getByTestId('repo-host-unbound')).toHaveCount(0)
        })
        const name = `e2e-${f.cfg.runPrefix.replace(/^e2e-ui-/, '')}`
        const expected = `${DEEPSEEK_REPO}-${name}`
        await f.step(
          `"+" on the project row creates worktree "${name}" next to the repo`,
          async () => {
            await createWorktreeFromProjectRow(page, f.project.name, name)
            const outcome = await waitForCreateOutcome(page, expected)
            await f.shots.take('worktree-created')
            expect(outcome, `the worktree was created at ${expected}`).toEqual({ kind: 'created' })
            expect(existsSync(`${expected}/.git`), 'the worktree is a real git checkout').toBe(true)
            expect(run.fx.worktrees(), 'git lists the new worktree').toContain(expected)
            await expect(
              page.getByRole('option', { name: new RegExp(name) }).first(),
              'the sidebar lists the new worktree'
            ).toBeVisible({ timeout: 30_000 })
          }
        )
      }
    )
  })
})
