import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { WorkspaceTab } from '../../components/workspace/WorkspaceTabBar'

export type { WorkspaceTab }

export type WorkspaceTabSlice = {
  activeWorkspaceTab: WorkspaceTab
  setActiveWorkspaceTab(tab: WorkspaceTab): void
}

// Why a store field, not prop-drilling or lifting WorkspaceLayout's local
// useState: TaskDetail is rendered as a sibling panel under WorkspaceLayout's
// right sidebar, several component layers away with no shared parent that
// isn't WorkspaceLayout itself. activeTaskId (task.ts) already uses this same
// store-field pattern for the equivalent problem (sidebar -> right panel).
export const createWorkspaceTabSlice: StateCreator<AppState, [], [], WorkspaceTabSlice> = (
  set
) => ({
  activeWorkspaceTab: 'git',
  setActiveWorkspaceTab: (tab) => set(() => ({ activeWorkspaceTab: tab }))
})
