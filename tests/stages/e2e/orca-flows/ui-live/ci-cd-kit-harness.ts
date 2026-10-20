/**
 * Wrapper of the ci-cd-kit spec: the safety net of real-flow-harness.ts around the owner's existing project, plus a before/after
 * comparison of everything the owner has in Orca. Unlike deepseek-flow-harness.ts nothing is made through Add Project and nothing is
 * removed at the end: the new worktrees and branches stay for inspection, and the guard still fails the run on any other change.
 */
import type { BrowserContext, Page, TestInfo } from '@playwright/test'
import { createCicdKitFixture, findExistingProject, type CicdKitFixture } from './ci-cd-kit-fixture'
import { GatewayRpcClient } from './gateway-rpc-client'
import { tapSentRpc, type SentRpc } from './live-add-project-own-project-steps'
import type { LiveFlow } from './live-flow-harness'
import { connectedDevServer } from './live-project-fixture'
import { loadLiveStackConfig } from './live-stack-config'
import {
  describeUserData,
  diffUserData,
  snapshotUserData,
  type UserDataSnapshot
} from './live-user-data-snapshot'
import { runRealFlow } from './real-flow-harness'

export type CicdKitRun = { fx: CicdKitFixture; tap: { calls: SentRpc[] } }

export async function runCicdKitFlow(
  args: { page: Page; context: BrowserContext; testInfo: TestInfo },
  body: (f: LiveFlow, run: CicdKitRun) => Promise<void>
): Promise<void> {
  process.env.ORCA_E2E_RUN_ID ||= new Date().toISOString().replace(/[-:T]/g, '').slice(4, 14)
  const cfg = loadLiveStackConfig()
  const pre = new GatewayRpcClient(cfg)
  await pre.connect()
  const server = await connectedDevServer(pre, cfg)
  const before: UserDataSnapshot = await snapshotUserData(pre, server.id)
  pre.close()
  // Why: the owner's other repos join the git fingerprint of the safety net (this one is excluded by the fixture and has its own rules).
  process.env.ORCA_E2E_GUARD_PATHS = [process.env.ORCA_E2E_GUARD_PATHS, ...before.repoPaths]
    .filter(Boolean)
    .join(':')
  for (const line of describeUserData(before)) {
    console.log(`[ci-cd-kit] user data BEFORE: ${line}`)
  }
  const tap = tapSentRpc(args.page)
  let fx: CicdKitFixture | undefined
  await runRealFlow(
    {
      ...args,
      slug: 'ci-cd-kit',
      mode: 'execute',
      createFixture: (prefix) => (fx = createCicdKitFixture(prefix)),
      provision: (rpc, c) => findExistingProject(rpc, c),
      afterCleanup: async () => {
        const rpc = new GatewayRpcClient(cfg)
        await rpc.connect()
        try {
          const after = await snapshotUserData(rpc, server.id)
          return [
            ...(fx?.afterCleanup() ?? []),
            ...diffUserData(before, after).map((p) => `user data changed: ${p}`)
          ]
        } finally {
          rpc.close()
        }
      }
    },
    async (f, fixture) => body(f, { fx: fixture as CicdKitFixture, tap })
  )
}
