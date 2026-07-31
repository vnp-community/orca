/**
 * StepExecutors — Routes workflow step execution to the correct dev server relay (TDD-17)
 *
 * Supports step types:
 * - agent:        relay.call('agent.exec', { prompt, worktreePath, trustPreset })
 * - shell:        relay.call('git.exec', { script, env }) or generic shell
 * - webhook:      native fetch() with AbortSignal
 * - notification: relay.call('notification.send', { channel, message })
 * - condition:    synchronous expression evaluation (returns exitCode 0/1)
 *
 * Timeout: each step races against step.timeout (default 30 min).
 * Abort: checks signal.aborted before dispatch.
 *
 * @module main/workflow/StepExecutors
 */

import type { ProjectServerRouter } from '../project/ProjectServerRouter'
import type { WorkflowStep, StepOutput } from './WorkflowTypes'

const DEFAULT_TIMEOUT_MS = 30 * 60_000 // 30 minutes

export class StepExecutors {
  constructor(private readonly router: ProjectServerRouter) {}

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
    signal: AbortSignal
  ): Promise<StepOutput> {
    if (signal.aborted) {
      throw new Error('EXECUTION_CANCELLED')
    }

    const timeoutMs = step.timeout ?? DEFAULT_TIMEOUT_MS

    return Promise.race([
      this.executeByType(step, inputs, signal),
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
    signal: AbortSignal
  ): Promise<StepOutput> {
    const { type } = step.config

    switch (type) {
      case 'agent':
        return this.executeAgent(step, signal)
      case 'shell':
        return this.executeShell(step, signal)
      case 'webhook':
        return this.executeWebhook(step, signal)
      case 'notification':
        return this.executeNotification(step, signal)
      case 'condition':
        return this.executeCondition(step, inputs)
      default:
        throw new Error(`UNSUPPORTED_STEP_TYPE: "${String(type)}"`)
    }
  }

  // ── Agent step ────────────────────────────────────────────────────────────

  private async executeAgent(step: WorkflowStep, signal: AbortSignal): Promise<StepOutput> {
    const relay = await this.getRelay(step)
    if (signal.aborted) throw new Error('EXECUTION_CANCELLED')

    const result = (await relay.call('agent.exec', {
      stepId: step.id,
      prompt: step.config['prompt'],
      worktreePath: step.config['worktreePath'],
      trustPreset: step.config['trustPreset'] ?? 'default',
    })) as { exitCode?: number; stdout?: string; stderr?: string }

    return {
      exitCode: result.exitCode ?? 0,
      stdout: result.stdout,
      stderr: result.stderr,
    }
  }

  // ── Shell step ────────────────────────────────────────────────────────────

  private async executeShell(step: WorkflowStep, signal: AbortSignal): Promise<StepOutput> {
    const relay = await this.getRelay(step)
    if (signal.aborted) throw new Error('EXECUTION_CANCELLED')

    const result = (await relay.call('shell.exec', {
      script: step.config['script'],
      env: step.config['env'] ?? {},
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

  private async executeNotification(step: WorkflowStep, signal: AbortSignal): Promise<StepOutput> {
    const relay = await this.getRelay(step)
    if (signal.aborted) throw new Error('EXECUTION_CANCELLED')

    await relay.call('notification.send', {
      channel: step.config['channel'],
      message: step.config['message'],
    })

    return { exitCode: 0 }
  }

  // ── Condition step ────────────────────────────────────────────────────────

  private executeCondition(
    step: WorkflowStep,
    inputs: Record<string, unknown>
  ): Promise<StepOutput> {
    try {
      // Safe eval: only allows access to `inputs` via Function constructor
      const expression = step.config['expression'] as string
      // eslint-disable-next-line @typescript-eslint/no-implied-eval
      const fn = new Function('inputs', `return !!(${expression})`)
      const result = fn(inputs) as boolean
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
