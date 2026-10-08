/**
 * useRowListKeyboardNavigation — CR-REQ-022-04
 *
 * j/k (and arrows) move a roving selection, Enter opens. Uses no modifier keys,
 * so there is no Mac/Windows branch. Shared by the approval inbox and backlog tables.
 *
 * @module hooks/useRowListKeyboardNavigation
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import type React from 'react'

export function isTypingTarget(target: EventTarget | null): boolean {
  if (!target || !(target instanceof Element)) {return false}
  const tag = target.tagName.toLowerCase()
  if (tag === 'input' || tag === 'textarea' || tag === 'select') {return true}
  if ((target as HTMLElement).isContentEditable) {return true}
  const editable = target.closest('[contenteditable]')
  if (editable && editable.getAttribute('contenteditable') !== 'false') {return true}
  // Why: dialogs portal outside the list but React events still bubble through the tree.
  return target.closest('[role="dialog"]') !== null
}

type Options<T> = {
  items: T[]
  getKey: (item: T) => string
  onOpen: (item: T) => void
  enabled?: boolean
  containerRole?: string
}

export function useRowListKeyboardNavigation<T>({
  items,
  getKey,
  onOpen,
  enabled = true,
  containerRole = 'listbox'
}: Options<T>) {
  const [activeKey, setActiveKey] = useState<string | null>(null)
  const keys = items.map(getKey)
  const keysRef = useRef(keys)
  keysRef.current = keys
  const lastIndex = useRef(0)
  const rows = useRef(new Map<string, HTMLElement>())

  // Keep the selection valid when rows are removed: fall back to the same position.
  useEffect(() => {
    if (keys.length === 0) {
      if (activeKey !== null) {setActiveKey(null)}
      return
    }
    if (activeKey !== null && !keys.includes(activeKey)) {
      setActiveKey(keys[Math.min(lastIndex.current, keys.length - 1)])
    }
  }, [keys.join('\u0000'), activeKey]) // eslint-disable-line react-hooks/exhaustive-deps

  const select = useCallback((key: string) => {
    lastIndex.current = Math.max(0, keysRef.current.indexOf(key))
    setActiveKey(key)
    rows.current.get(key)?.scrollIntoView?.({ block: 'nearest' })
  }, [])

  const onKeyDown = useCallback(
    (event: React.KeyboardEvent) => {
      if (!enabled || event.defaultPrevented) {return}
      if (event.nativeEvent?.isComposing || (event as { isComposing?: boolean }).isComposing) {return}
      if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) {return}
      if (isTypingTarget(event.target)) {return}
      const list = keysRef.current
      if (list.length === 0) {return}
      const current = activeKey === null ? -1 : list.indexOf(activeKey)
      switch (event.key) {
        case 'j':
        case 'ArrowDown':
          event.preventDefault()
          select(list[Math.min(current + 1, list.length - 1)])
          break
        case 'k':
        case 'ArrowUp':
          event.preventDefault()
          select(list[Math.max(current - 1, 0)])
          break
        case 'Home':
          event.preventDefault()
          select(list[0])
          break
        case 'End':
          event.preventDefault()
          select(list.at(-1) as string)
          break
        case 'Enter': {
          // Why: Enter on a button/link inside a row must activate that control, not open the row.
          const target = event.target as Element
          const interactive = target.closest('button, a')
          if (interactive && interactive !== event.currentTarget) {return}
          if (current < 0) {return}
          event.preventDefault()
          onOpen(items.at(current) as T)
          break
        }
      }
    },
    [enabled, activeKey, items, onOpen, select]
  )

  const onFocus = useCallback(() => {
    if (activeKey === null && keysRef.current.length > 0) {select(keysRef.current[0])}
  }, [activeKey, select])

  const containerProps = { role: containerRole, tabIndex: 0, onKeyDown, onFocus }

  const getRowProps = (key: string) => ({
    'data-row-key': key,
    'aria-selected': key === activeKey,
    // Roving tabindex: exactly one row is tabbable; with no selection the container is.
    tabIndex: key === activeKey ? 0 : -1,
    ref: (el: HTMLElement | null) => {
      if (el) {rows.current.set(key, el)} else {rows.current.delete(key)}
    },
    onClick: () => select(key)
  })

  return { containerProps, getRowProps, activeKey }
}
