// src/relay/agent-rpc-dispatch-git-hooks.ts
// git.checkHooks / git.readIssueCommand / git.writeIssueCommand /
// git.scanSetupScriptImports — split out of agent-rpc-dispatch-git.ts (which
// crossed the repo's oxlint max-lines budget) to keep both files under it.
// See agent-git-hooks-issue-handler.ts for the handler bodies and the wire
// contract these method names must match verbatim.

import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError } from './agent-rpc-dispatch'

export async function dispatchGitHooksRpc(
  rpc: JsonRpcRequest,
  config: AgentConfig,
  log: AgentLogger
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    case 'git.checkHooks': {
      try {
        const { handleGitCheckHooks } = await import('./agent-git-hooks-issue-handler')
        return (await handleGitCheckHooks(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.checkHooks unavailable: ${msg}`)
      }
    }

    case 'git.readIssueCommand': {
      try {
        const { handleGitReadIssueCommand } = await import('./agent-git-hooks-issue-handler')
        return (await handleGitReadIssueCommand(
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
          `git.readIssueCommand unavailable: ${msg}`
        )
      }
    }

    case 'git.writeIssueCommand': {
      try {
        const { handleGitWriteIssueCommand } = await import('./agent-git-hooks-issue-handler')
        return (await handleGitWriteIssueCommand(
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
          `git.writeIssueCommand unavailable: ${msg}`
        )
      }
    }

    case 'git.scanSetupScriptImports': {
      try {
        const { handleGitScanSetupScriptImports } = await import('./agent-git-hooks-issue-handler')
        return (await handleGitScanSetupScriptImports(
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
          `git.scanSetupScriptImports unavailable: ${msg}`
        )
      }
    }

    default:
      return null
  }
}
