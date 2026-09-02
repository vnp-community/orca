import {
  activateAndRevealFolderWorkspace,
  activateAndRevealWorktree
} from '@/lib/worktree-activation'
import { parseWorkspaceKey } from '../../../shared/workspace-scope'
import { toast } from 'sonner'
import { translate } from '@/i18n/i18n'
import { useAppStore } from '@/store'
import { resumeRuntimeEphemeralVmWorkspace } from '@/runtime/runtime-ephemeral-vm-client'

export async function activateWorktreeFromSidebar(worktreeId: string): Promise<void> {
  const workspaceScope = parseWorkspaceKey(worktreeId)
  if (workspaceScope?.type === 'folder') {
    activateAndRevealFolderWorkspace(workspaceScope.folderWorkspaceId)
    return
  }

  // Why settings.experimentalEphemeralVms, not just window.api?.ephemeralVm:
  // the web/paired preload's fallback Proxy makes window.api.ephemeralVm
  // truthy even when this experimental feature (default false) was never
  // enabled — every sidebar click otherwise fired a real ephemeralVm.
  // resumeWorkspace RPC that had nothing to resume, surfacing as a
  // "not yet implemented" console error for users who never opted in.
  const settings = useAppStore.getState().settings
  if (
    typeof window !== 'undefined' &&
    window.api?.ephemeralVm &&
    settings?.experimentalEphemeralVms === true
  ) {
    try {
      await resumeRuntimeEphemeralVmWorkspace(settings, {
        workspaceId: worktreeId
      })
    } catch (error) {
      toast.error(
        translate(
          'auto.lib.sidebarWorktreeActivation.wakeEphemeralVmFailed',
          'Failed to wake ephemeral VM workspace'
        ),
        {
          description: error instanceof Error ? error.message : String(error)
        }
      )
      return
    }
  }

  // Why: sidebar clicks already happen on a visible row; revealing again can
  // jump duplicate pinned/canonical entries back to the first mounted copy.
  activateAndRevealWorktree(worktreeId, { revealInSidebar: false })
}
