import { useMemo } from 'react'
import { useAppStore } from '@/store'
import { getHostDisplayLabelOverrides } from '../../../../shared/host-setting-overrides'
import {
  buildSidebarHostOptions,
  buildSidebarHostScopeOptions,
  type SidebarHostOption,
  type SidebarHostScopeOption
} from './sidebar-host-options'
import { isWebClientLocation } from '@/lib/web-client-location'
import type { ExecutionHostId } from '../../../../shared/execution-host'

/** Shared host-scope derivation for the sidebar scope strip and the workspace
 * options menu so both surfaces consume the same live runtime status without
 * duplicating store wiring. */
export function useSidebarHostScopeOptions(): {
  hostOptions: SidebarHostOption[]
  hostScopeOptions: SidebarHostScopeOption[]
} {
  const repos = useAppStore((s) => s.repos)
  const sshTargetLabels = useAppStore((s) => s.sshTargetLabels)
  const sshConnectionStates = useAppStore((s) => s.sshConnectionStates)
  const settings = useAppStore((s) => s.settings)
  const runtimeEnvironments = useAppStore((s) => s.runtimeEnvironments)
  const runtimeStatusByEnvironmentId = useAppStore((s) => s.runtimeStatusByEnvironmentId)
  // Why: in web mode, Dev Servers replace the local/ssh/runtime host options.
  const devServers = useAppStore((s) => s.devServers)

  const hostLabelOverrides = useMemo(() => getHostDisplayLabelOverrides(settings), [settings])
  const isWeb = isWebClientLocation()

  const hostOptions = useMemo(() => {
    if (isWeb) {
      // Web mode: only surface connected Dev Servers as selectable hosts.
      // Map each connected DevServer to a synthetic SidebarHostOption using
      // a `devserver:<id>` ExecutionHostId so the rest of the pipeline can
      // treat them uniformly without knowing about DevServer internals.
      return devServers
        .filter((ds) => ds.status === 'connected')
        .map<SidebarHostOption>((ds) => ({
          id: `devserver:${ds.id}` as ExecutionHostId,
          label: ds.name,
          detail: ds.platform ? `${ds.platform}${ds.arch ? ` · ${ds.arch}` : ''}` : 'connected',
          kind: 'runtime', // closest semantic match; treated as selectable remote host
          health: 'available',
          presence: 'active',
        }))
    }
    return buildSidebarHostOptions({
      repos,
      sshTargetLabels,
      sshConnectionStates,
      settings,
      runtimeEnvironments,
      runtimeStatusByEnvironmentId,
      hostLabelOverrides
    })
  }, [
    isWeb,
    devServers,
    repos,
    sshTargetLabels,
    sshConnectionStates,
    settings,
    runtimeEnvironments,
    runtimeStatusByEnvironmentId,
    hostLabelOverrides
  ])

  const hostScopeOptions = useMemo(() => buildSidebarHostScopeOptions(hostOptions), [hostOptions])

  return { hostOptions, hostScopeOptions }
}
