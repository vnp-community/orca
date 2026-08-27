/* Why: mirrors runtime-git-client.ts's hybrid-routing shape for the small set
   of `gh.*` preload methods that still called window.api.gh unconditionally
   (most gh.* call sites already branch on target.kind inline; this wrapper
   exists for the few that didn't, plus provides one shared place for any
   still-unmigrated site to route through). Desktop-local calls stay on
   window.api.gh (real IPC); paired/web callers go straight to the `github.*`
   runtime RPC instead of round-tripping through the web preload shim. */
import type { GlobalSettings } from '../../../shared/types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type RuntimeGitHubSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

export function getRuntimeGitHubRepoSlug(
  settings: RuntimeGitHubSettings | null | undefined,
  args: Parameters<typeof window.api.gh.repoSlug>[0]
): ReturnType<typeof window.api.gh.repoSlug> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.gh.repoSlug(args)
  }
  return callRuntimeRpc(target, 'github.repoSlug', { repo: args.repoId ? `id:${args.repoId}` : args.repoPath }, {
    timeoutMs: 30_000
  })
}

export function updateRuntimeGitHubPRTitle(
  settings: RuntimeGitHubSettings | null | undefined,
  args: Parameters<typeof window.api.gh.updatePRTitle>[0]
): ReturnType<typeof window.api.gh.updatePRTitle> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.gh.updatePRTitle(args)
  }
  const { repoPath, repoId, ...rest } = args
  return callRuntimeRpc(
    target,
    'github.updatePRTitle',
    { repo: repoId ? `id:${repoId}` : repoPath, ...rest },
    { timeoutMs: 30_000 }
  )
}

export function updateRuntimeGitHubIssueCommentBySlug(
  settings: RuntimeGitHubSettings | null | undefined,
  args: Parameters<typeof window.api.gh.updateIssueCommentBySlug>[0]
): ReturnType<typeof window.api.gh.updateIssueCommentBySlug> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.gh.updateIssueCommentBySlug(args)
  }
  return callRuntimeRpc(target, 'github.project.updateIssueCommentBySlug', args, {
    timeoutMs: 30_000
  })
}

export function deleteRuntimeGitHubIssueCommentBySlug(
  settings: RuntimeGitHubSettings | null | undefined,
  args: Parameters<typeof window.api.gh.deleteIssueCommentBySlug>[0]
): ReturnType<typeof window.api.gh.deleteIssueCommentBySlug> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.gh.deleteIssueCommentBySlug(args)
  }
  return callRuntimeRpc(target, 'github.project.deleteIssueCommentBySlug', args, {
    timeoutMs: 30_000
  })
}

export function checkRuntimeOrcaStarred(
  settings: RuntimeGitHubSettings | null | undefined
): ReturnType<typeof window.api.gh.checkOrcaStarred> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.gh.checkOrcaStarred()
  }
  return callRuntimeRpc(target, 'github.checkOrcaStarred', {}, { timeoutMs: 15_000 })
}

export function starRuntimeOrca(
  settings: RuntimeGitHubSettings | null | undefined,
  source: Parameters<typeof window.api.gh.starOrca>[0]
): ReturnType<typeof window.api.gh.starOrca> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.gh.starOrca(source)
  }
  return callRuntimeRpc(target, 'github.starOrca', { source }, { timeoutMs: 15_000 })
}
