// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/react'
import { useRovingListKeys, type RovingListKeysOptions } from './useRovingListKeys'

afterEach(cleanup)

function Harness(props: RovingListKeysOptions): React.JSX.Element {
  const { onKeyDown } = useRovingListKeys(props)
  return (
    <div onKeyDown={onKeyDown}>
      <ul tabIndex={0} data-testid="list" />
      <input data-testid="input" />
    </div>
  )
}

function setup(over: Partial<RovingListKeysOptions> = {}) {
  const spies = {
    onActiveChange: vi.fn(),
    onActivate: vi.fn(),
    onToggle: vi.fn(),
    isGroupRow: vi.fn(() => false),
    onCollapse: vi.fn()
  }
  const o = { count: 5, activeIndex: 2, ...spies, ...over }
  cleanup()
  const utils = render(<Harness {...o} />)
  return {
    o: { ...o, ...spies },
    list: utils.getByTestId('list'),
    input: utils.getByTestId('input')
  }
}

describe('useRovingListKeys', () => {
  it('j/k and arrows move by one and clamp at the ends', () => {
    const { o, list } = setup()
    fireEvent.keyDown(list, { key: 'j' })
    fireEvent.keyDown(list, { key: 'ArrowDown' })
    fireEvent.keyDown(list, { key: 'k' })
    fireEvent.keyDown(list, { key: 'ArrowUp' })
    expect(o.onActiveChange.mock.calls.map((c) => c[0])).toEqual([3, 3, 1, 1])
    const edge = setup({ activeIndex: 4 })
    fireEvent.keyDown(edge.list, { key: 'j' })
    expect(edge.o.onActiveChange).toHaveBeenCalledWith(4)
  })
  it('Home and End jump', () => {
    const { o, list } = setup()
    fireEvent.keyDown(list, { key: 'Home' })
    fireEvent.keyDown(list, { key: 'End' })
    expect(o.onActiveChange.mock.calls.map((c) => c[0])).toEqual([0, 4])
  })
  it('Enter activates, Space toggles and does not scroll', () => {
    const { o, list } = setup()
    fireEvent.keyDown(list, { key: 'Enter' })
    const notPrevented = fireEvent.keyDown(list, { key: ' ' })
    expect(o.onActivate).toHaveBeenCalledWith(2)
    expect(o.onToggle).toHaveBeenCalledWith(2)
    expect(notPrevented).toBe(false)
  })
  it('ignores keys typed in an input', () => {
    const { o, input } = setup()
    fireEvent.keyDown(input, { key: 'j' })
    fireEvent.keyDown(input, { key: ' ' })
    expect(o.onActiveChange).not.toHaveBeenCalled()
    expect(o.onToggle).not.toHaveBeenCalled()
  })
  it('ignores chords with modifiers', () => {
    const { o, list } = setup()
    fireEvent.keyDown(list, { key: 'j', ctrlKey: true })
    expect(o.onActiveChange).not.toHaveBeenCalled()
  })
  it('Left/Right collapse/expand only on group rows', () => {
    const plain = setup()
    fireEvent.keyDown(plain.list, { key: 'ArrowLeft' })
    expect(plain.o.onCollapse).not.toHaveBeenCalled()
    const grp = setup({ isGroupRow: () => true })
    fireEvent.keyDown(grp.list, { key: 'ArrowLeft' })
    fireEvent.keyDown(grp.list, { key: 'ArrowRight' })
    expect(grp.o.onCollapse.mock.calls).toEqual([
      [2, true],
      [2, false]
    ])
  })
  it('does nothing for an empty list', () => {
    const { o, list } = setup({ count: 0, activeIndex: 0 })
    fireEvent.keyDown(list, { key: 'j' })
    expect(o.onActiveChange).not.toHaveBeenCalled()
  })
})
