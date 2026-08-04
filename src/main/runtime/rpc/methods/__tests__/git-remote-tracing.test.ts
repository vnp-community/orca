/**
 * Tests for REMOTE git RPC handler tracing (TASK-BE-005.3/005.4).
 *
 * Covers `git.diff`, `git.generateCommitMessage`, `git.pr.create` in
 * `src/main/runtime/rpc/methods/git-remote.ts` — instrumented with
 * `Tracers.codeReviewDiff` / `Tracers.codeReviewAiCommit` / `Tracers.codeReviewCreatePr`,
 * forwarding `traceId` into every `relay.call()`.
 *
 * @module main/runtime/rpc/methods/__tests__/git-remote-tracing.test
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { registerRemoteGitRpcMethods } from '../git-remote'
import type { RpcMethod, RpcContext } from '../../core'
import type { ProjectServerRouter } from '../../../../project/ProjectServerRouter'
import type { AIProviderService } from '../../../../ai-providers/AIProviderService'
import type { TaskService } from '../../../../task/TaskService'
import type { TaskGrantService } from '../../../../task/TaskGrantService'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'
import { Tracers } from '../../../../../shared/trace/tracers'

// ── Mock factories (mirrors git-remote.test.ts conventions) ────────────────────

function makeRelay(callResults: Record<string, unknown> = {}) {
  return {
    call: vi.fn().mockImplementation((method: string) => {
      return Promise.resolve(callResults[method] ?? { stdout: '', stderr: '', exitCode: 0 })
    }),
    callStream: vi.fn().mockResolvedValue({ stdout: '', stderr: '', exitCode: 0 }),
  }
}

function makeRouter(relay = makeRelay()) {
  return {
    getRelayForProject: vi.fn().mockResolvedValue(relay),
  } as unknown as ProjectServerRouter
}

function makeTaskService() {
  return {
    get: vi.fn().mockResolvedValue(null),
    update: vi.fn().mockResolvedValue(undefined),
    addComment: vi.fn().mockResolvedValue(undefined),
    list: vi.fn().mockResolvedValue([]),
  } as unknown as TaskService
}

function makeTaskGrantService() {
  return {
    resolvePermission: vi.fn().mockResolvedValue('edit'),
  } as unknown as TaskGrantService
}

function makeAIService() {
  return {} as unknown as AIProviderService
}

function makeCtx(userId = 'user-1'): RpcContext {
  return { userId } as RpcContext
}

function findMethod(methods: RpcMethod[], name: string) {
  const m = methods.find((m) => m.name === name)
  if (!m) throw new Error(`Method ${name} not found`)
  return m
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('git-remote.ts REMOTE RPC handler tracing', () => {
  let relay: ReturnType<typeof makeRelay>
  let router: ProjectServerRouter
  let methods: RpcMethod[]

  beforeEach(() => {
    relay = makeRelay({
      'git.exec': { stdout: 'diff --git a b\n', stderr: '', exitCode: 0 },
    })
    router = makeRouter(relay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), makeTaskService(), makeTaskGrantService())
  })

  // ── git.diff (remote) ──────────────────────────────────────────────────────

  it('git.diff (remote) → start() mode:remote, step(routeRelay), ok() with fileCount', async () => {
    const { events, stop } = captureTraceEvents()

    await findMethod(methods, 'git.diff').handler(
      { projectId: 'proj-1', worktreePath: '/repo' },
      makeCtx()
    )
    stop()

    const flowEvents = events.filter((e) => e.flow === 'codeReview:diff')
    expect(flowEvents.filter((e) => e.level === 'start')).toHaveLength(1)
    expect(flowEvents[0]?.fields.mode).toBe('remote')
    expect(flowEvents.some((e) => e.level === 'step' && e.label === 'routeRelay')).toBe(true)
    const okEvent = flowEvents.find((e) => e.level === 'ok')
    expect(okEvent).toBeDefined()
    expect(okEvent?.fields.fileCount).toBeDefined()
  })

  it('git.diff (remote) → relay.call throws → span.fail() with mode:remote before exception propagates', async () => {
    const failingRelay = makeRelay()
    vi.mocked(failingRelay.call).mockRejectedValue(new Error('relay unreachable'))
    router = makeRouter(failingRelay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), makeTaskService(), makeTaskGrantService())
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod(methods, 'git.diff').handler({ projectId: 'proj-1', worktreePath: '/repo' }, makeCtx())
    ).rejects.toThrow('relay unreachable')
    stop()

    const failEvent = events.find((e) => e.flow === 'codeReview:diff' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.mode).toBe('remote')
  })

  it('git.diff (remote) → traceId forwarded → span.id resumes params.traceId (no fresh random id)', async () => {
    const { events, stop } = captureTraceEvents()

    await findMethod(methods, 'git.diff').handler(
      { projectId: 'proj-1', worktreePath: '/repo', traceId: 'abc123' },
      makeCtx()
    )
    stop()

    const startEvent = events.find((e) => e.flow === 'codeReview:diff' && e.level === 'start')
    expect(startEvent?.id).toBe('abc123')
  })

  it('git.diff (remote) → relay.call receives traceId === span.id', async () => {
    const { events, stop } = captureTraceEvents()

    await findMethod(methods, 'git.diff').handler(
      { projectId: 'proj-1', worktreePath: '/repo' },
      makeCtx()
    )
    stop()

    const startEvent = events.find((e) => e.flow === 'codeReview:diff' && e.level === 'start')
    const callParams = vi.mocked(relay.call).mock.calls[0]?.[1] as { traceId?: string }
    expect(callParams.traceId).toBe(startEvent?.id)
  })

  // ── git.generateCommitMessage (remote) ────────────────────────────────────

  it('git.generateCommitMessage (remote) → happy path emits diffStaged then aiComplete steps in order', async () => {
    relay = makeRelay({
      'git.exec': { stdout: '+changed\n', stderr: '', exitCode: 0 },
      'ai.complete': { content: 'feat: add tracing spans' },
    })
    router = makeRouter(relay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), makeTaskService(), makeTaskGrantService())
    const { events, stop } = captureTraceEvents()

    const result = await findMethod(methods, 'git.generateCommitMessage').handler(
      { projectId: 'proj-1', worktreePath: '/repo', devServerId: 'ds-1' },
      makeCtx()
    )
    stop()

    expect(result).toMatchObject({ message: 'feat: add tracing spans' })
    const steps = events.filter((e) => e.flow === 'codeReview:aiCommitMessage' && e.level === 'step')
    expect(steps.map((e) => e.label)).toEqual(['diffStaged', 'aiComplete'])
    const okEvent = events.find((e) => e.flow === 'codeReview:aiCommitMessage' && e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({ mode: 'remote' })
  })

  it('git.generateCommitMessage (remote) → empty staged diff → fail(GIT_NO_STAGED_CHANGES) before ai.complete', async () => {
    relay = makeRelay({ 'git.exec': { stdout: '', stderr: '', exitCode: 0 } })
    router = makeRouter(relay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), makeTaskService(), makeTaskGrantService())
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod(methods, 'git.generateCommitMessage').handler(
        { projectId: 'proj-1', worktreePath: '/repo', devServerId: 'ds-1' },
        makeCtx()
      )
    ).rejects.toThrow('GIT_NO_STAGED_CHANGES')
    stop()

    const failEvent = events.find(
      (e) => e.flow === 'codeReview:aiCommitMessage' && e.level === 'fail'
    )
    expect(failEvent?.fields.err).toContain('GIT_NO_STAGED_CHANGES')
    expect(relay.call).not.toHaveBeenCalledWith('ai.complete', expect.anything())
  })

  it('git.generateCommitMessage (remote) → empty AI response → fail(GIT_AI_EMPTY_RESPONSE)', async () => {
    relay = makeRelay({
      'git.exec': { stdout: '+changed\n', stderr: '', exitCode: 0 },
      'ai.complete': { content: '' },
    })
    router = makeRouter(relay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), makeTaskService(), makeTaskGrantService())
    const { events, stop } = captureTraceEvents()

    await expect(
      findMethod(methods, 'git.generateCommitMessage').handler(
        { projectId: 'proj-1', worktreePath: '/repo', devServerId: 'ds-1' },
        makeCtx()
      )
    ).rejects.toThrow('GIT_AI_EMPTY_RESPONSE')
    stop()

    const failEvent = events.find(
      (e) => e.flow === 'codeReview:aiCommitMessage' && e.level === 'fail'
    )
    expect(failEvent?.fields.err).toContain('GIT_AI_EMPTY_RESPONSE')
  })

  // ── git.pr.create (remote) ─────────────────────────────────────────────────

  it('git.pr.create (remote) → success → ok() with prUrl and exitCode:0', async () => {
    relay = makeRelay({
      'shell.exec': { stdout: 'https://github.com/org/repo/pull/42\n', stderr: '', exitCode: 0 },
    })
    router = makeRouter(relay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), makeTaskService(), makeTaskGrantService())
    const { events, stop } = captureTraceEvents()

    const result = await findMethod(methods, 'git.pr.create').handler(
      { projectId: 'proj-1', worktreePath: '/repo', title: 'My PR', base: 'main' },
      makeCtx()
    )
    stop()

    expect(result).toMatchObject({ url: 'https://github.com/org/repo/pull/42', exitCode: 0 })
    const okEvent = events.find((e) => e.flow === 'codeReview:createPr' && e.level === 'ok')
    expect(okEvent?.fields).toMatchObject({
      mode: 'remote',
      prUrl: 'https://github.com/org/repo/pull/42',
      exitCode: 0
    })
  })

  it('git.pr.create (remote) → gh CLI fails (exitCode!=0) → fail() called (not just relying on exception)', async () => {
    relay = makeRelay({
      'shell.exec': { stdout: '', stderr: 'gh: not authenticated', exitCode: 1 },
    })
    router = makeRouter(relay)
    methods = registerRemoteGitRpcMethods(router, makeAIService(), makeTaskService(), makeTaskGrantService())
    const { events, stop } = captureTraceEvents()

    // shell.exec resolves normally (does not throw) even when exitCode != 0
    const result = await findMethod(methods, 'git.pr.create').handler(
      { projectId: 'proj-1', worktreePath: '/repo', title: 'My PR', base: 'main' },
      makeCtx()
    )
    stop()

    expect(result).toMatchObject({ exitCode: 1 })
    const failEvent = events.find((e) => e.flow === 'codeReview:createPr' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.exitCode).toBe(1)
  })

  // ── Regression: annotate/feedback tracers reserved but not wired (TASK-BE-005.1) ─

  it('Tracers.codeReviewAnnotate / codeReviewFeedback exist but have no call site in git-remote.ts', () => {
    expect(Tracers.codeReviewAnnotate).toBeDefined()
    expect(Tracers.codeReviewFeedback).toBeDefined()

    for (const name of methods.map((m) => m.name)) {
      const src = findMethod(methods, name).handler.toString()
      expect(src).not.toContain('codeReviewAnnotate')
      expect(src).not.toContain('codeReviewFeedback')
    }
  })
})
