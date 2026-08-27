// Renderer-side hybrid RPC client for the `folderWorkspace.*` namespace. Mirrors
// the preload/RPC boundary so repos.ts routes local vs. remote runtime targets
// through one auditable surface instead of branching inline per call site.
import type { FolderWorkspace } from '../../../shared/types'
import type {
  FolderWorkspacePathStatus,
  FolderWorkspacePathStatusRequest
} from '../../../shared/folder-workspace-path-status'
import { callRuntimeRpc, type RuntimeClientTarget } from './runtime-rpc-client'

export type FolderWorkspaceCreateArgs = {
  projectGroupId: string
  name?: string
  folderPath?: string | null
  connectionId?: string | null
  linkedTask?: FolderWorkspace['linkedTask']
  createdWithAgent?: FolderWorkspace['createdWithAgent']
  pendingFirstAgentMessageRename?: boolean
}

export type FolderWorkspaceUpdates = Partial<
  Pick<
    FolderWorkspace,
    | 'name'
    | 'folderPath'
    | 'linkedTask'
    | 'comment'
    | 'isArchived'
    | 'isUnread'
    | 'isPinned'
    | 'sortOrder'
    | 'manualOrder'
    | 'workspaceStatus'
    | 'createdWithAgent'
    | 'pendingFirstAgentMessageRename'
    | 'firstAgentMessageRenameError'
    | 'lastActivityAt'
  >
>

export async function listRuntimeFolderWorkspaces(
  target: RuntimeClientTarget
): Promise<FolderWorkspace[]> {
  if (target.kind === 'local') {
    return window.api.folderWorkspaces.list()
  }
  const { folderWorkspaces } = await callRuntimeRpc<{ folderWorkspaces: FolderWorkspace[] }>(
    target,
    'folderWorkspace.list',
    undefined,
    { timeoutMs: 15_000, reuseRecentCompatibilityFailure: true }
  )
  return folderWorkspaces
}

export async function getRuntimeFolderWorkspacePathStatus(
  target: RuntimeClientTarget,
  request: FolderWorkspacePathStatusRequest
): Promise<FolderWorkspacePathStatus> {
  if (target.kind === 'local') {
    return window.api.folderWorkspaces.getPathStatus(request)
  }
  const { status } = await callRuntimeRpc<{ status: FolderWorkspacePathStatus }>(
    target,
    'folderWorkspace.getPathStatus',
    request,
    { timeoutMs: 15_000 }
  )
  return status
}

export async function createRuntimeFolderWorkspace(
  target: RuntimeClientTarget,
  args: FolderWorkspaceCreateArgs
): Promise<FolderWorkspace> {
  if (target.kind === 'local') {
    return window.api.folderWorkspaces.create(args)
  }
  const { folderWorkspace } = await callRuntimeRpc<{ folderWorkspace: FolderWorkspace }>(
    target,
    'folderWorkspace.create',
    args,
    { timeoutMs: 15_000 }
  )
  return folderWorkspace
}

export async function updateRuntimeFolderWorkspace(
  target: RuntimeClientTarget,
  folderWorkspaceId: string,
  updates: FolderWorkspaceUpdates
): Promise<FolderWorkspace | null> {
  if (target.kind === 'local') {
    return window.api.folderWorkspaces.update({ folderWorkspaceId, updates })
  }
  const { folderWorkspace } = await callRuntimeRpc<{ folderWorkspace: FolderWorkspace | null }>(
    target,
    'folderWorkspace.update',
    { folderWorkspaceId, updates },
    { timeoutMs: 15_000 }
  )
  return folderWorkspace
}

export async function deleteRuntimeFolderWorkspace(
  target: RuntimeClientTarget,
  folderWorkspaceId: string
): Promise<boolean> {
  if (target.kind === 'local') {
    return window.api.folderWorkspaces.delete({ folderWorkspaceId })
  }
  const { deleted } = await callRuntimeRpc<{ deleted: boolean }>(
    target,
    'folderWorkspace.delete',
    { folderWorkspaceId },
    { timeoutMs: 15_000 }
  )
  return deleted
}
