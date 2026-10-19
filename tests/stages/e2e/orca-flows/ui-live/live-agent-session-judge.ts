/**
 * The verdicts of the agent-session monitoring spec as pure functions of what was observed (TASK-FE-E2E-011), so the rules
 * can be unit-tested without a stack or a real Claude. Each verdict is a gap code plus who owns the fix; a rule that cannot
 * be judged (no backend output to compare with) returns nothing instead of guessing.
 */

export type Owner = 'frontend' | 'backend' | 'agent'
export type Verdict = { code: string; owner: Owner; text: string }

export type RunTimes = {
  /** First time the task was seen in_progress, and first time it was seen out of it (ms epoch). */
  startedAt: number
  leftRunningAt: number
}

/** Seconds a frame must precede the end of the run to count as "while the agent was working". */
export const LIVE_MARGIN_MS = 5_000
/** Runs shorter than this cannot tell a stream from a single final flush. */
export const MIN_RUN_FOR_STREAM_MS = 15_000

export function judgeBackendOutput(times: RunTimes, frameTimes: number[]): Verdict[] {
  if (frameTimes.length === 0) {
    return [
      {
        code: 'AGENT-OUTPUT-NOT-BROADCAST',
        owner: 'backend',
        text: 'task.activity.subscribe delivered no agent_output_partial frame for the task during or after the run (check task-service SimpleExecutor.publishThrottledOutput, infra-fleet StreamExecOutput, agent agent.execPrompt.output)'
      }
    ]
  }
  const live = frameTimes.some((t) => t < times.leftRunningAt - LIVE_MARGIN_MS)
  if (!live && times.leftRunningAt - times.startedAt >= MIN_RUN_FOR_STREAM_MS) {
    return [
      {
        code: 'AGENT-OUTPUT-NOT-STREAMED',
        owner: 'agent',
        text: `all ${frameTimes.length} output frame(s) arrived at the end of a ${Math.round((times.leftRunningAt - times.startedAt) / 1000)}s run: claude --print without a streaming output format only prints its final answer, so nothing about files read or tools used exists while it works`
      }
    ]
  }
  return []
}

/** While the task is in_progress the task sheet must say so on its Execute button ("Agent running..."). */
export function judgeRunningIndicator(
  sawInProgress: boolean,
  sawRunningButton: boolean
): Verdict[] {
  return !sawInProgress || sawRunningButton
    ? []
    : [
        {
          code: 'UI-NO-RUNNING-INDICATOR',
          owner: 'frontend',
          text: 'the task was in_progress but the task sheet never showed "Agent running" on its Execute button'
        }
      ]
}

export type UiShown = { seenAt: number | null }

/** The phrase the agent said (from the backend frames) must be readable on the task screen, during the run and after it. */
export function judgeUiShowsOutput(
  times: RunTimes,
  phrase: string,
  during: UiShown,
  backendStreamed: boolean
): Verdict[] {
  if (!phrase) {
    return []
  }
  if (during.seenAt === null) {
    return [
      {
        code: 'UI-AGENT-OUTPUT-NOT-SHOWN',
        owner: 'frontend',
        text: `the backend broadcast the agent's output ("${phrase}...") but no tab of the task sheet showed it; the web never subscribes to task.activity.subscribe (useTaskActivity.ts only polls task.get)`
      }
    ]
  }
  if (backendStreamed && during.seenAt >= times.leftRunningAt) {
    return [
      {
        code: 'UI-AGENT-OUTPUT-NOT-LIVE',
        owner: 'frontend',
        text: 'the agent output appeared on the task sheet only after the task left in_progress'
      }
    ]
  }
  return []
}

export function judgeHistoryReplay(phrase: string, seenAfterReload: boolean): Verdict[] {
  if (!phrase || seenAfterReload) {
    return []
  }
  return [
    {
      code: 'UI-NO-SESSION-HISTORY',
      owner: 'backend',
      text: `after a reload the task screens no longer show what the agent said ("${phrase}..."): task.activity is live-only and the stored last_execution_output is not part of the task view or any channel`
    }
  ]
}

/** Lines of `after` that were not on screen in `before`: what the run added, so a Plan that merely mentions a file is not evidence. */
export function linesAddedByRun(before: string, after: string): string {
  const seen = new Set(before.split('\n').map((l) => l.trim()))
  return after
    .split('\n')
    .filter((l) => l.trim() && !seen.has(l.trim()))
    .join('\n')
}

/** What a user needs to see afterwards: which files the agent changed and that tests were run (`corpus` = text the run added). */
export function judgeRunTrace(
  corpus: string,
  // Why: the defaults are the calc sandbox; a spec on another repo names its own file and test runner.
  trace: { file: RegExp; testsRan: RegExp; fileLabel: string } = {
    file: /src\/calc\.py/i,
    testsRan: /(tests?\/test_calc|unittest|pytest|tests? (passed|pass|ran)|ran \d+ tests?)/i,
    fileLabel: 'src/calc.py'
  }
): Verdict[] {
  const touchedFiles = trace.file.test(corpus)
  const ranTests = trace.testsRan.test(corpus)
  if (touchedFiles && ranTests) {
    return []
  }
  return [
    {
      code: 'UI-NO-RUN-TRACE',
      owner: 'frontend',
      text: `no Request or task screen says what the agent did (${touchedFiles ? '' : 'changed file ${trace.fileLabel} missing'}${!touchedFiles && !ranTests ? '; ' : ''}${ranTests ? '' : 'test run missing'}); check the task Result tab (execution.get) and the Agent activity panel`
    }
  ]
}

export function judgeWorktreeView(
  button: boolean,
  gitText: string | null,
  changedFile: RegExp = /calc\.py/
): Verdict[] {
  if (!button) {
    return [
      {
        code: 'UI-NO-WORKTREE-VIEW',
        owner: 'backend',
        text: 'the task sheet has no "Open in Git" button after the run: the task carries no worktreeId, so there is no way from the task to the files the agent changed'
      }
    ]
  }
  return gitText && changedFile.test(gitText)
    ? []
    : [
        {
          code: 'UI-NO-WORKTREE-VIEW',
          owner: 'frontend',
          text: `"Open in Git" did not land on a Source Control list of the task worktree that shows ${changedFile.source.replaceAll('\\', '')}`
        }
      ]
}

/** The terminal that opens with the task worktree must start: a pty that cannot spawn shows this error on the page. */
export function judgeWorktreeTerminal(pageText: string): Verdict[] {
  return /INFRA_AGENT_SPAWN_PTY_FAILED|failed to spawn pty/i.test(pageText)
    ? [
        {
          code: 'WORKTREE-TERMINAL-FAILED',
          owner: 'agent',
          text: 'the terminal opened with the task worktree reports INFRA_AGENT_SPAWN_PTY_FAILED (the dev server agent cannot spawn a pty; node-pty spawn-helper permissions?)'
        }
      ]
    : []
}

export type DevServerSample = {
  /** Badge label of the card. */
  ui: string
  /** The card's busy chip: "true" | "false" | "" (no chip). */
  busy: string
  /** Gateway row of the dev server (state fields only). */
  api: string
}

const sameSample = (a: DevServerSample, b: DevServerSample): boolean =>
  a.ui === b.ui && a.busy === b.busy && a.api === b.api

/** The dev server must stay online, and its state must differ between "agent working" and "idle" (the user's own words). */
export function judgeDevServer(
  isOnline: (label: string) => boolean,
  during: DevServerSample[],
  idle: DevServerSample | null
): Verdict[] {
  const verdicts: Verdict[] = []
  const offline = during.filter((s) => !isOnline(s.ui))
  if (offline.length > 0) {
    verdicts.push({
      code: 'DEVSERVER-OFFLINE-IN-UI',
      owner: 'frontend',
      text: `Settings > Dev Servers showed "${offline[0].ui}" while the agent was working (${offline.length} of ${during.length} samples)`
    })
  }
  if (during.length > 0 && !during.some((s) => s.busy === 'true')) {
    verdicts.push({
      code: 'DEVSERVER-NEVER-SHOWED-BUSY',
      owner: 'backend',
      text: `the dev server card never showed Busy in ${during.length} samples while the agent ran a task (busy chip: ${[...new Set(during.map((s) => s.busy || 'absent'))].join(', ')}; gateway ${during.at(-1)?.api})`
    })
  }
  if (idle && during.length > 0 && during.every((s) => sameSample(s, idle))) {
    verdicts.push({
      code: 'DEVSERVER-NO-BUSY-STATE',
      owner: 'backend',
      text: `the dev server looks identical while the agent runs a task and when idle (UI "${idle.ui}" busy=${idle.busy || 'absent'}, gateway ${idle.api})`
    })
  }
  if (idle && idle.busy !== 'false') {
    verdicts.push({
      code: 'DEVSERVER-NOT-IDLE-AFTER-RUN',
      owner: 'backend',
      text: `after the run the dev server card shows busy="${idle.busy || 'absent'}" instead of Idle (gateway ${idle.api})`
    })
  }
  return verdicts
}

/** Facts of an idle, connected dev server a user expects to read: its platform and a healthy status. */
export function judgeDevServerFacts(platformText: string, apiRow: string): Verdict[] {
  const verdicts: Verdict[] = []
  if (!platformText) {
    verdicts.push({
      code: 'DEVSERVER-NO-PLATFORM',
      owner: 'agent',
      text: 'a connected dev server shows no platform (dev-server-platform): the agent does not send platform/arch/node in its handshake, or the gateway drops them'
    })
  }
  if (!apiRow.includes('"healthStatus":"healthy"')) {
    verdicts.push({
      code: 'DEVSERVER-NOT-HEALTHY',
      owner: 'backend',
      text: `a connected dev server is not reported healthy (${apiRow})`
    })
  }
  return verdicts
}

export type ActivityPanelSample = { state: string; live: boolean; lines: number }

/** The "Agent activity" panel: present, "running" with the Live badge while in_progress, "finished" with lines afterwards. */
export function judgeActivityPanel(
  duringRun: ActivityPanelSample[],
  afterRun: ActivityPanelSample | null
): Verdict[] {
  if (duringRun.length > 0 && duringRun.every((s) => s.state === 'absent')) {
    return [
      {
        code: 'UI-NO-AGENT-ACTIVITY-PANEL',
        owner: 'frontend',
        text: 'the task sheet of a running direct-agent task has no Agent activity panel (task-agent-activity)'
      }
    ]
  }
  const verdicts: Verdict[] = []
  if (duringRun.length > 0 && !duringRun.some((s) => s.state === 'running' && s.live)) {
    verdicts.push({
      code: 'UI-ACTIVITY-PANEL-STATE',
      owner: 'frontend',
      text: `the panel never showed state=running with the Live badge while the task was in_progress (states: ${[...new Set(duringRun.map((s) => s.state))].join(', ')})`
    })
  }
  if (afterRun && (afterRun.state !== 'finished' || afterRun.lines === 0)) {
    verdicts.push({
      code: 'UI-ACTIVITY-PANEL-STATE',
      owner: 'frontend',
      text: `after the run the panel reads state=${afterRun.state} with ${afterRun.lines} line(s) instead of finished with the run's lines`
    })
  }
  return verdicts
}

/** After the task leaves in_progress the sheet (which polls task.get every 4 s) must show one of the statuses it moved to. */
export function judgeStatusFollows(api: string[], sheet: string[]): Verdict[] {
  const after = [...new Set(api)].filter((s) => s && s !== 'in_progress')
  if (after.length === 0 || sheet.length === 0) {
    return []
  }
  return sheet.some((line) => after.some((s) => line.includes(s)))
    ? []
    : [
        {
          code: 'UI-TASK-STATUS-STALE',
          owner: 'frontend',
          text: `the task moved to ${after.join(', ')} but the task sheet only showed: ${[...new Set(sheet)].slice(-3).join(' | ')}`
        }
      ]
}
