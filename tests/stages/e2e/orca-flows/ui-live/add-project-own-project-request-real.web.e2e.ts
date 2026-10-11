/**
 * Acceptance of the three fixes after the user's Request #1136 test (SOL-FE-E2E-005, TASK-FE-E2E-027), through the UI, with the REAL
 * Claude of the dev server: (2) Add Project makes a PROJECT of its own per repo, (3) the same folder is refused a second time, and a
 * Request can be created in that project; (1) the Analysis tab shows the answer (`answer_markdown`) instead of "No data".
 *
 * Two throwaway repos (alpha = decoy, bravo = the one asked about) under /private/tmp; never a repo or project of the user.
 * `ORCA_E2E_ADDPROJECT_STOP=projects` stops after the Add Project / duplicate checks (no AI), for a fast check of that half.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real addproject
 */
import { expect, test } from '@playwright/test'
import { GatewayRpcClient } from './gateway-rpc-client'
import {
  ADDPROJ_PREFIX,
  createAddProjectFixture,
  type AddProjectFixture
} from './add-project-two-repos-fixture'
import { createAddProjectLedger, type AddProjectLedger } from './live-add-project-ledger'
import {
  expectBackendRefusesDuplicate,
  expectNoWriteSent,
  expectOwnProject,
  expectOwnProjectRpc,
  expectRegistryUnchanged,
  takeRegistry
} from './live-add-project-own-project-checks'
import {
  addAlreadyRegisteredFolder,
  addProjectThroughTheUi,
  expectActiveWorktreeOf,
  sidebarRepoHeader,
  tapSentRpc
} from './live-add-project-own-project-steps'
import { requestAboutBravoSteps } from './live-add-project-request-about-bravo-steps'
import { connectedDevServer } from './live-project-fixture'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'
import {
  describeUserData,
  diffUserData,
  snapshotUserData,
  type UserDataSnapshot
} from './live-user-data-snapshot'
import { runRealFlow } from './real-flow-harness'

test.describe('Add Project makes its own project; Request in it; real Claude answers (TASK-FE-E2E-027)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('two folders -> two projects; the same folder is refused; a question about bravo is answered from bravo', async ({
    page,
    context
  }, testInfo) => {
    const cfg = loadLiveStackConfig()
    test.skip(
      cfg.aiMode === 'stub',
      'real-AI spec: never runs against the fake claude (drop --ai-stub)'
    )
    const stopAfterProjects = process.env.ORCA_E2E_ADDPROJECT_STOP === 'projects'

    // The user's own data, fingerprinted before anything is created; their repos join the git fingerprint of the safety net.
    const pre = new GatewayRpcClient(cfg)
    await pre.connect()
    const server = await connectedDevServer(pre, cfg)
    const userDataBefore: UserDataSnapshot = await snapshotUserData(pre, server.id)
    pre.close()
    process.env.ORCA_E2E_GUARD_PATHS = [
      process.env.ORCA_E2E_GUARD_PATHS,
      ...userDataBefore.repoPaths
    ]
      .filter(Boolean)
      .join(':')
    for (const line of describeUserData(userDataBefore)) {
      console.log(`[addproject] user data BEFORE: ${line}`)
    }

    // Why: registered before the flow so it also sees the WebSocket the SPA opens at sign-in.
    const tap = tapSentRpc(page)
    let fx: AddProjectFixture | undefined
    let ledger: AddProjectLedger | undefined
    await runRealFlow(
      {
        page,
        context,
        testInfo,
        slug: 'real-add-project',
        mode: 'readonly',
        createFixture: (prefix) => {
          fx = createAddProjectFixture(prefix)
          const stamp = prefix.replace(/^e2e-ui-/, '')
          ledger = createAddProjectLedger({
            namePrefix: `${ADDPROJ_PREFIX}${stamp}-`,
            paths: [fx.alpha.path, fx.bravo.path]
          })
          return fx
        },
        provision: (rpc, c) => ledger!.provision(rpc, c),
        afterCleanup: async () => {
          const rpc = new GatewayRpcClient(cfg)
          await rpc.connect()
          try {
            const after = await snapshotUserData(rpc, server.id)
            for (const line of describeUserData(after)) {
              console.log(`[addproject] user data AFTER:  ${line}`)
            }
            return [
              ...ledger!.problems(),
              ...(await ledger!.leftovers(rpc)),
              ...fx!.leftovers().map((n) => `temp entry not removed: ${n}`),
              ...diffUserData(userDataBefore, after).map((p) => `user data changed: ${p}`)
            ]
          } finally {
            rpc.close()
          }
        }
      },
      async (f, fixture) => {
        const { page: ui, rpc } = f
        const { alpha, bravo } = fixture as AddProjectFixture
        const heads = { alpha: alpha.headSha(), bravo: bravo.headSha() }
        const host = f.cfg.devServerName

        const added = {} as Record<'alpha' | 'bravo', { id: string; name: string }>
        for (const repo of [alpha, bravo]) {
          await f.step(`Add Project: ${repo.name} gets a project of its own`, async () => {
            const before = await takeRegistry(rpc)
            const from = tap.calls.length
            await addProjectThroughTheUi(ui, host, repo)
            const project = expectOwnProject(
              before,
              await takeRegistry(rpc),
              repo,
              f.project.devServerId
            )
            expectOwnProjectRpc(tap.calls, from, repo, f.project.devServerId)
            added[repo === alpha ? 'alpha' : 'bravo'] = project
            await f.shots.take(`project-added-${repo === alpha ? 'alpha' : 'bravo'}`)
          })
        }
        await f.step('the sidebar lists the two projects separately', async () => {
          await sidebarRepoHeader(ui, alpha.name).waitFor({ timeout: 15_000 })
          await sidebarRepoHeader(ui, bravo.name).waitFor({ timeout: 15_000 })
          await f.shots.take('sidebar-two-projects')
        })

        await f.step('Add Project refuses a folder that is already added', async () => {
          const registry = await takeRegistry(rpc)
          const from = tap.calls.length
          const { dialog, notice } = await addAlreadyRegisteredFolder(ui, host, alpha.path)
          await expect.soft(notice).toContainText(added.alpha.name)
          await expect.soft(dialog.getByTestId('add-project-open-existing')).toBeVisible()
          await f.shots.take('duplicate-blocked')
          expectNoWriteSent(tap.calls, from, 'the same folder')
          expectRegistryUnchanged(
            registry,
            await takeRegistry(rpc),
            'after the refused Add Project'
          )
          await dialog.getByTestId('add-project-open-existing').click()
          await expect(ui.getByRole('dialog')).toHaveCount(0, { timeout: 15_000 })
          await expectActiveWorktreeOf(ui, registry, added.alpha.id)
          await f.shots.take('open-existing-project')
          // A trailing slash is the same folder.
          const slash = await addAlreadyRegisteredFolder(ui, host, alpha.path, `${alpha.path}/`)
          await expect.soft(slash.notice).toContainText(added.alpha.name)
          expectNoWriteSent(tap.calls, from, 'the same folder with a trailing slash')
          await ui.keyboard.press('Escape')
          expectRegistryUnchanged(registry, await takeRegistry(rpc), 'after the slash variant')
        })
        await f.step(
          'the backend refuses the duplicate too (PROJECT_REPO_ALREADY_REGISTERED)',
          async () => {
            const registry = await takeRegistry(rpc)
            const holder = registry.projects.find((p) => p.id === added.alpha.id)!
            await expectBackendRefusesDuplicate(rpc, {
              name: `${ADDPROJ_PREFIX}${f.cfg.runPrefix.slice(7)}-duplicate-probe`,
              devServerId: f.project.devServerId,
              repoPath: alpha.path,
              holder
            })
            expectRegistryUnchanged(
              registry,
              await takeRegistry(rpc),
              'after the refused project.create'
            )
          }
        )
        if (stopAfterProjects) {
          console.log('[addproject] ORCA_E2E_ADDPROJECT_STOP=projects: the Request half is skipped')
          return
        }

        Object.assign(f.project, { id: added.bravo.id, name: added.bravo.name })
        await requestAboutBravoSteps(f, { alpha, bravo, decoy: added.alpha, heads })
      }
    )
  })
})
