// src/relay/agent-gitlab-cli-handler.ts
// Part A implementation of `gitlab.exec` — mirrors agent-github-cli-handler.ts
// for GitLab. See that file's header comment and
// specs/agent/api/gaps-and-findings.md for the ADR-018 migration rationale.

import type { AgentConfig } from './agent-config'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { createTracer } from '../shared/trace'
import { assertAllowedGlabArgs } from './hosted-cli-api-allowlist'
import { buildGlabEnv, execGlabCaptured } from './external-api-connector'

const execTracer = createTracer('agent:gitlab-exec')

const DEFAULT_TIMEOUT_MS = 30_000
const MIN_TIMEOUT_MS = 1_000
const MAX_TIMEOUT_MS = 5 * 60_000

export async function handleGitlabExec(
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
    assertAllowedGlabArgs(args)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg)
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.InvalidParams, message: msg } }
  }

  const env = { ...buildGlabEnv(userId, process.env), ...envOverrides }
  const result = await execGlabCaptured(args, { cwd, env, timeout: timeoutMs, idempotent })

  if (result.exitCode === 0) {
    span.ok({ exitCode: result.exitCode })
  } else {
    span.fail(`exit code ${result.exitCode}`, { exitCode: result.exitCode })
  }
  return { jsonrpc: '2.0', id, result }
}
