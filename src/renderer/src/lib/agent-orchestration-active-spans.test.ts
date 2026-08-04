import { describe, expect, it } from 'vitest'
import { createTracer } from '../../../shared/trace'
import {
  peekOpenAgentOrchSpan,
  registerOpenAgentOrchSpan,
  takeOpenAgentOrchSpan
} from './agent-orchestration-active-spans'

const testTracer = createTracer('test:agentOrchActiveSpans')

describe('agent-orchestration-active-spans', () => {
  it('registers a span and takes it back exactly once', () => {
    const span = testTracer.start()
    registerOpenAgentOrchSpan('wt-1', span)

    expect(takeOpenAgentOrchSpan('wt-1')).toBe(span)
    expect(takeOpenAgentOrchSpan('wt-1')).toBeUndefined()
  })

  it('peeks a span without removing it from the registry', () => {
    const span = testTracer.start()
    registerOpenAgentOrchSpan('wt-2', span)

    expect(peekOpenAgentOrchSpan('wt-2')).toBe(span)
    // Still there after peeking.
    expect(peekOpenAgentOrchSpan('wt-2')).toBe(span)
    expect(takeOpenAgentOrchSpan('wt-2')).toBe(span)
  })

  it('keeps spans for different worktree ids independent', () => {
    const spanA = testTracer.start()
    const spanB = testTracer.start()
    registerOpenAgentOrchSpan('wt-a', spanA)
    registerOpenAgentOrchSpan('wt-b', spanB)

    expect(peekOpenAgentOrchSpan('wt-a')).toBe(spanA)
    expect(peekOpenAgentOrchSpan('wt-b')).toBe(spanB)

    expect(takeOpenAgentOrchSpan('wt-a')).toBe(spanA)
    // Taking wt-a must not affect wt-b's still-open span.
    expect(peekOpenAgentOrchSpan('wt-b')).toBe(spanB)
  })

  it('returns undefined for a worktree id that was never registered', () => {
    expect(peekOpenAgentOrchSpan('never-registered')).toBeUndefined()
    expect(takeOpenAgentOrchSpan('never-registered')).toBeUndefined()
  })
})
