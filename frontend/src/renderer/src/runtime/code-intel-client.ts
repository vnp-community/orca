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
 * Only the contract's `Env<...>` view channels (CODE_INTEL_ENVELOPE_METHODS) go through
 * callEnvelope; status, reindex*, reviewState.*, c4.*, bindRepo, settings.*, dismissFinding
 * and quality.* return plain results.
 *
 * @module runtime/code-intel-client
 */

import { useAppStore } from '@/store'
import { resolveCodeIntelSelector } from '../lib/code-intel-worktree-selector'
import {
  codeIntelErrorKindForCode,
  parseCodeIntelErrorMessage,
  parseCodeIntelEnvelope
} from '../../../shared/code-intel-parsers'
import {
  CODE_INTEL_ENVELOPE_METHODS,
  getMethodMaxArgsBytes,
  toCodeIntelMethod
} from '../../../shared/code-intel-rpc-methods'
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

/** Client-side rejection (never reached the network) carrying its error kind. */
export class LocalCodeIntelError extends Error {
  constructor(
    readonly kind: CodeIntelErrorKind,
    message: string
  ) {
    super(message)
    this.name = 'LocalCodeIntelError'
  }
}

// Methods that receive raw JSON (no envelope wrapping)
const TENANT_SCOPED_METHODS = new Set(['codeIntel.settings.get', 'codeIntel.settings.set'])

// ---------------------------------------------------------------------------
// Error classifier
// ---------------------------------------------------------------------------

/**
 * Classify a raw bridge response or caught Error into a CodeIntelRpcError.
 */
export function classifyCodeIntelError(
  responseOrError: CodeIntelRawEnvelope | Error | unknown
): CodeIntelRpcError {
  if (responseOrError instanceof LocalCodeIntelError) {
    return {
      kind: responseOrError.kind,
      code: null,
      message: responseOrError.message,
      data: null,
      retryable: false
    }
  }

  if (responseOrError instanceof Error) {
    // Thrown by transport: the message may still carry a CODEINTEL_ prefix.
    const parsed = parseCodeIntelErrorMessage(responseOrError.message)
    const kind = codeIntelErrorKindForCode(parsed.code)
    return {
      kind,
      code: parsed.code,
      message: parsed.text,
      data: parsed.data,
      retryable: isRetryableKind(kind)
    }
  }

  if (
    typeof responseOrError === 'object' &&
    responseOrError !== null &&
    'ok' in responseOrError &&
    !(responseOrError as { ok: boolean }).ok
  ) {
    const err = (responseOrError as CodeIntelRawEnvelope).error
    const rawCode = err?.code ?? ''
    const rawMessage = err?.message ?? ''
    // The semantic code lives in error.message (contract §2.3); the RPC-level code is only a
    // fallback, and 'internal' is never a semantic code.
    const parsed = parseCodeIntelErrorMessage(rawMessage)
    const kind =
      parsed.code !== null
        ? codeIntelErrorKindForCode(parsed.code)
        : rawCode === 'internal'
          ? 'unknown'
          : codeIntelErrorKindForCode(rawCode)

    return {
      kind,
      code: parsed.code ?? (rawCode && rawCode !== 'internal' ? rawCode : null),
      message: parsed.code !== null ? parsed.text : rawMessage,
      data:
        parsed.data ??
        (typeof err?.data === 'object' && err.data !== null
          ? (err.data as Record<string, unknown>)
          : null),
      retryable: isRetryableKind(kind)
    }
  }

  return { kind: 'unknown', code: null, message: 'Unknown error', data: null, retryable: false }
}

function isRetryableKind(kind: CodeIntelErrorKind): boolean {
  return kind === 'offline' || kind === 'rate-limited' || kind === 'timeout'
}

// ---------------------------------------------------------------------------
// Byte-size guard
// ---------------------------------------------------------------------------

function checkArgSizeBytes(method: string, params: unknown): void {
  if (params === undefined || params === null) {
    return
  }
  const json = JSON.stringify(params)
  const bytes = new TextEncoder().encode(json).length
  const limit = getMethodMaxArgsBytes(method)
  if (bytes > limit) {
    throw new LocalCodeIntelError(
      'validation',
      `[code-intel] params exceed ${limit} bytes for ${method}`
    )
  }
}

// ---------------------------------------------------------------------------
// Forbidden param guards (U3)
// ---------------------------------------------------------------------------

const FORBIDDEN_PARAMS = new Set(['__proto__', 'constructor', 'prototype'])

function hasForbiddenKey(value: unknown): boolean {
  if (typeof value !== 'object' || value === null) {
    return false
  }
  for (const key of Object.keys(value as object)) {
    if (FORBIDDEN_PARAMS.has(key)) {
      return true
    }
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
  /** environmentId override; defaults to the environment resolved from the worktree selector */
  environmentId?: string | null
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
    worktreeId: string,
    method: string,
    params: unknown,
    opts: CodeIntelClientCallOpts
  ): Promise<CodeIntelRawEnvelope> {
    // Guard: forbidden proto-pollution keys
    if (Array.isArray(params)) {
      throw new LocalCodeIntelError(
        'validation',
        `[code-intel] array params are not allowed for ${method}`
      )
    }
    if (hasForbiddenKey(params)) {
      throw new LocalCodeIntelError(
        'validation',
        `[code-intel] forbidden key in params for ${method}`
      )
    }

    let environmentId = opts.environmentId ?? null
    let finalParams = params ?? {}
    // Why: settings.* is tenant-scoped; every other channel needs {projectId, worktreeId} (PQ-04)
    // and must not hit the network when the worktree has no addressable project.
    if (!TENANT_SCOPED_METHODS.has(method)) {
      const selector = resolveCodeIntelSelector(useAppStore.getState(), worktreeId)
      if (selector.state === 'unsupported') {
        throw new LocalCodeIntelError('unsupported', `[code-intel] ${selector.reason}`)
      }
      if (opts.environmentId === undefined) {
        environmentId = selector.environmentId
      }
      finalParams = {
        ...(finalParams as Record<string, unknown>),
        projectId: selector.projectId,
        worktreeId: selector.worktreeId
      }
    }

    checkArgSizeBytes(method, finalParams)

    const response = await bridge.call({ environmentId, method, params: finalParams })

    return response
  }

  function triggerConnectivityPollIfOffline(
    error: CodeIntelRpcError,
    environmentId: string | null | undefined
  ): void {
    if (error.kind === 'offline' && environmentId) {
      // Trigger connectivity poll via store action
      const store = useAppStore.getState()
      if (typeof store.maybeTriggerConnectivityPollAfterRpcFailure === 'function') {
        store.maybeTriggerConnectivityPollAfterRpcFailure(new Error(error.message), {
          kind: 'environment',
          environmentId
        })
      }
    }
  }

  return {
    async call(worktreeId, rawMethod, params, opts) {
      const method = toCodeIntelMethod(rawMethod)
      try {
        const response = await rawCall(worktreeId, method, params, opts)

        if (opts.signal?.aborted) {
          // Signal aborted — drop result
          return {
            ok: false,
            error: { kind: 'unknown', code: null, message: 'aborted', data: null, retryable: false }
          }
        }

        if (!response.ok) {
          const error = classifyCodeIntelError(response)
          triggerConnectivityPollIfOffline(error, opts.environmentId)
          return { ok: false, error }
        }

        return { ok: true, result: response.result as never }
      } catch (err) {
        if (opts.signal?.aborted) {
          return {
            ok: false,
            error: { kind: 'unknown', code: null, message: 'aborted', data: null, retryable: false }
          }
        }
        const error = classifyCodeIntelError(err)
        triggerConnectivityPollIfOffline(error, opts.environmentId)
        return { ok: false, error }
      }
    },

    async callEnvelope(worktreeId, rawMethod, params, parseData, opts) {
      const method = toCodeIntelMethod(rawMethod)
      if (!CODE_INTEL_ENVELOPE_METHODS.has(method)) {
        throw new Error(`[code-intel] ${method} does not return an envelope — use call()`)
      }

      try {
        const response = await rawCall(worktreeId, method, params, opts)

        if (opts.signal?.aborted) {
          return {
            ok: false,
            error: { kind: 'unknown', code: null, message: 'aborted', data: null, retryable: false }
          }
        }

        if (!response.ok) {
          const error = classifyCodeIntelError(response)
          triggerConnectivityPollIfOffline(error, opts.environmentId)
          return { ok: false, error }
        }

        const envelope = parseCodeIntelEnvelope(response.result, parseData)
        return { ok: true, envelope }
      } catch (err) {
        if (opts.signal?.aborted) {
          return {
            ok: false,
            error: { kind: 'unknown', code: null, message: 'aborted', data: null, retryable: false }
          }
        }
        const error = classifyCodeIntelError(err)
        triggerConnectivityPollIfOffline(error, opts.environmentId)
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
  // Why: no boot path calls initCodeIntelClient, so every Review/flag call threw and the flag
  // stayed 'unknown'; bind lazily to the preload/web bridge once it exists.
  const bridge =
    typeof window === 'undefined'
      ? undefined
      : (window as { api?: { codeIntel?: CodeIntelBridgeApi } }).api?.codeIntel
  if (!_client && bridge) {
    initCodeIntelClient(bridge)
  }
  if (!_client) {
    throw new Error('[code-intel] client not initialized — call initCodeIntelClient first')
  }
  return _client
}
