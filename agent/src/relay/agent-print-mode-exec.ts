// src/relay/agent-print-mode-exec.ts
// Part A implementation of `agent.execPrompt` — resolves a workflow/task-step
// prompt request into a one-shot, non-interactive AI-CLI invocation.
//
// Distinct from `agent.exec` (a generic, tested "run this binary" primitive —
// see agent-rpc-dispatch.ts's case 'agent.exec') and from `agent.spawn`
// (interactive PTY session). StepExecutors.executeAgent() was sending a
// domain-shaped payload ({prompt, worktreePath, trustPreset, model, accountId})
// straight to agent.exec, which only accepts {binary, args, cwd, stdin, env,
// timeoutMs} — every 'agent'-type workflow step failed with InvalidParams.
// See specs/agent/api/gaps-and-findings.md.

import { spawn } from 'node:child_process'
import type { AgentConfig } from './agent-config'
import type { AgentLogger } from './agent-logger'
import { resolveAgentSpec, buildAgentEnv } from './agent-spawner'
import { YOLO_TUI_AGENT_ARGS } from '../shared/tui-agent-permissions'
import { AgentErrorCode } from '../shared/agent-wire-protocol'
import { Tracers } from '../shared/trace/tracers'

const DEFAULT_TIMEOUT_MS = 5 * 60_000
const MAX_TIMEOUT_MS = 15 * 60_000
const MIN_TIMEOUT_MS = 1_000

type PrintModeExecResult = {
  stdout: string
  stderr: string
  exitCode: number | null
  timedOut: boolean
  stepId?: string
}

export async function handleAgentExecPrompt(
  id: string | number | null,
  params: Record<string, unknown>,
  config: AgentConfig,
  log: AgentLogger
): Promise<object> {
  const prompt = typeof params.prompt === 'string' ? params.prompt : ''
  const worktreePath = typeof params.worktreePath === 'string' ? params.worktreePath : ''
  const stepId = typeof params.stepId === 'string' ? params.stepId : undefined
  // 'default'/'standard'/'none' (StepExecutors.ts's and the old vocabulary's
  // non-'full' values) all mean "no extra flag" here — only 'full' is acted on.
  const trustPresetFull = params.trustPreset === 'full'
  const modelId = typeof params.model === 'string' && params.model ? params.model : 'claude'
  const accountId = typeof params.accountId === 'string' ? params.accountId : ''
  // ProfileAwareAgentSpawner forwards profile-resolved env (PATH additions,
  // ORCA_PROJECT_ID/ORCA_ACCOUNT_ID/etc.) here — merged on top of
  // buildAgentEnv()'s base env via its own extraEnv slot, same override order
  // agent.spawn already uses.
  const extraEnv =
    params.env && typeof params.env === 'object' && !Array.isArray(params.env)
      ? (params.env as Record<string, string>)
      : undefined
  const timeoutMs =
    typeof params.timeoutMs === 'number'
      ? Math.min(Math.max(params.timeoutMs, MIN_TIMEOUT_MS), MAX_TIMEOUT_MS)
      : DEFAULT_TIMEOUT_MS

  const span = Tracers.agentOrchSpawn.start({ stepId, modelId })

  if (!prompt || !worktreePath) {
    const missing = [!prompt && 'prompt', !worktreePath && 'worktreePath'].filter(Boolean).join(', ')
    span.fail(`missing ${missing}`)
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message: `agent.execPrompt: missing required field(s): ${missing}`
      }
    }
  }

  const spec = resolveAgentSpec(modelId)
  // Only claude's non-interactive `--print <prompt>` flag is a validated
  // precedent in this codebase (agent-tool-registry.ts's claude_code tool).
  // codex/gemini/opencode's print-mode flags are unverified even for the
  // existing *interactive* PTY path (see agent-spawner.ts's AGENT_SPECS
  // comments) — fail fast instead of guessing a flag that silently no-ops or
  // hangs waiting for interactive input.
  if (!spec || spec.binary !== 'claude') {
    span.fail('unsupported model for one-shot exec', { modelId })
    return {
      jsonrpc: '2.0',
      id,
      error: {
        code: AgentErrorCode.InvalidParams,
        message:
          `agent.execPrompt: model "${modelId}" is not supported for one-shot execution yet ` +
          `(only claude is validated) — UNSUPPORTED_MODEL_FOR_ONE_SHOT_EXEC`
      }
    }
  }

  const args = ['--print', prompt]
  if (trustPresetFull && YOLO_TUI_AGENT_ARGS.claude) {
    args.push(YOLO_TUI_AGENT_ARGS.claude)
  }

  let env: Record<string, string>
  try {
    // resolvedApiKey is always null here — no live backend caller forwards a
    // plaintext key for this RPC (that's the ADR-008 gap, left dormant by
    // design; see specs/agent/api/compliance-audit-2026-08-15.md). If
    // accountId is set, buildAgentEnv() throws a clear, actionable error
    // instead of silently running unauthenticated; if accountId is absent,
    // it proceeds relying on the CLI's own already-authenticated state.
    env = await buildAgentEnv(
      { accountId, userId: '', taskId: stepId ?? '', cwd: worktreePath, model: modelId, extraEnv },
      spec,
      config,
      null,
      log,
      span.id
    )
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    span.fail(msg, { accountId })
    return { jsonrpc: '2.0', id, error: { code: AgentErrorCode.PermissionDenied, message: msg } }
  }

  span.step('subprocess-spawn', { binary: spec.binary, cwd: worktreePath })
  const result = await new Promise<PrintModeExecResult>((resolve) => {
    let stdout = ''
    let stderr = ''
    let timedOut = false
    let settled = false
    const child = spawn(spec.binary, args, {
      cwd: worktreePath,
      env: { ...process.env, ...env },
      stdio: ['ignore', 'pipe', 'pipe']
    })

    const finish = (r: PrintModeExecResult): void => {
      if (settled) {return}
      settled = true
      clearTimeout(timer)
      resolve(r)
    }
    const timer = setTimeout(() => {
      timedOut = true
      try {
        child.kill('SIGKILL')
      } catch {
        /* ignore */
      }
      finish({ stdout, stderr, exitCode: null, timedOut })
    }, timeoutMs)

    child.stdout?.on('data', (d: Buffer) => {
      stdout += d.toString('utf8')
    })
    child.stderr?.on('data', (d: Buffer) => {
      stderr += d.toString('utf8')
    })
    child.on('error', (err) => {
      finish({ stdout, stderr: err.message, exitCode: null, timedOut })
    })
    child.on('close', (code) => {
      finish({ stdout, stderr, exitCode: code, timedOut })
    })
  })

  log.info(
    `agent.execPrompt: stepId=${stepId ?? '(none)'} model=${modelId} ` +
      `exitCode=${result.exitCode} timedOut=${result.timedOut}`
  )
  if (result.timedOut) {
    span.fail(`timeout after ${timeoutMs}ms`)
  } else if (result.exitCode !== 0) {
    span.fail(`exit code ${result.exitCode}`, { exitCode: result.exitCode ?? -1 })
  } else {
    span.ok({ exitCode: result.exitCode ?? 0 })
  }

  return { jsonrpc: '2.0', id, result: { ...result, stepId } }
}
