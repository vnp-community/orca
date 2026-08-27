/**
 * Team RPC Methods (§5.2)
 *
 * Registers team domain methods using the same factory pattern as
 * profile-rpc-handler.ts / project-rpc-handler.ts. Services injected via
 * createTeamMethods() at bootstrap time.
 *
 * Access control:
 * - team.create, addMember, removeMember → admin only (org-level role, same
 *   requireAdmin() gate as profile-rpc-handler.ts — Team metadata/membership
 *   is org-administered, not self-service)
 * - team.list, listMembers               → any authenticated user (view only)
 *
 * @module main/team/team-rpc-handler
 */

import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { TeamService } from './TeamService'
import type { OrcaUser } from '../../shared/rbac-types'

/** Tra org-level role của userId — cùng type với profile-rpc-handler.ts. */
export type UserRoleLookup = (userId: string) => Promise<OrcaUser['role'] | null>

// ── Param schemas ─────────────────────────────────────────────────────────────

const CreateTeamParam = z.object({
  name: z.string().min(1).max(200),
})

const TeamIdParam = z.object({
  teamId: z.string().min(1),
})

const AddMemberParam = z.object({
  teamId: z.string().min(1),
  userId: z.string().min(1),
  role: z.string().min(1),
  priority: z.number().int().optional(),
})

const RemoveMemberParam = z.object({
  teamId: z.string().min(1),
  userId: z.string().min(1),
})

// ── Factory ──────────────────────────────────────────────────────────────────

/**
 * Create team RPC methods with injected services.
 * Spread into ALL_RPC_METHODS at bootstrap.
 *
 * @example
 * // In server-bootstrap.ts:
 * rpcServer.addMethods(createTeamMethods(teamService, getUserRole))
 */
export function createTeamMethods(
  teamService: TeamService,
  getUserRole: UserRoleLookup
): RpcMethod[] {
  return [
    // ── team.create ───────────────────────────────────────────────────────────
    // Create a new team (admin only).
    defineMethod({
      name: 'team.create',
      params: CreateTeamParam,
      handler: async (params, ctx) => {
        await requireAdmin(ctx, getUserRole)
        return teamService.createTeam({ name: params.name })
      }
    }),

    // ── team.addMember ────────────────────────────────────────────────────────
    // Add or update a team member (admin only).
    defineMethod({
      name: 'team.addMember',
      params: AddMemberParam,
      handler: async (params, ctx) => {
        await requireAdmin(ctx, getUserRole)
        await teamService.addMember({
          teamId: params.teamId,
          userId: params.userId,
          role: params.role,
          priority: params.priority ?? 0
        })
        return { success: true }
      }
    }),

    // ── team.removeMember ─────────────────────────────────────────────────────
    // Remove a team member (admin only).
    defineMethod({
      name: 'team.removeMember',
      params: RemoveMemberParam,
      handler: async (params, ctx) => {
        await requireAdmin(ctx, getUserRole)
        await teamService.removeMember({ teamId: params.teamId, userId: params.userId })
        return { success: true }
      }
    }),

    // ── team.list ─────────────────────────────────────────────────────────────
    // List all teams (any authenticated user may view).
    defineMethod({
      name: 'team.list',
      params: null,
      handler: async (_params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        return teamService.listTeams()
      }
    }),

    // ── team.listMembers ──────────────────────────────────────────────────────
    // List members of a team (any authenticated user may view).
    defineMethod({
      name: 'team.listMembers',
      params: TeamIdParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        return teamService.listMembers(params.teamId)
      }
    }),
  ]
}

// ── helpers ───────────────────────────────────────────────────────────────────

async function requireAdmin(
  ctx: { userId?: string },
  getUserRole: UserRoleLookup
): Promise<void> {
  if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
  const role = await getUserRole(ctx.userId)
  if (role !== 'admin') {throw new Error('FORBIDDEN: admin role required')}
}
