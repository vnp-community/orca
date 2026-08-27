import type { SkillDiscoveryResult } from '../../../shared/skills'
import type { GlobalSettings } from '../../../shared/types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

export type RuntimeSkillsSettings =
  | Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>
  | null
  | undefined

/** Discovers skills for the active runtime target. Desktop's `skills:discover`
 *  IPC always scans the local main process's own repos, so a "focused" remote
 *  environment needs the RPC branch instead — otherwise the Skills page would
 *  silently show the desktop machine's skills while focused elsewhere. */
export async function discoverRuntimeSkills(
  settings: RuntimeSkillsSettings
): Promise<SkillDiscoveryResult> {
  const target = getActiveRuntimeTarget(settings)
  return target.kind === 'environment'
    ? callRuntimeRpc<SkillDiscoveryResult>(target, 'skills.discover', undefined, {
        timeoutMs: 15_000
      })
    : window.api.skills.discover()
}
