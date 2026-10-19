/** Runs git in `cwd` with `args` and resolves with stdout; rejects on a non-zero exit. */
export type GitStdoutRunnerInDir = (args: string[], cwd: string) => Promise<string>

export class WorktreeWrongBranchError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'WorktreeWrongBranchError'
  }
}

/**
 * Why: a task worktree that silently sits on another branch (git's DWIM can
 * check out a tracking branch instead of the requested `-b` one) would commit
 * to the person's release branch. Never leave such a worktree behind.
 */
export async function assertWorktreeOnBranch(
  run: GitStdoutRunnerInDir,
  repoPath: string,
  worktreePath: string,
  branch: string
): Promise<void> {
  let actual = ''
  try {
    actual = (await run(['symbolic-ref', '--short', 'HEAD'], worktreePath)).trim()
  } catch {
    // Detached or unreadable HEAD is also "not on the requested branch".
  }
  if (actual === branch) {
    return
  }

  let cleanup = 'the worktree was removed'
  try {
    await run(['worktree', 'remove', '--force', worktreePath], repoPath)
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    cleanup = `removing the worktree failed (${reason}); remove it manually`
  }
  throw new WorktreeWrongBranchError(
    `worktree was created on ${actual ? `branch "${actual}"` : 'a detached HEAD'} instead of ` +
      `the requested branch "${branch}"; ${cleanup}`
  )
}
