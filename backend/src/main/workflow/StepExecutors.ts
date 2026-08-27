/**
 * StepExecutors — Routes workflow step execution to the correct dev server relay (TDD-17)
 *
 * Supports step types:
 * - agent:        relay.call('agent.execPrompt', { prompt, worktreePath, trustPreset })
 * - shell:        relay.call('git.exec', { script, env }) or generic shell
 * - webhook:      native fetch() with AbortSignal
 * - notification: relay.call('notification.send', { channel, message })
 * - condition:    synchronous expression evaluation (returns exitCode 0/1)
 *
 * Timeout: each step races against step.timeout (default 30 min).
 * Abort: checks signal.aborted before dispatch.
 *
 * agent.execPrompt (not agent.exec): agent.exec is a generic "run this binary"
 * RPC ({binary, args, cwd, stdin, env, timeoutMs}) with no concept of a
 * prompt/model/trustPreset — sending this step's domain-shaped payload to it
 * always failed with InvalidParams. agent.execPrompt resolves the AI-CLI
 * binary/args/credentials from {prompt, worktreePath, trustPreset, model,
 * accountId} agent-side. See specs/agent/api/gaps-and-findings.md.
 *
 * @module main/workflow/StepExecutors
 */

import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { ProviderResolver } from '../ai-providers/ProviderResolver'      // [NEW BUG-BE-HLD-008]
import type { AIProviderService } from '../ai-providers/AIProviderService'    // [NEW BUG-BE-HLD-008]
import type { WorkflowStep, StepOutput, WorkflowStepProviderConfig } from './WorkflowTypes'

const DEFAULT_TIMEOUT_MS = 30 * 60_000 // 30 minutes

export class StepExecutors {
  constructor(
    private readonly router: ProjectServerRouter,
    private readonly providerResolver: ProviderResolver,     // [NEW BUG-BE-HLD-008]
    private readonly aiProviderService: AIProviderService     // [NEW BUG-BE-HLD-008]
  ) {}

  /**
   * Execute a single workflow step.
   * Respects AbortSignal and applies per-step timeout via Promise.race.
   *
   * @throws Error('EXECUTION_CANCELLED') if signal already aborted
   * @throws Error('STEP_TIMEOUT') if execution exceeds step.timeout
   * @throws Error('UNSUPPORTED_STEP_TYPE') if step.config.type is unknown
   */
  async execute(
    step: WorkflowStep,
    inputs: Record<string, unknown>,
    signal: AbortSignal,
    traceId?: string, // [NEW] forwarded from WorkflowOrchestrator.executeStep()'s stepSpan.id
    triggeredBy?: string // [NEW BUG-BE-HLD-008] execution.triggeredBy — chỉ agent step dùng, để ProviderResolver áp user-scope priority
  ): Promise<StepOutput> {
    if (signal.aborted) {
      throw new Error('EXECUTION_CANCELLED')
    }

    const timeoutMs = step.timeout ?? DEFAULT_TIMEOUT_MS

    return Promise.race([
      this.executeByType(step, inputs, signal, traceId, triggeredBy),
      new Promise<never>((_, reject) => {
        const timer = setTimeout(
          () => reject(new Error(`STEP_TIMEOUT: step "${step.id}" exceeded ${timeoutMs}ms`)),
          timeoutMs
        )
        // Clean up timer if signal aborts before timeout fires
        signal.addEventListener('abort', () => clearTimeout(timer), { once: true })
      }),
    ])
  }

  // ── Private dispatch ──────────────────────────────────────────────────────

  private async executeByType(
    step: WorkflowStep,
    inputs: Record<string, unknown>,
    signal: AbortSignal,
    traceId?: string,
    triggeredBy?: string
  ): Promise<StepOutput> {
    const { type } = step.config

    switch (type) {
      case 'agent':
        return this.executeAgent(step, signal, traceId, triggeredBy)
      case 'shell':
        return this.executeShell(step, signal, traceId)
      case 'webhook':
        return this.executeWebhook(step, signal) // không qua relay — không cần traceId
      case 'notification':
        return this.executeNotification(step, signal, traceId)
      case 'condition':
        return this.executeCondition(step, inputs) // sync, không I/O — không cần traceId
      default:
        throw new Error(`UNSUPPORTED_STEP_TYPE: "${String(type)}"`)
    }
  }

  // ── Agent step ────────────────────────────────────────────────────────────

  private async executeAgent(
    step: WorkflowStep,
    signal: AbortSignal,
    traceId?: string,
    triggeredBy?: string
  ): Promise<StepOutput> {
    const relay = await this.getRelay(step)
    if (signal.aborted) {throw new Error('EXECUTION_CANCELLED')}

    // [NEW BUG-BE-HLD-008] Resolve per-step provider BEFORE dispatch — F36 doc's core
    // use-case (Claude ở bước 1, GPT-4o ở bước 2 trong CÙNG 1 workflow).
    const resolved = await this.resolveAgentProvider(step, triggeredBy)

    const result = (await relay.call('agent.execPrompt', {
      stepId: step.id,
      prompt: step.config['prompt'],
      worktreePath: step.config['worktreePath'],
      trustPreset: step.config['trustPreset'] ?? 'default',
      traceId, // [NEW] — relay:agentCall (dev-server-relay-bridge.ts) resume theo id này
      // Omit entirely (not even as `undefined` keys) when no override AND no scope match —
      // dev server's agent.exec handler then falls back to its own pre-fix default account,
      // preserving current behavior for workflows that never pin a provider.
      ...(resolved ? { accountId: resolved.accountId, model: resolved.model } : {}),
    })) as { exitCode?: number; stdout?: string; stderr?: string }

    return {
      exitCode: result.exitCode ?? 0,
      stdout: result.stdout,
      stderr: result.stderr,
    }
  }

  /**
   * Resolve the AI provider account an 'agent' step should use.
   *
   * Priority:
   * 1. step.config.provider.accountId — explicit pin, validated + must be 'active'.
   * 2. ProviderResolver.resolve() fallback — user > project > server scope, same
   *    priority chain already used by every other AI-consuming feature (TDD-16).
   * 3. undefined — no scope match (or serverSpec is 'server:<id>', not yet
   *    resolvable to a devServerId here — same SERVER_SPEC_NOT_SUPPORTED gap as
   *    getRelay() below) → let the dev server apply its own configured default.
   *
   * @throws Error('WORKFLOW_STEP_PROVIDER_NOT_FOUND')  step.config.provider.accountId doesn't exist
   * @throws Error('WORKFLOW_STEP_PROVIDER_INACTIVE')   the pinned account isn't 'active'
   */
  private async resolveAgentProvider(
    step: WorkflowStep,
    triggeredBy: string | undefined
  ): Promise<{ accountId: string; model?: string } | undefined> {
    const providerCfg = step.config['provider'] as WorkflowStepProviderConfig | undefined

    // Case 1: explicit per-step pin — trust verbatim, no priority resolution.
    if (providerCfg?.accountId) {
      const account = await this.aiProviderService.getAccount(providerCfg.accountId)
      if (!account) {
        throw new Error(
          `WORKFLOW_STEP_PROVIDER_NOT_FOUND: step "${step.id}" references unknown provider account "${providerCfg.accountId}"`
        )
      }
      if (account.status !== 'active') {
        throw new Error(
          `WORKFLOW_STEP_PROVIDER_INACTIVE: step "${step.id}" provider account "${providerCfg.accountId}" is not active (status: "${account.status}")`
        )
      }
      return { accountId: account.id, model: providerCfg.model ?? account.model }
    }

    // Case 2: no override — fall back to ProviderResolver's priority chain, scoped to
    // this step's project. Only 'project:<id>' serverSpecs can resolve a devServerId today.
    const [specType, specId] = step.serverSpec.split(':')
    if (specType !== 'project' || !specId) {
      return undefined
    }

    const project = await this.router.getProject(specId)
    if (!project) {return undefined}

    try {
      const account = await this.providerResolver.resolve({
        devServerId: project.devServerId,
        projectId: specId,
        userId: triggeredBy ?? '__workflow_system__',
        modelHint: providerCfg?.model,
      })
      return { accountId: account.id, model: providerCfg?.model ?? account.model }
    } catch (err) {
      // NO_PROVIDER_AVAILABLE → step still runs, dev server applies its own default —
      // matches pre-fix "always default" behavior for workflows that never pin a provider.
      if (err instanceof Error && err.message.startsWith('NO_PROVIDER_AVAILABLE')) {return undefined}
      throw err
    }
  }

  // ── Shell step ────────────────────────────────────────────────────────────

  private async executeShell(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
    const relay = await this.getRelay(step)
    if (signal.aborted) {throw new Error('EXECUTION_CANCELLED')}

    const result = (await relay.call('shell.exec', {
      script: step.config['script'],
      env: step.config['env'] ?? {},
      traceId, // [NEW]
    })) as { exitCode?: number; stdout?: string; stderr?: string }

    return {
      exitCode: result.exitCode ?? 0,
      stdout: result.stdout,
      stderr: result.stderr,
    }
  }

  // ── Webhook step ──────────────────────────────────────────────────────────

  private async executeWebhook(step: WorkflowStep, signal: AbortSignal): Promise<StepOutput> {
    const url = step.config['url'] as string
    const method = (step.config['method'] as string | undefined) ?? 'POST'
    const body = step.config['body']

    const response = await fetch(url, {
      method,
      signal,
      headers: { 'Content-Type': 'application/json' },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })

    const text = await response.text().catch(() => '')
    return {
      exitCode: response.ok ? 0 : 1,
      stdout: text,
      stderr: response.ok ? undefined : `HTTP ${response.status}: ${response.statusText}`,
      data: { status: response.status, ok: response.ok },
    }
  }

  // ── Notification step ─────────────────────────────────────────────────────

  private async executeNotification(step: WorkflowStep, signal: AbortSignal, traceId?: string): Promise<StepOutput> {
    const relay = await this.getRelay(step)
    if (signal.aborted) {throw new Error('EXECUTION_CANCELLED')}

    await relay.call('notification.send', {
      channel: step.config['channel'],
      message: step.config['message'],
      traceId, // [NEW]
    })

    return { exitCode: 0 }
  }

  // ── Condition step ────────────────────────────────────────────────────────

  private executeCondition(
    step: WorkflowStep,
    inputs: Record<string, unknown>
  ): Promise<StepOutput> {
    try {
      const expression = step.config['expression'] as string
      // FIX TASK-WF-001: Replace new Function() (code injection risk) with
      // a sandboxed evaluator supporting only safe comparison operators.
      const result = evaluateSafeCondition(expression, inputs)
      return Promise.resolve({
        exitCode: result ? 0 : 1,
        data: { result },
      })
    } catch (err) {
      return Promise.resolve({
        exitCode: 1,
        stderr: err instanceof Error ? err.message : String(err),
      })
    }
  }

  // ── Helpers ───────────────────────────────────────────────────────────────

  /**
   * Resolve the relay bridge for a step's serverSpec.
   * serverSpec format: 'project:<projectId>' or 'server:<devServerId>'
   */
  private async getRelay(step: WorkflowStep) {
    const [specType, specId] = step.serverSpec.split(':')
    if (!specType || !specId) {
      throw new Error(`INVALID_SERVER_SPEC: "${step.serverSpec}" — expected 'project:<id>' or 'server:<id>'`)
    }

    if (specType === 'project') {
      // Use a system userId for server-triggered workflow steps
      return this.router.getRelayForProject(specId, '__workflow_system__')
    }

    if (specType === 'server') {
      // Direct server access — use internal router method if available
      // Fallback: create a synthetic project access via projectService.get
      throw new Error(`SERVER_SPEC_NOT_SUPPORTED: direct server specs require router.getRelayForServer (not yet implemented)`)
    }

    throw new Error(`UNKNOWN_SERVER_SPEC_TYPE: "${specType}"`)
  }
}

// ── Safe condition evaluator (no eval/Function) ─────────────────────────────
//
// Supports expressions in the form:
//   "${varName} == 'value'"   → string equality
//   "${varName} != 'value'"   → string inequality
//   "${varName} > 5"          → numeric comparison (>, <, >=, <=)
//   "true" / "false"          → literal boolean
//
// Anything else logs a warning and returns false (fail-safe).

function evaluateSafeCondition(
  expression: string,
  context:    Record<string, unknown>
): boolean {
  // Step 1: Interpolate ${varName} placeholders from context
  const interpolated = expression.replace(
    /\$\{([^}]+)\}/g,
    (_, key: string) => {
      const val = context[key.trim()]
      return val === undefined ? '' : String(val)
    }
  )

  // Step 2: Parse supported comparison patterns
  const normalize = (s: string): unknown => {
    const trimmed = s.trim().replace(/^['"](.*)['"]$/, '$1')
    if (trimmed === 'true')  {return true}
    if (trimmed === 'false') {return false}
    const n = Number(trimmed)
    return isNaN(n) ? trimmed : n
  }

  // Match operators in order of specificity (>= before >)
  const patterns: [RegExp, (a: unknown, b: unknown) => boolean][] = [
    [/^(.+?)\s*===\s*(.+)$/, (a, b) => a === b],
    [/^(.+?)\s*!==\s*(.+)$/, (a, b) => a !== b],
    [/^(.+?)\s*==\s*(.+)$/,  (a, b) => String(a) === String(b)],
    [/^(.+?)\s*!=\s*(.+)$/,  (a, b) => String(a) !== String(b)],
    [/^(.+?)\s*>=\s*(.+)$/,  (a, b) => Number(a) >= Number(b)],
    [/^(.+?)\s*<=\s*(.+)$/,  (a, b) => Number(a) <= Number(b)],
    [/^(.+?)\s*>\s*(.+)$/,   (a, b) => Number(a) >  Number(b)],
    [/^(.+?)\s*<\s*(.+)$/,   (a, b) => Number(a) <  Number(b)],
  ]

  for (const [pattern, compare] of patterns) {
    const m = interpolated.match(pattern)
    if (m) {return compare(normalize(m[1]!), normalize(m[2]!))}
  }

  // Literal boolean
  const trimmed = interpolated.trim()
  if (trimmed === 'true')  {return true}
  if (trimmed === 'false') {return false}

  // Unknown expression — fail-safe: return false and warn
  console.warn(
    `[StepExecutors] evaluateSafeCondition: unsupported expression "${expression}" ` +
    '— returning false. Supported: ==, !=, >, <, >=, <=, true, false, ${varName}.'
  )
  return false
}
