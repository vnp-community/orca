// src/relay/agent-rpc-dispatch-ai.ts
// ai.provider.*/ai.complete RPC methods — split out of agent-rpc-dispatch.ts's
// giant switch to keep each file under the oxlint max-lines budget.

import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError } from './agent-rpc-dispatch'

export async function dispatchAiRpc(
  rpc: JsonRpcRequest,
  config: AgentConfig,
  log: AgentLogger
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    // ── v5.0: ai.provider.writeCredential ────────────────────────────────────
    case 'ai.provider.writeCredential': {
      try {
        const { handleWriteCredential } = await import('./agent-credential-store')
        return (await handleWriteCredential(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `ai.provider.writeCredential unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: ai.provider.readCredential ─────────────────────────────────────
    case 'ai.provider.readCredential': {
      try {
        const { handleReadCredential } = await import('./agent-credential-store')
        return (await handleReadCredential(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `ai.provider.readCredential unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: ai.provider.healthCheck ────────────────────────────────────────
    case 'ai.provider.healthCheck': {
      try {
        const { handleHealthCheck } = await import('./agent-credential-store')
        return (await handleHealthCheck(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `ai.provider.healthCheck unavailable: ${msg}`
        )
      }
    }

    // ── ai.provider.testConnection ───────────────────────────────────────────
    // Called by AIProviderService.ts. Previously unimplemented
    // (specs/agent/api/gaps-and-findings.md #1).
    case 'ai.provider.testConnection': {
      try {
        const { handleTestConnection } = await import('./agent-credential-store')
        return (await handleTestConnection(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `ai.provider.testConnection unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: ai.provider.deleteCredential ───────────────────────────────────
    case 'ai.provider.deleteCredential': {
      try {
        const { handleDeleteCredential } = await import('./agent-credential-store')
        return (await handleDeleteCredential(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `ai.provider.deleteCredential unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: ai.complete ─────────────────────────────────────────────────────
    // TG-002: Non-interactive AI completion for task planning (TaskAIPlanner.decompose)
    // and git commit message generation.
    // Called by: relay.call('ai.complete', { prompt, format: 'json'|'text', model? })
    case 'ai.complete': {
      try {
        const p = rpc.params ?? {}
        const prompt = typeof p['prompt'] === 'string' ? p['prompt'] : ''
        if (!prompt.trim()) {
          return makeError(rpc.id, AgentErrorCode.InvalidParams, 'ai.complete: prompt is required')
        }
        const { handleAIComplete } = await import('./ai-complete-handler')
        const result = await handleAIComplete(
          {
            prompt,
            format: typeof p['format'] === 'string' ? (p['format'] as 'json' | 'text') : 'text',
            taskId: typeof p['taskId'] === 'string' ? p['taskId'] : undefined,
            model: typeof p['model'] === 'string' ? p['model'] : undefined,
            accountId: typeof p['accountId'] === 'string' ? p['accountId'] : undefined,
            resolvedApiKey:
              typeof p['resolvedApiKey'] === 'string' ? p['resolvedApiKey'] : undefined
          },
          config,
          log
        )
        return { jsonrpc: '2.0', id: rpc.id, result }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `ai.complete failed: ${msg}`)
      }
    }

    default:
      return null
  }
}
