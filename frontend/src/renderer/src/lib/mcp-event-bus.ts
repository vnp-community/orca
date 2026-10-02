import type { McpEvent } from '../../../shared/mcp-types'

type Listener = (event: McpEvent) => void

const listeners = new Set<Listener>()

export function subscribeMcpEvents(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function emitMcpEvent(event: McpEvent): void {
  for (const listener of Array.from(listeners)) {
    try {
      listener(event)
    } catch (error) {
      // Why: one broken listener must not starve the others.
      console.error('[mcp-event-bus] listener failed', error)
    }
  }
}
