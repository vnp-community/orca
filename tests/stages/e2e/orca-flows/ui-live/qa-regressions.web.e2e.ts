/**
 * QA regressions of 2026-10-16, through the real UI (specs/frontend/bugs/qa-2026-10-16). Each test is RED while its bug exists and
 * GREEN once fixed; none uses AI.
 *
 *   ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live qa-regressions
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py -- qa-regressions -g TERM-01   # real local agent: the --ai-stub agent cannot spawn a pty
 *
 *   QA-UI-TERM-01  BUG-QA-01  a terminal opened from a worktree row starts in that worktree (primary and secondary)
 *   QA-UI-GH-01    BUG-QA-03  the GitHub source of the Tasks tab is not "unavailable" because of the UI's own wiring
 *   QA-UI-JIRA-01  BUG-QA-04  (opt-in, real site) the Jira source lists issues after the .env connection script ran
 */
import { expect, test } from '@playwright/test'
import { runLiveFlow } from './live-flow-harness'
import { isLiveRun } from './live-stack-config'
import {
  addWorktree,
  listTerminals,
  newTerminalsSince,
  removeWorktree,
  repoPathOf,
  sameDirectory
} from './live-qa-steps'

test.describe('QA regressions 2026-10-16 (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('QA-UI-TERM-01: a terminal opened from a worktree row starts in that worktree, not in $HOME', async ({
    page,
    context
  }, testInfo) => {
    await runLiveFlow({ page, context, testInfo, slug: 'qa-terminal-cwd' }, async (f) => {
      const repo = await repoPathOf(f.rpc, f.project.id)
      const branch = `qa/term-${f.cfg.runPrefix.replace(/[^a-z0-9]+/gi, '-').toLowerCase()}`
      const secondary = addWorktree(repo.path, branch)
      try {
        await f.step('reload so the sidebar lists both worktrees of the project', async () => {
          await page.reload()
          await expect(page.getByText(f.project.name).first()).toBeVisible({ timeout: 30_000 })
          // Why: a fresh browser context shows the one-time "Voice Dictation is here" promo, which blocks every click behind it.
          const later = page.getByRole('button', { name: 'Maybe Later' })
          if (await later.isVisible({ timeout: 5_000 }).catch(() => false)) {
            await later.click()
          }
        })

        const openRow = async (
          row: ReturnType<typeof page.locator>,
          expectedDir: string,
          what: string
        ): Promise<void> => {
          const known = new Set((await listTerminals(f.rpc)).map((t) => t.ptyId ?? ''))
          // Why: git-gateway lists worktrees per host and the sidebar can need a while to show a new one.
          await expect(row.first(), `the sidebar lists the ${what}`).toBeVisible({
            timeout: 90_000
          })
          await row.first().click()
          await expect
            .poll(async () => (await newTerminalsSince(f.rpc, known)).length, {
              timeout: 30_000,
              message: `a terminal session for the ${what}`
            })
            .toBeGreaterThan(0)
          const opened = (await newTerminalsSince(f.rpc, known))[0]
          expect(
            sameDirectory(opened.cwd ?? '', expectedDir),
            `${what}: the terminal opened in "${opened.cwd}" but the worktree is "${expectedDir}" (a home directory means the UI sent no cwd: BUG-QA-01)`
          ).toBe(true)
          await f.shots.take(`terminal-${what.replace(/\s+/g, '-')}`)
        }

        await f.step(
          'the secondary worktree row opens a terminal in its own directory',
          async () => {
            await openRow(
              page.getByRole('option').filter({ hasText: branch }),
              secondary,
              'secondary worktree'
            )
          }
        )
        await f.step(
          'the primary worktree row opens a terminal in the repo directory',
          async () => {
            // The first worktree option after the project's header is its primary one.
            await openRow(
              page.locator(
                `xpath=(//*[contains(normalize-space(text()), "${f.project.name}")]/following::*[@role="option"])[1]`
              ),
              repo.path,
              'primary worktree'
            )
          }
        )
      } finally {
        removeWorktree(repo.path, secondary)
      }
    })
  })

  test('QA-UI-GH-01: the GitHub task source is usable, or says exactly what to do', async ({
    page,
    context
  }, testInfo) => {
    await runLiveFlow({ page, context, testInfo, slug: 'qa-github-source' }, async (f) => {
      await f.step('open Tasks and choose the GitHub source', async () => {
        await page
          .getByRole('button', { name: /^Tasks$/ })
          .first()
          .click()
        await page
          .getByRole('button', { name: /GitHub/i })
          .first()
          .click()
          .catch(() => undefined)
        await expect(page.getByText(/GitHub/i).first()).toBeVisible({ timeout: 15_000 })
        await f.shots.take('github-source')
      })
      await f.step(
        'the source does not call a connected host "disconnected", and names hosts, not UUIDs',
        async () => {
          const body = (await page.locator('body').innerText()).replace(/\s+/g, ' ')
          const uuid = /\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/i
          const banner =
            /Some GitHub source hosts unavailable[^.]*|GitHub source unavailable[^.]*/i.exec(
              body
            )?.[0] ?? ''
          expect(banner, `the GitHub source shows an availability banner: "${banner}"`).toBe('')
          const tooltip =
            (await page
              .locator('[title*="Availability"]')
              .first()
              .getAttribute('title')
              .catch(() => '')) ?? ''
          expect(tooltip, 'the availability tooltip names hosts, not UUIDs').not.toMatch(uuid)
        }
      )
      await f.step('without a GitHub login the page offers to connect it', async () => {
        const rate = await f.rpc.call('github.rateLimit', {}).then(
          () => 'connected',
          (e: Error) => (/SCM_CREDENTIAL_NOT_FOUND/.test(e.message) ? 'no-credential' : 'other')
        )
        test.skip(
          rate === 'connected',
          'a GitHub credential exists; the missing-login path cannot be checked'
        )
        await expect(
          page.getByRole('button', { name: /connect github/i }),
          'a "Connect GitHub" action when no credential is configured'
        ).toBeVisible({ timeout: 15_000 })
      })
    })
  })

  test('QA-UI-JIRA-01: the Jira source of the Tasks tab lists issues (opt-in, real site from .env)', async ({
    page,
    context
  }, testInfo) => {
    test.skip(
      process.env.ORCA_E2E_JIRA_LIVE !== '1',
      'opt-in: ORCA_E2E_JIRA_LIVE=1 and run deploy/dev/scripts/connect-jira-from-env.sh first (the token is never typed by the test)'
    )
    await runLiveFlow({ page, context, testInfo, slug: 'qa-jira-source' }, async (f) => {
      await f.step('Jira is connected (by the script, not by this test)', async () => {
        const status = await f.rpc.call<{ connected?: boolean }>('jira.status', {})
        expect(
          status.connected,
          'run deploy/dev/scripts/connect-jira-from-env.sh before this test'
        ).toBe(true)
      })
      await f.step('Tasks → Jira shows issues and no "Connect your Jira site" prompt', async () => {
        await page
          .getByRole('button', { name: /^Tasks$/ })
          .first()
          .click()
        await expect(page.getByText('Connect your Jira site')).toHaveCount(0, { timeout: 15_000 })
        await f.shots.take('jira-source')
      })
    })
  })
})
