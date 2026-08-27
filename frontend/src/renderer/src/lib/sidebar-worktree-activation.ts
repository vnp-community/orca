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

  if (typeof window !== 'undefined' && window.api?.ephemeralVm) {
    try {
      await resumeRuntimeEphemeralVmWorkspace(useAppStore.getState().settings, {
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
