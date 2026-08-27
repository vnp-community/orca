// Why: mirrors runtime-workspace-cleanup-client.ts's hybrid-routing shape for
// the `ephemeralVm:*` preload methods that manage per-workspace VM/container
// recipes and runtimes. Local calls stay on window.api.ephemeralVm (real IPC,
// unchanged behavior); paired/web callers route through the `ephemeralVm.*`
// runtime RPC instead. ephemeralVm.provision/cancelProvision/onProvisionEvent
// stay window.api-only — see EPHEMERAL_VM_METHODS's "Why" comment in
// desktop/src/main/runtime/rpc/methods/ephemeral-vm.ts for why those three
// don't have a clean request/response RPC shape.
import type { GlobalSettings } from '../../../shared/types'
import { callRuntimeRpc, getActiveRuntimeTarget } from './runtime-rpc-client'

type EphemeralVmSettings = Pick<GlobalSettings, 'activeRuntimeEnvironmentId'>

export function listRuntimeEphemeralVmRecipes(
  settings: EphemeralVmSettings | null | undefined,
  args: { repoId: string }
): ReturnType<typeof window.api.ephemeralVm.listRecipes> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.listRecipes(args)
  }
  return callRuntimeRpc(target, 'ephemeralVm.listRecipes', args)
}

export function listRuntimeEphemeralVmRecipeCatalog(
  settings: EphemeralVmSettings | null | undefined
): ReturnType<typeof window.api.ephemeralVm.listRecipeCatalog> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.listRecipeCatalog()
  }
  return callRuntimeRpc(target, 'ephemeralVm.listRecipeCatalog')
}

export function doctorRuntimeEphemeralVmRecipe(
  settings: EphemeralVmSettings | null | undefined,
  args: { repoId: string; recipeId: string }
): ReturnType<typeof window.api.ephemeralVm.doctor> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.doctor(args)
  }
  return callRuntimeRpc(target, 'ephemeralVm.doctor', args)
}

export function listRuntimeEphemeralVmRuntimes(
  settings: EphemeralVmSettings | null | undefined
): ReturnType<typeof window.api.ephemeralVm.listRuntimes> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.listRuntimes()
  }
  return callRuntimeRpc(target, 'ephemeralVm.listRuntimes')
}

export function attachRuntimeEphemeralVmWorkspace(
  settings: EphemeralVmSettings | null | undefined,
  args: { runtimeId: string; workspaceId: string }
): ReturnType<typeof window.api.ephemeralVm.attachWorkspace> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.attachWorkspace(args)
  }
  return callRuntimeRpc(target, 'ephemeralVm.attachWorkspace', args)
}

export function suspendRuntimeEphemeralVmWorkspace(
  settings: EphemeralVmSettings | null | undefined,
  args: { workspaceId: string }
): ReturnType<typeof window.api.ephemeralVm.suspendWorkspace> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.suspendWorkspace(args)
  }
  return callRuntimeRpc(target, 'ephemeralVm.suspendWorkspace', args)
}

export function resumeRuntimeEphemeralVmWorkspace(
  settings: EphemeralVmSettings | null | undefined,
  args: { workspaceId: string }
): ReturnType<typeof window.api.ephemeralVm.resumeWorkspace> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.resumeWorkspace(args)
  }
  return callRuntimeRpc(target, 'ephemeralVm.resumeWorkspace', args)
}

export function cleanupRuntimeEphemeralVmWorkspace(
  settings: EphemeralVmSettings | null | undefined,
  args: { runtimeId: string }
): ReturnType<typeof window.api.ephemeralVm.cleanup> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.cleanup(args)
  }
  return callRuntimeRpc(target, 'ephemeralVm.cleanup', args)
}

export function getRuntimeEphemeralVmCleanupCommand(
  settings: EphemeralVmSettings | null | undefined,
  args: { runtimeId: string }
): ReturnType<typeof window.api.ephemeralVm.getCleanupCommand> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    return window.api.ephemeralVm.getCleanupCommand(args)
  }
  return callRuntimeRpc(target, 'ephemeralVm.getCleanupCommand', args)
}
