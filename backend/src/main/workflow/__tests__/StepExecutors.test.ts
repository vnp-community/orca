/**
 * Tests for StepExecutors' agent.execPrompt fix.
 *
 * StepExecutors.executeAgent() used to send a domain-shaped payload
 * ({prompt, worktreePath, trustPreset, model, accountId}) to relay.call('agent.exec', ...),
 * an RPC that only accepts {binary, args, cwd, stdin, env, timeoutMs} — every
 * 'agent'-type workflow step failed with InvalidParams. See
 * specs/agent/api/gaps-and-findings.md.
 *
 * @module main/workflow/__tests__/StepExecutors.test
 */

import { describe, it, expect, vi } from 'vitest'
import { StepExecutors } from '../StepExecutors'
import type { ProjectServerRouter } from '../../project/ProjectServerRouter'
import type { ProviderResolver } from '../../ai-providers/ProviderResolver'
import type { AIProviderService } from '../../ai-providers/AIProviderService'
import type { WorkflowStep } from '../WorkflowTypes'

// ── helpers ──────────────────────────────────────────────────────────────────

function makeRouter(relayCall: ReturnType<typeof vi.fn>): ProjectServerRouter {
  return {
    getRelayForProject: vi.fn().mockResolvedValue({ call: relayCall }),
    // No providerCfg pin + no 'project:'-resolvable step in these tests reaches
    // getProject() before returning early — kept here only so a future test can
    // extend into the provider-resolution branch without a fresh mock shape.
    getProject: vi.fn().mockResolvedValue(null)
  } as unknown as ProjectServerRouter
}

function makeExecutors(relayCall: ReturnType<typeof vi.fn>): StepExecutors {
  const providerResolver = { resolve: vi.fn() } as unknown as ProviderResolver
  const aiProviderService = { getAccount: vi.fn() } as unknown as AIProviderService
  return new StepExecutors(makeRouter(relayCall), providerResolver, aiProviderService)
}

function agentStep(overrides: Partial<WorkflowStep['config']> = {}): WorkflowStep {
  return {
    id: 's-agent',
    name: 'Agent Step',
    serverSpec: 'project:p1',
    config: { type: 'agent', prompt: 'do it', worktreePath: '/wt', ...overrides }
  }
}

describe('StepExecutors — agent.execPrompt', () => {
  it('executeAgent calls relay.call("agent.execPrompt", ...), not "agent.exec"', async () => {
    const relayCall = vi.fn().mockResolvedValue({ exitCode: 0 })
    const executors = makeExecutors(relayCall)

    await executors.execute(agentStep(), {}, new AbortController().signal, 'trace-agent-1')

    expect(relayCall).toHaveBeenCalledWith(
      'agent.execPrompt',
      expect.objectContaining({
        stepId: 's-agent',
        prompt: 'do it',
        worktreePath: '/wt',
        traceId: 'trace-agent-1'
      })
    )
    expect(relayCall).not.toHaveBeenCalledWith('agent.exec', expect.anything())
  })

  it('forwards trustPreset, defaulting to "default" when the step omits it', async () => {
    const relayCall = vi.fn().mockResolvedValue({ exitCode: 0 })
    const executors = makeExecutors(relayCall)

    await executors.execute(agentStep(), {}, new AbortController().signal)

    expect(relayCall).toHaveBeenCalledWith(
      'agent.execPrompt',
      expect.objectContaining({ trustPreset: 'default' })
    )
  })

  it('maps the RPC result back into StepOutput (exitCode/stdout/stderr)', async () => {
    const relayCall = vi.fn().mockResolvedValue({ exitCode: 1, stdout: 'out', stderr: 'err' })
    const executors = makeExecutors(relayCall)

    const output = await executors.execute(agentStep(), {}, new AbortController().signal)

    expect(output).toEqual({ exitCode: 1, stdout: 'out', stderr: 'err' })
  })

  it('omits accountId/model entirely when no provider override resolves (falls back to dev server default)', async () => {
    const relayCall = vi.fn().mockResolvedValue({ exitCode: 0 })
    const executors = makeExecutors(relayCall)

    await executors.execute(agentStep(), {}, new AbortController().signal)

    const params = relayCall.mock.calls[0]?.[1] as Record<string, unknown>
    expect('accountId' in params).toBe(false)
    expect('model' in params).toBe(false)
  })
})
