/**
 * request-list-keyboard.ts — CR-REQ-019-02
 *
 * Keyboard navigation handler for the request list.
 * j/k/ArrowDown/ArrowUp move, Enter opens, Escape closes.
 * Ignored when focus is in an input, IME composition is active, or modifier keys are held.
 *
 * @module components/request/request-list-keyboard
 */

export type RequestListKeyboardHandlerParams = {
  ids: string[]
  activeId: string | null
  onMove: (id: string) => void
  onOpen: (id: string) => void
  onClose: () => void
}

function isInputTarget(target: EventTarget | null): boolean {
  if (!target || !(target instanceof Element)) return false
  const tag = target.tagName.toLowerCase()
  return (
    tag === 'input' ||
    tag === 'textarea' ||
    tag === 'select' ||
    (target as HTMLElement).isContentEditable
  )
}

export function handleRequestListKey(
  event: React.KeyboardEvent | KeyboardEvent,
  params: RequestListKeyboardHandlerParams
): void {
  const { ids, activeId, onMove, onOpen, onClose } = params
  const { key, isComposing } = event as KeyboardEvent & { isComposing: boolean }

  // Ignore when in a text input or during IME composition
  if (isInputTarget(('target' in event ? event.target : null))) return
  if (isComposing) return

  // Ignore when modifier keys are held (avoids conflicts with global shortcuts)
  if (
    (event as KeyboardEvent).ctrlKey ||
    (event as KeyboardEvent).metaKey ||
    (event as KeyboardEvent).altKey
  ) return

  const currentIndex = activeId ? ids.indexOf(activeId) : -1

  switch (key) {
    case 'ArrowDown':
    case 'j': {
      event.preventDefault()
      const nextIndex = currentIndex < ids.length - 1 ? currentIndex + 1 : currentIndex
      const nextId = ids[nextIndex]
      if (nextId && nextId !== activeId) onMove(nextId)
      break
    }

    case 'ArrowUp':
    case 'k': {
      event.preventDefault()
      const prevIndex = currentIndex > 0 ? currentIndex - 1 : 0
      const prevId = ids[prevIndex]
      if (prevId && prevId !== activeId) onMove(prevId)
      break
    }

    case 'Enter': {
      event.preventDefault()
      if (activeId) onOpen(activeId)
      break
    }

    case 'Escape': {
      event.preventDefault()
      onClose()
      break
    }
  }
}
