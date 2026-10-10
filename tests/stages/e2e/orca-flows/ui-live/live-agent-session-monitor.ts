/**
 * Watches, from the web UI, the agent that runs a Request task (TASK-FE-E2E-011). While the real Claude works it keeps the task
 * sheet open on the main page and Settings > Dev Servers on a second tab, sampling both and the backend's own activity stream;
 * afterwards it reloads the page and looks for the session again. The judging rules are in live-agent-session-judge.ts; a rule
 * that fails is a product gap (frontend, backend or agent), never a loosened expectation.
 */
import type { BrowserContext } from '@playwright/test'
import { DevServerTab } from './live-agent-session-devtab'
import type { LiveFlow } from './live-flow-harness'
import {
  judgeActivityPanel,
  judgeBackendOutput,
  judgeDevServer,
  judgeDevServerFacts,
  judgeHistoryReplay,
  judgeRunningIndicator,
  judgeRunTrace,
  judgeStatusFollows,
  judgeUiShowsOutput,
  judgeWorktreeTerminal,
  judgeWorktreeView,
  linesAddedByRun,
  type ActivityPanelSample,
  type DevServerSample,
  type Verdict
} from './live-agent-session-judge'
import {
  closeTaskSheet,
  isOnlineLabel,
  openTaskSheetFromPlan,
  openTaskWorktreeInGit,
  peekDiffOfChangedFile,
  readAgentActivity,
  readRequestTabs,
  readTaskSheet,
  recordGitRpc,
  type TaskSheetReading
} from './live-agent-session-ui'
import type { WorktreeChangeState } from './live-fixture-change-checks'
import type { PlanTask } from './live-execution-steps'
import type { RequestUnderTest } from './live-flow-steps'
import { reloadAndReopen } from './live-request-ui-steps'
import { findSecrets, loadSecretCatalog } from './live-secret-leak-scan'
import { claudeRunningUnder } from './live-safety-invariants'
import {
  distinctivePhrase,
  normalizeForMatch,
  type TaskActivityTap
} from './live-task-activity-tap'

const SAMPLE_EVERY_MS = 3_000
const WATCH_AFTER_END_MS = 20_000
const WATCH_CAP_MS = 20 * 60_000
const STUCK_TASK_MS = 90_000
const DISPATCH_WAIT_MS = 180_000

const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms))
const joined = (r: TaskSheetReading): string => Object.values(r.textByTab).join('\n')

export class AgentSessionMonitor {
  private taskId = ''
  private readonly devTab: DevServerTab
  private startedAt = 0
  private watchedAt = 0
  private leftRunningAt = 0
  private uiSeenAt: number | null = null
  private baseline = ''
  private readonly apiStatuses: string[] = []
  private readonly sampleTrace: string[] = []
  private readonly sheetStatuses: string[] = []
  private readonly devDuring: DevServerSample[] = []
  private readonly texts: string[] = []
  private sawRunningButton = false
  private sawInProgress = false
  private readonly panelDuring: ActivityPanelSample[] = []
  private panelAfter: ActivityPanelSample | null = null
  private panelKinds = new Set<string>()
  private pathToTask = true
  private readonly verdicts: Verdict[] = []
  readonly evidence: string[] = []

  constructor(
    private readonly f: LiveFlow,
    context: BrowserContext,
    private readonly r: RequestUnderTest,
    readonly tap: TaskActivityTap,
    private readonly fixtureBase: string
  ) {
    this.devTab = new DevServerTab(f, context)
  }

  private note(line: string): void {
    this.evidence.push(line)
    console.log(`[monitor] ${line}`)
  }

  private add(verdicts: Verdict[]): void {
    for (const v of verdicts) {
      this.verdicts.push(v)
      this.f.gaps.record(v.code, `[${v.owner}] ${v.text}`)
    }
  }

  /** Before the Plan is approved: the dev server tab and the on-screen text the run must add something to. */
  async prepare(): Promise<void> {
    await this.f.step(
      'prepare the dev server tab and the Request texts before the agent starts',
      async () => {
        await this.devTab.open()
        this.baseline = Object.values(await readRequestTabs(this.f.page)).join('\n')
      }
    )
  }

  /**
   * Right after the Plan is approved: opens the task sheet and samples it, the dev server tab and the activity panel until the
   * task has run and left in_progress. Must start before the agent does (a run takes about 20 to 60 seconds).
   */
  async watchTask(task: PlanTask): Promise<void> {
    const { f } = this
    await f.step('watch the task run from the UI (task sheet and dev server tab)', async () => {
      this.taskId = task.id
      this.startedAt = Date.now()
      this.watchedAt = this.startedAt
      this.pathToTask = await openTaskSheetFromPlan(f.page, task.id)
      if (!this.pathToTask) {
        f.gaps.record(
          'UI-NO-PATH-TO-TASK',
          `[frontend] the Plan tab has no row plan-task-row-${task.id} to open the running task from the Request`
        )
        return
      }
      const first = await readTaskSheet(f.page, true)
      this.baseline += `\n${joined(first)}`
      this.note(
        `task sheet before the run: engine "${first.engine}", tabs [${first.tabsPresent.join(', ')}], status "${first.statusLine}", run button "${first.runButton}"`
      )
      await f.shots.take('task-sheet-before-run')
      await this.sampleUntilDone(task.id)
      await f.shots.take('task-sheet-after-run')
      await closeTaskSheet(f.page)
    })
  }

  private async sampleUntilDone(taskId: string): Promise<void> {
    const { f } = this
    const stopAt = Date.now() + WATCH_CAP_MS
    let idleSince = 0
    for (let i = 0; Date.now() < stopAt; i++) {
      const api = await f.rpc.call<{ status?: string }>('task.get', { id: taskId })
      const status = String(api.status ?? '')
      this.apiStatuses.push(status)
      this.sampleTrace.push(`+${Math.round((Date.now() - this.watchedAt) / 1000)}s ${status}`)
      const sheet = await readTaskSheet(f.page, i % 3 === 0)
      this.sheetStatuses.push(sheet.statusLine)
      this.texts.push(joined(sheet))
      this.sawRunningButton ||= status === 'in_progress' && /agent running/i.test(sheet.runButton)
      const activity = await readAgentActivity(f.page)
      activity.lines.forEach((l) => this.panelKinds.add(l.kind))
      this.texts.push(activity.lines.map((l) => l.text).join('\n'))
      const panel = { state: activity.state, live: activity.live, lines: activity.lines.length }
      if (status === 'in_progress') {
        this.panelDuring.push(panel)
      } else {
        this.panelAfter = panel
      }
      if (status === 'in_progress') {
        this.devDuring.push(await this.devTab.sample())
        // Why: this loop replaces the driver's own stuck-task check while it runs, so it repeats it.
        idleSince = claudeRunningUnder(this.fixtureBase) ? 0 : idleSince || Date.now()
        if (idleSince && Date.now() - idleSince > STUCK_TASK_MS) {
          throw new Error(
            `TASK-STUCK-IN-PROGRESS: the task has been in_progress for ${Math.round(STUCK_TASK_MS / 1000)}s with no claude process running`
          )
        }
      }
      this.checkPhraseOnScreen(sheet)
      if (status === 'in_progress') {
        this.startedAt = this.sawInProgress ? this.startedAt : Date.now()
        this.sawInProgress = true
      } else if (this.sawInProgress || ['review', 'done'].includes(status)) {
        this.leftRunningAt ||= Date.now()
        if (Date.now() - this.leftRunningAt > WATCH_AFTER_END_MS) {
          return
        }
      } else if (Date.now() - this.startedAt > DISPATCH_WAIT_MS) {
        throw new Error(
          `the task is still "${status}" ${Math.round(DISPATCH_WAIT_MS / 1000)}s after the Plan was approved`
        )
      }
      await sleep(SAMPLE_EVERY_MS)
    }
    this.leftRunningAt ||= Date.now()
  }

  private checkPhraseOnScreen(sheet: TaskSheetReading): void {
    const phrase = distinctivePhrase(this.tap.outputFramesOf(this.taskId))
    if (phrase && this.uiSeenAt === null && normalizeForMatch(joined(sheet)).includes(phrase)) {
      this.uiSeenAt = Date.now()
    }
  }

  /** After the run: backend broadcast, UI during the run, reload and replay, trace, worktree, dev server, secrets. */
  async judgeAfterRun(worktree: WorktreeChangeState): Promise<void> {
    const { f } = this
    await f.step('judge what the UI showed of the agent session', async () => {
      if (!this.taskId) {
        throw new Error(
          'the monitor never saw a working task go in_progress, so there was no session to judge'
        )
      }
      await sleep(5_000) // the last output frame goes through the outbox relay after the task is done
      const frames = this.tap.outputFramesOf(this.taskId)
      const times = { startedAt: this.startedAt, leftRunningAt: this.leftRunningAt || Date.now() }
      const phrase = distinctivePhrase(frames)
      const last = frames.at(-1)
      this.note(
        `backend: ${frames.length} agent_output_partial frame(s) for the task; first at +${frames[0] ? Math.round((frames[0].receivedAt - this.startedAt) / 1000) : '-'}s, ` +
          `run ${Math.round((times.leftRunningAt - times.startedAt) / 1000)}s, last frame stdout ${last?.stdout.length ?? 0} chars`
      )
      this.note(`task samples (every ~3s): ${this.sampleTrace.join(', ')}`)
      this.note(
        `ui while running: Agent running button ${this.sawRunningButton ? 'seen' : 'NOT seen'}; status line samples ${[...new Set(this.sheetStatuses)].join(' | ')}`
      )
      const backend = judgeBackendOutput(
        times,
        frames.map((x) => x.receivedAt)
      )
      this.add(backend)
      this.add(
        judgeRunningIndicator(this.apiStatuses.includes('in_progress'), this.sawRunningButton)
      )
      // Why: statuses before the run ("open") are on the sheet from the start and would hide a sheet that never follows.
      const fromRun = Math.max(0, this.apiStatuses.indexOf('in_progress'))
      this.add(judgeStatusFollows(this.apiStatuses.slice(fromRun), this.sheetStatuses))
      this.note(
        `activity panel while running: ${this.panelDuring.length} sample(s), max ${Math.max(0, ...this.panelDuring.map((p) => p.lines))} line(s), states ${[...new Set(this.panelDuring.map((p) => p.state))].join('/')}; after: ${this.panelAfter ? `${this.panelAfter.state} ${this.panelAfter.lines} line(s)` : 'n/a'}; line kinds ${[...this.panelKinds].join(',') || 'none'}`
      )
      this.add(judgeActivityPanel(this.panelDuring, this.panelAfter))
      this.add(judgeUiShowsOutput(times, phrase, { seenAt: this.uiSeenAt }, backend.length === 0))
      await this.judgeDevServerAfterRun()
      await this.devTab.close()
      await this.replayAfterReload(phrase, worktree)
      this.leakScan(frames.map((x) => `${x.stdout}\n${x.stderr}`).join('\n'))
    })
  }

  private async replayAfterReload(phrase: string, worktree: WorktreeChangeState): Promise<void> {
    const { f } = this
    const git = recordGitRpc(f.page) // before the reload: only sockets opened afterwards are seen
    await reloadAndReopen(f.page, this.r.title)
    const opened = await openTaskSheetFromPlan(f.page, this.taskId)
    if (!opened) {
      f.gaps.record(
        'UI-NO-PATH-TO-TASK',
        '[frontend] after a reload the Plan tab has no row to reopen the finished task from the Request'
      )
      return
    }
    const sheet = await readTaskSheet(f.page, true)
    await f.shots.take('task-sheet-after-reload')
    this.texts.push(joined(sheet))
    this.note(
      `after reload: sheet tabs [${sheet.tabsPresent.join(', ')}], status "${sheet.statusLine}", run button "${sheet.runButton}"`
    )
    this.add(judgeHistoryReplay(phrase, normalizeForMatch(joined(sheet)).includes(phrase)))
    const buttonPresent = (await f.page.getByTestId('open-in-git-btn').count()) > 0
    await closeTaskSheet(f.page)
    const requestTabs = Object.values(await readRequestTabs(f.page)).join('\n')
    this.texts.push(requestTabs)
    const added = linesAddedByRun(this.baseline, `${requestTabs}\n${joined(sheet)}`)
    this.add(judgeRunTrace(added))
    if (await openTaskSheetFromPlan(f.page, this.taskId)) {
      const gitText = await openTaskWorktreeInGit(f.page, /calc\.py/)
      const onTask = git.calls.filter((c) => /repo-task/.test(c))
      this.note(
        `git/worktree calls seen after the reload: ${git.calls.length}, naming the task worktree: ${onTask.length}; last on task: ${onTask.slice(-2).join(' || ') || 'none'}; last git.status: ${
          git.calls
            .filter((c) => c.startsWith('git.status'))
            .slice(-2)
            .join(' || ') || 'none'
        }; methods: ${[...new Set(git.calls.map((c) => c.split(' ')[0]))].join(',')}`
      )
      this.texts.push(gitText ?? '')
      this.note(
        `worktree on disk: ${worktree.uncommitted.length} uncommitted file(s) [${worktree.uncommitted.join(', ')}], ${worktree.commitsAhead} commit(s) ahead of the base`
      )
      this.note(
        `Open in Git: button ${buttonPresent ? 'present' : 'absent'}; Source Control rows: ${gitText === null ? 'n/a' : `${gitText.length} chars "${gitText.replace(/\s+/g, ' ').slice(0, 160)}"`}`
      )
      await f.shots.take('worktree-in-git')
      this.add(judgeWorktreeView(buttonPresent, gitText))
      const peek = await peekDiffOfChangedFile(f.page, 'negate')
      this.note(`diff peek (information only, G5d/G5e): ${peek.text}`)
      if (!peek.shown) {
        // Why: backlog evidence, not a verdict of this spec; the crash is the known G5d (git.diff shape) and is allowlisted with that reason.
        f.testInfo.annotations.push({
          type: 'known-gap',
          description: `G5d/G5e: opening a changed file shows no diff (${peek.text})`
        })
        console.log(`[known-gap] G5d/G5e: ${peek.text}`)
      }
      if (gitText !== null) {
        await f.page.waitForTimeout(4_000) // the terminal's spawn error arrives a moment after the view opens
        const body = await f.page.locator('body').innerText()
        this.note(
          `worktree terminal: ${judgeWorktreeTerminal(body).length === 0 ? 'no spawn error on the page' : 'SPAWN ERROR on the page'}`
        )
        this.add(judgeWorktreeTerminal(body))
      }
    } else {
      await f.shots.take('cannot-reopen-task-sheet')
      f.gaps.record(
        'UI-NO-PATH-TO-TASK',
        '[frontend] after reading the Request tabs the Plan tab no longer offers the task row'
      )
    }
  }

  private async judgeDevServerAfterRun(): Promise<void> {
    if (!this.devTab.isOpen) {
      return
    }
    const idle = await this.devTab.sample()
    this.note(
      `dev server: idle UI "${idle.ui}" busy=${idle.busy || 'absent'} platform "${idle.platform || 'absent'}"; while running ${[...new Set(this.devDuring.map((s) => `${s.ui}/busy=${s.busy || 'absent'}`))].join(' | ')}; gateway idle ${idle.api}`
    )
    this.add(judgeDevServer(isOnlineLabel, this.devDuring, idle))
    this.add(judgeDevServerFacts(idle.platform, idle.api))
  }

  /** A secret in anything the user can read, or in what the backend broadcast, fails the run outright (not a gap). */
  private leakScan(frameText: string): void {
    const catalog = loadSecretCatalog()
    const leaks = findSecrets(catalog, `${this.texts.join('\n')}\n${frameText}`)
    this.note(
      `secret scan: ${catalog.values.length} secret-looking variable(s) checked against ${this.texts.length} screen read(s) and the activity frames: ${leaks.length === 0 ? 'clean' : `LEAK ${leaks.join(', ')}`}`
    )
    if (leaks.length > 0) {
      throw new Error(
        `SECRET-LEAK: the agent session screens or activity stream expose ${leaks.join(', ')}`
      )
    }
  }

  summary(): string {
    return `${this.evidence.join(' || ')} || verdicts: ${this.verdicts.map((v) => `${v.code}[${v.owner}]`).join(', ') || 'none'}`
  }
}
