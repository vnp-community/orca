/**
 * Project RPC Methods (TDD-15)
 *
 * Registers project domain methods using the factory pattern.
 * Services injected via createProjectMethods() at bootstrap time.
 *
 * Access control:
 * - project.list, project.get   → any authenticated user (access enforced per-project)
 * - project.create              → any authenticated user
 * - project.update, delete      → owner or admin
 * - project.addMember, removeMember, updateMemberRole → owner or admin
 * - project.getMembers          → any project member
 * - project.agentSpawn          → any project member
 *
 * @module main/project/project-rpc-handler
 */

import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { ProjectService } from './ProjectService'
import type { ProfileAwareAgentSpawner } from './ProfileAwareAgentSpawner'
import { Tracers } from '../../shared/trace/tracers'
// FIX BUG-BE-HLD-002: cần org-level role để global admin override được project owner-only actions.
import type { OrcaUser } from '../../shared/rbac-types'

/** Tra org-level role của userId — cùng type với profile-rpc-handler.ts (xem SOLUTION-rbac-exact.md). */
export type UserRoleLookup = (userId: string) => Promise<OrcaUser['role'] | null>

// ── Param schemas ─────────────────────────────────────────────────────────────

const ProjectIdParam = z.object({
  projectId: z.string().min(1),
})

const CreateProjectParam = z.object({
  name: z.string().min(1).max(200),
  description: z.string().optional(),
  devServerId: z.string().min(1),
  repoPath: z.string().min(1),
  defaultBranch: z.string().optional(),
  visibility: z.enum(['private', 'team', 'company']).optional(),
})

const UpdateProjectParam = z.object({
  projectId: z.string().min(1),
  patch: z.object({
    name: z.string().min(1).optional(),
    description: z.string().optional(),
    defaultBranch: z.string().optional(),
    visibility: z.enum(['private', 'team', 'company']).optional(),
  }),
})

const MemberParam = z.object({
  projectId: z.string().min(1),
  userId: z.string().min(1),
})

const AddMemberParam = z.object({
  projectId: z.string().min(1),
  userId: z.string().min(1),
  role: z.enum(['owner', 'member', 'viewer']),
})

const UpdateMemberRoleParam = z.object({
  projectId: z.string().min(1),
  userId: z.string().min(1),
  role: z.enum(['owner', 'member', 'viewer']),
})

const AgentSpawnParam = z.object({
  projectId: z.string().min(1),
  command: z.string().min(1),
  extraEnv: z.record(z.string(), z.string()).optional(),
  workdir: z.string().optional(),
  traceId: z.string().optional(), // [NEW CR-TRACE-002]
})

// ── Factory ──────────────────────────────────────────────────────────────────

/**
 * Create project RPC methods with injected services.
 * Spread into ALL_RPC_METHODS at bootstrap when project phase completes.
 */
export function createProjectMethods(
  projectService: ProjectService,
  getUserRole: UserRoleLookup,
  agentSpawner?: ProfileAwareAgentSpawner
): RpcMethod[] {
  return [
    // ── project.list ─────────────────────────────────────────────────────────
    // Lists all projects the calling user is a member of.
    defineMethod({
      name: 'project.list',
      params: null,
      handler: async (_params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        return projectService.list(userId)
      }
    }),

    // ── project.get ──────────────────────────────────────────────────────────
    // Get a single project (verifies membership).
    defineMethod({
      name: 'project.get',
      params: ProjectIdParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        await projectService.assertAccess(params.projectId, userId)
        return projectService.get(params.projectId)
      }
    }),

    // ── project.create ───────────────────────────────────────────────────────
    // Create a new project (creator automatically becomes owner).
    defineMethod({
      name: 'project.create',
      params: CreateProjectParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        return projectService.create({ ...params, createdBy: userId })
      }
    }),

    // ── project.update ───────────────────────────────────────────────────────
    // Update project metadata (owner or admin only).
    defineMethod({
      name: 'project.update',
      params: UpdateProjectParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        const member = await projectService.assertAccess(params.projectId, userId)
        await requireOwnerOrAdmin(member.role, userId, getUserRole)
        await projectService.update(params.projectId, params.patch, userId)
        return { success: true }
      }
    }),

    // ── project.delete ───────────────────────────────────────────────────────
    // Delete a project (owner or admin only).
    defineMethod({
      name: 'project.delete',
      params: ProjectIdParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        const member = await projectService.assertAccess(params.projectId, userId)
        await requireOwnerOrAdmin(member.role, userId, getUserRole)
        await projectService.delete(params.projectId, userId)
        return { success: true }
      }
    }),

    // ── project.addMember ────────────────────────────────────────────────────
    // Add or update a member (owner or admin only).
    defineMethod({
      name: 'project.addMember',
      params: AddMemberParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        const member = await projectService.assertAccess(params.projectId, userId)
        await requireOwnerOrAdmin(member.role, userId, getUserRole)
        await projectService.addMember(params.projectId, params.userId, params.role)
        return { success: true }
      }
    }),

    // ── project.removeMember ─────────────────────────────────────────────────
    // Remove a member (owner or admin only).
    defineMethod({
      name: 'project.removeMember',
      params: MemberParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        const member = await projectService.assertAccess(params.projectId, userId)
        await requireOwnerOrAdmin(member.role, userId, getUserRole)
        await projectService.removeMember(params.projectId, params.userId)
        return { success: true }
      }
    }),

    // ── project.updateMemberRole ──────────────────────────────────────────────
    // Update a member's role (owner or admin only).
    defineMethod({
      name: 'project.updateMemberRole',
      params: UpdateMemberRoleParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        const member = await projectService.assertAccess(params.projectId, userId)
        await requireOwnerOrAdmin(member.role, userId, getUserRole)
        await projectService.updateMemberRole(params.projectId, params.userId, params.role)
        return { success: true }
      }
    }),

    // ── project.getMembers ───────────────────────────────────────────────────
    // Get all members of a project (any member can call).
    defineMethod({
      name: 'project.getMembers',
      params: ProjectIdParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        await projectService.assertAccess(params.projectId, userId)
        return projectService.getMembers(params.projectId)
      }
    }),

    // ── project.agentSpawn ───────────────────────────────────────────────────
    // Spawn an agent in project context with profile-injected env (any member).
    // profile:agentSpawnRoute wraps only the profile/project-domain access-check
    // (assertAccess) BEFORE delegating to spawn() — it does NOT re-wrap spawn()
    // itself (that's agentOrch:spawn, owned by TASK-BE-002.2). Forwarding
    // routeSpan.id as traceId lets agentOrch:spawn RESUME the same span id,
    // producing one continuous profile:agentSpawnRoute → agentOrch:spawn →
    // relay:agentCall chain instead of two competing root spans.
    defineMethod({
      name: 'project.agentSpawn',
      params: AgentSpawnParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) {throw new Error('UNAUTHENTICATED')}
        if (!agentSpawner) {throw new Error('AGENT_SPAWNER_NOT_AVAILABLE')}

        const routeSpan = Tracers.profileAgentSpawnFlow.start(
          { projectId: params.projectId, userId },
          params.traceId ? { id: params.traceId } : undefined
        )
        try {
          await projectService.assertAccess(params.projectId, userId)
          routeSpan.ok({ projectId: params.projectId })
        } catch (err) {
          routeSpan.fail(err, { projectId: params.projectId })
          throw err
        }

        return agentSpawner.spawn({ ...params, userId, traceId: routeSpan.id })
      }
    }),
  ]
}

// ── helpers ───────────────────────────────────────────────────────────────────

type ProjectRole = 'owner' | 'member' | 'viewer'

// FIX BUG-BE-HLD-002: thêm nhánh global-admin override — trước đây tên hàm hứa
// "OrAdmin" nhưng userId chưa từng được dùng để check admin thật.
async function requireOwnerOrAdmin(
  role: ProjectRole,
  userId: string,
  getUserRole: UserRoleLookup
): Promise<void> {
  if (role === 'owner') {return}
  const globalRole = await getUserRole(userId)
  if (globalRole === 'admin') {return}
  throw new Error('FORBIDDEN: only project owners or global admins can perform this action')
}
