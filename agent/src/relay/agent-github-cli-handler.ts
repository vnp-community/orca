// src/relay/agent-github-cli-handler.ts
// Part A implementation of `github.exec` — the ADR-018 migration that moves
// `gh` CLI execution out of backend/ (which must never execute dev-server
// work) into agent/. Backend's runner.ts (ghExecFileAsync) now routes
// through this RPC via a connection-scoped provider instead of spawning
// `gh` in its own process. See specs/agent/api/gaps-and-findings.md.
//
// Reuses external-api-connector.ts's already-correct per-user-isolated
// primitives (buildGhEnv, execGhCaptured — which itself wraps the agent's
// own ghExecFileAsync in agent/src/main/git/runner.ts for retry/rate-limit
// handling) rather than re-deriving gh execution here.

import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'
import { assertAllowedGhArgs } from './hosted-cli-api-allowlist'
import { buildGhEnv, execGhCaptured } from './external-api-connector'

const execTracer = createTracer('agent:github-exec')

const DEFAULT_TIMEOUT_MS = 30_000
const MIN_TIMEOUT_MS = 1_000
const MAX_TIMEOUT_MS = 5 * 60_000

export async function handleGithubExec(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig
): Promise<object> {
  const args = Array.isArray(params.args) ? (params.args as unknown[]).map(String) : []
  const cwd = typeof params.cwd === 'string' && params.cwd ? params.cwd : config.workDir
  const userId = typeof params.userId === 'string' ? params.userId : ''
  const idempotent = typeof params.idempotent === 'boolean' ? params.idempotent : undefined
  const envOverrides =
    params.env && typeof params.env === 'object' && !Array.isArray(params.env)
      ? (params.env as Record<string, string>)
      : undefined
  const timeoutMs =
    typeof params.timeoutMs === 'number'
      ? Math.min(Math.max(params.timeoutMs, MIN_TIMEOUT_MS), MAX_TIMEOUT_MS)
      : DEFAULT_TIMEOUT_MS

  const span = execTracer.start({ subcommand: args[0] ?? '(none)', userId })

  try {
    assertAllowedGhArgs(args)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: msg } }
  }

  const env = { ...buildGhEnv(userId, process.env), ...envOverrides }
  const result = await execGhCaptured(args, { cwd, env, timeout: timeoutMs, idempotent })

  if (result.exitCode === 0) {
    span.ok({ exitCode: result.exitCode })
  } else {
    span.fail(`exit code ${result.exitCode}`, { exitCode: result.exitCode })
  }
  return { jsonrpc: '2.0', id, result }
}
