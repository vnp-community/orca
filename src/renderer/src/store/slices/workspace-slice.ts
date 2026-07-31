// workspace-slice.ts — Project-level workspace state (TDD-FE-12, TDD-FE-17)
import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { OrcaProject } from '../../types/workspace-types'

// ─── Types ────────────────────────────────────────────────────────────────────

export type WorkspaceSlice = {
  /** All available projects for the current user */
  projects: OrcaProject[]
  /** Currently active project */
  activeProject: OrcaProject | null

  setProjects: (projects: OrcaProject[]) => void
  setActiveProject: (project: OrcaProject | null) => void
  addProject: (project: OrcaProject) => void
  removeProject: (projectId: string) => void
  updateProject: (projectId: string, patch: Partial<OrcaProject>) => void
}

// ─── Slice Factory ────────────────────────────────────────────────────────────

export const createWorkspaceSlice: StateCreator<AppState, [], [], WorkspaceSlice> = (set) => ({
  projects:      [],
  activeProject: null,

  setProjects: (projects) =>
    set(() => ({ projects })),

  setActiveProject: (project) =>
    set(() => ({ activeProject: project })),

  addProject: (project) =>
    set((state) => ({ projects: [...state.projects, project] })),

  removeProject: (projectId) =>
    set((state) => ({
      projects:      state.projects.filter(p => p.id !== projectId),
      activeProject: state.activeProject?.id === projectId ? null : state.activeProject,
    })),

  updateProject: (projectId, patch) =>
    set((state) => ({
      projects: state.projects.map(p =>
        p.id === projectId ? { ...p, ...patch } : p
      ),
      activeProject:
        state.activeProject?.id === projectId
          ? { ...state.activeProject, ...patch }
          : state.activeProject,
    })),
})
