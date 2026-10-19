// @vitest-environment happy-dom
import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { emitRequestEvent } from '../../../lib/request-event-bus'
import { REQUEST_POLL_EVENT_TYPE } from '../../../hooks/useRequestEvents'
import type { RequestRpcError } from '../../../../../shared/request-errors'
import type { OrcaRequest, Solution } from '../../../../../shared/request-types'
import { useAnalysisRevision, type UseAnalysisRevisionParams } from './useAnalysisRevision'

type Req = UseAnalysisRevisionParams['request']

const atGate: Req = {
  id: 'r1',
  type: 'bug',
  status: 'awaiting_analysis_approval',
  version: 7,
  updatedAt: '2026-10-19T10:00:00Z'
}
const solution: Pick<Solution, 'id' | 'status' | 'seq'> = { id: 's1', status: 'ready', seq: 2 }

const ok = (value: unknown = {}) => ({ ok: true as const, value })
function fail(over: Partial<RequestRpcError>) {
  return {
    ok: false as const,
    error: { kind: 'unknown', code: 'runtime_error', message: 'x', ...over } as RequestRpcError
  }
}

type Props = { request: Req; solution: UseAnalysisRevisionParams['solution'] }

function setup(
  generate = vi.fn().mockResolvedValue(ok()),
  initial: Props = { request: atGate, solution }
) {
  const reject = vi.fn()
  const onSettled = vi.fn()
  const actions = { generate, reject } as unknown as UseAnalysisRevisionParams['actions']
  const hook = renderHook((props: Props) => useAnalysisRevision({ ...props, actions, onSettled }), {
    initialProps: initial
  })
  return { generate, reject, onSettled, ...hook }
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('useAnalysisRevision submit', () => {
  it('calls generate once with the trimmed text and a key, and never rejects', async () => {
    const { result, generate, reject, onSettled } = setup()
    let outcome: Awaited<ReturnType<typeof result.current.submit>> | undefined
    await act(async () => {
      outcome = await result.current.submit('  Cover the rollback path  ')
    })
    expect(outcome).toEqual({ ok: true })
    expect(generate).toHaveBeenCalledTimes(1)
    expect(generate).toHaveBeenCalledWith({
      feedback: 'Cover the rollback path',
      idempotencyKey: expect.stringMatching(/^[0-9a-f-]{36}$/)
    })
    expect(reject).not.toHaveBeenCalled()
    expect(onSettled).toHaveBeenCalledTimes(1)
    expect(result.current.error).toBeNull()
  })

  it('submits the draft when called without text, and empties it on success', async () => {
    const { result, generate } = setup()
    act(() => result.current.setDraft('Add a cheaper option'))
    expect(result.current.draft).toBe('Add a cheaper option')
    await act(async () => {
      await result.current.submit()
    })
    expect(generate).toHaveBeenCalledWith(
      expect.objectContaining({ feedback: 'Add a cheaper option' })
    )
    expect(result.current.draft).toBe('')
  })

  it('refuses empty and whitespace-only text without calling the backend', async () => {
    const { result, generate } = setup()
    for (const text of ['', '   \n\t ']) {
      await act(async () => {
        const outcome = await result.current.submit(text)
        expect(outcome.ok).toBe(false)
      })
      expect(result.current.error?.kind).toBe('empty')
    }
    expect(generate).not.toHaveBeenCalled()
  })

  it('counts characters, not UTF-16 units: 2000 emoji pass, 2001 do not', async () => {
    const { result, generate } = setup()
    await act(async () => {
      await result.current.submit('😀'.repeat(2001))
    })
    expect(result.current.error?.kind).toBe('tooLong')
    expect(result.current.error?.message).toContain('2000')
    expect(generate).not.toHaveBeenCalled()

    await act(async () => {
      await result.current.submit('😀'.repeat(2000))
    })
    expect(generate).toHaveBeenCalledTimes(1)
  })

  it('keeps the typed text and shows a permission message on FORBIDDEN', async () => {
    const generate = vi
      .fn()
      .mockResolvedValue(fail({ kind: 'forbidden', backendCode: 'REQUEST_SOLUTION_FORBIDDEN' }))
    const { result, onSettled } = setup(generate)
    act(() => result.current.setDraft('please add rollback'))
    await act(async () => {
      await result.current.submit()
    })
    expect(result.current.error?.kind).toBe('forbidden')
    expect(result.current.error?.message).toMatch(/approvers/)
    expect(result.current.draft).toBe('please add rollback')
    expect(result.current.busy).toBe(false)
    expect(onSettled).not.toHaveBeenCalled()
  })

  it('refreshes and keeps the text on WRONG_STATE', async () => {
    const generate = vi.fn().mockResolvedValue(
      fail({
        kind: 'invalid_state',
        message: 'REQUEST_SOLUTION_WRONG_STATE: not awaiting approval'
      })
    )
    const { result, onSettled } = setup(generate)
    act(() => result.current.setDraft('keep me'))
    await act(async () => {
      await result.current.submit()
    })
    expect(result.current.error?.kind).toBe('wrongState')
    expect(result.current.error?.message).toMatch(/Someone else/)
    expect(onSettled).toHaveBeenCalledTimes(1)
    expect(result.current.draft).toBe('keep me')
    expect(result.current.busy).toBe(false)
  })

  it('treats a version conflict like WRONG_STATE', async () => {
    const generate = vi.fn().mockResolvedValue(fail({ kind: 'conflict' }))
    const { result, onSettled } = setup(generate)
    await act(async () => {
      await result.current.submit('x')
    })
    expect(result.current.error?.kind).toBe('wrongState')
    expect(onSettled).toHaveBeenCalled()
  })

  it.each(['REQUEST_ANALYSIS_NO_CONNECTION', 'REQUEST_SOLUTION_NO_CONNECTION'])(
    'says the current version is unchanged on %s and keeps the text',
    async (code) => {
      const generate = vi
        .fn()
        .mockResolvedValue(fail({ kind: 'no_dev_server', message: `${code}: no dev server` }))
      const { result } = setup(generate)
      act(() => result.current.setDraft('keep me'))
      await act(async () => {
        await result.current.submit()
      })
      expect(result.current.error?.kind).toBe('noConnection')
      expect(result.current.error?.message).toMatch(/current version is unchanged/)
      expect(result.current.draft).toBe('keep me')
      expect(result.current.busy).toBe(false)
    }
  )

  it('maps the server length limit', async () => {
    const generate = vi.fn().mockResolvedValue(
      fail({
        kind: 'validation',
        message: 'REQUEST_SOLUTION_FEEDBACK_TOO_LONG: feedback is too long'
      })
    )
    const { result } = setup(generate)
    await act(async () => {
      await result.current.submit('x')
    })
    expect(result.current.error?.kind).toBe('tooLong')
  })

  it.each([
    ['rate_limited', 'rateLimited'],
    ['network', 'network'],
    ['unavailable', 'other']
  ] as const)('maps a %s failure to %s with a sentence', async (kind, expected) => {
    const generate = vi.fn().mockResolvedValue(fail({ kind }))
    const { result } = setup(generate)
    act(() => result.current.setDraft('keep me'))
    await act(async () => {
      await result.current.submit()
    })
    expect(result.current.error?.kind).toBe(expected)
    expect(result.current.error?.message.length).toBeGreaterThan(0)
    expect(result.current.draft).toBe('keep me')
  })

  it('never rejects: a throwing action becomes an error and keeps the text', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const generate = vi.fn().mockRejectedValue(new Error('socket closed'))
    const { result } = setup(generate)
    act(() => result.current.setDraft('keep me'))
    await act(async () => {
      await expect(result.current.submit()).resolves.toMatchObject({ ok: false })
    })
    expect(result.current.error?.kind).toBe('other')
    expect(result.current.draft).toBe('keep me')
    expect(result.current.busy).toBe(false)
    expect(console.error).toHaveBeenCalled()
  })

  it('makes one call for a double click', async () => {
    let release!: (value: unknown) => void
    const generate = vi.fn().mockReturnValue(new Promise((resolve) => (release = resolve)))
    const { result } = setup(generate)
    let first!: Promise<unknown>
    let second!: Promise<unknown>
    await act(async () => {
      first = result.current.submit('change it')
      second = result.current.submit('change it')
    })
    // The button is locked as soon as the call is in flight.
    expect(result.current.busy).toBe(true)
    await act(async () => {
      release(ok())
      await Promise.all([first, second])
    })
    expect(generate).toHaveBeenCalledTimes(1)
  })

  it('sends a fresh key on every submit', async () => {
    const generate = vi
      .fn()
      .mockResolvedValueOnce(fail({ kind: 'network' }))
      .mockResolvedValueOnce(ok())
    const { result } = setup(generate)
    await act(async () => {
      await result.current.submit('again')
    })
    await act(async () => {
      await result.current.submit('again')
    })
    const keys = generate.mock.calls.map((call) => call[0].idempotencyKey)
    expect(new Set(keys).size).toBe(2)
  })

  it('still makes a key where crypto.randomUUID is unavailable (plain HTTP pages)', async () => {
    const original = Object.getOwnPropertyDescriptor(globalThis.crypto, 'randomUUID')
    Object.defineProperty(globalThis.crypto, 'randomUUID', { value: undefined, configurable: true })
    try {
      const { result, generate } = setup()
      await act(async () => {
        await result.current.submit('change it')
      })
      expect(generate.mock.calls[0][0].idempotencyKey).toMatch(/^[0-9a-f-]{36}$/)
    } finally {
      if (original) {
        Object.defineProperty(globalThis.crypto, 'randomUUID', original)
      } else {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        delete (globalThis.crypto as any).randomUUID
      }
    }
  })

  it('does not call the backend when the panel is not available', async () => {
    const { result, generate } = setup(undefined, {
      request: { ...atGate, status: 'analyzing' },
      solution
    })
    await act(async () => {
      const outcome = await result.current.submit('change it')
      expect(outcome.ok).toBe(false)
    })
    expect(generate).not.toHaveBeenCalled()
  })

  it('setDraft clears an earlier error', async () => {
    const { result } = setup()
    await act(async () => {
      await result.current.submit('')
    })
    expect(result.current.error).not.toBeNull()
    act(() => result.current.setDraft('now it has text'))
    expect(result.current.error).toBeNull()
  })

  it('does not carry a draft over to another request', () => {
    const { result, rerender } = setup()
    act(() => result.current.setDraft('for r1'))
    rerender({ request: { ...atGate, id: 'r2' }, solution })
    expect(result.current.draft).toBe('')
  })
})

describe('useAnalysisRevision canShow', () => {
  it('is true for a ready version at the gate of a flow that has one', () => {
    expect(setup().result.current.canShow).toBe(true)
  })

  it.each(['hotfix', 'task', 'docs', 'ops_request', 'unknown'] as OrcaRequest['type'][])(
    'is false for %s, which has no Analysis gate',
    (type) => {
      const { result } = setup(undefined, { request: { ...atGate, type }, solution })
      expect(result.current.canShow).toBe(false)
    }
  )

  it.each([
    'analyzing',
    'planning',
    'executing',
    'completed',
    'submitted'
  ] as OrcaRequest['status'][])('is false while the request is %s', (status) => {
    const { result } = setup(undefined, { request: { ...atGate, status }, solution })
    expect(result.current.canShow).toBe(false)
  })

  it.each(['superseded', 'rejected', 'chosen', 'generating'] as Solution['status'][])(
    'is false for a %s version (an older version cannot be revised)',
    (status) => {
      const { result } = setup(undefined, {
        request: atGate,
        solution: { ...solution, status }
      })
      expect(result.current.canShow).toBe(false)
    }
  )

  it('is false before any version exists', () => {
    expect(setup(undefined, { request: atGate, solution: undefined }).result.current.canShow).toBe(
      false
    )
  })
})

describe('useAnalysisRevision busy', () => {
  async function submitted() {
    const view = setup()
    await act(async () => {
      await view.result.current.submit('change it')
    })
    return view
  }

  it('lasts past the call, while the Request has not left the gate on screen', async () => {
    const { result } = await submitted()
    expect(result.current.busy).toBe(true)
    expect(result.current.nextVersion).toBe(3)
  })

  it('keeps going while the Request is analyzing', async () => {
    const { result, rerender } = await submitted()
    rerender({ request: { ...atGate, status: 'analyzing', version: 8 }, solution })
    expect(result.current.busy).toBe(true)
    expect(result.current.canShow).toBe(false)
  })

  it('ends when the Request is back at the gate with the next version', async () => {
    const { result, rerender } = await submitted()
    rerender({ request: { ...atGate, status: 'analyzing', version: 8 }, solution })
    rerender({
      request: { ...atGate, version: 9, updatedAt: '2026-10-19T10:05:00Z' },
      solution: { id: 's2', status: 'ready', seq: 3 }
    })
    expect(result.current.busy).toBe(false)
    expect(result.current.nextVersion).toBeUndefined()
    expect(result.current.canShow).toBe(true)
  })

  it('ends when the round came and went without the screen ever seeing analyzing', async () => {
    const { result, rerender } = await submitted()
    rerender({
      request: { ...atGate, version: 9, updatedAt: '2026-10-19T10:05:00Z' },
      solution: { id: 's2', status: 'ready', seq: 3 }
    })
    expect(result.current.busy).toBe(false)
  })

  it('ends when only the current version changed (no version or timestamp on an old gateway)', async () => {
    const view = setup(undefined, {
      request: { id: 'r1', type: 'bug', status: 'awaiting_analysis_approval' },
      solution
    })
    await act(async () => {
      await view.result.current.submit('change it')
    })
    expect(view.result.current.busy).toBe(true)
    view.rerender({
      request: { id: 'r1', type: 'bug', status: 'awaiting_analysis_approval' },
      solution: { id: 's2', status: 'ready', seq: 3 }
    })
    expect(view.result.current.busy).toBe(false)
  })

  it('ends when the Request moves on to something else', async () => {
    const { result, rerender } = await submitted()
    rerender({ request: { ...atGate, status: 'cancelled' }, solution })
    expect(result.current.busy).toBe(false)
  })

  it('ends for the next request, not only for the one that was revised', async () => {
    const { result, rerender } = await submitted()
    rerender({ request: { ...atGate, id: 'r2' }, solution })
    expect(result.current.busy).toBe(false)
  })

  it('asks for a refresh on progress events while busy, and not otherwise', async () => {
    const idle = setup()
    emitRequestEvent({
      requestId: 'r1',
      eventType: 'orca.request.request.status_changed',
      occurredAt: ''
    })
    expect(idle.onSettled).not.toHaveBeenCalled()
    idle.unmount()

    const { onSettled } = await submitted()
    onSettled.mockClear()
    emitRequestEvent({
      requestId: 'r1',
      eventType: 'orca.request.request.status_changed',
      occurredAt: ''
    })
    emitRequestEvent({
      requestId: 'r1',
      eventType: 'orca.request.solution.proposed',
      occurredAt: ''
    })
    emitRequestEvent({ requestId: '', eventType: REQUEST_POLL_EVENT_TYPE, occurredAt: '' })
    expect(onSettled).toHaveBeenCalledTimes(3)

    emitRequestEvent({
      requestId: 'other',
      eventType: 'orca.request.request.status_changed',
      occurredAt: ''
    })
    emitRequestEvent({
      requestId: 'r1',
      eventType: 'orca.request.request.commented',
      occurredAt: ''
    })
    expect(onSettled).toHaveBeenCalledTimes(3)
  })

  it('refuses a second revision while the first is being written', async () => {
    const { result, generate } = await submitted()
    await act(async () => {
      const outcome = await result.current.submit('another change')
      expect(outcome.ok).toBe(false)
    })
    expect(generate).toHaveBeenCalledTimes(1)
  })
})
