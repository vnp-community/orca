// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const toast = vi.hoisted(() => Object.assign(vi.fn(), { error: vi.fn(), success: vi.fn() }))
vi.mock('sonner', () => ({ toast }))
const rpc = vi.hoisted(() => vi.fn())
vi.mock('../../../../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => rpc(...a)
}))

import { RegeneratePanel } from './RegeneratePanel'

const estimate = (over: Record<string, unknown> = {}) => ({
  ok: true,
  value: {
    phase: 'delivered',
    currentInputsDigest: 'd',
    regeneration: {
      agentRuns: 1,
      diagramsToDraw: 2,
      costKnown: false,
      confirmRequired: false,
      drawingsDependOnResult: false,
      ...over
    }
  }
})

function script(
  est = estimate(),
  regen: unknown = {
    ok: true,
    value: { drawRun: { runId: 'u1', status: 'running' }, replayed: false }
  }
): void {
  rpc.mockImplementation(async (method: string) =>
    method === 'request.review.estimate'
      ? est
      : method === 'request.review.regenerate'
        ? regen
        : { ok: false, error: { kind: 'not_found' } }
  )
}

const onClose = vi.fn()
const onStarted = vi.fn()
const show = (preset = { sections: ['flow'] as string[] }) =>
  render(
    <RegeneratePanel
      requestId="r1"
      reviewVersion={7}
      preset={preset}
      onClose={onClose}
      onStarted={onStarted}
    />
  )

beforeEach(() => {
  rpc.mockReset()
  onClose.mockReset()
  onStarted.mockReset()
  toast.success.mockReset()
  script()
})
afterEach(cleanup)

describe('RegeneratePanel', () => {
  it('shows what the regeneration would do before anything starts, and starts nothing by asking', async () => {
    show()
    const box = await screen.findByTestId('regenerate-estimate')
    await waitFor(() => expect(box).toHaveTextContent('1 agent run(s), 2 diagram(s) to draw'))
    expect(box).toHaveTextContent('No cost figures have been measured yet')
    expect(rpc.mock.calls.map((c) => c[0])).toEqual(['request.review.estimate'])
    expect(rpc.mock.calls[0][1]).toEqual({ id: 'r1', scope: 'artifact', sections: ['flow'] })
  })

  it('asks again when the scope changes, and a context-only run names no sections', async () => {
    show()
    await screen.findByTestId('regenerate-estimate')
    fireEvent.change(screen.getByLabelText('What to do'), { target: { value: 'context' } })
    await waitFor(() =>
      expect(rpc.mock.calls.filter((c) => c[0] === 'request.review.estimate')).toHaveLength(2)
    )
    expect(rpc.mock.calls[1][1]).toEqual({ id: 'r1', scope: 'context', sections: [] })
  })

  it('a reason is required, and the button waits for one', async () => {
    show()
    await screen.findByTestId('regenerate-estimate')
    const submit = screen.getByTestId('regenerate-submit')
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Why'), { target: { value: 'looks_wrong' } })
    expect(submit).toBeEnabled()
  })

  it('a reason can come with the preset (retrying a failed drawing)', async () => {
    show({ sections: ['data'], reason: 'retry_failed' } as never)
    await screen.findByTestId('regenerate-estimate')
    expect(screen.getByLabelText('Why')).toHaveValue('retry_failed')
    expect(screen.getByTestId('regenerate-submit')).toBeEnabled()
  })

  it('sends the version of the review it was looking at, one key, and the note trimmed', async () => {
    show()
    await screen.findByTestId('regenerate-estimate')
    fireEvent.change(screen.getByLabelText('Why'), { target: { value: 'other' } })
    fireEvent.change(screen.getByLabelText('Note (optional)'), {
      target: { value: '  the arrow is backwards  ' }
    })
    fireEvent.click(screen.getByTestId('regenerate-submit'))
    await waitFor(() => expect(onStarted).toHaveBeenCalled())
    const call = rpc.mock.calls.find((c) => c[0] === 'request.review.regenerate')!
    expect(call[1]).toMatchObject({
      id: 'r1',
      scope: 'artifact',
      sections: ['flow'],
      reason: 'other',
      note: 'the arrow is backwards',
      expectedVersion: 7
    })
    expect(call[1].clientRequestId).toBeTruthy()
    expect(call[2]).toEqual({ idempotent: false })
    expect(call[1].confirmCost).toBeUndefined()
    expect(onClose).toHaveBeenCalled()
    expect(toast.success).toHaveBeenCalledWith('Regeneration started')
  })

  it('a cost over the limit needs a second yes before the button works', async () => {
    script(estimate({ confirmRequired: true, costKnown: true, costUsd: 3.5 }))
    show()
    await waitFor(() =>
      expect(screen.getByTestId('regenerate-estimate')).toHaveTextContent('About 3.50 USD')
    )
    fireEvent.change(screen.getByLabelText('Why'), { target: { value: 'looks_wrong' } })
    expect(screen.getByTestId('regenerate-submit')).toBeDisabled()
    fireEvent.click(screen.getByRole('checkbox'))
    expect(screen.getByTestId('regenerate-submit')).toBeEnabled()
    fireEvent.click(screen.getByTestId('regenerate-submit'))
    await waitFor(() => expect(onStarted).toHaveBeenCalled())
    expect(rpc.mock.calls.find((c) => c[0] === 'request.review.regenerate')![1].confirmCost).toBe(
      true
    )
  })

  it('says why in words when the server refuses, and stays open', async () => {
    script(estimate(), {
      ok: false,
      error: { kind: 'conflict', backendCode: 'REQUEST_REVIEW_RUN_IN_PROGRESS', message: 'RAW' }
    })
    show()
    await screen.findByTestId('regenerate-estimate')
    fireEvent.change(screen.getByLabelText('Why'), { target: { value: 'looks_wrong' } })
    fireEvent.click(screen.getByTestId('regenerate-submit'))
    const alert = await screen.findByTestId('regenerate-failure')
    expect(alert).toHaveTextContent('A review run is already going. Wait for it to finish.')
    expect(alert).not.toHaveTextContent('RAW')
    expect(onClose).not.toHaveBeenCalled()
    expect(onStarted).not.toHaveBeenCalled()
  })

  it('a stale version and a spent budget each get their sentence', async () => {
    for (const [code, text] of [
      ['REQUEST_REVIEW_STALE_REVISION', 'Someone changed this review. It was reloaded; try again.'],
      ['REQUEST_REVIEW_BUDGET_EXCEEDED', 'The AI budget for reviews is used up.']
    ] as const) {
      cleanup()
      script(estimate(), {
        ok: false,
        error: { kind: 'conflict', backendCode: code, message: 'x' }
      })
      show()
      await screen.findByTestId('regenerate-estimate')
      fireEvent.change(screen.getByLabelText('Why'), { target: { value: 'looks_wrong' } })
      fireEvent.click(screen.getByTestId('regenerate-submit'))
      expect(await screen.findByTestId('regenerate-failure')).toHaveTextContent(text)
    }
  })

  it('an estimate that fails is explained and does not stop the person from deciding', async () => {
    script({ ok: false, error: { kind: 'network', message: 'x' } } as never)
    show()
    expect(
      await screen.findByText('The review could not be loaded. Try again.')
    ).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Why'), { target: { value: 'looks_wrong' } })
    expect(screen.getByTestId('regenerate-submit')).toBeEnabled()
  })
})
