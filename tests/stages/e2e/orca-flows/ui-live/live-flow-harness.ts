/**
 * Per-test harness of the live UI flow: provisions the project, signs in through the form, and always cleans up
 * (open Requests cancelled, project deleted) while reporting what the run saw. The step bodies live in live-flow-steps.ts.
 */
import { execFileSync } from 'node:child_process'
import { test, type BrowserContext, type Page, type TestInfo } from '@playwright/test'
import { GatewayRpcClient } from './gateway-rpc-client'
import { GapReporter } from './live-gap-reporter'
import { provisionLiveProject, type LiveProject } from './live-project-fixture'
import { aiBudgetMs, loadLiveStackConfig, type LiveStackConfig } from './live-stack-config'
import {
  type BrowserNoise,
  signInThroughTheForm,
  startTraceAfterSignIn,
  StepShots,
  stopTrace,
  watchBrowserNoise
} from './live-ui-session'

// Why: decision.* is on the contract's allowlist of channels the gateway does not serve yet (FE-E2E-001); it is not a finding.
const KNOWN_NOISE = /^decision\.[a-zA-Z]+ -> internal channel ".*" is not yet implemented/

export type LiveFlow = {
  cfg: LiveStackConfig
  rpc: GatewayRpcClient
  gaps: GapReporter
  shots: StepShots
  page: Page
  testInfo: TestInfo
  project: LiveProject
  noise: BrowserNoise
  lags: string[]
  step: (name: string, body: () => Promise<void>) => Promise<void>
  budgets: { classifyMs: number; solutionMs: number; planMs: number }
}

/** Restart count of the gateway container, or -1 when docker is not usable (the check is best effort). */
function gatewayRestartCount(): number {
  const name = process.env.ORCA_E2E_GATEWAY_CONTAINER || 'orca-go-api-gateway'
  try {
    const out = execFileSync('docker', ['inspect', name, '--format', '{{.RestartCount}}'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
      timeout: 10_000
    })
    return Number(out.trim())
  } catch {
    return -1
  }
}

export async function runLiveFlow(
  args: { page: Page; context: BrowserContext; testInfo: TestInfo; slug: string },
  body: (flow: LiveFlow) => Promise<void>
): Promise<void> {
  const { page, context, testInfo } = args
  const cfg = loadLiveStackConfig()
  test.skip(
    !cfg.adminEmail || !cfg.adminPassword,
    'no admin credentials (deploy/dev/.env BOOTSTRAP_ADMIN_* or ORCA_E2E_ADMIN_*)'
  )
  test.setTimeout(20 * 60_000)
  const gaps = new GapReporter(testInfo, cfg.gapMode)
  const noise = watchBrowserNoise(page)
  const rpc = new GatewayRpcClient(cfg)
  const restartsBefore = gatewayRestartCount()
  let project: LiveProject | undefined
  let traceOn = false
  // Why: a stuck run is diagnosed from this log; Playwright's list reporter prints steps only when they end.
  const step = async (name: string, fn: () => Promise<void>): Promise<void> => {
    const t0 = Date.now()
    console.log(`[live-flow:${args.slug}] > ${name}`)
    await test.step(name, fn)
    console.log(`[live-flow:${args.slug}] < ${name} (${Math.round((Date.now() - t0) / 1000)}s)`)
  }
  const lags: string[] = []
  let bodyFailed = false
  try {
    await step('provision a project bound to the dev server (the only non-UI setup)', async () => {
      await rpc.connect()
      project = await provisionLiveProject(rpc, cfg)
    })
    const shots = new StepShots(page, testInfo)
    await step('sign in through the login form', async () => {
      await signInThroughTheForm(page, cfg)
      await startTraceAfterSignIn(context)
      traceOn = true
      await shots.take('signed-in')
    })
    await body({
      cfg,
      rpc,
      gaps,
      shots,
      page,
      testInfo,
      project: project!,
      noise,
      lags,
      step,
      budgets: {
        classifyMs: aiBudgetMs(cfg, 60, 180),
        solutionMs: aiBudgetMs(cfg, 90, 300),
        planMs: aiBudgetMs(cfg, 90, 300)
      }
    })
  } catch (e) {
    bodyFailed = true
    throw e
  } finally {
    const failed = bodyFailed || testInfo.errors.length > 0
    if (failed) {
      await page.screenshot({ path: testInfo.outputPath('99-at-failure.png') }).catch(() => {})
    }
    if (traceOn) {
      await stopTrace(context, testInfo, failed || gaps.gaps.length > 0)
    }
    const problems = project ? await project.dispose() : []
    if (rpc.reconnects > 0) {
      gaps.record(
        'GATEWAY-CONNECTION-DROPPED',
        `the gateway dropped the gateway-client socket ${rpc.reconnects} time(s) during the run`
      )
    }
    const restartsAfter = gatewayRestartCount()
    if (restartsBefore >= 0 && restartsAfter > restartsBefore) {
      gaps.record(
        'GATEWAY-RESTARTED',
        `api-gateway restarted ${restartsAfter - restartsBefore} time(s) during the run (check its logs for a panic)`
      )
    }
    rpc.close()
    // Why: closing the page while request events are still in flight is exactly what used to crash the gateway.
    await page.waitForTimeout(3_000).catch(() => {})
    for (const p of problems) {
      console.log(`[cleanup] ${p}`)
    }
    const summary = [
      `slug: ${args.slug}`,
      `ui lag behind the service: ${lags.join('; ') || 'n/a'}`,
      `product gaps: ${gaps.gaps.length}`,
      `gateway restarts during run: ${restartsBefore < 0 ? 'unknown (no docker)' : restartsAfter - restartsBefore}`,
      `rate-limit retries on the gateway client: ${rpc.rateLimitRetries}`,
      `request-flow rpc errors the UI received: ${[...new Set(noise.rpcErrors.filter((e) => /^(request|solution|approval|clarification|decision|task)\./.test(e) && !KNOWN_NOISE.test(e)))].slice(0, 8).join(' | ') || 'none'}`,
      `known noise ignored: ${noise.rpcErrors.filter((e) => KNOWN_NOISE.test(e)).length} call(s) to decision.* (allowlisted as not yet in the gateway)`,
      `browser console errors: ${noise.consoleErrors.length}, page errors: ${noise.pageErrors.length}, failed calls: ${[...new Set(noise.failedCalls)].slice(0, 6).join(', ') || 'none'}`
    ]
    testInfo.annotations.push({ type: 'run-summary', description: summary.join(' | ') })
    console.log(`[live-flow:${args.slug}] ${summary.join(`\n[live-flow:${args.slug}] `)}`)
  }
}
