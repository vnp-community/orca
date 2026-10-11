import type { ExecutionHostId, parseExecutionHostId } from '../../../../shared/execution-host'

type ParsedHost = ReturnType<typeof parseExecutionHostId>

export type AddRepoHostRouting = {
  runtimeEnvironmentId: string | null
  devServerId: string | null
  isRuntimeEnvironmentActive: boolean
  selectedHostLabel: string | null
  // Where Browse goes: SSH picker, host path entry (runtime or dev server) or the OS folder picker.
  browseTarget: 'ssh' | 'server-path' | 'local'
  browseHostKind: 'local' | 'ssh' | 'runtime'
  manualCreateParentEntry: boolean
}

/** What the chosen Add Project host means for browsing and typed paths (SSH, runtime, dev server). */
export function deriveAddRepoHostRouting(selection: {
  selectedHostId: ExecutionHostId | null
  selectedParsedHost: ParsedHost
  hostOptions: readonly { id: string; label: string }[]
}): AddRepoHostRouting {
  const parsed = selection.selectedParsedHost
  const runtimeEnvironmentId = parsed?.kind === 'runtime' ? parsed.environmentId : null
  const devServerId = parsed?.kind === 'devServer' ? parsed.devServerId : null
  const isSsh = parsed?.kind === 'ssh'
  const isServerPath = parsed?.kind === 'runtime' || !!devServerId
  return {
    runtimeEnvironmentId,
    devServerId,
    isRuntimeEnvironmentActive: Boolean(runtimeEnvironmentId),
    selectedHostLabel:
      selection.hostOptions.find((host) => host.id === selection.selectedHostId)?.label ??
      selection.selectedHostId,
    browseTarget: isSsh ? 'ssh' : isServerPath ? 'server-path' : 'local',
    browseHostKind: isSsh ? 'ssh' : isServerPath ? 'runtime' : 'local',
    manualCreateParentEntry: Boolean(runtimeEnvironmentId) || isSsh || !!devServerId
  }
}
