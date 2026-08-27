import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { getAnnotationStore } from '../../../code-review/annotation-store'

// Why: matches annotation-panel.tsx's callRuntimeRpc<Annotation[]>('annotation.list', {...})
// param shape exactly — projectId/filePath/lineNumber required, reviewId optional
// (the panel can be opened outside a hosted review context).
const AnnotationListParams = z.object({
  projectId: z.string().min(1, 'Missing projectId'),
  reviewId: z.string().optional(),
  filePath: z.string().min(1, 'Missing filePath'),
  lineNumber: z.number().int()
})

const AnnotationCreateParams = z.object({
  projectId: z.string().min(1, 'Missing projectId'),
  reviewId: z.string().optional(),
  filePath: z.string().min(1, 'Missing filePath'),
  lineNumber: z.number().int(),
  content: z.string().min(1, 'Comment cannot be empty'),
  // Why: client-side trace correlation id (Tracers.codeReviewAnnotateFlow) —
  // accepted but not persisted, matches how terminal.create treats params.traceId.
  traceId: z.string().optional()
})

// Why: BL-CR-02 — annotation-panel.tsx already called annotation.list/create
// against a namespace that never existed backend-side (gaps-and-mismatches.md
// §"Category 2"). This wires it to the real orca_annotations table
// (migration 0018) via the AnnotationStore singleton.
export const ANNOTATION_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'annotation.list',
    params: AnnotationListParams,
    handler: async (params) => {
      const store = getAnnotationStore()
      return store.list({
        projectId: params.projectId,
        reviewId: params.reviewId,
        filePath: params.filePath,
        lineNumber: params.lineNumber
      })
    }
  }),
  defineMethod({
    name: 'annotation.create',
    params: AnnotationCreateParams,
    handler: async (params, ctx) => {
      const store = getAnnotationStore()
      return store.create({
        projectId: params.projectId,
        reviewId: params.reviewId,
        filePath: params.filePath,
        lineNumber: params.lineNumber,
        content: params.content,
        // Why: ctx.userId is only populated on transports that thread the
        // authenticated session through (see RpcContext.userId doc comment
        // in core.ts) — fall back to a stable placeholder rather than throw,
        // matching credentials.ts's optimistic ctx.userId usage.
        authorId: ctx.userId ?? 'anonymous'
      })
    }
  })
]
