/**
 * OrcaProject Sharing RPC Methods
 *
 * Registers the cross-user Project sharing methods on top of `OrcaProjectSourceProjectService`
 * + `ProjectService`. Reuses `ProjectService.assertAccess()` for all membership checks — this
 * file does NOT implement a parallel permission model.
 *
 * ⚠️ SECURITY-CRITICAL — read docs/guides/terminal-workspace-project-devserver-architecture.md
 * "Điểm cần thiết kế cẩn thận nhất" before touching `orcaProjects.getProjectData`. The golden
 * rule: NEVER return an owner's full per-user orca-data.json to another user — filter to
 * exactly the one shared Project (+ its Repos/worktree metadata).
 *
 * Access control (docs/guides/user-profile-team-department-rbac.md §5.3):
 * - orcaProjects.linkSourceProject   → any OrcaProject member (contributing your own
 *   Project into a shared OrcaProject is not owner-gated — only removing/altering is)
 * - orcaProjects.unlinkSourceProject → owner (or global admin) only
 * - orcaProjects.getProjectData      → any OrcaProject member (viewer included)
 * - orcaProjects.list                → any authenticated user (scoped to their own membership)
 *
 * @module main/project/orca-project-sharing-rpc-handler
 */

import { z } from 'zod'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { OrcaProjectSourceProjectService, SourceProjectRef } from './OrcaProjectSourceProjectService'
import type { ProjectService } from './ProjectService'
import { requireOwnerOrAdmin } from './project-rpc-handler'
import { getCanonicalUserDataPath } from '../persistence-paths'
import { getRepoIdFromWorktreeId } from '../../shared/worktree-id'
import type { PersistedState, Project, Repo, WorktreeMeta } from '../../shared/types'
import type { OrcaUser } from '../../shared/rbac-types'
import { Tracers } from '../../shared/trace/tracers'

/** Tra org-level role của userId — cùng type với project-rpc-handler.ts/team-rpc-handler.ts. */
export type UserRoleLookup = (userId: string) => Promise<OrcaUser['role'] | null>

// ── Param schemas ─────────────────────────────────────────────────────────────

const LinkSourceProjectParam = z.object({
  orcaProjectId: z.string().min(1),
  projectId: z.string().min(1),
})

const UnlinkSourceProjectParam = LinkSourceProjectParam
const GetProjectDataParam = LinkSourceProjectParam

// ── Cross-user read result shape ────────────────────────────────────────────

/**
 * Filtered slice of an owner's orca-data.json — exactly 1 Project plus the
 * Repos/worktree metadata that belong to it. NEVER widen this to the full
 * PersistedState; that would leak every other project the owner has.
 */
export type OrcaProjectSharedProjectData = {
  project: Project
  repos: Repo[]
  worktreeMeta: Record<string, WorktreeMeta>
}

// Same message for "orcaProjectId doesn't exist" and "you're not a member" —
// deliberately indistinguishable so a probing caller can't learn which OrcaProject
// ids exist (see architecture doc's "Điểm cần thiết kế cẩn thận nhất").
const ACCESS_DENIED_MESSAGE = 'FORBIDDEN: no access to this OrcaProject'

/**
 * Resolve the absolute path to ownerUserId's orca-data.json.
 *
 * Why derived, not threaded through as a param: in ORCA_MULTI_USER mode each
 * RPC handler runs inside a per-user child process that only knows its OWN
 * userData dir — ORCA_USER_DATA_PATH = `<baseDataPath>/users/<selfUserId>`
 * (session-manager.ts spawnUserProcess). getCanonicalUserDataPath() (existing
 * helper, persistence-paths.ts) returns that same directory for the CURRENT
 * user, so walking up one level reaches the shared `users/` root without
 * needing a new baseDataPath env var. Matches the exact `<root>/users/<userId>/...`
 * layout WebCredentialStore already uses for per-user credential storage.
 */
export function resolveOwnerDataFile(ownerUserId: string): string {
  const usersRootDir = dirname(getCanonicalUserDataPath())
  return join(usersRootDir, ownerUserId, 'orca-data.json')
}

/** Read + JSON.parse an owner's orca-data.json. Throws OWNER_DATA_NOT_FOUND if absent. */
export function readOwnerPersistedState(dataFilePath: string): PersistedState {
  if (!existsSync(dataFilePath)) {
    throw new Error('OWNER_DATA_NOT_FOUND')
  }
  const raw = readFileSync(dataFilePath, 'utf-8')
  return JSON.parse(raw) as PersistedState
}

/**
 * Filter exactly 1 Project (+ its Repos/worktree metadata) out of an owner's
 * full PersistedState. This is the ONLY function allowed to touch the raw
 * PersistedState in the getProjectData path — never pass the raw state itself
 * back over RPC.
 */
export function filterOwnerProjectData(
  state: PersistedState,
  projectId: string
): OrcaProjectSharedProjectData {
  const project = state.projects.find((p) => p.id === projectId)
  if (!project) {
    throw new Error('PROJECT_DATA_NOT_FOUND')
  }
  const repoIds = new Set(project.sourceRepoIds)
  const repos = state.repos.filter((r) => repoIds.has(r.id))
  const worktreeMeta: Record<string, WorktreeMeta> = {}
  for (const [worktreeId, meta] of Object.entries(state.worktreeMeta ?? {})) {
    if (repoIds.has(getRepoIdFromWorktreeId(worktreeId))) {
      worktreeMeta[worktreeId] = meta
    }
  }
  return { project, repos, worktreeMeta }
}

// ── Factory ──────────────────────────────────────────────────────────────────

/** Injectable filesystem access for getProjectData — production default reads
 *  the real per-user orca-data.json; tests substitute an in-memory fixture. */
export type OrcaProjectSharingFileDeps = {
  resolveOwnerDataFile?: (ownerUserId: string) => string
  readOwnerPersistedState?: (path: string) => PersistedState
}

/**
 * Create OrcaProject sharing RPC methods with injected services.
 * Spread into ALL_RPC_METHODS at bootstrap (wiring done by the Wave 3 integration agent).
 */
export function createOrcaProjectSharingMethods(
  sourceProjectService: OrcaProjectSourceProjectService,
  projectService: ProjectService,
  getUserRole: UserRoleLookup,
  fileDeps: OrcaProjectSharingFileDeps = {}
): RpcMethod[] {
  const resolveDataFile = fileDeps.resolveOwnerDataFile ?? resolveOwnerDataFile
  const readPersistedState = fileDeps.readOwnerPersistedState ?? readOwnerPersistedState

  return [
    // ── orcaProjects.linkSourceProject ─────────────────────────────────────────
    // Link one of the caller's OWN per-user JSON Projects into an OrcaProject
    // they belong to. ownerUserId always comes from ctx.userId — never from
    // client input, so A cannot link a project on B's behalf.
    defineMethod({
      name: 'orcaProjects.linkSourceProject',
      params: LinkSourceProjectParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        // Any OrcaProject member (any role) may link one of their own Projects in.
        await projectService.assertAccess(params.orcaProjectId, userId)
        await sourceProjectService.linkProject(
          { orcaProjectId: params.orcaProjectId, ownerUserId: userId, projectId: params.projectId },
          userId
        )
        return { success: true }
      }
    }),

    // ── orcaProjects.unlinkSourceProject ───────────────────────────────────────
    // Owner (or global admin) only — §5.3 "Thêm/xoá Project khỏi OrcaProject".
    defineMethod({
      name: 'orcaProjects.unlinkSourceProject',
      params: UnlinkSourceProjectParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        const member = await projectService.assertAccess(params.orcaProjectId, userId)
        await requireOwnerOrAdmin(member.role, userId, getUserRole)

        const sourceProjects = await sourceProjectService.listSourceProjects(params.orcaProjectId)
        const source = sourceProjects.find((s) => s.projectId === params.projectId)
        if (!source) {
          // Nothing linked under this projectId — idempotent no-op, not an error.
          return { success: true }
        }
        await sourceProjectService.unlinkProject({
          orcaProjectId: params.orcaProjectId,
          ownerUserId: source.ownerUserId,
          projectId: params.projectId
        })
        return { success: true }
      }
    }),

    // ── orcaProjects.getProjectData ────────────────────────────────────────────
    // THE cross-user read. See module docblock — never return the owner's full
    // orca-data.json, only the filtered slice for this one Project.
    defineMethod({
      name: 'orcaProjects.getProjectData',
      params: GetProjectDataParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}

        const span = Tracers.orcaProjectSharingFlow.start({
          op: 'getProjectData',
          orcaProjectId: params.orcaProjectId,
          actingUserId: userId,
          projectId: params.projectId
        })
        try {
          // (a) Caller must be a member (any role, including viewer) of orcaProjectId.
          // Same generic error for "no such OrcaProject" and "not a member" — do not
          // leak which OrcaProject ids exist.
          await assertOrcaProjectMemberForRead(projectService, params.orcaProjectId, userId, span)

          // (b) projectId must actually be linked to THIS orcaProjectId — blocks a
          // member from passing an arbitrary projectId that belongs to a different
          // OrcaProject (or no OrcaProject at all).
          const sourceProjects = await sourceProjectService.listSourceProjects(params.orcaProjectId)
          const source = findSourceProject(sourceProjects, params.projectId)
          if (!source) {
            span.step('sourceProjectNotLinked', { orcaProjectId: params.orcaProjectId })
            throw new Error(ACCESS_DENIED_MESSAGE)
          }

          // (c)/(d) Read the owner's per-user JSON and filter to exactly this Project.
          const dataFile = resolveDataFile(source.ownerUserId)
          const state = readPersistedState(dataFile)
          const data = filterOwnerProjectData(state, params.projectId)

          // (e) Audit trail — orcaProjectId, actingUserId, ownerUserId, projectId.
          span.ok({
            orcaProjectId: params.orcaProjectId,
            actingUserId: userId,
            ownerUserId: source.ownerUserId,
            projectId: params.projectId
          })
          return data
        } catch (err) {
          span.fail(err, {
            orcaProjectId: params.orcaProjectId,
            actingUserId: userId,
            projectId: params.projectId
          })
          throw err
        }
      }
    }),

    // ── orcaProjects.list ───────────────────────────────────────────────────────
    // OrcaProjects the caller is a member of, each with its linked source Projects.
    defineMethod({
      name: 'orcaProjects.list',
      params: null,
      handler: async (_params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        const orcaProjects = await projectService.list(userId)
        return Promise.all(
          orcaProjects.map(async (orcaProject) => ({
            orcaProject,
            sourceProjects: await sourceProjectService.listSourceProjects(orcaProject.id)
          }))
        )
      }
    }),
  ]
}

// ── helpers ───────────────────────────────────────────────────────────────────

function findSourceProject(
  sourceProjects: SourceProjectRef[],
  projectId: string
): SourceProjectRef | undefined {
  return sourceProjects.find((s) => s.projectId === projectId)
}

/**
 * getProjectData-only: assert membership and normalize ProjectService's error
 * into the same FORBIDDEN message used for "projectId not linked" — a probing
 * caller can't tell "no such OrcaProject" apart from "exists but not a member".
 * link/unlink intentionally do NOT use this — they let assertAccess's own
 * PROJECT_ACCESS_DENIED propagate, matching project-rpc-handler.ts convention.
 */
async function assertOrcaProjectMemberForRead(
  projectService: ProjectService,
  orcaProjectId: string,
  userId: string,
  span: { step: (label: string, fields?: Record<string, string | number | boolean | undefined>) => void }
): Promise<void> {
  try {
    await projectService.assertAccess(orcaProjectId, userId)
  } catch {
    span.step('membershipDenied', { orcaProjectId })
    throw new Error(ACCESS_DENIED_MESSAGE)
  }
}
