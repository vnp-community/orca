import { useCallback, useEffect, useRef, useState } from 'react'
import type { McpConsentRequest, McpScopeId } from '../../../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { validateConsentRedirect } from '@/lib/mcp-redirect-validation'

export type McpConsentErrorCode =
  | 'MCP_CONSENT_EXPIRED'
  | 'MCP_CONSENT_NOT_FOUND'
  | 'MCP_DISABLED'
  | 'UNKNOWN'

export type McpConsentState =
  | { kind: 'loading' }
  | { kind: 'ready'; req: McpConsentRequest; notice?: 'scope' }
  | { kind: 'submitting'; req: McpConsentRequest }
  | { kind: 'redirecting'; url: string; clientName: string }
  | { kind: 'error'; code: McpConsentErrorCode; message: string }

const REQUEST_ID = /^[A-Za-z0-9-]{8,64}$/

export function readConsentRequestId(search: string): string | null {
  const id = new URLSearchParams(search).get('request_id')
  return id && REQUEST_ID.test(id) ? id : null
}

function toError(e: unknown): McpConsentState {
  const err = e instanceof McpRpcError ? e : null
  const code: McpConsentErrorCode =
    err?.code === 'MCP_CONSENT_EXPIRED' ||
    err?.code === 'MCP_CONSENT_NOT_FOUND' ||
    err?.code === 'MCP_DISABLED'
      ? err.code
      : 'UNKNOWN'
  return {
    kind: 'error',
    code,
    message: err?.detail ?? (e instanceof Error ? e.message : '')
  }
}

// Why: no logging/telemetry/storage of requestId, scopes or redirectUrl anywhere in this hook.
export function useMcpConsent(
  requestId: string | null,
  navigate: (url: string) => void = (u) => window.location.assign(u)
): {
  state: McpConsentState
  decide: (decision: 'approve' | 'deny', scopes: McpScopeId[]) => void
  retry: () => void
} {
  const [state, setState] = useState<McpConsentState>(
    requestId ? { kind: 'loading' } : { kind: 'error', code: 'MCP_CONSENT_NOT_FOUND', message: '' }
  )
  const busy = useRef(false)
  const reqRef = useRef<McpConsentRequest | null>(null)
  const [attempt, setAttempt] = useState(0)
  const navigateRef = useRef(navigate)
  navigateRef.current = navigate

  useEffect(() => {
    if (!requestId) {
      return
    }
    let cancelled = false
    setState({ kind: 'loading' })
    mcpClient.call('mcp.consent.get', { requestId }).then(
      (req) => {
        if (cancelled) {
          return
        }
        reqRef.current = req
        setState({ kind: 'ready', req })
      },
      (e) => {
        if (!cancelled) {
          setState(toError(e))
        }
      }
    )
    return () => {
      cancelled = true
    }
  }, [requestId, attempt])

  const decide = useCallback(
    (decision: 'approve' | 'deny', scopes: McpScopeId[]): void => {
      const req = reqRef.current
      // Why: the ref flips synchronously so a double click can never send two decisions.
      if (!requestId || !req || busy.current) {
        return
      }
      busy.current = true
      setState({ kind: 'submitting', req })
      mcpClient.call('mcp.consent.decide', { requestId, decision, scopes }).then(
        (res) => {
          const url = validateConsentRedirect(res?.redirectUrl)
          if (!url) {
            setState({ kind: 'error', code: 'UNKNOWN', message: '' })
            return
          }
          setState({ kind: 'redirecting', url, clientName: req.clientName })
          navigateRef.current(url)
        },
        (e) => {
          busy.current = false
          const err = e instanceof McpRpcError ? e : null
          if (err?.code === 'MCP_SCOPE_INVALID' || err?.code === 'MCP_SCOPE_NOT_ALLOWED') {
            setState({ kind: 'ready', req, notice: 'scope' })
            return
          }
          setState(toError(e))
        }
      )
    },
    [requestId]
  )

  const retry = useCallback(() => {
    busy.current = false
    setAttempt((n) => n + 1)
  }, [])

  return { state, decide, retry }
}
