// Sync between the local diffComments store and annotation-service — the
// 'environment' (web/multi-user) runtime target's source of truth for
// review comments. Split out of diffComments.ts (TASK-FE-ANNOTATE-001/002)
// to keep that file under the max-lines budget; this module owns only the
// wire-format mapping, hydration-on-mount, and per-record CRUD diffing.
import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { DiffComment } from '../../../../shared/types'
import { findWorktreeById, getRepoIdFromWorktreeId } from './worktree-helpers'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { settingsForWorktreeOwner, type DiffCommentsSlice } from './diffComments'
import { mutateComments } from './diff-comments-mutations'

// Wire shape of annotation.list/create's result after TASK-BE-ANNOTATE-003's
// fix — camelCase + Unix-ms timestamps (annotationView/anchorView in
// backend-go's channels.go). Kept minimal: only the fields
// annotationWireToDiffComment actually reads.
type AnnotationWire = {
  id: string
  anchor: {
    worktreeId: string
    filePath: string
    line: number
    endLine: number
  }
  content: string
  createdAtUnixMs: number
  updatedAtUnixMs: number
  originalCode: string
  sentToAgent: boolean
  sentAtUnixMs: number
}

// Why (TASK-FE-ANNOTATE-001): tracks, per worktree, the last comment array
// this module itself wrote to annotation-service — used only to diff
// against the next call so persistRemote() knows which comments are new/
// edited/removed since annotation-service's API is per-record CRUD, not a
// bulk-overwrite like the desktop/local path in diffComments.ts. Module-
// level (not store state) for the same reason persistQueueByWorktree there
// is: it tracks this module's own write history, not UI-visible state.
const lastPersistedByWorktree: Map<string, DiffComment[]> = new Map()

// Why: annotation-service generates its own primary key (Postgres
// gen_random_uuid()), distinct from DiffComment.id (a client-generated UUID
// used as annotation.create's idempotency requestId, and as the identity
// every other part of diffComments.ts — rollback, mutateComments — already
// keys on). annotation.update/delete need THAT server id, not ours, so it
// must be tracked separately rather than assumed equal to DiffComment.id.
const serverAnnotationIdByCommentId: Map<string, string> = new Map()

// Why (TASK-FE-ANNOTATE-002): tracks which worktrees this module has
// already hydrated from annotation-service in the current session, so
// re-mounting the same worktree's diff view doesn't re-fetch. Session-
// lifetime by design (comments this module itself creates/deletes stay
// consistent via persistRemote's own diffing — a stale hydration cache is
// not a correctness risk, only re-hydrating on every mount would be
// wasteful).
const hydratedWorktreeIds: Set<string> = new Set()
const hydrationInFlight: Map<string, Promise<void>> = new Map()

// Why: test-only reset for the module-level maps above, mirroring
// runtime-rpc-client.ts's clearRuntimeCompatibilityCacheForTests convention
// — without this, state from one test's persistRemote() calls leaks into
// the next test in the same file (these maps are deliberately long-lived
// for the real app session, not per-test).
export function clearAnnotationSyncCacheForTests(): void {
  lastPersistedByWorktree.clear()
  serverAnnotationIdByCommentId.clear()
  hydratedWorktreeIds.clear()
  hydrationInFlight.clear()
}

function annotationWireToDiffComment(a: AnnotationWire): DiffComment {
  // Why: mirrors persistRemote's own create-args mapping in reverse
  // (line: startLine ?? lineNumber, endLine: startLine!==undefined ?
  // lineNumber : 0) — endLine 0 or equal to line means "single line, no
  // range" per annotation.proto's Anchor.end_line doc comment.
  const hasRange = a.anchor.endLine !== 0 && a.anchor.endLine !== a.anchor.line
  return {
    id: a.id, // server id used directly as DiffComment.id for hydrated rows — see design note below
    worktreeId: a.anchor.worktreeId,
    filePath: a.anchor.filePath,
    lineNumber: hasRange ? a.anchor.endLine : a.anchor.line,
    startLine: hasRange ? a.anchor.line : undefined,
    body: a.content,
    createdAt: a.createdAtUnixMs || Date.now(),
    updatedAt: a.updatedAtUnixMs || undefined,
    sentAt: a.sentToAgent ? a.sentAtUnixMs || Date.now() : undefined,
    side: 'modified'
  }
}

// Why (TASK-FE-ANNOTATE-002): only the 'environment' (web/multi-user)
// target has an annotation-service bridge — see persistRemote's identical
// guard and SOL-FE-ANNOTATE-001 §0. Idempotent per worktree per session
// (hydratedWorktreeIds) and de-duplicates concurrent callers
// (hydrationInFlight) — DiffViewer mounts once per open file, so the same
// worktree can trigger this from several components at once.
export async function hydrateFromAnnotationService(
  worktreeId: string,
  get: () => AppState,
  set: Parameters<StateCreator<AppState, [], [], DiffCommentsSlice>>[0]
): Promise<void> {
  if (hydratedWorktreeIds.has(worktreeId)) {
    return
  }
  const inFlight = hydrationInFlight.get(worktreeId)
  if (inFlight) {
    return inFlight
  }

  const run = (async () => {
    const target = getActiveRuntimeTarget(settingsForWorktreeOwner(get(), worktreeId))
    if (target.kind === 'local') {
      // Why: no annotation-service bridge in desktop mode — mark hydrated
      // anyway so this early-return doesn't get re-checked on every mount.
      hydratedWorktreeIds.add(worktreeId)
      return
    }

    const listResp = await callRuntimeRpc<{ annotations: AnnotationWire[] }>(
      target,
      'annotation.list',
      { worktreeId, sentToAgent: false },
      { timeoutMs: 15_000 }
    )
    const serverComments = (listResp.annotations ?? []).map(annotationWireToDiffComment)
    for (const c of serverComments) {
      serverAnnotationIdByCommentId.set(c.id, c.id)
    }

    // Why (backfill, SOL-FE-ANNOTATE-001 §4): a comment already in local
    // state (from the old WorktreeMeta.metadata JSONB path, or added while
    // offline) that has no matching server row yet gets created once.
    // Matched by content, not id — pre-existing local comments were never
    // assigned a server id, so id can't be the join key here.
    const repoId = getRepoIdFromWorktreeId(worktreeId)
    const localBefore = findWorktreeById(get().worktreesByRepo, worktreeId)?.diffComments ?? []
    const matchesServer = (local: DiffComment): boolean =>
      serverComments.some(
        (s) =>
          s.filePath === local.filePath &&
          s.lineNumber === local.lineNumber &&
          s.body === local.body
      )
    const backfilled: DiffComment[] = []
    for (const c of localBefore) {
      if (matchesServer(c)) {
        continue
      }
      try {
        const created = await callRuntimeRpc<AnnotationWire | undefined>(
          target,
          'annotation.create',
          {
            anchor: {
              repoId,
              worktreeId,
              filePath: c.filePath,
              line: c.startLine ?? c.lineNumber,
              endLine: c.startLine !== undefined ? c.lineNumber : 0,
              side: 2, // annotationv1.Side_SIDE_NEW
              ref: ''
            },
            content: c.body,
            requestId: c.id,
            originalCode: c.selectedText ?? ''
          },
          { timeoutMs: 15_000 }
        )
        if (created) {
          serverAnnotationIdByCommentId.set(created.id, created.id)
          backfilled.push(annotationWireToDiffComment(created))
        }
      } catch (err) {
        // Why: one bad row must not abort hydrating everything else — the
        // comment simply stays local-only (as it already was) until the
        // next hydration attempt.
        console.error('Failed to backfill diff comment to annotation-service:', err)
      }
    }

    const merged = [...serverComments, ...backfilled]
    const mergedIdentities = new Set(merged.map((c) => `${c.filePath}|${c.lineNumber}|${c.body}`))
    // Why: re-read `existing` inside mutateComments' updater (not
    // localBefore, captured before the awaits above) so a comment added
    // concurrently while this hydration's network calls were in flight
    // isn't dropped by this overwrite.
    mutateComments(set, worktreeId, (existing) => [
      ...merged,
      ...existing.filter((c) => !mergedIdentities.has(`${c.filePath}|${c.lineNumber}|${c.body}`))
    ])
    lastPersistedByWorktree.set(worktreeId, get().getDiffComments(worktreeId))
    hydratedWorktreeIds.add(worktreeId)
  })()

  hydrationInFlight.set(worktreeId, run)
  try {
    await run
  } finally {
    hydrationInFlight.delete(worktreeId)
  }
}

export async function persistRemote(
  target: Exclude<ReturnType<typeof getActiveRuntimeTarget>, { kind: 'local' }>,
  worktreeId: string,
  diffComments: DiffComment[]
): Promise<void> {
  const previous = lastPersistedByWorktree.get(worktreeId) ?? []
  const previousById = new Map(previous.map((c) => [c.id, c]))
  const currentById = new Map(diffComments.map((c) => [c.id, c]))
  const repoId = getRepoIdFromWorktreeId(worktreeId)

  for (const c of diffComments) {
    const prev = previousById.get(c.id)
    if (!prev) {
      // Why: side is always SIDE_NEW (2) — types.ts's DiffComment.side is
      // typed 'modified' only in v1 (no old-side comment support yet), and
      // annotation-service's Side enum's closest match is SIDE_NEW. ref is
      // left empty — domain.NewAnchor does not require it (only repoId/
      // filePath are validated), and this slice has no commit-ref concept
      // to supply one from.
      const created = await callRuntimeRpc<{ id: string } | undefined>(
        target,
        'annotation.create',
        {
          anchor: {
            repoId,
            worktreeId,
            filePath: c.filePath,
            line: c.startLine ?? c.lineNumber,
            endLine: c.startLine !== undefined ? c.lineNumber : 0,
            side: 2, // annotationv1.Side_SIDE_NEW
            ref: ''
          },
          content: c.body,
          requestId: c.id, // idempotency key — see annotation-service's per-(tenant,request) dedup
          originalCode: c.selectedText ?? ''
        },
        { timeoutMs: 15_000 }
      )
      if (created?.id) {
        serverAnnotationIdByCommentId.set(c.id, created.id)
      }
    } else if (prev.body !== c.body) {
      const serverId = serverAnnotationIdByCommentId.get(c.id)
      if (serverId) {
        await callRuntimeRpc(
          target,
          'annotation.update',
          { id: serverId, content: c.body, resolved: false },
          { timeoutMs: 15_000 }
        )
      }
      // Why: no `else` fallback to annotation.create here — if we somehow
      // never captured a server id for an existing local comment (e.g. an
      // earlier create call failed silently before this task's error
      // handling — see addDiffComment's catch), the edit is silently
      // dropped server-side rather than creating a duplicate row. Accepted
      // as a known limitation for this first cut, not fixed here.
    }
  }

  for (const c of previous) {
    if (!currentById.has(c.id)) {
      const serverId = serverAnnotationIdByCommentId.get(c.id)
      if (serverId) {
        await callRuntimeRpc(
          target,
          'annotation.delete',
          { id: serverId, confirmed: Boolean(c.sentAt) }, // BR-CR-08
          { timeoutMs: 15_000 }
        )
        serverAnnotationIdByCommentId.delete(c.id)
      }
    }
  }

  lastPersistedByWorktree.set(worktreeId, diffComments)
}
