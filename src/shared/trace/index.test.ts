import { describe, expect, it } from 'vitest'
import { createTracer, registerTraceSink, type TraceEvent } from './index'

describe('Tracer.start resume option', () => {
  it('generates a random id when resume is omitted', () => {
    const tracer = createTracer('test:omittedResume')
    const spanA = tracer.start({})
    const spanB = tracer.start({})

    expect(spanA.id).toBeTruthy()
    expect(spanB.id).toBeTruthy()
    expect(spanA.id).not.toBe(spanB.id)
  })

  it('uses resume.id exactly when provided', () => {
    const tracer = createTracer('test:withResume')
    const span = tracer.start({ foo: 'bar' }, { id: 'abc123' })

    expect(span.id).toBe('abc123')
  })

  it('computes elapsedMs from this layer\'s own startMs, not inherited from the resumed id', async () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))

    try {
      const upstreamTracer = createTracer('test:upstream')
      const upstreamSpan = upstreamTracer.start({})

      // Simulate time passing in the upstream layer before the id crosses the boundary.
      await new Promise((resolve) => setTimeout(resolve, 30))
      upstreamSpan.ok({})

      const downstreamTracer = createTracer('test:downstream')
      const downstreamSpan = downstreamTracer.start({}, { id: upstreamSpan.id })
      downstreamSpan.ok({})

      const downstreamOkEvent = events.find(
        (e) => e.flow === 'test:downstream' && e.level === 'ok'
      )

      expect(downstreamSpan.id).toBe(upstreamSpan.id)
      expect(downstreamOkEvent?.elapsedMs).toBeDefined()
      // Downstream span started fresh — its own elapsed time must be small,
      // not include the ~30ms already spent in the upstream layer.
      expect(downstreamOkEvent!.elapsedMs!).toBeLessThan(30)
    } finally {
      unregister()
    }
  })

  it('existing call sites that omit resume are unaffected (backward compatible)', () => {
    const tracer = createTracer('test:legacyCallSite')
    const events: TraceEvent[] = []
    const unregister = registerTraceSink((e) => events.push(e))

    const span = tracer.start({ devServerId: 'dev-01' })
    span.step('relay', { session: 'connected' })
    span.ok({ entries: 12 })

    unregister()
    expect(events.map((e) => e.level)).toEqual(['start', 'step', 'ok'])
    expect(events.every((e) => e.id === span.id)).toBe(true)
  })
})
