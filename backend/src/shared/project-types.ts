/**
 * Project Types — v5.0 (TDD-15)
 *
 * Shared types for project entities and project membership.
 * Used by ProjectService (main), ProjectServerRouter (main), and renderer components.
 *
 * @module shared/project-types
 */

import type { PersistedDevServer } from './dev-server-types'

/** Visibility scopes for projects */
export type ProjectVisibility = 'private' | 'team' | 'company'

/** Member roles within a project */
export type ProjectRole = 'owner' | 'member' | 'viewer'

/** A persisted project linked to a dev server */
export interface OrcaProject {
  id: string
  name: string
  description?: string
  /** The dev server where this project's repo lives */
  devServerId: string
  /** Absolute path on the dev server's filesystem */
  repoPath: string
  defaultBranch: string
  visibility: ProjectVisibility
  createdBy: string
  createdAt: Date
  updatedAt: Date
}

/** Project membership record */
export interface ProjectMember {
  projectId: string
  userId: string
  role: ProjectRole
  addedAt: Date
}

/** Full project context passed to agent spawners and relay operations */
export interface ProjectContext {
  project: OrcaProject
  /** The requesting user's membership record */
  member: ProjectMember
  /** The dev server where the project resides */
  devServer: PersistedDevServer
  /** Merged 3-layer profile for the requesting user */
  resolvedProfile: import('../main/profile/OrcaProfile').ResolvedProfile
}

/** Parameters required to create a new project */
export interface CreateProjectParams {
  name: string
  description?: string
  devServerId: string
  repoPath: string
  defaultBranch?: string
  visibility?: ProjectVisibility
  createdBy: string
}

/** Partial update payload for an existing project */
export interface UpdateProjectParams {
  name?: string
  description?: string
  defaultBranch?: string
  visibility?: ProjectVisibility
  /** Rebind project to a different Dev Server. Validated against
   *  DevServerManager before being persisted — see ProjectService.update(). */
  devServerId?: string
}
