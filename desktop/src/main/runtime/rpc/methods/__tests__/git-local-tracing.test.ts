/**
 * Tests for LOCAL git RPC handler tracing (TASK-BE-005.2/005.4).
 *
 * Covers `git.diff`, `git.generateCommitMessage`, `git.generatePullRequestFields`
 * in `src/main/runtime/rpc/methods/git.ts` — instrumented with
 * `Tracers.codeReviewDiff` / `Tracers.codeReviewAiCommit` / `Tracers.codeReviewCreatePr`.
 *
 * @module main/runtime/rpc/methods/__tests__/git-local-tracing.test
 */

import { describe, it, expect, vi } from 'vitest'
import { GIT_METHODS } from '../git'
import type { RpcContext, RpcMethod } from '../../core'
import type { OrcaRuntimeService } from '../../../orca-runtime'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'
import { Tracers } from '../../../../../shared/trace/tracers'

function findMethod(name: string): RpcMethod {
  const m = GIT_METHODS.find((m) => m.name === name)
  if (!m) {throw new Error(`Method ${name} not found`)}
  return m
}

function makeCtx(runtime: Partial<OrcaRuntimeService>): RpcContext {
  return { runtime: runtime as OrcaRuntimeService } as RpcContext
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

describe('git.ts LOCAL RPC handler tracing', () => {
  // ── git.diff (local) ──────────────────────────────────────────────────────

  it('git.diff (local) → start()/ok() with mode:"local"', async () => {
    const { events, stop } = captureTraceEvents()
    const runtime = {
      getRuntimeGitDiff: vi.fn().mockResolvedValue({ diff: 'patch text' })
    }

    const result = await findMethod('git.diff').handler(
      { worktree: 'id:wt-1', staged: true },
      makeCtx(runtime)
    )
    stop()

    expect(result).toMatchObject({ diff: 'patch text' })
    expect(runtime.getRuntimeGitDiff).toHaveBeenCalledWith('id:wt-1', undefined, true, undefined)

    const spanEvents = events.filter((e) => e.flow === 'codeReview:diff')
    expect(spanEvents.some((e) => e.level === 'start' && e.fields.mode === 'local')).toBe(true)
    expect(spanEvents.some((e) => e.level === 'ok' && e.fields.mode === 'local')).toBe(true)
  })

  it('git.diff (local) → runtime throws → span.fail(err, {mode:"local"}) before re-throw', async () => {
    const { events, stop } = captureTraceEvents()
    const runtime = {
      getRuntimeGitDiff: vi.fn().mockRejectedValue(new Error('git diff exec failed'))
    }

    await expect(
      findMethod('git.diff').handler({ worktree: 'id:wt-1' }, makeCtx(runtime))
    ).rejects.toThrow('git diff exec failed')
    stop()

    const failEvent = events.find((e) => e.flow === 'codeReview:diff' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.mode).toBe('local')
    expect(failEvent?.fields.err).toContain('git diff exec failed')
  })

  // ── git.generateCommitMessage (local) ─────────────────────────────────────

  it('git.generateCommitMessage (local) → success:true → ok() with messageChars', async () => {
    const { events, stop } = captureTraceEvents()
    const runtime = {
      generateRuntimeCommitMessage: vi.fn().mockResolvedValue({
        success: true,
        message: 'feat: add tracing'
      })
    }

    const result = await findMethod('git.generateCommitMessage').handler(
      { worktree: 'id:wt-1' },
      makeCtx(runtime)
    )
    stop()

    expect(result).toMatchObject({ success: true, message: 'feat: add tracing' })
    const okEvent = events.find((e) => e.flow === 'codeReview:aiCommitMessage' && e.level === 'ok')
    expect(okEvent).toBeDefined()
    expect(okEvent?.fields).toMatchObject({ mode: 'local', messageChars: 'feat: add tracing'.length })
  })

  it('git.generateCommitMessage (local) → success:false → fail() called, handler does NOT throw', async () => {
    const { events, stop } = captureTraceEvents()
    const runtime = {
      generateRuntimeCommitMessage: vi.fn().mockResolvedValue({
        success: false,
        error: 'GIT_NO_STAGED_CHANGES'
      })
    }

    const result = await findMethod('git.generateCommitMessage').handler(
      { worktree: 'id:wt-1' },
      makeCtx(runtime)
    )
    stop()

    expect(result).toMatchObject({ success: false, error: 'GIT_NO_STAGED_CHANGES' })
    const failEvent = events.find(
      (e) => e.flow === 'codeReview:aiCommitMessage' && e.level === 'fail'
    )
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.mode).toBe('local')
  })

  it('git.generateCommitMessage (local) → has a diffStaged step before the runtime call', async () => {
    const { events, stop } = captureTraceEvents()
    const runtime = {
      generateRuntimeCommitMessage: vi.fn().mockResolvedValue({ success: true, message: 'chore: x' })
    }

    await findMethod('git.generateCommitMessage').handler({ worktree: 'id:wt-1' }, makeCtx(runtime))
    stop()

    const stepEvent = events.find(
      (e) => e.flow === 'codeReview:aiCommitMessage' && e.level === 'step' && e.label === 'diffStaged'
    )
    expect(stepEvent).toBeDefined()
  })

  // ── git.generatePullRequestFields (local) ─────────────────────────────────

  it('git.generatePullRequestFields (local) → start()/ok() with mode:"local" and an aiGenerate step', async () => {
    const { events, stop } = captureTraceEvents()
    const runtime = {
      generateRuntimePullRequestFields: vi.fn().mockResolvedValue({
        success: true,
        fields: { title: 'PR title', body: 'PR body' }
      })
    }

    const result = await findMethod('git.generatePullRequestFields').handler(
      { worktree: 'id:wt-1', base: 'main' },
      makeCtx(runtime)
    )
    stop()

    expect(result).toMatchObject({ success: true })
    const flowEvents = events.filter((e) => e.flow === 'codeReview:createPr')
    expect(flowEvents.some((e) => e.level === 'step' && e.label === 'aiGenerate')).toBe(true)
    expect(flowEvents.some((e) => e.level === 'ok' && e.fields.mode === 'local')).toBe(true)
  })

  it('git.generatePullRequestFields (local) → runtime throws → span.fail() before re-throw', async () => {
    const { events, stop } = captureTraceEvents()
    const runtime = {
      generateRuntimePullRequestFields: vi.fn().mockRejectedValue(new Error('AI generation failed'))
    }

    await expect(
      findMethod('git.generatePullRequestFields').handler(
        { worktree: 'id:wt-1', base: 'main' },
        makeCtx(runtime)
      )
    ).rejects.toThrow('AI generation failed')
    stop()

    const failEvent = events.find((e) => e.flow === 'codeReview:createPr' && e.level === 'fail')
    expect(failEvent?.fields.mode).toBe('local')
  })

  // ── Regression: LOCAL git.ts must not call relay.call() anywhere ──────────

  it('git.ts LOCAL handlers never reference relay.call (backend-only boundary)', () => {
    // Static/behavioral guard, complementing manual review per SOL-BE-TRACE-005 §1.1:
    // the patched handlers only ever destructure { runtime } from ctx, never a relay.
    for (const name of ['git.diff', 'git.generateCommitMessage', 'git.generatePullRequestFields']) {
      const src = findMethod(name).handler.toString()
      expect(src).not.toContain('relay.call')
    }
  })

  // ── Regression: annotate/feedback tracers reserved but not wired (TASK-BE-005.1) ─

  it('Tracers.codeReviewAnnotate / codeReviewFeedback exist but have no call site in git.ts', () => {
    expect(Tracers.codeReviewAnnotate).toBeDefined()
    expect(Tracers.codeReviewFeedback).toBeDefined()

    for (const name of GIT_METHODS.map((m) => m.name)) {
      const src = findMethod(name).handler.toString()
      expect(src).not.toContain('codeReviewAnnotate')
      expect(src).not.toContain('codeReviewFeedback')
    }
  })
})
