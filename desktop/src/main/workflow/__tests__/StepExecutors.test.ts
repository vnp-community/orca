/**
 * Tests for StepExecutors trace forwarding (TASK-BE-017.3).
 *
 * `WorkflowOrchestrator.executeStep()` forwards `stepSpan.id` as the 4th
 * argument to `executor(...)`. StepExecutors.execute()/executeByType() must
 * forward it into `relay.call()` for the relay-backed step types
 * (agent/shell/notification) — `webhook`/`condition` don't go through the
 * relay and must not require it.
 *
 * @module main/workflow/__tests__/StepExecutors.test
 */

import { describe, it, expect, vi, afterEach } from 'vitest'
import { StepExecutors } from '../StepExecutors'
import type { ProjectServerRouter } from '../../project/ProjectServerRouter'
import type { WorkflowStep } from '../WorkflowTypes'

// ── helpers ──────────────────────────────────────────────────────────────────

function makeRouter(relayCall: ReturnType<typeof vi.fn>): ProjectServerRouter {
  return {
    getRelayForProject: vi.fn().mockResolvedValue({ call: relayCall }),
  } as unknown as ProjectServerRouter
}

function agentStep(): WorkflowStep {
  return {
    id: 's-agent',
    name: 'Agent Step',
    serverSpec: 'project:p1',
    config: { type: 'agent', prompt: 'do it', worktreePath: '/wt' },
  }
}

function shellStep(): WorkflowStep {
  return {
    id: 's-shell',
    name: 'Shell Step',
    serverSpec: 'project:p1',
    config: { type: 'shell', script: 'echo hi' },
  }
}

function notificationStep(): WorkflowStep {
  return {
    id: 's-notify',
    name: 'Notification Step',
    serverSpec: 'project:p1',
    config: { type: 'notification', channel: 'slack', message: 'hi' },
  }
}

function webhookStep(url: string): WorkflowStep {
  return {
    id: 's-webhook',
    name: 'Webhook Step',
    serverSpec: 'project:p1',
    config: { type: 'webhook', url },
  }
}

function conditionStep(expression: string): WorkflowStep {
  return {
    id: 's-condition',
    name: 'Condition Step',
    serverSpec: 'project:p1',
    config: { type: 'condition', expression },
  }
}

describe('StepExecutors — CR-TRACE-017 trace forwarding', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('executeAgent forwards traceId into relay.call("agent.exec", { ...traceId })', async () => {
    const relayCall = vi.fn().mockResolvedValue({ exitCode: 0 })
    const executors = new StepExecutors(makeRouter(relayCall))

    await executors.execute(agentStep(), {}, new AbortController().signal, 'trace-agent-1')

    expect(relayCall).toHaveBeenCalledWith(
      'agent.exec',
      expect.objectContaining({ stepId: 's-agent', traceId: 'trace-agent-1' })
    )
  })

  it('executeShell forwards traceId into relay.call("shell.exec", { ...traceId })', async () => {
    const relayCall = vi.fn().mockResolvedValue({ exitCode: 0 })
    const executors = new StepExecutors(makeRouter(relayCall))

    await executors.execute(shellStep(), {}, new AbortController().signal, 'trace-shell-1')

    expect(relayCall).toHaveBeenCalledWith(
      'shell.exec',
      expect.objectContaining({ script: 'echo hi', traceId: 'trace-shell-1' })
    )
  })

  it('executeNotification forwards traceId into relay.call("notification.send", { ...traceId })', async () => {
    const relayCall = vi.fn().mockResolvedValue({})
    const executors = new StepExecutors(makeRouter(relayCall))

    await executors.execute(notificationStep(), {}, new AbortController().signal, 'trace-notify-1')

    expect(relayCall).toHaveBeenCalledWith(
      'notification.send',
      expect.objectContaining({ channel: 'slack', traceId: 'trace-notify-1' })
    )
  })

  it('executeWebhook does not go through relay and does not require traceId', async () => {
    const relayCall = vi.fn()
    const executors = new StepExecutors(makeRouter(relayCall))
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      statusText: 'OK',
      text: () => Promise.resolve('body'),
    })
    vi.stubGlobal('fetch', fetchMock)

    const output = await executors.execute(
      webhookStep('http://example.com'),
      {},
      new AbortController().signal,
      'trace-webhook-1'
    )

    expect(output.exitCode).toBe(0)
    expect(relayCall).not.toHaveBeenCalled()
  })

  it('executeCondition is synchronous, does not go through relay, and does not require traceId', async () => {
    const relayCall = vi.fn()
    const executors = new StepExecutors(makeRouter(relayCall))

    const output = await executors.execute(
      conditionStep('true'),
      {},
      new AbortController().signal,
      'trace-condition-1'
    )

    expect(output.exitCode).toBe(0)
    expect(relayCall).not.toHaveBeenCalled()
  })

  it('omitting traceId (undefined) is backward compatible — relay.call still receives traceId: undefined', async () => {
    const relayCall = vi.fn().mockResolvedValue({ exitCode: 0 })
    const executors = new StepExecutors(makeRouter(relayCall))

    await executors.execute(agentStep(), {}, new AbortController().signal)

    expect(relayCall).toHaveBeenCalledWith(
      'agent.exec',
      expect.objectContaining({ traceId: undefined })
    )
  })
})
