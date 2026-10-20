/**
 * useReviewArtifact — FE-OR-T211
 *
 * Reads one drawing of a review when it is shown. A drawing never changes once stored, so one that was read is kept (the eight
 * most recent) and shown at once; the server is still asked with the digest, and an unchanged drawing costs one small answer.
 * Opening a tab again therefore never draws anything.
 *
 * @module hooks/useReviewArtifact
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseArtifactContent } from '../../../shared/outcome-review-response-parsers'
import type { ReviewArtifactMeta } from '../../../shared/outcome-review-types'

export const ARTIFACT_CACHE_SIZE = 8

type Cached = { meta: ReviewArtifactMeta; html: string }
const cache = new Map<string, Cached>()

/** For tests: forget what was read. */
export function resetReviewArtifactCache(): void {
  cache.clear()
}

function remember(artifactId: string, value: Cached): void {
  cache.delete(artifactId)
  cache.set(artifactId, value)
  while (cache.size > ARTIFACT_CACHE_SIZE) {
    const oldest = cache.keys().next().value
    if (oldest === undefined) {
      break
    }
    cache.delete(oldest)
  }
}

export type ReviewArtifactStatus = 'idle' | 'loading' | 'ready' | 'not_found' | 'error'

export type UseReviewArtifactResult = {
  status: ReviewArtifactStatus
  meta: ReviewArtifactMeta | null
  /** The drawing as stored (untrusted: build a srcdoc from it, never insert it into the page). */
  html: string | null
  code: string
  reload: () => void
}

export function useReviewArtifact(
  requestId: string,
  artifactId: string | undefined,
  enabled = true
): UseReviewArtifactResult {
  const [state, setState] = useState<{
    status: ReviewArtifactStatus
    meta: ReviewArtifactMeta | null
    html: string | null
    code: string
  }>({
    status: 'idle',
    meta: null,
    html: null,
    code: ''
  })
  const generation = useRef(0)

  const load = useCallback(() => {
    if (!enabled || !artifactId) {
      setState({ status: 'idle', meta: null, html: null, code: '' })
      return
    }
    const mine = ++generation.current
    const kept = cache.get(artifactId)
    if (kept) {
      setState({ status: 'ready', meta: kept.meta, html: kept.html, code: '' })
    } else {
      setState({ status: 'loading', meta: null, html: null, code: '' })
    }
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.REVIEW_ARTIFACT, {
      id: requestId,
      artifactId,
      ...(kept?.meta.blobDigest ? { ifDigest: kept.meta.blobDigest } : {})
    }).then((res) => {
      if (mine !== generation.current) {
        return
      }
      if (!res.ok) {
        if (kept) {
          return // what was read stays on screen; a failed refresh is not worth hiding it
        }
        setState({
          status:
            res.error.backendCode?.endsWith('NOT_FOUND') || res.error.kind === 'not_found'
              ? 'not_found'
              : 'error',
          meta: null,
          html: null,
          code: res.error.backendCode ?? ''
        })
        return
      }
      const got = parseArtifactContent(res.value)
      if (!got) {
        setState({ status: 'error', meta: null, html: null, code: '' })
        return
      }
      if (got.notModified && kept) {
        return
      }
      if (typeof got.content !== 'string') {
        setState({ status: 'error', meta: got.artifact, html: null, code: '' })
        return
      }
      remember(artifactId, { meta: got.artifact, html: got.content })
      setState({ status: 'ready', meta: got.artifact, html: got.content, code: '' })
    })
  }, [requestId, artifactId, enabled])

  useEffect(() => {
    const reads = generation
    load()
    return () => {
      reads.current++
    }
  }, [load])

  return { ...state, reload: load }
}
