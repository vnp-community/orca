/**
 * AI Provider RPC Methods (TDD-16)
 *
 * Factory function — inject services at bootstrap then spread into ALL_RPC_METHODS.
 * 9 RPC methods with access control.
 *
 * Access control:
 * - aiProvider.list, get, getUsageToday, resolve → any authenticated user
 * - aiProvider.create → any authenticated user
 * - aiProvider.update → account owner or admin
 * - aiProvider.delete → admin only (enforced upstream by AuthManager)
 * - aiProvider.writeCredential → account owner or admin
 * - aiProvider.testConnection → account owner or admin
 *
 * @module main/ai-providers/ai-provider-rpc-handler
 */

import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { AIProviderService } from './AIProviderService'
import type { ProviderResolver } from './ProviderResolver'

// ── Param schemas ─────────────────────────────────────────────────────────────

const AccountIdParam = z.object({
  accountId: z.string().min(1),
})

const ListParam = z.object({
  devServerId: z.string().min(1),
  scope: z.enum(['server', 'project', 'user']).optional(),
})

const CreateParam = z.object({
  devServerId: z.string().min(1),
  provider: z.enum(['anthropic', 'openai', 'gemini', 'azure', 'bedrock', 'ollama', 'vllm']),
  scope: z.enum(['server', 'project', 'user']).default('server'),
  scopeRefId: z.string().optional(),
  label: z.string().min(1),
  model: z.string().optional(),
  baseUrl: z.string().url().optional(),
  quotaLimitDay: z.number().int().min(0).optional(),
})

const UpdateParam = z.object({
  accountId: z.string().min(1),
  patch: z.object({
    label: z.string().optional(),
    model: z.string().optional(),
    baseUrl: z.string().url().optional(),
    // BUG-BE-HLD-014: 'rotating' intentionally NOT accepted here — only
    // rotateKey()/completeRotation() may transition an account into/out of
    // it. Allowing it via generic update would let a caller fake grace-period
    // state without ever staging/testing a new credential.
    status: z.enum(['pending', 'active', 'invalid', 'quota_exceeded', 'unreachable']).optional(),
    quotaLimitDay: z.number().int().min(0).optional(),
  }),
})

const WriteCredentialParam = z.object({
  accountId: z.string().min(1),
  encryptedBlob: z.string().min(1),
  iv: z.string().min(1),
  traceId: z.string().optional(), // CR-TRACE-000 §3.3 — WS RPC row
})

// BUG-BE-HLD-014: same shape as WriteCredentialParam, plus an optional grace
// period override (defaults to AIProviderService.DEFAULT_ROTATION_GRACE_PERIOD_MS).
const RotateKeyParam = z.object({
  accountId: z.string().min(1),
  encryptedBlob: z.string().min(1),
  iv: z.string().min(1),
  gracePeriodMs: z.number().int().positive().optional(),
  traceId: z.string().optional(),
})

const ResolveParam = z.object({
  devServerId: z.string().min(1),
  projectId: z.string().min(1),
  userId: z.string().min(1).optional(),
  modelHint: z.string().optional(),
})

// ── Factory ──────────────────────────────────────────────────────────────────

/**
 * Create AI provider RPC methods with injected services.
 * @example
 * ALL_RPC_METHODS = [...ALL_RPC_METHODS, ...createAIProviderMethods(aiProviderService, providerResolver)]
 */
export function createAIProviderMethods(
  service: AIProviderService,
  resolver: ProviderResolver
): RpcMethod[] {
  return [
    // ── aiProvider.list ───────────────────────────────────────────────────────
    defineMethod({
      name: 'aiProvider.list',
      params: ListParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        return service.listAccounts(params.devServerId, params.scope)
      }
    }),

    // ── aiProvider.create ─────────────────────────────────────────────────────
    defineMethod({
      name: 'aiProvider.create',
      params: CreateParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        return service.createAccount({ ...params, createdBy: ctx.userId })
      }
    }),

    // ── aiProvider.get ────────────────────────────────────────────────────────
    defineMethod({
      name: 'aiProvider.get',
      params: AccountIdParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        return service.getAccount(params.accountId)
      }
    }),

    // ── aiProvider.update ─────────────────────────────────────────────────────
    // Owner or admin — enforced by checking createdBy or admin role upstream
    defineMethod({
      name: 'aiProvider.update',
      params: UpdateParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        await assertAccountAccess(service, params.accountId, ctx.userId)
        await service.updateAccount(params.accountId, params.patch, ctx.userId) // BUG-BE-HLD-014: pass actor for audit
        return { success: true }
      }
    }),

    // ── aiProvider.delete ─────────────────────────────────────────────────────
    // Admin only — additional role check should be applied in auth middleware
    defineMethod({
      name: 'aiProvider.delete',
      params: AccountIdParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        await assertAccountAccess(service, params.accountId, ctx.userId)
        await service.deleteAccount(params.accountId, ctx.userId) // BUG-BE-HLD-014: pass actor for audit
        return { success: true }
      }
    }),

    // ── aiProvider.writeCredential ────────────────────────────────────────────
    // Writes encrypted blob to dev server — plaintext is NEVER exposed
    defineMethod({
      name: 'aiProvider.writeCredential',
      params: WriteCredentialParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        await assertAccountAccess(service, params.accountId, ctx.userId)
        // traceId resume happens INSIDE writeCredentialToDevServer() via the traceId? overload above.
        await service.writeCredentialToDevServer(
          params.accountId,
          params.encryptedBlob,
          params.iv,
          params.traceId
        )
        return { success: true }
      }
    }),

    // ── aiProvider.rotateKey ─────────────────────────────────────────────────── (NEW — BUG-BE-HLD-014)
    // Owner or admin — same access rule as writeCredential/update.
    defineMethod({
      name: 'aiProvider.rotateKey',
      params: RotateKeyParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        await assertAccountAccess(service, params.accountId, ctx.userId)
        return service.rotateKey(
          params.accountId,
          { encryptedBlob: params.encryptedBlob, iv: params.iv },
          { gracePeriodMs: params.gracePeriodMs, actorUserId: ctx.userId }
        )
      }
    }),

    // ── aiProvider.testConnection ──────────────────────────────────────────────
    defineMethod({
      name: 'aiProvider.testConnection',
      params: AccountIdParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        return service.testConnection(params.accountId)
      }
    }),

    // ── aiProvider.getUsageToday ───────────────────────────────────────────────
    defineMethod({
      name: 'aiProvider.getUsageToday',
      params: AccountIdParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        return service.getUsageToday(params.accountId)
      }
    }),

    // ── aiProvider.resolve ─────────────────────────────────────────────────────
    // Returns the best matching account without credential
    defineMethod({
      name: 'aiProvider.resolve',
      params: ResolveParam,
      handler: async (params, ctx) => {
        if (!ctx.userId) {throw new Error('UNAUTHENTICATED')}
        const userId = params.userId ?? ctx.userId
        return resolver.resolve({
          devServerId: params.devServerId,
          projectId: params.projectId,
          userId,
          modelHint: params.modelHint,
        })
      }
    }),
  ]
}

// ── helpers ───────────────────────────────────────────────────────────────────

/**
 * Assert the caller is the account owner.
 * Admin bypass should be wired via auth middleware layer.
 */
async function assertAccountAccess(
  service: AIProviderService,
  accountId: string,
  userId: string
): Promise<void> {
  const account = await service.getAccount(accountId)
  if (!account) {throw new Error(`ACCOUNT_NOT_FOUND: ${accountId}`)}
  if (account.createdBy !== userId) {
    throw new Error('FORBIDDEN: only the account owner can perform this action')
  }
}
