/**
 * What the agent-session monitor looks for in the task worktree of the repo under test. The defaults describe the calc sandbox of
 * the throwaway fixture; a spec on another repo passes its own.
 */
export type MonitorTargets = {
  /** A changed file of the task as Source Control lists it. */
  changedFile: RegExp
  /** A git/worktree call that names the task's worktree. */
  worktreeCall: RegExp
  /** A word that the diff of the changed file contains. */
  diffWord: string
  /** What the screens must say the agent changed, and that tests ran (the run trace). */
  trace: { file: RegExp; testsRan: RegExp; fileLabel: string }
}

export const CALC_TARGETS: MonitorTargets = {
  changedFile: /calc\.py/,
  worktreeCall: /repo-task/,
  diffWord: 'negate',
  trace: {
    file: /src\/calc\.py/i,
    testsRan: /(tests?\/test_calc|unittest|pytest|tests? (passed|pass|ran)|ran \d+ tests?)/i,
    fileLabel: 'src/calc.py'
  }
}
