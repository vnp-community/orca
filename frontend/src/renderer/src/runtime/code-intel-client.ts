/**
 * code-intel-client.ts — FE-CV-TASK-050-07
 *
 * Renderer-side code-intel RPC client.
 * Wraps CodeIntelBridgeApi with:
 *  - Error classification (parseCodeIntelErrorMessage + classifyCodeIntelError)
 *  - Argument size validation per CODE_INTEL_METHOD_LIMITS
 *  - AbortSignal support (drops result, does NOT cancel the in-flight RPC)
 *  - Offline → connectivity poll trigger
 *  - Envelope parsing for methods that return envelopes
 *
 * Methods that do NOT return envelopes (per spec §4.8):
 *  status, reindex, reindexStatus, reviewState.*, c4.*, bindRepo,
 *  settings.*, dismissFinding
 *
 * @module runtime/code-intel-client
 */

import { useAppStore } from '@/store'
import {
  CODE_INTEL_ERROR_KIND_BY_CODE,
  parseCodeIntelErrorMessage,
  parseCodeIntelEnvelope,
  parseIndexStatus
} from '../../../shared/code-intel-parsers'
import { getMethodMaxArgsBytes } from '../../../shared/code-intel-rpc-methods'
import type { CodeIntelBridgeApi, CodeIntelRawEnvelope } from '../../../shared/code-intel-bridge'
import type { CodeIntelErrorKind } from '../../../shared/code-intel-parsers'
import type { CodeIntelEnvelope } from '../../../shared/code-intel-types'

// ---------------------------------------------------------------------------
// Classified error
// ---------------------------------------------------------------------------

export type CodeIntelRpcError = {
  kind: CodeIntelErrorKind
  code: string | null
  message: string
  data: Record<string, unknown> | null
  retryable: boolean
}

// Methods that receive raw JSON (no envelope wrapping)
const NO_ENVELOPE_METHODS = new Set([
  'codeIntel.status',
  'codeIntel.reindex',
  'codeIntel.reindexStatus',
  'codeIntel.reviewState.get',
  'codeIntel.reviewState.save',
  'codeIntel.reviewState.approve',
  'codeIntel.reviewState.reset',
  'codeIntel.reviewComment.add',
  'codeIntel.reviewComment.resolve',
  'codeIntel.reviewComment.delete',
  'codeIntel.reviewChecklist.set',
  'codeIntel.c4.get',
  'codeIntel.c4.save',
  'codeIntel.bindRepo',
  'codeIntel.settings.get',
  'codeIntel.settings.save',
  'codeIntel.dismissFinding',
])

// ---------------------------------------------------------------------------
// Error classifier
// ---------------------------------------------------------------------------

/**
 * Classify a raw bridge response or caught Error into a CodeIntelRpcError.
 */
export function classifyCodeIntelError(
  responseOrError: CodeIntelRawEnvelope | Error | unknown
): CodeIntelRpcError {
  if (responseOrError instanceof Error) {
    // Thrown by transport — parse the message for a CODEINTEL_ code
    const parsed = parseCodeIntelErrorMessage(responseOrError.message)
    const kind = (parsed.code && CODE_INTEL_ERROR_KIND_BY_CODE[parsed.code]) ?? 'unknown'
    return {
      kind,
      code: parsed.code,
      message: parsed.text,
      data: parsed.data,
      retryable: kind === 'offline' || kind === 'rate_limited'
    }
  }

  if (
    typeof responseOrError === 'object' &&
    responseOrError !== null &&
    'ok' in responseOrError &&
    !(responseOrError as { ok: boolean }).ok
  ) {
    const raw = responseOrError as CodeIntelRawEnvelope
    const err = raw.error
    const rawCode = err?.code ?? ''
    const rawMessage = err?.message ?? ''

    // Map RPC-level codes
    let kind: CodeIntelErrorKind
    if (rawCode === 'method_not_found') kind = 'unsupported'
    else if (rawCode === 'forbidden') kind = 'forbidden'
    else if (
      rawCode === 'connection_refused' ||
      rawCode === 'network_error' ||
      rawCode === 'timeout'
    )
      kind = 'offline'
    // `internal` is NOT a semantic code — fall through to CODEINTEL_ prefix check
    else {
      const parsed = parseCodeIntelErrorMessage(rawMessage)
      kind = (parsed.code && CODE_INTEL_ERROR_KIND_BY_CODE[parsed.code]) ??
        CODE_INTEL_ERROR_KIND_BY_CODE[rawCode] ?? 'unknown'
    }

    return {
      kind,
      code: err?.code ?? null,
      message: rawMessage,
      data: typeof err?.data === 'object' && err?.data !== null
        ? (err.data as Record<string, unknown>)
        : null,
      retryable: kind === 'offline' || kind === 'rate_limited'
    }
  }

  return {
    kind: 'unknown',
    code: null,
    message: 'Unknown error',
    data: null,
    retryable: false
  }
}

// ---------------------------------------------------------------------------
// Byte-size guard
// ---------------------------------------------------------------------------

function checkArgSizeBytes(method: string, params: unknown): void {
  if (params === undefined || params === null) return
  const json = JSON.stringify(params)
  const bytes = new TextEncoder().encode(json).length
  const limit = getMethodMaxArgsBytes(method)
  if (bytes > limit) {
    throw Object.assign(new Error(`[code-intel] params exceed ${limit} bytes for ${method}`), {
      kind: 'validation' as CodeIntelErrorKind
    })
  }
}

// ---------------------------------------------------------------------------
// Forbidden param guards (U3)
// ---------------------------------------------------------------------------

const FORBIDDEN_PARAMS = new Set(['__proto__', 'constructor', 'prototype'])

function hasForbiddenKey(value: unknown): boolean {
  if (typeof value !== 'object' || value === null) return false
  for (const key of Object.keys(value as object)) {
    if (FORBIDDEN_PARAMS.has(key)) return true
  }
  return Array.isArray(value)
    ? (value as unknown[]).some(hasForbiddenKey)
    : Object.values(value as object).some(hasForbiddenKey)
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

export type CodeIntelClientCallOpts = {
  /** AbortSignal: result is dropped if aborted (RPC is NOT cancelled) */
  signal?: AbortSignal
  /** environmentId override; defaults to resolved env from selector */
  environmentId: string | null
}

export type CodeIntelClient = {
  call<T = unknown>(
    worktreeId: string,
    method: string,
    params: unknown,
    opts: CodeIntelClientCallOpts
  ): Promise<{ ok: true; result: T } | { ok: false; error: CodeIntelRpcError }>

  callEnvelope<T = unknown>(
    worktreeId: string,
    method: string,
    params: unknown,
    parseData: (raw: unknown) => T,
    opts: CodeIntelClientCallOpts
  ): Promise<{ ok: true; envelope: CodeIntelEnvelope<T> } | { ok: false; error: CodeIntelRpcError }>
}

/**
 * Create the code-intel RPC client.
 * bridge comes from window.api.codeIntel (PreloadApi).
 */
export function createCodeIntelClient(bridge: CodeIntelBridgeApi): CodeIntelClient {
  async function rawCall(
    method: string,
    params: unknown,
    opts: CodeIntelClientCallOpts
  ): Promise<CodeIntelRawEnvelope> {
    // Guard: forbidden proto-pollution keys
    if (Array.isArray(params)) {
      throw new Error(`[code-intel] array params are not allowed for ${method}`)
    }
    if (hasForbiddenKey(params)) {
      throw new Error(`[code-intel] forbidden key in params for ${method}`)
    }

    checkArgSizeBytes(method, params)

    const response = await bridge.call({
      environmentId: opts.environmentId,
      method,
      params: params ?? {}
    })

    return response
  }

  function triggerConnectivityPollIfOffline(error: CodeIntelRpcError): void {
    if (error.kind === 'offline') {
      // Trigger connectivity poll via store action
      const store = useAppStore.getState()
      if (typeof store.maybeTriggerConnectivityPollAfterRpcFailure === 'function') {
        store.maybeTriggerConnectivityPollAfterRpcFailure(
          new Error(error.message),
          'environment'
        )
      }
    }
  }

  return {
    async call(worktreeId, method, params, opts) {
      try {
        const response = await rawCall(method, params, opts)

        if (opts.signal?.aborted) {
          // Signal aborted — drop result
          return { ok: false, error: { kind: 'unknown', code: null, message: 'aborted', data: null, retryable: false } }
        }

        if (!response.ok) {
          const error = classifyCodeIntelError(response)
          triggerConnectivityPollIfOffline(error)
          return { ok: false, error }
        }

        return { ok: true, result: response.result as never }
      } catch (err) {
        if (opts.signal?.aborted) {
          return { ok: false, error: { kind: 'unknown', code: null, message: 'aborted', data: null, retryable: false } }
        }
        const error = classifyCodeIntelError(err)
        triggerConnectivityPollIfOffline(error)
        return { ok: false, error }
      }
    },

    async callEnvelope(worktreeId, method, params, parseData, opts) {
      if (NO_ENVELOPE_METHODS.has(method)) {
        throw new Error(`[code-intel] ${method} does not return an envelope — use call()`)
      }

      try {
        const response = await rawCall(method, params, opts)

        if (opts.signal?.aborted) {
          return { ok: false, error: { kind: 'unknown', code: null, message: 'aborted', data: null, retryable: false } }
        }

        if (!response.ok) {
          const error = classifyCodeIntelError(response)
          triggerConnectivityPollIfOffline(error)
          return { ok: false, error }
        }

        const envelope = parseCodeIntelEnvelope(response.result, parseData)
        return { ok: true, envelope }
      } catch (err) {
        if (opts.signal?.aborted) {
          return { ok: false, error: { kind: 'unknown', code: null, message: 'aborted', data: null, retryable: false } }
        }
        const error = classifyCodeIntelError(err)
        triggerConnectivityPollIfOffline(error)
        return { ok: false, error }
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Singleton — initialized after the preload API is ready
// ---------------------------------------------------------------------------

let _client: CodeIntelClient | null = null

export function initCodeIntelClient(bridge: CodeIntelBridgeApi): void {
  _client = createCodeIntelClient(bridge)
}

export function getCodeIntelClient(): CodeIntelClient {
  if (!_client) {
    throw new Error('[code-intel] client not initialized — call initCodeIntelClient first')
  }
  return _client
}
