/* Why: mirrors runtime-git-client.ts's hybrid-routing shape for the `gl.*`
   preload namespace — desktop-local calls stay on window.api.gl (real IPC),
   paired/web callers go straight to the `gitlab.*` runtime RPC instead of
   round-tripping through the web preload shim's own copy of this mapping. */
import type { GlobalSettings } from '../../../shared/types'
import type { PreloadApi } from '../../../preload/api-types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type GitLabApi = NonNullable<PreloadApi['gl']>
type GitLabArgs<K extends keyof GitLabApi> = Parameters<GitLabApi[K]>[0]
type GitLabResult<K extends keyof GitLabApi> = Awaited<ReturnType<GitLabApi[K]>>

type RuntimeGitLabSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

// Why: runtime repo selectors accept loose path/name forms, but duplicate
// checked-out repos can make those ambiguous — prefer the explicit repo id
// selector when the caller has one. Mirrors web-preload-api.ts's mapRepoPathArg.
function toGitLabRuntimeArgs(args: unknown): unknown {
  if (!args || typeof args !== 'object' || !('repoPath' in args)) {
    return args
  }
  const record = args as Record<string, unknown>
  const repoId = typeof record.repoId === 'string' && record.repoId.trim() ? record.repoId : null
  return {
    ...record,
    repo: repoId ? `id:${repoId}` : record.repoPath
  }
}

async function routeGitLab<K extends keyof GitLabApi>(
  settings: RuntimeGitLabSettings | null | undefined,
  localMethod: K,
  rpcMethod: string,
  args: GitLabArgs<K>,
  rpcArgs: unknown = args
): Promise<GitLabResult<K>> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return (window.api.gl[localMethod] as (a: GitLabArgs<K>) => Promise<GitLabResult<K>>)(args)
  }
  return callRuntimeRpc<GitLabResult<K>>(target, rpcMethod, toGitLabRuntimeArgs(rpcArgs), {
    timeoutMs: 30_000
  })
}

export function getRuntimeGitLabWorkItemDetails(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'workItemDetails'>
): Promise<GitLabResult<'workItemDetails'>> {
  return routeGitLab(settings, 'workItemDetails', 'gitlab.workItemDetails', args)
}

export function listRuntimeGitLabLabels(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'listLabels'>
): Promise<GitLabResult<'listLabels'>> {
  return routeGitLab(settings, 'listLabels', 'gitlab.listLabels', args)
}

export function updateRuntimeGitLabMR(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'updateMR'>
): Promise<GitLabResult<'updateMR'>> {
  return routeGitLab(settings, 'updateMR', 'gitlab.updateMR', args)
}

export function getRuntimeGitLabJobTrace(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'jobTrace'>
): Promise<GitLabResult<'jobTrace'>> {
  return routeGitLab(settings, 'jobTrace', 'gitlab.jobTrace', args)
}

export function retryRuntimeGitLabJob(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'retryJob'>
): Promise<GitLabResult<'retryJob'>> {
  return routeGitLab(settings, 'retryJob', 'gitlab.retryJob', args)
}

export function updateRuntimeGitLabMRReviewers(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'updateMRReviewers'>
): Promise<GitLabResult<'updateMRReviewers'>> {
  return routeGitLab(settings, 'updateMRReviewers', 'gitlab.updateMRReviewers', args)
}

export function addRuntimeGitLabMRInlineComment(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'addMRInlineComment'>
): Promise<GitLabResult<'addMRInlineComment'>> {
  return routeGitLab(settings, 'addMRInlineComment', 'gitlab.addMRInlineComment', args)
}

export function closeRuntimeGitLabMR(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'closeMR'>
): Promise<GitLabResult<'closeMR'>> {
  return routeGitLab(settings, 'closeMR', 'gitlab.updateMRState', args, { ...args, state: 'closed' })
}

export function reopenRuntimeGitLabMR(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'reopenMR'>
): Promise<GitLabResult<'reopenMR'>> {
  return routeGitLab(settings, 'reopenMR', 'gitlab.updateMRState', args, {
    ...args,
    state: 'opened'
  })
}

export function mergeRuntimeGitLabMR(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'mergeMR'>
): Promise<GitLabResult<'mergeMR'>> {
  return routeGitLab(settings, 'mergeMR', 'gitlab.mergeMR', args)
}

export function addRuntimeGitLabMRComment(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'addMRComment'>
): Promise<GitLabResult<'addMRComment'>> {
  return routeGitLab(settings, 'addMRComment', 'gitlab.addMRComment', args)
}

export function addRuntimeGitLabIssueComment(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'addIssueComment'>
): Promise<GitLabResult<'addIssueComment'>> {
  return routeGitLab(settings, 'addIssueComment', 'gitlab.addIssueComment', args)
}

export function resolveRuntimeGitLabMRDiscussion(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'resolveMRDiscussion'>
): Promise<GitLabResult<'resolveMRDiscussion'>> {
  return routeGitLab(settings, 'resolveMRDiscussion', 'gitlab.resolveMRDiscussion', args)
}

export function listRuntimeGitLabIssues(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'listIssues'>
): Promise<GitLabResult<'listIssues'>> {
  return routeGitLab(settings, 'listIssues', 'gitlab.listIssues', args)
}

export function listRuntimeGitLabMRs(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'listMRs'>
): Promise<GitLabResult<'listMRs'>> {
  return routeGitLab(settings, 'listMRs', 'gitlab.listMRs', args)
}

export function listRuntimeGitLabTodos(
  settings: RuntimeGitLabSettings | null | undefined,
  args: GitLabArgs<'todos'>
): Promise<GitLabResult<'todos'>> {
  return routeGitLab(settings, 'todos', 'gitlab.todos', args)
}
