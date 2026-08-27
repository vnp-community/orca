// FIX TASK-AT-001: Removed 'electron' import — use RendererBridge interface instead.
// AutomationService now works in server mode (ORCA_MULTI_USER=1) without Electron.
// ADR-021 Phase 1: narrowed from the concrete `Store` type to the exact
// methods used, so a future Postgres-backed store can satisfy this interface
// without AutomationService depending on persistence.ts's Store at all — see
// automation-store-dependency.ts's module doc comment. Every store call below
// is `await`ed because the interface is async (Postgres-backed implementations
// need real I/O) even though `wrapStoreAsAutomationStoreDependency()` resolves
// instantly for the Electron-desktop `Store` case.
import type { AutomationStoreDependency } from './automation-store-dependency'
import {
  isFinalAutomationRunStatus,
  type Automation,
  type AutomationDispatchRequest,
  type AutomationDispatchResult,
  type AutomationPrecheckResult,
  type AutomationRun
} from '../../shared/automations-types'
import type { ClaudeUsageStore } from '../claude-usage/store'
import type { CodexUsageStore } from '../codex-usage/store'
import { runAutomationPrecheck } from './precheck-runner'
import { resolveAutomationRunTarget, type AutomationRunTargetResult } from './run-target-resolution'
import { collectAutomationRunUsage } from './run-usage-collection'
import type { HeadlessAutomationDispatcher } from './headless-dispatch'
import { clearAutomationDispatchTokens, createAutomationDispatchToken } from './dispatch-tokens'
import {
  didAutomationPrecheckPass,
  formatAutomationPrecheckFailure
} from '../../shared/automation-precheck'

const DEFAULT_TICK_MS = 60 * 1000

/** FIX TASK-AT-001: Minimal interface replacing WebContents — Electron's WebContents satisfies this. */
type RendererBridge = {
  isDestroyed(): boolean
  send(channel: string, ...args: unknown[]): void
}

export class AutomationService {
  private readonly store: AutomationStoreDependency
  private readonly tickMs: number
  private timer: ReturnType<typeof setInterval> | null = null
  private webContents: RendererBridge | null = null
  private rendererReady = false
  private evaluating = false
  private readonly claudeUsage: ClaudeUsageStore | null
  private readonly codexUsage: CodexUsageStore | null
  private readonly allowRemoteHostScheduling: boolean
  private readonly headlessDispatcher: HeadlessAutomationDispatcher | null

  constructor(
    store: AutomationStoreDependency,
    opts: {
      tickMs?: number
      claudeUsage?: ClaudeUsageStore
      codexUsage?: CodexUsageStore
      allowRemoteHostScheduling?: boolean
      headlessDispatcher?: HeadlessAutomationDispatcher
    } = {}
  ) {
    this.store = store
    this.tickMs = opts.tickMs ?? DEFAULT_TICK_MS
    this.claudeUsage = opts.claudeUsage ?? null
    this.codexUsage = opts.codexUsage ?? null
    this.allowRemoteHostScheduling = opts.allowRemoteHostScheduling ?? false
    this.headlessDispatcher = opts.headlessDispatcher ?? null
  }

  // FIX TASK-AT-001: Accept RendererBridge instead of WebContents — Electron's WebContents satisfies it.
  setWebContents(webContents: RendererBridge | null): void {
    this.webContents = webContents
    this.rendererReady = false
  }

  setRendererReady(): void {
    this.rendererReady = true
    void this.evaluateDueRuns()
  }

  start(): void {
    if (this.timer) {
      return
    }
    this.timer = setInterval(() => {
      void this.evaluateDueRuns()
    }, this.tickMs)
    // Why: headless serve never gets a renderer-ready IPC, but due runs still
    // need the same startup catch-up pass desktop gets after renderer attach.
    if (this.rendererReady || this.headlessDispatcher) {
      void this.evaluateDueRuns()
    }
  }

  stop(): void {
    if (!this.timer) {
      return
    }
    clearInterval(this.timer)
    this.timer = null
  }

  async runNow(automationId: string): Promise<AutomationRun> {
    const automations = await this.store.listAutomations()
    const automation = automations.find((entry) => entry.id === automationId)
    if (!automation) {
      throw new Error('Automation not found.')
    }
    const run = await this.store.createAutomationRun(automation, Date.now(), 'manual')
    return await this.requestDispatch(automation, run)
  }

  async runPrecheck(automationId: string, runId: string): Promise<AutomationPrecheckResult | null> {
    const automations = await this.store.listAutomations()
    const automation = automations.find((entry) => entry.id === automationId)
    if (!automation) {
      throw new Error('Automation not found.')
    }
    const runs = await this.store.listAutomationRuns(automationId)
    const run = runs.find((entry) => entry.id === runId)
    if (!run) {
      throw new Error('Automation run not found.')
    }
    if (run.trigger !== 'scheduled' || !automation.precheck) {
      return null
    }
    const target = await resolveAutomationRunTarget(this.store, automation, {
      allowRemoteHostScheduling: this.allowRemoteHostScheduling
    })
    if (!target.ok) {
      return {
        command: automation.precheck.command,
        exitCode: null,
        timedOut: false,
        durationMs: 0,
        stdout: '',
        stderr: '',
        stdoutTruncated: false,
        stderrTruncated: false,
        error: target.error,
        startedAt: Date.now(),
        completedAt: Date.now()
      }
    }
    return await runAutomationPrecheck({
      precheck: automation.precheck,
      target:
        automation.executionTargetType === 'ssh'
          ? { type: 'ssh', cwd: target.cwd, connectionId: automation.executionTargetId }
          : { type: 'local', cwd: target.cwd }
    })
  }

  async markDispatchResult(result: AutomationDispatchResult): Promise<AutomationRun> {
    const run = await this.store.updateAutomationRun(result)
    clearAutomationDispatchTokens(run.automationId, run.id)
    if (!isFinalAutomationRunStatus(run.status)) {
      return run
    }
    // Why: the renderer's mark-completed effect can re-fire for the same run
    // before refresh() flips its status snapshot off 'dispatched'. Re-running
    // collectRunUsage advances the attribution window and can rewrite an
    // already-collected 'known' usage to 'unavailable'/'ambiguous_session'.
    if (run.usage) {
      return run
    }
    const automations = await this.store.listAutomations()
    const usage = await collectAutomationRunUsage({
      automation: automations.find((entry) => entry.id === run.automationId),
      run,
      claudeUsage: this.claudeUsage,
      codexUsage: this.codexUsage
    })
    // Why: the run is final during the await above, so a concurrent create-time
    // retention prune may have evicted it — the usage write must not throw then.
    const runsAfterUsageCollection = await this.store.listAutomationRuns(run.automationId)
    if (!runsAfterUsageCollection.some((entry) => entry.id === run.id)) {
      return run
    }
    return await this.store.updateAutomationRun({
      runId: run.id,
      status: run.status,
      workspaceId: run.workspaceId,
      terminalSessionId: run.terminalSessionId,
      usage,
      error: run.error
    })
  }

  private async evaluateDueRuns(): Promise<void> {
    if (this.evaluating) {
      return
    }
    this.evaluating = true
    try {
      const now = Date.now()
      const automations = await this.store.listAutomations()
      for (const automation of automations) {
        if (!automation.enabled || automation.nextRunAt > now) {
          continue
        }
        await this.evaluateAutomation(automation, now)
      }
    } finally {
      this.evaluating = false
    }
  }

  private async evaluateAutomation(automation: Automation, now: number): Promise<void> {
    const scheduledFor = this.store.getLatestAutomationOccurrence(automation, now)
    if (scheduledFor === null) {
      await this.store.advanceAutomationNextRun(automation.id, now)
      return
    }
    const run = await this.store.createAutomationRun(automation, scheduledFor)
    const graceMs = automation.missedRunGraceMinutes * 60 * 1000
    if (now - scheduledFor > graceMs) {
      await this.store.updateAutomationRun({
        runId: run.id,
        status: 'skipped_missed',
        workspaceId: automation.workspaceId,
        error: 'Orca was unavailable during the missed-run grace window.'
      })
      await this.store.advanceAutomationNextRun(automation.id, now)
      return
    }

    await this.requestDispatch(automation, run)
    await this.store.advanceAutomationNextRun(automation.id, now)
  }

  private async requestDispatch(
    automation: Automation,
    run: AutomationRun
  ): Promise<AutomationRun> {
    const target = await resolveAutomationRunTarget(this.store, automation, {
      allowRemoteHostScheduling: this.allowRemoteHostScheduling
    })
    if (!target.ok) {
      return await this.store.updateAutomationRun({
        runId: run.id,
        status: 'skipped_unavailable',
        workspaceId: automation.workspaceId,
        error: target.error
      })
    }
    const webContents = this.webContents
    if (!webContents || webContents.isDestroyed() || !this.rendererReady) {
      if (this.headlessDispatcher) {
        return await this.requestHeadlessDispatch(automation, run, target)
      }
      return await this.store.updateAutomationRun({
        runId: run.id,
        status: 'skipped_unavailable',
        workspaceId: automation.workspaceId,
        error: 'No Orca window was available to launch the automation.'
      })
    }
    const updated = await this.store.updateAutomationRun({
      runId: run.id,
      status: 'dispatching',
      workspaceId: automation.workspaceId,
      error: null
    })
    const payload: AutomationDispatchRequest = {
      automation,
      run: updated,
      dispatchToken: createAutomationDispatchToken(automation.id, updated.id)
    }
    webContents.send('automations:dispatchRequested', payload)
    return updated
  }

  private async requestHeadlessDispatch(
    automation: Automation,
    run: AutomationRun,
    target: Extract<AutomationRunTargetResult, { ok: true }>
  ): Promise<AutomationRun> {
    const precheckResult =
      run.trigger === 'scheduled' && automation.precheck
        ? await this.runPrecheck(automation.id, run.id)
        : null
    if (precheckResult && !didAutomationPrecheckPass(precheckResult)) {
      return await this.store.updateAutomationRun({
        runId: run.id,
        status: 'skipped_precheck',
        workspaceId: automation.workspaceId,
        precheckResult,
        error: formatAutomationPrecheckFailure(precheckResult)
      })
    }
    try {
      const launch = await this.headlessDispatcher!({ automation, run, target })
      const launchRunTarget = {
        workspaceId: launch.workspaceId,
        workspaceDisplayName: launch.workspaceDisplayName ?? null,
        terminalSessionId: launch.terminalSessionId,
        terminalPaneKey: launch.terminalPaneKey ?? null,
        terminalPtyId: launch.terminalPtyId ?? null
      }
      const updated = await this.store.updateAutomationRun({
        runId: run.id,
        status: 'dispatched',
        ...launchRunTarget,
        error: null
      })
      if (launch.completion) {
        void launch.completion
          .then((completion) =>
            this.markDispatchResult({
              runId: run.id,
              status: completion.status,
              ...launchRunTarget,
              precheckResult,
              outputSnapshot: completion.outputSnapshot ?? null,
              error: completion.error ?? null
            })
          )
          .catch((error) =>
            this.markDispatchResult({
              runId: run.id,
              status: 'dispatch_failed',
              ...launchRunTarget,
              error: error instanceof Error ? error.message : String(error)
            })
          )
      }
      return updated
    } catch (error) {
      return await this.store.updateAutomationRun({
        runId: run.id,
        status: 'dispatch_failed',
        workspaceId: automation.workspaceId,
        error: error instanceof Error ? error.message : String(error)
      })
    }
  }
}
