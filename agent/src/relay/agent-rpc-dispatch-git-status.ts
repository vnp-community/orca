// src/relay/agent-rpc-dispatch-git-status.ts
// TASK-227: git.status/diff/commit/push/pull/... — split out of
// agent-rpc-dispatch.ts's giant switch to keep each file under the oxlint
// max-lines budget. These 20 methods were fully implemented on Part B
// (git-handler.ts + its ops modules) but unreachable here, so relay-based
// git calls from backend-go's RelayExecutor failed with `Method not found`
// against a real Dev Server WS agent. See
// specs/backend-go/bugs/missing-v1/tasks/TASK-227-expose-git-status-diff-commit-on-agent-part-a.md.

import type WebSocket from 'ws'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError } from './agent-rpc-dispatch'

export async function dispatchGitStatusRpc(
  rpc: JsonRpcRequest,
  ws: WebSocket
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    case 'git.status': {
      try {
        const { handleGitStatus } = await import('./agent-git-handler-local-ops')
        return (await handleGitStatus(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.status unavailable: ${msg}`)
      }
    }

    case 'git.diff': {
      try {
        const { handleGitDiff } = await import('./agent-git-handler-local-ops')
        return (await handleGitDiff(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.diff unavailable: ${msg}`)
      }
    }

    case 'git.commit': {
      try {
        const { handleGitCommit } = await import('./agent-git-handler-local-ops')
        return (await handleGitCommit(rpc.id, rpc.params ?? {}, ws)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.commit unavailable: ${msg}`)
      }
    }

    case 'git.push': {
      try {
        const { handleGitPush } = await import('./agent-git-handler-remote-ops')
        return (await handleGitPush(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.push unavailable: ${msg}`)
      }
    }

    case 'git.pull': {
      try {
        const { handleGitPull } = await import('./agent-git-handler-remote-ops')
        return (await handleGitPull(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.pull unavailable: ${msg}`)
      }
    }

    case 'git.checkout': {
      try {
        const { handleGitCheckout } = await import('./agent-git-handler-local-ops')
        return (await handleGitCheckout(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.checkout unavailable: ${msg}`)
      }
    }

    case 'git.localBranches': {
      try {
        const { handleGitLocalBranches } = await import('./agent-git-handler-local-ops')
        return (await handleGitLocalBranches(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `git.localBranches unavailable: ${msg}`
        )
      }
    }

    case 'git.fastForward': {
      try {
        const { handleGitFastForward } = await import('./agent-git-handler-remote-ops')
        return (await handleGitFastForward(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.fastForward unavailable: ${msg}`)
      }
    }

    case 'git.rebaseFromBase': {
      try {
        const { handleGitRebaseFromBase } = await import('./agent-git-handler-remote-ops')
        return (await handleGitRebaseFromBase(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `git.rebaseFromBase unavailable: ${msg}`
        )
      }
    }

    case 'git.abortRebase': {
      try {
        const { handleGitAbortRebase } = await import('./agent-git-handler-local-ops')
        return (await handleGitAbortRebase(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.abortRebase unavailable: ${msg}`)
      }
    }

    case 'git.abortMerge': {
      try {
        const { handleGitAbortMerge } = await import('./agent-git-handler-local-ops')
        return (await handleGitAbortMerge(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.abortMerge unavailable: ${msg}`)
      }
    }

    case 'git.conflictOperation': {
      try {
        const { handleGitConflictOperation } = await import('./agent-git-handler-local-ops')
        return (await handleGitConflictOperation(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `git.conflictOperation unavailable: ${msg}`
        )
      }
    }

    case 'git.discard': {
      try {
        const { handleGitDiscard } = await import('./agent-git-handler-local-ops')
        return (await handleGitDiscard(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.discard unavailable: ${msg}`)
      }
    }

    case 'git.bulkDiscard': {
      try {
        const { handleGitBulkDiscard } = await import('./agent-git-handler-local-ops')
        return (await handleGitBulkDiscard(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.bulkDiscard unavailable: ${msg}`)
      }
    }

    case 'git.stage': {
      try {
        const { handleGitStage } = await import('./agent-git-handler-local-ops')
        return (await handleGitStage(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.stage unavailable: ${msg}`)
      }
    }

    case 'git.unstage': {
      try {
        const { handleGitUnstage } = await import('./agent-git-handler-local-ops')
        return (await handleGitUnstage(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.unstage unavailable: ${msg}`)
      }
    }

    case 'git.bulkStage': {
      try {
        const { handleGitBulkStage } = await import('./agent-git-handler-local-ops')
        return (await handleGitBulkStage(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.bulkStage unavailable: ${msg}`)
      }
    }

    case 'git.bulkUnstage': {
      try {
        const { handleGitBulkUnstage } = await import('./agent-git-handler-local-ops')
        return (await handleGitBulkUnstage(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.bulkUnstage unavailable: ${msg}`)
      }
    }

    case 'git.fetch': {
      try {
        const { handleGitFetch } = await import('./agent-git-handler-remote-ops')
        return (await handleGitFetch(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.fetch unavailable: ${msg}`)
      }
    }

    case 'git.upstreamStatus': {
      try {
        const { handleGitUpstreamStatus } = await import('./agent-git-handler-remote-ops')
        return (await handleGitUpstreamStatus(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `git.upstreamStatus unavailable: ${msg}`
        )
      }
    }

    default:
      return null
  }
}
