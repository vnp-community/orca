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
})

// ── Factory ──────────────────────────────────────────────────────────────────

/**
 * Create project RPC methods with injected services.
 * Spread into ALL_RPC_METHODS at bootstrap when project phase completes.
 */
export function createProjectMethods(
  projectService: ProjectService,
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
        if (!userId) throw new Error('UNAUTHENTICATED')
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
        if (!userId) throw new Error('UNAUTHENTICATED')
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
        if (!userId) throw new Error('UNAUTHENTICATED')
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
        if (!userId) throw new Error('UNAUTHENTICATED')
        const member = await projectService.assertAccess(params.projectId, userId)
        requireOwnerOrAdmin(member.role, userId)
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
        if (!userId) throw new Error('UNAUTHENTICATED')
        const member = await projectService.assertAccess(params.projectId, userId)
        requireOwnerOrAdmin(member.role, userId)
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
        if (!userId) throw new Error('UNAUTHENTICATED')
        const member = await projectService.assertAccess(params.projectId, userId)
        requireOwnerOrAdmin(member.role, userId)
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
        if (!userId) throw new Error('UNAUTHENTICATED')
        const member = await projectService.assertAccess(params.projectId, userId)
        requireOwnerOrAdmin(member.role, userId)
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
        if (!userId) throw new Error('UNAUTHENTICATED')
        const member = await projectService.assertAccess(params.projectId, userId)
        requireOwnerOrAdmin(member.role, userId)
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
        if (!userId) throw new Error('UNAUTHENTICATED')
        await projectService.assertAccess(params.projectId, userId)
        return projectService.getMembers(params.projectId)
      }
    }),

    // ── project.agentSpawn ───────────────────────────────────────────────────
    // Spawn an agent in project context with profile-injected env (any member).
    defineMethod({
      name: 'project.agentSpawn',
      params: AgentSpawnParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        if (!userId) throw new Error('UNAUTHENTICATED')
        if (!agentSpawner) throw new Error('AGENT_SPAWNER_NOT_AVAILABLE')
        await projectService.assertAccess(params.projectId, userId)
        return agentSpawner.spawn({ ...params, userId })
      }
    }),
  ]
}

// ── helpers ───────────────────────────────────────────────────────────────────

type ProjectRole = 'owner' | 'member' | 'viewer'

function requireOwnerOrAdmin(role: ProjectRole, _userId: string): void {
  if (role !== 'owner') {
    throw new Error('FORBIDDEN: only project owners can perform this action')
  }
}
