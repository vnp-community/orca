import { callRuntimeRpc, getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'

export type RepoHostRebindResult =
  | { ok: true }
  | {
      ok: false
      kind: 'active-workflows' | 'dev-server-not-found' | 'forbidden' | 'failed'
      message: string
    }

function describeRebindFailure(err: unknown): Extract<RepoHostRebindResult, { ok: false }> {
  const raw = err instanceof Error ? err.message : String(err)
  const lower = raw.toLowerCase()
  if (lower.includes('project_has_active_workflows')) {
    return {
      ok: false,
      kind: 'active-workflows',
      message: translate(
        'auto.lib.repoHostRebind.activeWorkflows',
        'Stop the tasks running on this project before changing its dev server.'
      )
    }
  }
  if (lower.includes('project_dev_server_not_found') || lower.includes('dev_server_not_found')) {
    return {
      ok: false,
      kind: 'dev-server-not-found',
      message: translate(
        'auto.lib.repoHostRebind.devServerNotFound',
        'That dev server no longer exists. Choose another one.'
      )
    }
  }
  if (/^forbidden/i.test(raw) || raw === 'UNAUTHENTICATED') {
    return {
      ok: false,
      kind: 'forbidden',
      message: translate(
        'auto.lib.repoHostRebind.forbidden',
        'You do not have permission to change this project’s dev server.'
      )
    }
  }
  return {
    ok: false,
    kind: 'failed',
    message: raw || translate('auto.lib.repoHostRebind.failed', 'Failed to change the dev server.')
  }
}

/**
 * Re-binds one repo to another dev server (repo.rebindDevServer) and refreshes what depends on the
 * host. Never called automatically: moving a repo is always the user's explicit action.
 */
export async function rebindRepoToDevServer(
  repoId: string,
  newDevServerId: string
): Promise<RepoHostRebindResult> {
  const state = useAppStore.getState()
  try {
    await callRuntimeRpc(getActiveRuntimeTarget(state.settings), 'repo.rebindDevServer', {
      repoId,
      newDevServerId
    })
  } catch (err) {
    return describeRebindFailure(err)
  }
  try {
    await state.fetchRepos()
    await useAppStore.getState().fetchWorktrees(repoId)
  } catch (err) {
    // The rebind itself succeeded; a stale list refreshes on the next sync.
    console.warn('[repo-host-rebind] refresh after rebind failed', err)
  }
  return { ok: true }
}
