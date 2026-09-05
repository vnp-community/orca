// src/relay/agent-rpc-dispatch-scm.ts
// github.*/gitlab.* RPC methods — split out of agent-rpc-dispatch.ts's giant
// switch to keep each file under the oxlint max-lines budget.

import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError } from './agent-rpc-dispatch'

export async function dispatchScmRpc(
  rpc: JsonRpcRequest,
  config: AgentConfig,
  log: AgentLogger
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    // ── v5.0: github.pr.create ───────────────────────────────────────────────
    case 'github.pr.create': {
      try {
        const { handleGitHubPrCreate } = await import('./external-api-connector')
        return (await handleGitHubPrCreate(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.pr.merge ────────────────────────────────────────────────
    case 'github.pr.merge': {
      try {
        const { handleGitHubPrMerge } = await import('./external-api-connector')
        return (await handleGitHubPrMerge(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.pr.merge unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.issue.list ──────────────────────────────────────────────
    case 'github.issue.list': {
      try {
        const { handleGitHubIssueList } = await import('./external-api-connector')
        return (await handleGitHubIssueList(
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
          `github.issue.list unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: github.issue.create ────────────────────────────────────────────
    case 'github.issue.create': {
      try {
        const { handleGitHubIssueCreate } = await import('./external-api-connector')
        return (await handleGitHubIssueCreate(
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
          `github.issue.create unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: gitlab.mr.create ───────────────────────────────────────────────
    case 'gitlab.mr.create': {
      try {
        const { handleGitLabMrCreate } = await import('./external-api-connector')
        return (await handleGitLabMrCreate(
          rpc.id,
          rpc.params ?? {},
          config,
          log
        )) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.create unavailable: ${msg}`)
      }
    }

    // ── v5.0: gitlab.pipeline.status ─────────────────────────────────────────
    case 'gitlab.pipeline.status': {
      try {
        const { handleGitLabPipelineStatus } = await import('./external-api-connector')
        return (await handleGitLabPipelineStatus(
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
          `gitlab.pipeline.status unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: gitlab.mr.list ─────────────────────────────────────────────────
    case 'gitlab.mr.list': {
      try {
        const { handleGitLabMrList } = await import('./external-api-connector')
        return (await handleGitLabMrList(rpc.id, rpc.params ?? {}, config, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.mr.list unavailable: ${msg}`)
      }
    }

    // ── v5.0: github.auth.status ─────────────────────────────────────────────
    case 'github.auth.status': {
      try {
        const { handleGitHubAuthStatus } = await import('./external-api-connector')
        return (await handleGitHubAuthStatus(
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
          `github.auth.status unavailable: ${msg}`
        )
      }
    }

    // ── v5.0: gitlab.auth.status ─────────────────────────────────────────────
    case 'gitlab.auth.status': {
      try {
        const { handleGitLabAuthStatus } = await import('./external-api-connector')
        return (await handleGitLabAuthStatus(
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
          `gitlab.auth.status unavailable: ${msg}`
        )
      }
    }

    // ── github.exec / gitlab.exec ────────────────────────────────────────────
    // Generic, allowlist-validated gh/glab CLI passthrough — backs the
    // ADR-018 migration: backend/src/main/git/runner.ts's ghExecFileAsync/
    // glabExecFileAsync now route here via a connection-scoped provider
    // instead of spawning gh/glab in the backend process. See
    // specs/agent/api/gaps-and-findings.md.
    case 'github.exec': {
      try {
        const { handleGithubExec } = await import('./agent-github-cli-handler')
        return (await handleGithubExec(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `github.exec unavailable: ${msg}`)
      }
    }

    case 'gitlab.exec': {
      try {
        const { handleGitlabExec } = await import('./agent-gitlab-cli-handler')
        return (await handleGitlabExec(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `gitlab.exec unavailable: ${msg}`)
      }
    }

    default:
      return null
  }
}
