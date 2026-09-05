// src/relay/agent-rpc-dispatch-misc.ts
// tools/list, tools/call, preflight.*, host.capabilities, cli.*, shell.eval,
// shell.exec, notification.send, and accounts.* RPC methods — split out of
// agent-rpc-dispatch.ts's giant switch to keep each file under the oxlint
// max-lines budget.

import type WebSocket from 'ws'
import type { ToolDefinition, ToolResult } from './agent-tool-registry'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import type { JsonRpcRequest, JsonRpcResponse } from './agent-rpc-dispatch'
import { makeError, formatMcpResult } from './agent-rpc-dispatch'

export async function dispatchMiscRpc(
  rpc: JsonRpcRequest,
  tools: ToolDefinition[],
  config: AgentConfig,
  log: AgentLogger,
  ws: WebSocket
): Promise<JsonRpcResponse | null> {
  switch (rpc.method) {
    // ── MCP: tools/list ──────────────────────────────────────────────────────
    case 'tools/list':
      return {
        jsonrpc: '2.0',
        id: rpc.id,
        result: {
          tools: tools.map((t) => ({
            name: t.name,
            description: t.description,
            inputSchema: t.inputSchema
          }))
        }
      }

    // ── MCP: tools/call ──────────────────────────────────────────────────────
    case 'tools/call': {
      const params = rpc.params ?? {}
      const name = typeof params.name === 'string' ? params.name : ''
      const args =
        typeof params.arguments === 'object' && params.arguments !== null
          ? (params.arguments as Record<string, unknown>)
          : {}

      const tool = tools.find((t) => t.name === name)
      if (!tool) {
        return makeError(rpc.id, AgentErrorCode.MethodNotFound, `Tool not found: ${name}`)
      }

      log.info(`tools/call name=${name} args=${JSON.stringify(args).slice(0, 120)}`)

      let result: ToolResult
      try {
        result = await tool.handler(args, config)
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        log.error(`tools/call handler threw name=${name}: ${msg}`)
        return makeError(rpc.id, AgentErrorCode.ServerError, `Tool handler error: ${msg}`)
      }

      return formatMcpResult(rpc.id, result)
    }

    // ── v5.0: preflight.check ────────────────────────────────────────────────
    case 'preflight.check': {
      try {
        const { handlePreflightCheck } = await import('./fs-agent-extensions')
        return (await handlePreflightCheck(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `preflight.check unavailable: ${msg}`)
      }
    }

    // ── preflight.detectAgents ───────────────────────────────────────────────
    // Called by dev-server-relay-bridge.ts's detectAgents() (onboarding-ipc.ts).
    // Previously Part-B-only; see specs/agent/api/gaps-and-findings.md #5.
    case 'preflight.detectAgents': {
      try {
        const { handleDetectAgents } = await import('./agent-preflight-handler')
        return (await handleDetectAgents(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `preflight.detectAgents unavailable: ${msg}`
        )
      }
    }

    // ── preflight.detectWindowsTerminalCapabilities ──────────────────────────
    case 'preflight.detectWindowsTerminalCapabilities': {
      try {
        const { handleDetectWindowsTerminalCapabilities } =
          await import('./agent-preflight-handler')
        return (await handleDetectWindowsTerminalCapabilities(rpc.id)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `preflight.detectWindowsTerminalCapabilities unavailable: ${msg}`
        )
      }
    }

    // ── preflight.detectGhosttyConfig ────────────────────────────────────────
    case 'preflight.detectGhosttyConfig': {
      try {
        const { handleDetectGhosttyConfig } = await import('./agent-preflight-handler')
        return (await handleDetectGhosttyConfig(rpc.id)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `preflight.detectGhosttyConfig unavailable: ${msg}`
        )
      }
    }

    // ── preflight.setGitIdentity ─────────────────────────────────────────────
    // BUG-AG-HLD-003 parity for Part A — stores identity per-connection
    // (git-identity-registry.ts), consumed by git.exec's `commit` subcommand.
    case 'preflight.setGitIdentity': {
      try {
        const { handleSetGitIdentity } = await import('./agent-preflight-handler')
        return (await handleSetGitIdentity(rpc.id, rpc.params ?? {}, ws)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `preflight.setGitIdentity unavailable: ${msg}`
        )
      }
    }

    // ── host.capabilities ────────────────────────────────────────────────────
    // TASK-070: relayed by infra-fleet-service's GetHostCapabilities usecase.
    // See get_host_capabilities.go's doc comment for the full gap this closes.
    case 'host.capabilities': {
      try {
        const { handleHostCapabilities } = await import('./agent-preflight-handler')
        return (await handleHostCapabilities(rpc.id)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `host.capabilities unavailable: ${msg}`
        )
      }
    }

    // ─── cli.* (Orca ADR — server-mode CLI install on Dev Server) ───────────
    // Backend relays cli.* to the Dev Server Agent instead of running it on
    // the Orca backend container — see backend/src/main/runtime/rpc/methods/cli.ts
    // and agent-cli-handler.ts for the full rationale.
    case 'cli.getInstallStatus': {
      try {
        const { handleCliGetInstallStatus } = await import('./agent-cli-handler')
        return (await handleCliGetInstallStatus(rpc.id)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `cli.getInstallStatus unavailable: ${msg}`
        )
      }
    }

    case 'cli.install': {
      try {
        const { handleCliInstall } = await import('./agent-cli-handler')
        return (await handleCliInstall(rpc.id)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `cli.install unavailable: ${msg}`)
      }
    }

    case 'cli.remove': {
      try {
        const { handleCliRemove } = await import('./agent-cli-handler')
        return (await handleCliRemove(rpc.id)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `cli.remove unavailable: ${msg}`)
      }
    }

    case 'cli.getWslInstallStatus': {
      try {
        const { handleCliGetWslInstallStatus } = await import('./agent-cli-handler')
        return (await handleCliGetWslInstallStatus(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `cli.getWslInstallStatus unavailable: ${msg}`
        )
      }
    }

    case 'cli.installWsl': {
      try {
        const { handleCliInstallWsl } = await import('./agent-cli-handler')
        return (await handleCliInstallWsl(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `cli.installWsl unavailable: ${msg}`)
      }
    }

    case 'cli.removeWsl': {
      try {
        const { handleCliRemoveWsl } = await import('./agent-cli-handler')
        return (await handleCliRemoveWsl(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `cli.removeWsl unavailable: ${msg}`)
      }
    }
    // ─── end cli.* ───────────────────────────────────────────────────────────

    // Runs a short shell command and returns stdout/stderr.
    // Used by devServer.browseDir on the Orca server to resolve '~' on the remote.
    // SECURITY: only used internally via relay — not exposed to browser directly.
    case 'shell.eval': {
      try {
        const { handleShellEval } = await import('./fs-agent-extensions')
        return (await handleShellEval(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `shell.eval unavailable: ${msg}`)
      }
    }

    // ── shell.exec ───────────────────────────────────────────────────────────
    // Workflow 'shell' step executor. Called by:
    //   StepExecutors.executeShell() via relay.call('shell.exec', { script, env, traceId })
    // Previously unimplemented (specs/agent/api/gaps-and-findings.md #1).
    case 'shell.exec': {
      try {
        const { handleShellExec } = await import('./fs-agent-extensions')
        return (await handleShellExec(rpc.id, rpc.params ?? {}, config)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(rpc.id, AgentErrorCode.ServerError, `shell.exec unavailable: ${msg}`)
      }
    }

    // ── notification.send ────────────────────────────────────────────────────
    // Workflow 'notification' step executor. Called by:
    //   StepExecutors.executeNotification() via
    //   relay.call('notification.send', { channel, message, traceId })
    // Previously unimplemented (specs/agent/api/gaps-and-findings.md #1).
    case 'notification.send': {
      try {
        const { handleNotificationSend } = await import('./notification-send-handler')
        return (await handleNotificationSend(rpc.id, rpc.params ?? {}, log)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `notification.send unavailable: ${msg}`
        )
      }
    }

    // ── accounts.selectClaude / accounts.selectCodex / accounts.removeClaude /
    //    accounts.removeCodex ──────────────────────────────────────────────
    // TASK-023: backs infra-fleet-service's Relay RPC and api-gateway's
    // wscompat channels_accounts.go, which forward {accountId} params
    // straight through to these methods (see accounts-handler.ts's module
    // doc comment for the single-pseudo-account design this implements).
    case 'accounts.selectClaude': {
      try {
        const { handleAccountsSelectClaude } = await import('./accounts-handler')
        return (await handleAccountsSelectClaude(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `accounts.selectClaude unavailable: ${msg}`
        )
      }
    }

    case 'accounts.selectCodex': {
      try {
        const { handleAccountsSelectCodex } = await import('./accounts-handler')
        return (await handleAccountsSelectCodex(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `accounts.selectCodex unavailable: ${msg}`
        )
      }
    }

    case 'accounts.removeClaude': {
      try {
        const { handleAccountsRemoveClaude } = await import('./accounts-handler')
        return (await handleAccountsRemoveClaude(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `accounts.removeClaude unavailable: ${msg}`
        )
      }
    }

    case 'accounts.removeCodex': {
      try {
        const { handleAccountsRemoveCodex } = await import('./accounts-handler')
        return (await handleAccountsRemoveCodex(rpc.id, rpc.params ?? {})) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `accounts.removeCodex unavailable: ${msg}`
        )
      }
    }

    // ── accounts.getSnapshot ─────────────────────────────────────────────────
    // Backs api-gateway's accounts.subscribe poll loop (BUG-005/SOL-005's
    // session-client push bridge) — read-only, no accountId param. See
    // accounts-handler.ts's getAccountsSnapshot doc comment.
    case 'accounts.getSnapshot': {
      try {
        const { handleAccountsGetSnapshot } = await import('./accounts-handler')
        return (await handleAccountsGetSnapshot(rpc.id)) as JsonRpcResponse
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : String(err)
        return makeError(
          rpc.id,
          AgentErrorCode.ServerError,
          `accounts.getSnapshot unavailable: ${msg}`
        )
      }
    }

    default:
      return null
  }
}
