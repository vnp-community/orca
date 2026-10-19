import { parseExecutionHostId, type ExecutionHostId } from '../../../../shared/execution-host'
import type { DevServerSummary } from '../../../../shared/execution-host-registry'

export type AddRepoHostUnavailableReason = 'connecting' | 'disconnected' | 'error' | 'removed'

// Web only: desktop always has a usable local/SSH/runtime host, so its gate is always 'ready'.
export type AddRepoHostGate =
  | { status: 'ready' }
  | { status: 'no-host' }
  | { status: 'choose-host' }
  | {
      status: 'host-unavailable'
      hostId: ExecutionHostId
      label: string
      reason: AddRepoHostUnavailableReason
    }

export const READY_ADD_REPO_HOST_GATE: AddRepoHostGate = { status: 'ready' }

/** One connected host is chosen for the user; with several, only a remembered one still connected. */
export function pickAutoSelectedWebHostId(
  connectedHostIds: readonly ExecutionHostId[],
  rememberedHostId: string | null
): ExecutionHostId | null {
  if (connectedHostIds.length === 1) {
    return connectedHostIds[0]
  }
  return connectedHostIds.find((id) => id === rememberedHostId) ?? null
}

export function deriveAddRepoHostGate(args: {
  chosenHostId: ExecutionHostId | null
  connectedHostIds: readonly ExecutionHostId[]
  devServers: readonly Pick<DevServerSummary, 'id' | 'name' | 'status'>[]
}): AddRepoHostGate {
  const { chosenHostId, connectedHostIds, devServers } = args
  if (chosenHostId) {
    if (connectedHostIds.includes(chosenHostId)) {
      return READY_ADD_REPO_HOST_GATE
    }
    // Why keep the choice: dropping it would close the Browse step silently; the user must see why.
    const devServerId = parseExecutionHostId(chosenHostId)
    const known =
      devServerId?.kind === 'devServer'
        ? devServers.find((server) => server.id === devServerId.devServerId)
        : undefined
    return {
      status: 'host-unavailable',
      hostId: chosenHostId,
      label:
        known?.name || (devServerId?.kind === 'devServer' ? devServerId.devServerId : chosenHostId),
      reason: !known
        ? 'removed'
        : known.status === 'connecting'
          ? 'connecting'
          : known.status === 'error'
            ? 'error'
            : 'disconnected'
    }
  }
  return connectedHostIds.length === 0 ? { status: 'no-host' } : { status: 'choose-host' }
}

export function isAddRepoHostGateBlocking(gate: AddRepoHostGate): boolean {
  return gate.status !== 'ready'
}

const LAST_HOST_STORAGE_KEY = 'orca.addProject.lastDevServerId'

// localStorage is only a convenience (it can be blocked or empty), never the source of truth.
export function readRememberedWebHostId(): string | null {
  try {
    return window.localStorage.getItem(LAST_HOST_STORAGE_KEY)
  } catch {
    return null
  }
}

export function rememberWebHostId(hostId: ExecutionHostId): void {
  try {
    window.localStorage.setItem(LAST_HOST_STORAGE_KEY, hostId)
  } catch {
    // ignore: remembering the host is optional
  }
}
