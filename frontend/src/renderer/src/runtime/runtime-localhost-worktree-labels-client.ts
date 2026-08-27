import type { GlobalSettings } from '../../../shared/types'
import type {
  LocalhostWorktreeLabelResult,
  LocalhostWorktreeLabelRoute
} from '../../../shared/localhost-worktree-labels'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type LocalhostWorktreeLabelsSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

// Why: not `async` — an async wrapper adopts its returned promise through an
// extra microtask tick versus returning the preload call's promise directly,
// which shifts call-site timing existing tests assert against.
export function registerLocalhostWorktreeLabel(
  settings: LocalhostWorktreeLabelsSettings | null | undefined,
  route: LocalhostWorktreeLabelRoute
): Promise<LocalhostWorktreeLabelResult> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.localhostWorktreeLabels.register(route)
  }
  return callRuntimeRpc<LocalhostWorktreeLabelResult>(
    target,
    'localhostWorktreeLabels.register',
    route
  )
}
