// workspace-types.ts — Shared types for Workspace + File Explorer (TDD-FE-12, 17)

// Why no repoPath: matches project.proto's Project message exactly — a repo
// path is a property of a Repo (project.repos, a separate resource FK'd to
// a project), never of the project itself. CreateProjectDialog collects a
// repo path only to make a follow-up repo.add call after project.create.
export type OrcaProject = {
  id: string
  name: string
  description?: string
  defaultBranch: string
  devServerId: string
  visibility: 'private' | 'team' | 'department' | 'company'
  createdAt: number
  updatedAt: number
}

// Why role is only 'owner'|'member': matches project.project_members' DB
// CHECK constraint and project.rego's action_roles exactly — this is the
// project-level *access* tier (who's in the project at all), not the
// per-repo functional-role tier (admin/developer/lead) that's a separate,
// repo-scoped concept. Why no email/name: project.proto's Member message
// carries only user_id + role — no profile fields to display yet.
export type ProjectMember = {
  userId: string
  role: 'owner' | 'member'
}

// ─── OrcaProject source-project sharing (orca_project_source_projects) ────────
// BUG-FE-PW-002: links a caller-owned per-user Project (frontend/src/shared/
// types.ts's Project — the legacy, multi-host, per-user-JSON model) into an
// OrcaProject so other OrcaProject members can view it. Shapes below mirror
// backend/src/main/project/OrcaProjectSourceProjectService.ts's SourceProjectRef
// and orca-project-sharing-rpc-handler.ts's RPC params/results exactly.

export type SourceProjectRef = {
  ownerUserId: string
  projectId: string
}

export type LinkSourceProjectParam = {
  orcaProjectId: string
  projectId: string
}

export type UnlinkSourceProjectParam = {
  orcaProjectId: string
  projectId: string
}

export type OrcaProjectListItemWithSources = {
  orcaProject: OrcaProject
  sourceProjects: SourceProjectRef[]
}

export type FileNode = {
  name: string
  path: string // relative to project root
  type: 'file' | 'directory'
  size?: number // bytes (files only)
  children?: FileNode[] // lazy loaded
  isLoading?: boolean
}

export type GitStatus = {
  branch: string
  aheadBy: number
  behindBy: number
  hasUncommitted: boolean
  staged: number
  unstaged: number
}

export type Worktree = {
  id: string
  path: string // absolute path on dev server
  branch: string
  isMain: boolean
  createdAt?: number
}
