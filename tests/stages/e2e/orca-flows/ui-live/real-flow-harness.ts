/**
 * Wrapper of the real-AI specs (TASK-FE-E2E-010): real Claude Code on the dev server, never the fake one, against a throwaway
 * repo, inside the safety net. Order matters: gates, fixture, fingerprint, flow (with a watchdog and a hard deadline), then
 * ALWAYS cleanup and the before/after comparison, even when the flow failed.
 */
import { test, type BrowserContext, type Page, type TestInfo } from '@playwright/test'
import { runLiveFlow, type LiveFlow } from './live-flow-harness'
import { loadLiveStackConfig } from './live-stack-config'
import {
  captureGuardedState,
  waitForQuietTree,
  claudePrintProcesses,
  diffGuardedState,
  killClaudeUnder
} from './live-safety-invariants'
import { createCalcFixture, leftoversOf, type RealFixture } from './real-repo-fixture'

export type RealMode = 'readonly' | 'execute'

const WATCHDOG_MS = 15_000
const QUIET_WAIT_MAX_MS = 15 * 60_000
const CLEANUP_MARGIN_MS = 5 * 60_000

function truthy(name: string): boolean {
  return ['1', 'true', 'yes'].includes((process.env[name] ?? '').toLowerCase())
}

/** Hard cap on one real-AI run; ORCA_E2E_REAL_TIMEOUT_MIN overrides (default 25 for execution, 15 for read-only). */
export function realRunBudgetMs(mode: RealMode): number {
  const minutes = Number(process.env.ORCA_E2E_REAL_TIMEOUT_MIN) || (mode === 'execute' ? 25 : 15)
  return minutes * 60_000
}

async function waitForNoNewClaude(knownPids: number[], ms: number): Promise<number[]> {
  const deadline = Date.now() + ms
  for (;;) {
    const fresh = claudePrintProcesses()
      .map((p) => p.pid)
      .filter((p) => !knownPids.includes(p))
    if (fresh.length === 0 || Date.now() > deadline) {
      return fresh
    }
    await new Promise((r) => setTimeout(r, 1_000))
  }
}

export async function runRealFlow(
  args: { page: Page; context: BrowserContext; testInfo: TestInfo; slug: string; mode: RealMode },
  body: (f: LiveFlow, fixture: RealFixture) => Promise<void>
): Promise<void> {
  const cfg = loadLiveStackConfig()
  test.skip(
    cfg.aiMode === 'stub',
    'real-AI spec: never runs against the fake claude (drop --ai-stub)'
  )
  test.skip(
    args.mode === 'execute' && !cfg.executeAgent,
    'approving a Plan runs a real AI agent on this machine: set ORCA_E2E_EXECUTE_AGENT=1 (run_ui_flow.py --real does)'
  )
  test.skip(
    args.mode === 'readonly' && !truthy('ORCA_E2E_READONLY_ANALYSIS'),
    'starts a real read-only Claude on the fixture repo: set ORCA_E2E_READONLY_ANALYSIS=1 (run_ui_flow.py --real does)'
  )
  const budget = realRunBudgetMs(args.mode)
  // Why: the quiet-tree wait and the cleanup come on top of the run's own hard limit.
  const totalMs = QUIET_WAIT_MAX_MS + budget + CLEANUP_MARGIN_MS
  test.setTimeout(totalMs)
  const startedAt = Date.now()
  console.log(
    `[real:${args.slug}] AI mode: REAL Claude Code (${args.mode}); hard limit ${Math.round(budget / 60_000)} min`
  )

  const quietMs = Number(process.env.ORCA_E2E_QUIET_SECONDS ?? 90) * 1000
  const waited = await waitForQuietTree(quietMs, QUIET_WAIT_MAX_MS)
  console.log(
    `[real:${args.slug}] guarded checkout quiet for ${Math.round(quietMs / 1000)}s after waiting ${waited}s`
  )
  const fixture = createCalcFixture(cfg.runPrefix)
  try {
    fixture.selfTestGuard()
  } catch (e) {
    fixture.dispose()
    throw e
  }
  const before = captureGuardedState()
  const cheapBefore = captureGuardedState(true)
  console.log(
    `[real:${args.slug}] fixture ${fixture.base}; guarded repos: ${before.repos.map((r) => r.path).join(', ')}; ` +
      `claude --print processes already running: ${before.claudePrintPids.length}`
  )

  let violation = ''
  let flowError: unknown
  const timers: NodeJS.Timeout[] = []
  const abort = (why: string, page: Page): void => {
    violation ||= why
    console.log(`[real:${args.slug}] ABORT: ${why}`)
    killClaudeUnder(fixture.base)
    // Why: closing the page makes the next UI step of the flow throw, which stops the run at once.
    void page.close({ runBeforeUnload: false }).catch(() => {})
  }
  try {
    await runLiveFlow(
      {
        page: args.page,
        context: args.context,
        testInfo: args.testInfo,
        slug: args.slug,
        repoPath: fixture.repoPath,
        beforeDispose: () => {
          killClaudeUnder(fixture.base)
        },
        timeoutMs: totalMs
      },
      async (f) => {
        timers.push(
          setInterval(() => {
            try {
              const leaks = diffGuardedState(cheapBefore, captureGuardedState(true), {
                processes: false
              })
              if (leaks.length > 0) {
                abort(`guarded state changed during the run: ${leaks.join('; ')}`, f.page)
              }
              if (fixture.guardDenials().length > 0) {
                abort(
                  `the agent tried to leave the fixture: ${fixture.guardDenials().join(' | ')}`,
                  f.page
                )
              }
            } catch (e) {
              abort(`the safety watchdog failed: ${(e as Error).message}`, f.page)
            }
          }, WATCHDOG_MS),
          setTimeout(
            () => abort(`hard time limit of ${Math.round(budget / 60_000)} min exceeded`, f.page),
            budget
          )
        )
        await body(f, fixture)
      }
    )
  } catch (e) {
    flowError = e
  }
  // Why: not in a `finally`: everything below runs whether the flow passed or failed, and may itself throw (an unsafe run is red).
  {
    timers.forEach((t) => {
      clearInterval(t)
      clearTimeout(t)
    })
    const lingering = killClaudeUnder(fixture.base)
    const stray = await waitForNoNewClaude(before.claudePrintPids, 20_000)
    const calls = fixture.guardCalls().length
    const denials = fixture.guardDenials()
    fixture.dispose()
    const leaks = diffGuardedState(before, captureGuardedState())
    const leftovers = leftoversOf(cfg.runPrefix)
    const problems = [
      ...(violation ? [violation] : []),
      ...(lingering.length > 0
        ? [`claude still running inside the fixture at the end (killed ${lingering.join(',')})`]
        : []),
      ...(denials.length > 0
        ? [`tool guard refused ${denials.length} call(s): ${denials.slice(0, 3).join(' | ')}`]
        : []),
      ...leaks,
      ...(stray.length > 0
        ? [`unexpected claude --print pids after the run: ${stray.join(',')}`]
        : []),
      ...(leftovers.length > 0 ? [`temp entries not removed: ${leftovers.join(',')}`] : [])
    ]
    const summary =
      `[real:${args.slug}] duration ${Math.round((Date.now() - startedAt) / 1000)}s; tool calls seen by the guard: ${calls}; ` +
      `guard denials: ${denials.length}; safety problems: ${problems.length}`
    console.log(summary)
    args.testInfo.annotations.push({ type: 'real-ai-run', description: summary })
    if (problems.length > 0) {
      for (const p of problems) {
        console.log(`[real:${args.slug}] SAFETY PROBLEM: ${p}`)
      }
      const alsoFailed = flowError
        ? ` (flow also failed: ${(flowError as Error).message.split('\n')[0]})`
        : ''
      throw new Error(`safety invariants violated: ${problems.join(' || ')}${alsoFailed}`)
    }
  }
  if (flowError) {
    throw flowError
  }
}
