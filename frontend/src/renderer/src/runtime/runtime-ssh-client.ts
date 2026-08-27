import type { GlobalSettings } from '../../../shared/types'
import type {
  EnrichedDetectedPort,
  PortForwardEntry,
  SshConnectionState,
  SshTarget
} from '../../../shared/ssh-types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

// Why: SSH connections are owned by the paired host. Fleet management (target
// CRUD, config import/export) stays desktop-only (unavailable in the web client
// per web-preload-api.ts's createSshApi()); the read/connect/disconnect/
// passphrase-probe/port-forward surface below has a real runtime RPC backing,
// mirroring createSshApi()'s routing.

export async function listRuntimeSshTargets(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined
): Promise<SshTarget[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.listTargets()
  }
  const { targets } = await callRuntimeRpc<{ targets: SshTarget[] }>(
    target,
    'ssh.listTargets',
    undefined,
    { timeoutMs: 15_000 }
  )
  return targets
}

export async function listRuntimeSshRemovedTargetLabels(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined
): Promise<Record<string, string>> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.listRemovedTargetLabels()
  }
  const { labels } = await callRuntimeRpc<{ labels: Record<string, string> }>(
    target,
    'ssh.listRemovedTargetLabels',
    undefined,
    { timeoutMs: 15_000 }
  )
  return labels
}

export async function connectRuntimeSsh(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  targetId: string
): Promise<SshConnectionState | null> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.connect({ targetId })
  }
  const { state } = await callRuntimeRpc<{ state: SshConnectionState | null }>(
    target,
    'ssh.connect',
    { targetId },
    { timeoutMs: 30_000 }
  )
  return state
}

export async function getRuntimeSshState(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  targetId: string
): Promise<SshConnectionState | null> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.getState({ targetId })
  }
  const { state } = await callRuntimeRpc<{ state: SshConnectionState | null }>(
    target,
    'ssh.getState',
    { targetId },
    { timeoutMs: 15_000 }
  )
  return state
}

export async function disconnectRuntimeSsh(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  targetId: string
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    await window.api.ssh.disconnect({ targetId })
    return
  }
  await callRuntimeRpc<void>(target, 'ssh.disconnect', { targetId }, { timeoutMs: 15_000 })
}

// Why: callers use this to decide whether auto-firing ssh.connect would pop an
// unprompted credential dialog (Cmd+J jump, terminal reattach, automation
// dispatch) — same probe desktop's ssh:needsPassphrasePrompt IPC exposes.
export async function needsRuntimeSshPassphrasePrompt(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  targetId: string
): Promise<boolean> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.needsPassphrasePrompt({ targetId })
  }
  const { needsPrompt } = await callRuntimeRpc<{ needsPrompt: boolean }>(
    target,
    'ssh.needsPassphrasePrompt',
    { targetId },
    { timeoutMs: 15_000 }
  )
  return needsPrompt
}

export async function addRuntimeSshPortForward(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  args: { targetId: string; localPort: number; remoteHost: string; remotePort: number; label?: string }
): Promise<PortForwardEntry> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.addPortForward(args)
  }
  const { entry } = await callRuntimeRpc<{ entry: PortForwardEntry }>(
    target,
    'ssh.addPortForward',
    args,
    { timeoutMs: 15_000 }
  )
  return entry
}

export async function updateRuntimeSshPortForward(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  args: {
    id: string
    targetId: string
    localPort: number
    remoteHost: string
    remotePort: number
    label?: string
  }
): Promise<PortForwardEntry> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.updatePortForward(args)
  }
  const { entry } = await callRuntimeRpc<{ entry: PortForwardEntry }>(
    target,
    'ssh.updatePortForward',
    args,
    { timeoutMs: 15_000 }
  )
  return entry
}

export async function removeRuntimeSshPortForward(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  id: string
): Promise<PortForwardEntry | null> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.removePortForward({ id })
  }
  const { entry } = await callRuntimeRpc<{ entry: PortForwardEntry | null }>(
    target,
    'ssh.removePortForward',
    { id },
    { timeoutMs: 15_000 }
  )
  return entry
}

export async function listRuntimeSshPortForwards(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  targetId?: string
): Promise<PortForwardEntry[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.listPortForwards(targetId ? { targetId } : undefined)
  }
  const { forwards } = await callRuntimeRpc<{ forwards: PortForwardEntry[] }>(
    target,
    'ssh.listPortForwards',
    targetId ? { targetId } : undefined,
    { timeoutMs: 15_000 }
  )
  return forwards
}

export async function listRuntimeSshDetectedPorts(
  settings: Pick<GlobalSettings, 'activeRuntimeEnvironmentId'> | null | undefined,
  targetId: string
): Promise<EnrichedDetectedPort[]> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind !== 'environment') {
    return window.api.ssh.listDetectedPorts({ targetId })
  }
  const { ports } = await callRuntimeRpc<{ ports: EnrichedDetectedPort[] }>(
    target,
    'ssh.listDetectedPorts',
    { targetId },
    { timeoutMs: 15_000 }
  )
  return ports
}
