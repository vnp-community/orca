/**
 * Tests for AI Provider RPC handler tracing (TASK-BE-016.1/016.4).
 *
 * Covers `src/main/ai-providers/ai-provider-rpc-handler.ts` — specifically that
 * `aiProvider.writeCredential`'s optional `params.traceId` is forwarded as the
 * 4th argument to `service.writeCredentialToDevServer()` so the backend span
 * (`aiProvider:writeCredential`) resumes the caller's trace instead of minting
 * a fresh one.
 *
 * Security focus: `params.encryptedBlob`/`params.iv` must never be echoed into
 * any assertion target other than the exact call args passed straight through
 * to the service (the RPC layer never inspects/logs them).
 *
 * @module main/ai-providers/__tests__/ai-provider-rpc.test
 */

import { describe, it, expect, vi } from 'vitest'
import { createAIProviderMethods } from '../ai-provider-rpc-handler'
import type { AIProviderService } from '../AIProviderService'
import type { ProviderResolver } from '../ProviderResolver'
import type { RpcContext, RpcMethod } from '../../runtime/rpc/core'
import type { AIProviderAccount } from '../../../shared/ai-provider-types'

function makeAccount(overrides: Partial<AIProviderAccount> = {}): AIProviderAccount {
  return {
    id: 'acc-1',
    devServerId: 'srv-1',
    provider: 'anthropic',
    scope: 'server',
    label: 'Test',
    status: 'pending',
    quotaLimitDay: 0,
    createdBy: 'user-1',
    createdAt: new Date(),
    updatedAt: new Date(),
    ...overrides,
  }
}

function makeCtx(userId = 'user-1'): RpcContext {
  return { userId } as RpcContext
}

function findMethod(methods: RpcMethod[], name: string): RpcMethod {
  const m = methods.find((m) => m.name === name)
  if (!m) {throw new Error(`Method ${name} not found`)}
  return m
}

describe('aiProvider.writeCredential RPC handler', () => {
  it('forwards params.traceId as the 4th argument to writeCredentialToDevServer()', async () => {
    const account = makeAccount()
    const writeCredentialToDevServer = vi.fn().mockResolvedValue(undefined)
    const service = {
      getAccount: vi.fn().mockResolvedValue(account),
      writeCredentialToDevServer,
    } as unknown as AIProviderService
    const resolver = {} as ProviderResolver
    const methods = createAIProviderMethods(service, resolver)

    const result = await findMethod(methods, 'aiProvider.writeCredential').handler(
      { accountId: account.id, encryptedBlob: 'blob-value', iv: 'iv-value', traceId: 'parent-trace-42' },
      makeCtx(account.createdBy)
    )

    expect(result).toEqual({ success: true })
    expect(writeCredentialToDevServer).toHaveBeenCalledWith(
      account.id,
      'blob-value',
      'iv-value',
      'parent-trace-42'
    )
  })

  it('omitted params.traceId → forwards undefined as the 4th argument (fresh span, no forced resume)', async () => {
    const account = makeAccount()
    const writeCredentialToDevServer = vi.fn().mockResolvedValue(undefined)
    const service = {
      getAccount: vi.fn().mockResolvedValue(account),
      writeCredentialToDevServer,
    } as unknown as AIProviderService
    const resolver = {} as ProviderResolver
    const methods = createAIProviderMethods(service, resolver)

    await findMethod(methods, 'aiProvider.writeCredential').handler(
      { accountId: account.id, encryptedBlob: 'blob-value', iv: 'iv-value' },
      makeCtx(account.createdBy)
    )

    expect(writeCredentialToDevServer).toHaveBeenCalledWith(
      account.id,
      'blob-value',
      'iv-value',
      undefined
    )
  })

  it('unauthenticated caller → throws UNAUTHENTICATED before touching the service', async () => {
    const service = {
      getAccount: vi.fn(),
      writeCredentialToDevServer: vi.fn(),
    } as unknown as AIProviderService
    const resolver = {} as ProviderResolver
    const methods = createAIProviderMethods(service, resolver)

    await expect(
      findMethod(methods, 'aiProvider.writeCredential').handler(
        { accountId: 'acc-1', encryptedBlob: 'blob', iv: 'iv' },
        { userId: undefined } as RpcContext
      )
    ).rejects.toThrow('UNAUTHENTICATED')
    expect(service.writeCredentialToDevServer).not.toHaveBeenCalled()
  })

  it('caller is not the account owner → throws FORBIDDEN, never calls writeCredentialToDevServer', async () => {
    const account = makeAccount({ createdBy: 'owner-user' })
    const service = {
      getAccount: vi.fn().mockResolvedValue(account),
      writeCredentialToDevServer: vi.fn(),
    } as unknown as AIProviderService
    const resolver = {} as ProviderResolver
    const methods = createAIProviderMethods(service, resolver)

    await expect(
      findMethod(methods, 'aiProvider.writeCredential').handler(
        { accountId: account.id, encryptedBlob: 'blob', iv: 'iv' },
        makeCtx('someone-else')
      )
    ).rejects.toThrow('FORBIDDEN')
    expect(service.writeCredentialToDevServer).not.toHaveBeenCalled()
  })
})
