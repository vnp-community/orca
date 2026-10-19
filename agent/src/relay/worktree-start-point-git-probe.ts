import type { WorktreeStartPointProbe } from '../shared/worktree-base-ref'

/** Runs git with `args` and resolves with stdout; rejects on a non-zero exit. */
export type GitStdoutRunner = (args: string[]) => Promise<string>

const COMMIT_SHA = /^[0-9a-f]{40}([0-9a-f]{24})?$/

export function createWorktreeStartPointProbe(run: GitStdoutRunner): WorktreeStartPointProbe {
  return {
    async resolveCommit(revision) {
      try {
        const sha = (await run(['rev-parse', '--verify', '--quiet', `${revision}^{commit}`])).trim()
        return COMMIT_SHA.test(sha) ? sha : null
      } catch (error) {
        // Why: with --quiet git exits 1 only for "no such commit"; other failures
        // (not a repo, git missing, timeout) must not be reported as a missing ref.
        if ((error as { code?: unknown } | null)?.code === 1) {
          return null
        }
        throw error
      }
    },
    async listRemotes() {
      const stdout = await run(['remote'])
      return stdout
        .split('\n')
        .map((line) => line.trim())
        .filter((line) => line.length > 0)
    }
  }
}
