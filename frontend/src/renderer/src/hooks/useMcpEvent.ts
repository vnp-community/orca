import { useEffect, useRef } from 'react'
import type { McpEvent } from '../../../shared/mcp-types'
import { subscribeMcpEvents } from '@/lib/mcp-event-bus'

export function useMcpEvent<T extends McpEvent['type']>(
  type: T,
  handler: (event: Extract<McpEvent, { type: T }>) => void
): void {
  const handlerRef = useRef(handler)
  handlerRef.current = handler
  useEffect(
    () =>
      subscribeMcpEvents((event) => {
        if (event.type === type) {
          handlerRef.current(event as Extract<McpEvent, { type: T }>)
        }
      }),
    [type]
  )
}
