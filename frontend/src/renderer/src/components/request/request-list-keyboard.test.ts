// @vitest-environment happy-dom
/**
 * Tests for request-list-keyboard.ts (CR-REQ-019-02)
 */

import { describe, it, expect, vi } from 'vitest'
import { handleRequestListKey } from './request-list-keyboard'

function makeEvent(key: string, opts: { ctrlKey?: boolean; metaKey?: boolean; altKey?: boolean; isComposing?: boolean; target?: HTMLElement } = {}): KeyboardEvent & { isComposing: boolean; preventDefault: ReturnType<typeof vi.fn> } {
  const ev = {
    key,
    ctrlKey: opts.ctrlKey ?? false,
    metaKey: opts.metaKey ?? false,
    altKey: opts.altKey ?? false,
    isComposing: opts.isComposing ?? false,
    target: opts.target ?? null,
    preventDefault: vi.fn()
  }
  return ev as unknown as KeyboardEvent & { isComposing: boolean; preventDefault: ReturnType<typeof vi.fn> }
}

const IDS = ['r1', 'r2', 'r3']

describe('handleRequestListKey — movement', () => {
  it('ArrowDown moves to next item', () => {
    const onMove = vi.fn()
    const ev = makeEvent('ArrowDown')
    handleRequestListKey(ev, { ids: IDS, activeId: 'r1', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).toHaveBeenCalledWith('r2')
    expect(ev.preventDefault).toHaveBeenCalled()
  })

  it('j moves to next item', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('j'), { ids: IDS, activeId: 'r2', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).toHaveBeenCalledWith('r3')
  })

  it('ArrowUp moves to previous item', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('ArrowUp'), { ids: IDS, activeId: 'r2', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).toHaveBeenCalledWith('r1')
  })

  it('k moves to previous item', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('k'), { ids: IDS, activeId: 'r3', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).toHaveBeenCalledWith('r2')
  })

  it('ArrowDown at last item does not move', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('ArrowDown'), { ids: IDS, activeId: 'r3', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).not.toHaveBeenCalled()
  })

  it('ArrowUp at first item does not move', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('ArrowUp'), { ids: IDS, activeId: 'r1', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).not.toHaveBeenCalled()
  })
})

describe('handleRequestListKey — Enter / Escape', () => {
  it('Enter calls onOpen with activeId', () => {
    const onOpen = vi.fn()
    handleRequestListKey(makeEvent('Enter'), { ids: IDS, activeId: 'r2', onMove: vi.fn(), onOpen, onClose: vi.fn() })
    expect(onOpen).toHaveBeenCalledWith('r2')
  })

  it('Enter with no activeId does nothing', () => {
    const onOpen = vi.fn()
    handleRequestListKey(makeEvent('Enter'), { ids: IDS, activeId: null, onMove: vi.fn(), onOpen, onClose: vi.fn() })
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('Escape calls onClose', () => {
    const onClose = vi.fn()
    handleRequestListKey(makeEvent('Escape'), { ids: IDS, activeId: 'r1', onMove: vi.fn(), onOpen: vi.fn(), onClose })
    expect(onClose).toHaveBeenCalled()
  })
})

describe('handleRequestListKey — guards', () => {
  it('ignores when target is input element', () => {
    const input = document.createElement('input')
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('j', { target: input }), { ids: IDS, activeId: 'r1', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).not.toHaveBeenCalled()
  })

  it('ignores when target is textarea', () => {
    const textarea = document.createElement('textarea')
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('j', { target: textarea }), { ids: IDS, activeId: 'r1', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).not.toHaveBeenCalled()
  })

  it('ignores when isComposing is true', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('j', { isComposing: true }), { ids: IDS, activeId: 'r1', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).not.toHaveBeenCalled()
  })

  it('ignores when ctrlKey is held', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('j', { ctrlKey: true }), { ids: IDS, activeId: 'r1', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).not.toHaveBeenCalled()
  })

  it('ignores when metaKey is held', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('j', { metaKey: true }), { ids: IDS, activeId: 'r1', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).not.toHaveBeenCalled()
  })

  it('ignores when altKey is held', () => {
    const onMove = vi.fn()
    handleRequestListKey(makeEvent('ArrowDown', { altKey: true }), { ids: IDS, activeId: 'r1', onMove, onOpen: vi.fn(), onClose: vi.fn() })
    expect(onMove).not.toHaveBeenCalled()
  })
})
