/**
 * "Open in Git" of the finished task and the judgement of what it shows (part of the agent-session monitor): the Source Control
 * list of the task worktree, the git calls the page made, the diff peek and the terminal's spawn error. Split from
 * live-agent-session-monitor.ts to keep both under the line budget.
 */
import type { LiveFlow } from './live-flow-harness'
import { judgeWorktreeTerminal, judgeWorktreeView, type Verdict } from './live-agent-session-judge'
import type { MonitorTargets } from './live-agent-session-targets'
import {
  openTaskSheetFromPlan,
  openTaskWorktreeInGit,
  peekDiffOfChangedFile
} from './live-agent-session-ui'
import type { WorktreeChangeState } from './live-fixture-change-checks'

export type WorktreeViewSink = {
  note: (line: string) => void
  add: (verdicts: Verdict[]) => void
  texts: string[]
}

export async function inspectWorktreeInGit(
  f: LiveFlow,
  o: {
    taskId: string
    targets: MonitorTargets
    git: { calls: string[] }
    worktree: WorktreeChangeState
    buttonPresent: boolean
  },
  sink: WorktreeViewSink
): Promise<void> {
  const { git, worktree, buttonPresent } = o
  if (await openTaskSheetFromPlan(f.page, o.taskId)) {
    const gitText = await openTaskWorktreeInGit(f.page, o.targets.changedFile)
    const onTask = git.calls.filter((c) => o.targets.worktreeCall.test(c))
    sink.note(
      `git/worktree calls seen after the reload: ${git.calls.length}, naming the task worktree: ${onTask.length}; last on task: ${onTask.slice(-2).join(' || ') || 'none'}; last git.status: ${
        git.calls
          .filter((c) => c.startsWith('git.status'))
          .slice(-2)
          .join(' || ') || 'none'
      }; methods: ${[...new Set(git.calls.map((c) => c.split(' ')[0]))].join(',')}`
    )
    sink.texts.push(gitText ?? '')
    sink.note(
      `worktree on disk: ${worktree.uncommitted.length} uncommitted file(s) [${worktree.uncommitted.join(', ')}], ${worktree.commitsAhead} commit(s) ahead of the base`
    )
    sink.note(
      `Open in Git: button ${buttonPresent ? 'present' : 'absent'}; Source Control rows: ${gitText === null ? 'n/a' : `${gitText.length} chars "${gitText.replace(/\s+/g, ' ').slice(0, 160)}"`}`
    )
    await f.shots.take('worktree-in-git')
    sink.add(judgeWorktreeView(buttonPresent, gitText, o.targets.changedFile))
    const peek = await peekDiffOfChangedFile(f.page, o.targets.diffWord)
    sink.note(`diff peek (information only, G5d/G5e): ${peek.text}`)
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
      sink.note(
        `worktree terminal: ${judgeWorktreeTerminal(body).length === 0 ? 'no spawn error on the page' : 'SPAWN ERROR on the page'}`
      )
      sink.add(judgeWorktreeTerminal(body))
    }
  } else {
    await f.shots.take('cannot-reopen-task-sheet')
    f.gaps.record(
      'UI-NO-PATH-TO-TASK',
      '[frontend] after reading the Request tabs the Plan tab no longer offers the task row'
    )
  }
}
