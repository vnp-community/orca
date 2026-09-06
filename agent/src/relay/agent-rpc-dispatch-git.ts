// src/relay/agent-rpc-dispatch-git.ts
// git.* RPC methods (excluding the TASK-227 status/diff/commit/push/pull/...
// block, which lives in agent-rpc-dispatch-git-status.ts) — split out of
// agent-rpc-dispatch.ts's giant switch to keep each file under the oxlint
// max-lines budget. See agent-rpc-dispatch.ts for the outer dispatch/tracing
// wrapper this is called from.

import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import type { WireState } from 'orca-dev-agent-transport'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError, makeNotifier } from './agent-rpc-dispatch'

export async function dispatchGitRpc(
  rpc: JsonRpcRequest,
  config: AgentConfig,
  log: AgentLogger,
  ws: WebSocket,
  state: WireState
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    // ── v5.0: git.exec ───────────────────────────────────────────────────────
    case 'git.exec': {
      try {
        const { handleGitExec } = await import('./agent-git-handler')
        return (await handleGitExec(rpc.id, rpc.params ?? {}, config, log, ws)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.exec unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.execStream ─────────────────────────────────────────────────
    case 'git.execStream': {
      try {
        const { handleGitExecStream } = await import('./agent-git-handler')
        // Streaming: fire-and-forget, sends multiple frames asynchronously
        void handleGitExecStream(ws, state, rpc.id, rpc.params ?? {}, config, log)
        return { jsonrpc: '2.0', id: rpc.id, result: { type: 'stream.started' } }
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.execStream unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.history ────────────────────────────────────────────────────
    case 'git.history': {
      try {
        const { handleGitHistory } = await import('./agent-git-handler-extended')
        return (await handleGitHistory(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.history unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.branchCompare ─────────────────────────────────────────────
    case 'git.branchCompare': {
      try {
        const { handleGitBranchCompare } = await import('./agent-git-handler-extended')
        return (await handleGitBranchCompare(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `git.branchCompare unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: git.commitCompare ─────────────────────────────────────────────
    case 'git.commitCompare': {
      try {
        const { handleGitCommitCompare } = await import('./agent-git-handler-extended')
        return (await handleGitCommitCompare(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `git.commitCompare unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: git.branchDiff ────────────────────────────────────────────────
    case 'git.branchDiff': {
      try {
        const { handleGitBranchDiff } = await import('./agent-git-handler-extended')
        return (await handleGitBranchDiff(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.branchDiff unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.commitDiff ────────────────────────────────────────────────
    case 'git.commitDiff': {
      try {
        const { handleGitCommitDiff } = await import('./agent-git-handler-extended')
        return (await handleGitCommitDiff(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.commitDiff unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.checkIgnored ──────────────────────────────────────────────
    case 'git.checkIgnored': {
      try {
        const { handleGitCheckIgnored } = await import('./agent-git-handler-extended')
        return (await handleGitCheckIgnored(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.checkIgnored unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.forkSync ──────────────────────────────────────────────────
    case 'git.forkSync': {
      try {
        const { handleGitForkSync } = await import('./agent-git-handler-extended')
        return (await handleGitForkSync(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.forkSync unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.submoduleStatus ───────────────────────────────────────────
    case 'git.submoduleStatus': {
      try {
        const { handleGitSubmoduleStatus } = await import('./agent-git-handler-extended')
        return (await handleGitSubmoduleStatus(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `git.submoduleStatus unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: git.pr.create ──────────────────────────────────────────────────
    case 'git.pr.create': {
      try {
        // BUG-AG-HLD-004: 'git.pr.create' and 'github.pr.create' are the same
        // action — route both to the one implementation with the idempotency
        // check (handleGitHubPrCreate) so a retry/double-click never creates
        // a duplicate PR regardless of which method name the caller used.
        const { handleGitHubPrCreate } = await import('./external-api-connector')
        return (await handleGitHubPrCreate(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.pr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.list ──────────────────────────────────────────────
    case 'git.worktree.list': {
      try {
        const { handleGitWorktreeList } = await import('./agent-git-worktree-handler')
        return (await handleGitWorktreeList(
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
          `git.worktree.list unavailable: ${msg}`
        )
      }
    }

    // ── git.baseRefDefault / git.searchRefs: backfilled gap, see
    // agent-git-handler.ts's doc comment on these two handlers ──────────────
    case 'git.baseRefDefault': {
      try {
        const { handleGitBaseRefDefault } = await import('./agent-git-base-ref-handler')
        return (await handleGitBaseRefDefault(
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
          `git.baseRefDefault unavailable: ${msg}`
        )
      }
    }

    case 'git.searchRefs': {
      try {
        const { handleGitSearchRefs } = await import('./agent-git-base-ref-handler')
        return (await handleGitSearchRefs(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.searchRefs unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.add ───────────────────────────────────────────────
    case 'git.worktree.add': {
      try {
        const { handleGitWorktreeAdd } = await import('./agent-git-worktree-handler')
        return (await handleGitWorktreeAdd(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.worktree.add unavailable: ${msg}`)
      }
    }

    // ── v5.0: git.worktree.remove ────────────────────────────────────────────
    case 'git.worktree.remove': {
      try {
        const { handleGitWorktreeRemove } = await import('./agent-git-worktree-handler')
        return (await handleGitWorktreeRemove(
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
          `git.worktree.remove unavailable: ${msg}`
        )
      }
    }

    // ── git.clone ─────────────────────────────────────────────────────────────
    // Called by backend/src/main/ipc/repo-remote-ipc.ts's repo.cloneRemote,
    // { url, targetPath }. Previously Part-B-only; see
    // specs/agent/api/gaps-and-findings.md #5.
    case 'git.clone': {
      try {
        const { handleGitClone } = await import('./agent-git-clone-handler')
        return (await handleGitClone(
          rpc.id,
          rpc.params ?? {},
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.clone unavailable: ${msg}`)
      }
    }

    case 'git.init': {
      try {
        const { handleGitInit } = await import('./agent-git-init-handler')
        return (await handleGitInit(
          rpc.id,
          rpc.params ?? {},
          makeNotifier(ws, state)
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `git.init unavailable: ${msg}`)
      }
    }

    default:
      return null
  }
}
