/**
 * Wrapper of every spec that works on the user's repo deepseek-harness with the REAL Claude (TASK-FE-E2E-028): the safety net of
 * real-flow-harness.ts, with the user's repo as the fixture (deepseek-repo-fixture.ts), a project made through the UI under a run
 * prefix (so cleanup removes exactly it), and a before/after comparison of everything the user owns in Orca.
 */
import type { BrowserContext, Page, TestInfo } from '@playwright/test'
import { GatewayRpcClient } from './gateway-rpc-client'
import { createAddProjectLedger } from './live-add-project-ledger'
import type { ProjectWithRepos } from './live-add-project-ledger'
import { tapSentRpc, type SentRpc } from './live-add-project-own-project-steps'
import { addDeepseekProject } from './deepseek-add-project-steps'
import { createDeepseekFixture, DEEPSEEK_REPO, type DeepseekFixture } from './deepseek-repo-fixture'
import type { LiveFlow } from './live-flow-harness'
import { connectedDevServer } from './live-project-fixture'
import { loadLiveStackConfig } from './live-stack-config'
import {
  describeUserData,
  diffUserData,
  snapshotUserData,
  type UserDataSnapshot
} from './live-user-data-snapshot'
import { runRealFlow, type RealMode } from './real-flow-harness'

// Read once: later tests of the same worker must not mistake the stamp of an earlier test for the operator's choice.
const OPERATOR_RUN_ID = process.env.ORCA_E2E_RUN_ID

export type DeepseekRun = {
  fx: DeepseekFixture
  /** Everything the SPA sent over its WebSocket, from the first frame. */
  tap: { calls: SentRpc[] }
  /** The project this run made for the repo (name carries the run prefix). */
  project: ProjectWithRepos
  repoName: string
}

export async function runDeepseekFlow(
  args: { page: Page; context: BrowserContext; testInfo: TestInfo; slug: string; mode: RealMode },
  body: (f: LiveFlow, run: DeepseekRun) => Promise<void>
): Promise<void> {
  // Why: every loadLiveStackConfig() stamps the current second; the project name, the ledger and the fixture must share ONE stamp.
  process.env.ORCA_E2E_RUN_ID =
    OPERATOR_RUN_ID || new Date().toISOString().replace(/[-:T]/g, '').slice(4, 14)
  const cfg = loadLiveStackConfig()
  // The user's own data, fingerprinted before anything is made; their other repos join the git fingerprint of the safety net.
  const pre = new GatewayRpcClient(cfg)
  await pre.connect()
  const server = await connectedDevServer(pre, cfg)
  const before: UserDataSnapshot = await snapshotUserData(pre, server.id)
  pre.close()
  process.env.ORCA_E2E_GUARD_PATHS = [process.env.ORCA_E2E_GUARD_PATHS, ...before.repoPaths]
    .filter(Boolean)
    .join(':')
  for (const line of describeUserData(before)) {
    console.log(`[deepseek] user data BEFORE: ${line}`)
  }
  // Why: registered before the flow so it also sees the WebSocket the SPA opens at sign-in.
  const tap = tapSentRpc(args.page)
  let fx: DeepseekFixture | undefined
  const ledger = createAddProjectLedger({
    namePrefix: `e2e-ds-${cfg.runPrefix.replace(/^e2e-ui-/, '')}-`,
    paths: [DEEPSEEK_REPO]
  })
  await runRealFlow(
    {
      ...args,
      createFixture: (prefix) => (fx = createDeepseekFixture(prefix)),
      provision: (rpc, c) => ledger.provision(rpc, c),
      afterCleanup: async () => {
        const rpc = new GatewayRpcClient(cfg)
        await rpc.connect()
        try {
          const after = await snapshotUserData(rpc, server.id)
          return [
            ...ledger.problems(),
            ...(await ledger.leftovers(rpc)),
            ...(fx?.afterCleanup() ?? []),
            ...diffUserData(before, after)
              .filter((p) => !p.includes(DEEPSEEK_REPO))
              .map((p) => `user data changed: ${p}`)
          ]
        } finally {
          rpc.close()
        }
      }
    },
    async (f, fixture) => {
      const repoName = (fixture as DeepseekFixture).repoName
      const project = await addDeepseekProject(f, tap, repoName)
      await body(f, { fx: fixture as DeepseekFixture, tap, project, repoName })
    }
  )
}
