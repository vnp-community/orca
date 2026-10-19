import { useCallback, useEffect, useMemo, useState } from 'react'
import { useAppStore } from '@/store'
import type { ExecutionHostId } from '../../../../shared/execution-host'
import type { DevServer } from '../../../../shared/dev-server-types'
import type { SidebarHostOption } from './sidebar-host-options'
import {
  deriveAddRepoHostGate,
  pickAutoSelectedWebHostId,
  readRememberedWebHostId,
  READY_ADD_REPO_HOST_GATE,
  rememberWebHostId,
  type AddRepoHostGate
} from './add-repo-web-host-gate'

const NO_DEV_SERVERS: DevServer[] = []

/**
 * Web "Add Project" host choice: one connected dev server is picked for the user, several must be
 * chosen, none blocks. A chosen host that drops stays chosen (as `host-unavailable`) so the dialog
 * explains instead of silently resetting to another host.
 */
export function useAddRepoWebHostGate({
  isOpen,
  isWeb,
  connectedHostOptions
}: {
  isOpen: boolean
  isWeb: boolean
  connectedHostOptions: SidebarHostOption[]
}): {
  webSelectedHostId: ExecutionHostId | null
  hostGate: AddRepoHostGate
  gatedHostOptions: SidebarHostOption[]
  selectWebHost: (hostId: ExecutionHostId) => void
} {
  const devServers = useAppStore((s) => s.devServers) ?? NO_DEV_SERVERS
  const [chosenHostId, setChosenHostId] = useState<ExecutionHostId | null>(null)
  const connectedHostIds = useMemo(
    () => connectedHostOptions.map((host) => host.id),
    [connectedHostOptions]
  )
  const remembered = useMemo(
    () => (isWeb && isOpen ? readRememberedWebHostId() : null),
    [isWeb, isOpen]
  )
  const autoPickedHostId = isWeb ? pickAutoSelectedWebHostId(connectedHostIds, remembered) : null
  const webSelectedHostId = isWeb ? (chosenHostId ?? autoPickedHostId) : null

  useEffect(() => {
    if (!isOpen) {
      // Each opening starts from the automatic pick, not from a previous dialog's choice.
      setChosenHostId(null)
      return
    }
    if (isWeb && chosenHostId === null && autoPickedHostId) {
      // Pin the automatic pick so a later disconnect shows "unavailable" instead of re-picking.
      setChosenHostId(autoPickedHostId)
    }
  }, [autoPickedHostId, chosenHostId, isOpen, isWeb])

  const hostGate = useMemo(
    () =>
      isWeb
        ? deriveAddRepoHostGate({ chosenHostId: webSelectedHostId, connectedHostIds, devServers })
        : READY_ADD_REPO_HOST_GATE,
    [connectedHostIds, devServers, isWeb, webSelectedHostId]
  )

  const gatedHostOptions = useMemo(() => {
    if (!isWeb || hostGate.status !== 'host-unavailable') {
      return connectedHostOptions
    }
    const unavailable: SidebarHostOption = {
      id: hostGate.hostId,
      label: hostGate.label,
      detail: 'Dev Server',
      kind: 'devServer',
      health:
        hostGate.reason === 'connecting'
          ? 'connecting'
          : hostGate.reason === 'error'
            ? 'error'
            : 'disconnected',
      presence: 'project'
    }
    return [unavailable, ...connectedHostOptions]
  }, [connectedHostOptions, hostGate, isWeb])

  const selectWebHost = useCallback((hostId: ExecutionHostId): void => {
    setChosenHostId(hostId)
    rememberWebHostId(hostId)
  }, [])

  return { webSelectedHostId, hostGate, gatedHostOptions, selectWebHost }
}
