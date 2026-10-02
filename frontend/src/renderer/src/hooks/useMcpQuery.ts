import { useCallback, useEffect, useRef, useState } from 'react'
import type { McpRpcMethod, McpRpcParams, McpRpcResult } from '../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import {
  McpRpcError,
  isMcpDisabledError,
  isMcpNotAdminError,
  isMcpNotImplementedError
} from '@/runtime/runtime-mcp-error'

export type McpQueryStatus = 'loading' | 'ready' | 'error' | 'unavailable' | 'forbidden'

export type McpQuery<T> = {
  status: McpQueryStatus
  data: T
  error: string | null
  reload: () => void
  /** Local patch after a mutation; never triggers a request. */
  setData: (updater: (prev: T) => T) => void
}

export function classifyMcpError(e: unknown): { status: McpQueryStatus; message: string } {
  const message = e instanceof McpRpcError ? e.detail : e instanceof Error ? e.message : String(e)
  if (isMcpNotAdminError(e)) {
    return { status: 'forbidden', message }
  }
  if (isMcpNotImplementedError(e) || isMcpDisabledError(e)) {
    return { status: 'unavailable', message }
  }
  return { status: 'error', message }
}

/**
 * Local list loader shared by the MCP tabs. Why local state: these lists are tab-private and may
 * hold user data, so they stay out of the global store. Stale responses are dropped via a sequence.
 */
export function useMcpQuery<M extends McpRpcMethod, T = McpRpcResult<M>>(
  method: M,
  params: McpRpcParams<M> | undefined,
  empty: T,
  options: { refetchOnFocus?: boolean; quietReload?: boolean } = {}
): McpQuery<T> {
  const [state, setState] = useState<{ status: McpQueryStatus; data: T; error: string | null }>({
    status: 'loading',
    data: empty,
    error: null
  })
  const seq = useRef(0)
  const paramsKey = JSON.stringify(params ?? null)
  const emptyRef = useRef(empty)
  const invalidate = useCallback(() => {
    seq.current++
  }, [])

  const load = useCallback(
    (quiet: boolean) => {
      const id = ++seq.current
      if (!quiet) {
        setState((s) => ({ ...s, status: 'loading', error: null }))
      }
      const args = (params === undefined ? [] : [params]) as never[]
      ;(mcpClient.call as (m: M, ...a: never[]) => Promise<unknown>)(method, ...args).then(
        (res) => {
          if (id === seq.current) {
            setState({ status: 'ready', data: res as T, error: null })
          }
        },
        (e) => {
          if (id !== seq.current) {
            return
          }
          const c = classifyMcpError(e)
          setState({ status: c.status, data: emptyRef.current, error: c.message })
        }
      )
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [method, paramsKey]
  )

  useEffect(() => {
    load(false)
    return invalidate
  }, [load, invalidate])

  const refetchOnFocus = options.refetchOnFocus ?? false
  useEffect(() => {
    if (!refetchOnFocus) {
      return
    }
    const onFocus = (): void => load(true)
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [load, refetchOnFocus])

  return {
    ...state,
    reload: useCallback(() => load(options.quietReload ?? false), [load, options.quietReload]),
    setData: useCallback((updater) => setState((s) => ({ ...s, data: updater(s.data) })), [])
  }
}
