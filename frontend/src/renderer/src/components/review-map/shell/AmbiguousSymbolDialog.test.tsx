// @vitest-environment happy-dom
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import {
  AmbiguousSymbolDialog,
  toSymbolChoice,
  type AmbiguousSymbolCandidate
} from './AmbiguousSymbolDialog'

beforeAll(() => {
  // cmdk measures with ResizeObserver / scrollIntoView, absent in happy-dom.
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as never
  Element.prototype.scrollIntoView ??= () => undefined
})
afterEach(cleanup)

const cands: AmbiguousSymbolCandidate[] = [
  { key: 'k1', uid: 'u1', name: 'Foo', kind: 'function', filePath: 'a.ts', line: 3 },
  {
    uid: 'u2',
    name: 'Foo',
    kind: 'method',
    filePath: 'b.ts',
    line: 9,
    impactedCount: 4,
    risk: 'HIGH'
  }
]

describe('toSymbolChoice', () => {
  it('uses the key when present, else name + file', () => {
    expect(toSymbolChoice(cands[0])).toEqual({ key: 'k1' })
    expect(toSymbolChoice(cands[1])).toEqual({ name: 'Foo', file: 'b.ts' })
  })
})

describe('AmbiguousSymbolDialog', () => {
  it('lists candidates with file:line and the search input is focused-ready', () => {
    render(<AmbiguousSymbolDialog candidates={cands} onChoose={vi.fn()} onCancel={vi.fn()} />)
    expect(screen.getByPlaceholderText('Search matches')).toBeTruthy()
    expect(screen.getByText(/a\.ts:3/)).toBeTruthy()
    expect(screen.getByText(/b\.ts:9/)).toBeTruthy()
  })
  it('choosing a row reports the stable choice', () => {
    const onChoose = vi.fn()
    render(<AmbiguousSymbolDialog candidates={cands} onChoose={onChoose} onCancel={vi.fn()} />)
    fireEvent.click(screen.getByText(/b\.ts:9/))
    expect(onChoose).toHaveBeenCalledWith({ name: 'Foo', file: 'b.ts' })
  })
  it('Escape cancels', () => {
    const onCancel = vi.fn()
    render(<AmbiguousSymbolDialog candidates={cands} onChoose={vi.fn()} onCancel={onCancel} />)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onCancel).toHaveBeenCalled()
  })
})
