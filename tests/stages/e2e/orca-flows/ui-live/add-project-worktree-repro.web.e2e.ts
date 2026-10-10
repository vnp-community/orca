/**
 * Regression spec of the user's bug (TASK-FE-E2E-015, fixed by SOL-FE-E2E-004), driven through the real web UI, no shortcut:
 * sign in -> Add Project -> host dev-local-mac -> Browse folder -> pick a git repo -> Add Git Project -> "+" on the new project ->
 * worktree "test-02" -> Create. Before the fix the user saw "Couldn't create worktree ... WORKTREE_CREATE_FAILED ... chdir <repo>:
 * no such file or directory", because the repo was saved without the chosen dev server.
 *
 * The repo is a throwaway one made here (never a repo of the user). The final assertions describe the CORRECT behaviour: the worktree
 * is created, and the repo AND the project holding it are bound to the chosen dev server. "BUG REPRODUCED" is printed (with the
 * evidence) only if the original failure comes back.
 *
 *   ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live add-project-worktree-repro
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --repro add-project-worktree
 */
import { existsSync, writeFileSync } from 'node:fs'
import { expect, test } from '@playwright/test'
import { GatewayRpcClient, type RpcRow } from './gateway-rpc-client'
import {
  browseAndSelectFolder,
  BUG_PATTERN,
  chooseHostInDialog,
  clickAddGitProject,
  containerSeesPath,
  createReproRepo,
  createWorktreeFromProjectRow,
  gitGatewayLogLines,
  openAddProjectDialog,
  readRepoEvidence,
  removeReproRepoFromOrca,
  waitForCreateOutcome,
  WORKTREE_NAME,
  type CreateOutcome,
  type RepoEvidence
} from './live-add-project-repro-steps'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'
import {
  signInThroughTheForm,
  startTraceAfterSignIn,
  StepShots,
  stopTrace,
  watchBrowserNoise
} from './live-ui-session'

const log = (msg: string): void => console.log(`[repro] ${msg}`)

test.describe('Add a project and create a worktree, through the UI (TASK-FE-E2E-015)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test(`the repo added from host dev-local-mac accepts worktree "${WORKTREE_NAME}"`, async ({
    page,
    context
  }, testInfo) => {
    const cfg = loadLiveStackConfig()
    test.skip(!cfg.adminEmail || !cfg.adminPassword, 'no admin credentials (deploy/dev/.env)')
    test.setTimeout(5 * 60_000)
    const repo = createReproRepo()
    const rpc = new GatewayRpcClient(cfg)
    const noise = watchBrowserNoise(page)
    const shots = new StepShots(page, testInfo)
    const startedAt = new Date(Date.now() - 2_000)
    let traceOn = false
    let failed = true
    let hostLabel = ''
    let outcome = { kind: 'timeout', text: 'not reached' } as CreateOutcome
    let evidence: RepoEvidence | null = null
    let devServerId = ''
    let cleanupProblems: string[] = []
    log(`throwaway repo ${repo.path}`)
    try {
      await rpc.connect()
      const servers = await rpc.call<RpcRow[]>('devServer.list', {})
      const server = servers.find((s) => s.name === cfg.devServerName)
      expect(
        server?.status,
        `dev server '${cfg.devServerName}' must be connected (deploy/dev/scripts/start-local-agent.sh)`
      ).toBe('connected')
      devServerId = server!.id

      await test.step('1. sign in through the login form', async () => {
        await signInThroughTheForm(page, cfg)
        await startTraceAfterSignIn(context)
        traceOn = true
        await shots.take('1-signed-in')
      })
      await test.step('2. click Add Project (Projects header in the sidebar)', async () => {
        await openAddProjectDialog(page)
        await shots.take('2-add-project-dialog')
      })
      await test.step('3. choose the host dev-local-mac', async () => {
        hostLabel = await chooseHostInDialog(page, cfg.devServerName)
        log(`host selector shows: "${hostLabel}"`)
        await shots.take('3-host-chosen')
      })
      await test.step('4+5. Browse folder, pick the repo, Select folder', async () => {
        await browseAndSelectFolder(page, repo.path)
        await shots.take('4-folder-selected')
      })
      await test.step('5. Add Git Project', async () => {
        await clickAddGitProject(page, repo.name)
        await shots.take('5-project-added')
      })
      await test.step(`6. "+" on the new project, worktree "${WORKTREE_NAME}", Create worktree`, async () => {
        await createWorktreeFromProjectRow(page, repo.name, WORKTREE_NAME)
        outcome = await waitForCreateOutcome(page, repo.worktreePath)
        await shots.take('6-after-create-worktree')
      })

      // Evidence below is read-only.
      evidence = await readRepoEvidence(rpc, repo.path)
      const gw = gitGatewayLogLines(repo.name, startedAt)
      const seen = containerSeesPath(repo.path)
      log('---- evidence ----')
      log(`UI outcome: ${outcome.kind}${outcome.kind === 'created' ? '' : ` -> ${outcome.text}`}`)
      log(
        `repo.list: repo ${evidence?.repoId ?? '(not found)'} devServerId=${JSON.stringify(evidence?.devServerId ?? '')}`
      )
      log(
        `project holding the repo: "${evidence?.projectName ?? '?'}" (project devServerId=${JSON.stringify(evidence?.projectDevServerId ?? '')}); chosen host ${cfg.devServerName} = ${devServerId}`
      )
      log(`worktree.list for the repo: ${JSON.stringify(evidence?.worktrees)}`)
      log(
        `git-gateway container sees ${repo.path}: ${seen === null ? 'unknown (no docker)' : seen}`
      )
      for (const line of gw.slice(0, 6)) {
        log(`git-gateway log: ${line}`)
      }
      if (outcome.kind === 'error' && BUG_PATTERN.test(outcome.text)) {
        log(
          `BUG REPRODUCED: the UI shows WORKTREE_CREATE_FAILED with "chdir ${repo.path}: no such file or directory" although the directory exists on this machine (existsSync=${existsSync(repo.path)}).`
        )
        log(
          `CAUSE (read-only evidence): the repo was saved with devServerId=${JSON.stringify(evidence?.devServerId ?? '')} instead of ${devServerId}, so git-gateway ran git inside its own container${seen === false ? ' (the path does not exist there)' : ''}.`
        )
      } else if (outcome.kind === 'created') {
        log('NOT REPRODUCED: the worktree was created.')
      } else {
        log(`DIFFERENT OUTCOME (not the reported error): ${JSON.stringify(outcome)}`)
      }

      // The CORRECT behaviour; any of these going red is a regression of the host binding.
      expect
        .soft(outcome.kind, `worktree ${WORKTREE_NAME}: the UI must not show an error`)
        .toBe('created')
      expect
        .soft(existsSync(repo.worktreePath), `directory ${repo.worktreePath} exists on disk`)
        .toBe(true)
      expect
        .soft(
          (evidence?.worktrees ?? []).some((w) => w.includes(WORKTREE_NAME)),
          `worktree ${WORKTREE_NAME} is in the worktree list of the repo (got ${JSON.stringify(evidence?.worktrees)})`
        )
        .toBe(true)
      expect
        .soft(
          evidence?.devServerId,
          `the repo added from host ${cfg.devServerName} must be bound to that dev server (repo.list devServerId)`
        )
        .toBe(devServerId)
      // The project holding the repo follows the same host (one project per host), so repo and project never disagree.
      expect
        .soft(
          evidence?.projectDevServerId,
          `the project holding the repo must be bound to ${cfg.devServerName} too (project.list devServerId)`
        )
        .toBe(devServerId)
      failed = testInfo.errors.length > 0
    } finally {
      if (failed) {
        await page.screenshot({ path: testInfo.outputPath('99-at-failure.png') }).catch(() => {})
      }
      if (traceOn) {
        await stopTrace(context, testInfo, failed)
      }
      // Cleanup: only the repo of this run, on every path.
      try {
        cleanupProblems = await removeReproRepoFromOrca(rpc, repo.path)
      } catch (e) {
        cleanupProblems = [`orca cleanup failed: ${(e as Error).message.slice(0, 160)}`]
      }
      const leftovers = repo.dispose()
      rpc.close()
      for (const p of cleanupProblems) {
        log(`cleanup problem: ${p}`)
      }
      log(
        `cleanup: repo removed from Orca, temp dirs removed (left on disk: ${leftovers.length}), cleanup problems: ${cleanupProblems.length}`
      )
      writeFileSync(
        testInfo.outputPath('browser-noise.json'),
        JSON.stringify(
          {
            rpcErrors: noise.rpcErrors.slice(0, 40),
            consoleErrors: noise.consoleErrors.slice(0, 20)
          },
          null,
          2
        )
      )
      expect.soft(leftovers, 'no temp entry of the repro remains').toEqual([])
      expect.soft(cleanupProblems, 'no e2e-ui-repro-* repo/project remains in Orca').toEqual([])
    }
  })
})
