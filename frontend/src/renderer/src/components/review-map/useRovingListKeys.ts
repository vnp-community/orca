/**
 * useRovingListKeys.ts — FE-CV-TASK-052-04
 *
 * Keyboard model for a role="listbox" with an active descendant: j/k/arrows, Home/End,
 * Enter (activate), Space (toggle), Left/Right on group rows (collapse/expand).
 * Reused by the data-flow step list (SOL-056). No platform-dependent chords.
 */

import { useCallback, type KeyboardEvent } from 'react'
import { isEditableTarget } from '@/lib/editable-target'

export type RovingListKeysOptions = {
  count: number
  activeIndex: number
  onActiveChange: (index: number) => void
  onActivate?: (index: number) => void
  onToggle?: (index: number) => void
  isGroupRow?: (index: number) => boolean
  /** collapse=true on ArrowLeft, false on ArrowRight; only called for group rows. */
  onCollapse?: (index: number, collapse: boolean) => void
}

export function useRovingListKeys(opts: RovingListKeysOptions): {
  onKeyDown: (e: KeyboardEvent) => void
} {
  const { count, activeIndex, onActiveChange, onActivate, onToggle, isGroupRow, onCollapse } = opts

  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) {
        return
      }
      if (isEditableTarget(e.target)) {
        return
      }
      if (count === 0) {
        return
      }
      const move = (next: number): void => {
        e.preventDefault()
        onActiveChange(Math.min(count - 1, Math.max(0, next)))
      }
      switch (e.key) {
        case 'j':
        case 'ArrowDown':
          return move(activeIndex + 1)
        case 'k':
        case 'ArrowUp':
          return move(activeIndex - 1)
        case 'Home':
          return move(0)
        case 'End':
          return move(count - 1)
        case 'Enter':
          if (onActivate) {
            e.preventDefault()
            onActivate(activeIndex)
          }
          return
        case ' ':
          // Always swallow Space so the pane does not scroll.
          e.preventDefault()
          onToggle?.(activeIndex)
          return
        case 'ArrowLeft':
        case 'ArrowRight':
          if (isGroupRow?.(activeIndex) && onCollapse) {
            e.preventDefault()
            onCollapse(activeIndex, e.key === 'ArrowLeft')
          }
          break
        default:
      }
    },
    [count, activeIndex, onActiveChange, onActivate, onToggle, isGroupRow, onCollapse]
  )

  return { onKeyDown }
}
