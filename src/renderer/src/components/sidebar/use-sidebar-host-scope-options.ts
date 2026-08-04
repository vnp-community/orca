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
    const options = buildSidebarHostOptions({
      repos,
      sshTargetLabels,
      sshConnectionStates,
      settings,
      runtimeEnvironments,
      runtimeStatusByEnvironmentId,
      devServers,
      hostLabelOverrides
    })
    if (isWeb) {
      // Why: web mode has no local filesystem/SSH access, so only connected
      // Dev Servers are viable selectable hosts there.
      return options.filter((option) => option.kind === 'devServer' && option.health === 'available')
    }
    return options
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
