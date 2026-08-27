/**
 * Remote sync operations (push/pull/fetch/upstream status) extracted from
 * git-handler.ts (GitHandler.push/pullWithArgs/pull/fastForward/fetch/
 * upstreamStatus) so the Dev Server WS agent's Part A dispatcher
 * (agent-git-handler-extended.ts) can re-export them without depending on
 * the GitHandler class.
 *
 * Why the cache-clearing calls are gone: see git-handler-staging-ops.ts —
 * Part A has no per-connection read cache to invalidate.
 */
import type { GitExec } from './git-handler-ops'
import { resolveRelayPushTarget } from './git-handler-push-target'
import { assertGitPushTargetShape } from '../shared/git-push-target-validation'
import { isNoUpstreamError, normalizeGitErrorMessage } from '../shared/git-remote-error'
import { resolveEffectiveGitUpstream, getEffectiveGitUpstreamStatus } from '../shared/git-effective-upstream'
import { getPublishTargetStatus, type GitCommandRunner } from '../shared/git-publish-target-status'
import { upstreamOnlyCommitsArePatchEquivalent } from '../shared/git-upstream-status'
import type { GitPushTarget } from '../shared/types'

export async function push(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  // Why: mirror src/main/git/remote.ts. Push to a configured upstream when
  // present so SSH worktrees with non-origin targets do not get repointed.
  void params.publish
  try {
    const target = await resolveRelayPushTarget(git, worktreePath, params.pushTarget)
    const args = [
      'push',
      ...(params.forceWithLease === true ? ['--force-with-lease'] : []),
      '--set-upstream',
      ...(target ? [target.remote, target.refspec] : ['origin', 'HEAD'])
    ]
    await git(args, worktreePath)
  } catch (error) {
    // Why: mirror the local gitPush normalization so users see the same
    // "non-fast-forward / pull first" guidance instead of raw git stderr.
    throw new Error(normalizeGitErrorMessage(error, 'push'))
  }
}

export async function pullWithArgs(
  git: GitExec,
  params: Record<string, unknown>,
  pullArgs: string[]
): Promise<void> {
  const worktreePath = params.worktreePath as string
  try {
    if (params.pushTarget !== undefined) {
      assertGitPushTargetShape(params.pushTarget)
      const pushTarget = params.pushTarget as GitPushTarget
      await git(['check-ref-format', '--branch', pushTarget.branchName], worktreePath)
      await git(['pull', ...pullArgs, pushTarget.remoteName, pushTarget.branchName], worktreePath)
      return
    }
    const upstream = await resolveEffectiveGitUpstream((args) => git(args, worktreePath))
    if (upstream && !upstream.isConfiguredUpstream) {
      // Why: legacy Orca branches may still track origin/main while pushes
      // target origin/<branch>. Pull the same effective branch the UI reports.
      await git(['pull', ...pullArgs, upstream.remoteName, upstream.branchName], worktreePath)
      return
    }
    await git(['pull', ...pullArgs], worktreePath)
  } catch (error) {
    // Why: mirror the local gitPull normalization so users see the same
    // actionable messages instead of raw git stderr.
    throw new Error(normalizeGitErrorMessage(error, 'pull'))
  }
}

export async function pull(git: GitExec, params: Record<string, unknown>): Promise<void> {
  // Why: plain `git pull` honors the user's configured merge/rebase/ff policy.
  // If no policy exists, Git's policy error is normalized with setup guidance.
  await pullWithArgs(git, params, [])
}

export async function fastForward(git: GitExec, params: Record<string, unknown>): Promise<void> {
  await pullWithArgs(git, params, ['--ff-only'])
}

export async function fetch(git: GitExec, params: Record<string, unknown>): Promise<void> {
  const worktreePath = params.worktreePath as string
  try {
    if (params.pushTarget !== undefined) {
      assertGitPushTargetShape(params.pushTarget)
      const pushTarget = params.pushTarget as GitPushTarget
      await git(['check-ref-format', '--branch', pushTarget.branchName], worktreePath)
      await git(['fetch', '--prune', pushTarget.remoteName], worktreePath)
      return
    }
    await git(['fetch', '--prune'], worktreePath)
  } catch (error) {
    // Why: mirror the local gitFetch normalization so users see the same
    // actionable messages instead of raw git stderr (which varies across
    // versions/locales and may embed credentials).
    throw new Error(normalizeGitErrorMessage(error, 'fetch'))
  }
}

async function getBehindCommitsArePatchEquivalent(
  git: GitExec,
  worktreePath: string,
  upstreamName: string
): Promise<boolean> {
  try {
    const { stdout } = await git(
      ['log', '--oneline', '--cherry-mark', '--right-only', `HEAD...${upstreamName}`, '--'],
      worktreePath
    )
    return upstreamOnlyCommitsArePatchEquivalent(stdout)
  } catch {
    // Why: this only identifies stale post-rebase upstreams. If the probe
    // fails, keep the conservative pull-first sync path.
    return false
  }
}

export async function upstreamStatus(git: GitExec, params: Record<string, unknown>) {
  const worktreePath = params.worktreePath as string

  try {
    if (params.pushTarget !== undefined) {
      assertGitPushTargetShape(params.pushTarget)
      const pushTarget = params.pushTarget as GitPushTarget
      await git(['check-ref-format', '--branch', pushTarget.branchName], worktreePath)
      return await getPublishTargetStatus(
        ((args) => git(args, worktreePath)) as GitCommandRunner,
        pushTarget,
        (upstreamName) => getBehindCommitsArePatchEquivalent(git, worktreePath, upstreamName)
      )
    }
    return await getEffectiveGitUpstreamStatus(
      (args) => git(args, worktreePath),
      (upstreamName) => getBehindCommitsArePatchEquivalent(git, worktreePath, upstreamName)
    )
  } catch (error) {
    // Why: we only swallow the 'no upstream configured' error — that's an
    // expected state, not a failure. Other errors (auth, corruption, network)
    // should surface to the user so they can act on them.
    if (isNoUpstreamError(error)) {
      return { hasUpstream: false, ahead: 0, behind: 0 }
    }
    // Why: match fetch/push/pull normalization so execFile preamble and local
    // paths don't leak to the renderer.
    throw new Error(normalizeGitErrorMessage(error, 'upstream'))
  }
}
