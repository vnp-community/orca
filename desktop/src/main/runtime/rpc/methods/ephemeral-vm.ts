import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import type { OrcaRuntimeService } from '../../orca-runtime'
import { requiredString } from '../schemas'
import {
  listRecipeCatalog,
  listRecipes,
  type EphemeralVmRecipeStore
} from '../../../ipc/ephemeral-vm-recipe-context'
import { doctorEphemeralVmRecipeForRepo } from '../../../ipc/ephemeral-vm'
import {
  attachEphemeralVmWorkspace,
  cleanupEphemeralVmWorkspace,
  getEphemeralVmCleanupCommand,
  listEphemeralVmRuntimeRecords,
  resumeEphemeralVmWorkspace,
  suspendEphemeralVmWorkspace
} from '../../../ipc/ephemeral-vm-runtime-handlers'

// Why: ephemeralVm.provision / cancelProvision are intentionally NOT covered
// here. provision streams stdout/stderr chunks to the renderer via a
// broadcast 'ephemeralVm:provisionEvent' IPC event decoupled from the
// request/response call (not a subscribe/unsubscribe stream keyed to one rpc
// call), and cancelProvision aborts an in-flight provision by a provisionId
// shared across that broadcast. Forcing this into defineStreamingMethod would
// require redesigning the cancellation/broadcast contract; left on
// window.api per the task's "flag genuinely streaming work" guidance.

const RepoSelector = z.object({
  repoId: requiredString('Missing repoId')
})

const DoctorArgs = z.object({
  repoId: requiredString('Missing repoId'),
  recipeId: requiredString('Missing recipeId')
})

const RuntimeSelector = z.object({
  runtimeId: requiredString('Missing runtimeId')
})

const WorkspaceSelector = z.object({
  workspaceId: requiredString('Missing workspaceId')
})

const AttachWorkspaceArgs = z.object({
  runtimeId: requiredString('Missing runtimeId'),
  workspaceId: requiredString('Missing workspaceId')
})

// Why: ephemeral VM recipes are local-desktop-only (see getRecipeRepo's
// 'runs on the local desktop host in v1' guard), so a null store here means
// the RPC server was constructed without one — a desktop bootstrap bug, not
// a reachable end-user state. Fail loudly rather than silently no-op.
function requireStore(runtime: OrcaRuntimeService): EphemeralVmRecipeStore {
  const store = runtime.getStore()
  if (!store) {
    throw new Error('Ephemeral VM RPC methods require a runtime store.')
  }
  return store
}

export const EPHEMERAL_VM_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'ephemeralVm.listRecipes',
    params: RepoSelector,
    handler: (params, { runtime }) => listRecipes(requireStore(runtime), params.repoId)
  }),
  defineMethod({
    name: 'ephemeralVm.listRecipeCatalog',
    params: null,
    handler: (_params, { runtime }) => listRecipeCatalog(requireStore(runtime))
  }),
  defineMethod({
    name: 'ephemeralVm.doctor',
    params: DoctorArgs,
    handler: (params, { runtime }) => doctorEphemeralVmRecipeForRepo(requireStore(runtime), params)
  }),
  defineMethod({
    name: 'ephemeralVm.listRuntimes',
    params: null,
    handler: () => listEphemeralVmRuntimeRecords()
  }),
  defineMethod({
    name: 'ephemeralVm.attachWorkspace',
    params: AttachWorkspaceArgs,
    handler: (params) => attachEphemeralVmWorkspace(params)
  }),
  defineMethod({
    name: 'ephemeralVm.suspendWorkspace',
    params: WorkspaceSelector,
    handler: (params, { runtime }) => suspendEphemeralVmWorkspace(requireStore(runtime), params)
  }),
  defineMethod({
    name: 'ephemeralVm.resumeWorkspace',
    params: WorkspaceSelector,
    handler: (params, { runtime }) => resumeEphemeralVmWorkspace(requireStore(runtime), params)
  }),
  defineMethod({
    name: 'ephemeralVm.cleanup',
    params: RuntimeSelector,
    handler: (params, { runtime }) => cleanupEphemeralVmWorkspace(requireStore(runtime), params)
  }),
  defineMethod({
    name: 'ephemeralVm.getCleanupCommand',
    params: RuntimeSelector,
    handler: (params, { runtime }) => getEphemeralVmCleanupCommand(requireStore(runtime), params)
  })
]
