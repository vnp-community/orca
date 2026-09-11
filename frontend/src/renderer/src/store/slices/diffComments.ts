/* eslint-disable max-lines -- Why: this slice keeps optimistic note
mutation, rollback, persistence ordering, and sent-state transitions together
so every write follows the same queue and rollback invariants. */
import type { StateCreator } from 'zustand'
import type { AppState } from '../types'
import type { DiffComment, Worktree } from '../../../../shared/types'
import { findWorktreeById, getRepoIdFromWorktreeId } from './worktree-helpers'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../../runtime/runtime-rpc-client'
import { createBrowserUuid } from '@/lib/browser-uuid'
import { getRuntimeEnvironmentIdForWorktree } from '@/lib/worktree-runtime-owner'

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

export type DiffCommentsSlice = {
  getDiffComments: (worktreeId: string | null | undefined) => DiffComment[]
  // TASK-FE-ANNOTATE-002: idempotent per-session hydration from
  // annotation-service for the 'environment' (web/multi-user) runtime
  // target — see this file's hydrateFromAnnotationService for the design
  // note on why this isn't wired into sync-runtime-graph.ts.
  ensureDiffCommentsHydrated: (worktreeId: string | undefined) => Promise<void>
  addDiffComment: (input: Omit<DiffComment, 'id' | 'createdAt'>) => Promise<DiffComment | null>
  updateDiffComment: (worktreeId: string, commentId: string, body: string) => Promise<boolean>
  clearDeliveredDiffComments: (
    worktreeId: string,
    comments: readonly DiffCommentDeliverySnapshot[]
  ) => Promise<boolean>
  markDiffCommentsSent: (
    worktreeId: string,
    commentIds: readonly string[],
    sentAt?: number
  ) => Promise<boolean>
  deleteDiffComment: (worktreeId: string, commentId: string) => Promise<void>
  clearDiffComments: (worktreeId: string) => Promise<boolean>
  clearDiffCommentsForFile: (worktreeId: string, filePath: string) => Promise<boolean>
}

export type DiffCommentDeliverySnapshot = Pick<
  DiffComment,
  'body' | 'filePath' | 'id' | 'lineNumber' | 'selectedText' | 'source' | 'startLine'
>

function generateId(): string {
  return createBrowserUuid()
}

function normalizeDiffComment(comment: DiffComment): DiffComment {
  const rawSource = (comment as { source?: unknown }).source
  const source = rawSource === 'markdown' || rawSource === 'diff' ? rawSource : undefined
  const rawStartLine = (comment as { startLine?: unknown }).startLine
  const startLine =
    Number.isInteger(rawStartLine) &&
    typeof rawStartLine === 'number' &&
    rawStartLine >= 1 &&
    rawStartLine <= comment.lineNumber
      ? rawStartLine
      : undefined
  const rawSelectedText = (comment as { selectedText?: unknown }).selectedText
  const selectedText =
    typeof rawSelectedText === 'string' && rawSelectedText.trim().length > 0
      ? rawSelectedText.trim()
      : undefined
  const rawSentAt = (comment as { sentAt?: unknown }).sentAt
  const sentAt =
    typeof rawSentAt === 'number' && Number.isFinite(rawSentAt) && rawSentAt > 0
      ? rawSentAt
      : undefined

  return {
    ...comment,
    ...(source !== undefined ? { source } : {}),
    ...(source === undefined ? { source: undefined } : {}),
    ...(selectedText !== undefined ? { selectedText } : {}),
    ...(selectedText === undefined ? { selectedText: undefined } : {}),
    ...(startLine !== undefined ? { startLine } : {}),
    ...(startLine === undefined ? { startLine: undefined } : {}),
    ...(sentAt !== undefined ? { sentAt } : {}),
    ...(sentAt === undefined ? { sentAt: undefined } : {})
  }
}

function deliverySnapshotMatches(
  comment: DiffComment,
  snapshot: DiffCommentDeliverySnapshot
): boolean {
  return (
    comment.id === snapshot.id &&
    comment.body === snapshot.body &&
    comment.filePath === snapshot.filePath &&
    comment.lineNumber === snapshot.lineNumber &&
    comment.startLine === snapshot.startLine &&
    comment.selectedText === snapshot.selectedText &&
    comment.source === snapshot.source
  )
}

// Why: return a stable reference when no comments exist so selectors don't
// produce a fresh `[]` on every store update. A new array identity would
// trigger re-renders in any consumer using referential equality.
// Frozen + typed `readonly` so an accidental `list.push(...)` on the returned
// value is both a runtime TypeError and a TypeScript compile error, preventing
// the sentinel from being corrupted globally.
const EMPTY_COMMENTS: readonly DiffComment[] = Object.freeze([])

// Why (TASK-FE-ANNOTATE-001): tracks, per worktree, the last comment array
// this module itself wrote to annotation-service — used only to diff
// against the next call so persist() knows which comments are new/edited/
// removed since annotation-service's API is per-record CRUD, not a
// bulk-overwrite like the desktop/local path below. Module-level (not
// store state) for the same reason persistQueueByWorktree is: it tracks
// this module's own write history, not UI-visible state.
const lastPersistedByWorktree: Map<string, DiffComment[]> = new Map()

// Why: annotation-service generates its own primary key (Postgres
// gen_random_uuid()), distinct from DiffComment.id (a client-generated UUID
// used as annotation.create's idempotency requestId, and as the identity
// every other part of this slice — rollback, mutateComments — already keys
// on). annotation.update/delete need THAT server id, not ours, so it must
// be tracked separately rather than assumed equal to DiffComment.id.
const serverAnnotationIdByCommentId: Map<string, string> = new Map()

// Why: test-only reset for the two module-level maps above, mirroring
// runtime-rpc-client.ts's clearRuntimeCompatibilityCacheForTests convention
// — without this, state from one test's persistRemote() calls leaks into
// the next test in the same file (these maps are deliberately long-lived
// for the real app session, not per-test).
// Why (TASK-FE-ANNOTATE-002): tracks which worktrees this module has
// already hydrated from annotation-service in the current session, so
// re-mounting the same worktree's diff view doesn't re-fetch. Session-
// lifetime by design (comments this module itself creates/deletes stay
// consistent via persistRemote's own diffing — a stale hydration cache is
// not a correctness risk, only re-hydrating on every mount would be
// wasteful).
const hydratedWorktreeIds: Set<string> = new Set()
const hydrationInFlight: Map<string, Promise<void>> = new Map()

export function clearDiffCommentsPersistCacheForTests(): void {
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
async function hydrateFromAnnotationService(
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

async function persistRemote(
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

async function persist(
  settings: AppState['settings'],
  worktreeId: string,
  diffComments: DiffComment[]
): Promise<void> {
  const target = getActiveRuntimeTarget(settings)
  if (target.kind === 'local') {
    await window.api.worktrees.updateMeta({
      worktreeId,
      updates: { diffComments }
    })
    return
  }
  // Why (TASK-FE-ANNOTATE-001): desktop/local mode has no bridge to
  // annotation-service (no IPC handler for annotation.create/list/delete —
  // confirmed by grep across desktop/src) and no multi-user/OPA concern to
  // justify building one — only the 'environment' (web/multi-user) target
  // routes through annotation-service. worktree.set's bulk-metadata path
  // above (desktop) is unchanged; this branch replaces the old
  // worktree.set-based remote persistence with per-record CRUD against
  // annotation-service, per SOL-FE-ANNOTATE-001 §1-2.
  await persistRemote(target, worktreeId, diffComments)
}

function settingsForWorktreeOwner(state: AppState, worktreeId: string): AppState['settings'] {
  const runtimeEnvironmentId = getRuntimeEnvironmentIdForWorktree(state, worktreeId)
  return state.settings
    ? { ...state.settings, activeRuntimeEnvironmentId: runtimeEnvironmentId }
    : ({ activeRuntimeEnvironmentId: runtimeEnvironmentId } as AppState['settings'])
}

// Why: IPC writes from `persist` are not ordered with respect to each other.
// If two mutations (e.g. rapid add then delete, or two adds) are in flight
// concurrently, their `updateMeta` resolutions can arrive out of call order,
// letting an older snapshot overwrite a newer one on disk. We serialize per
// worktree so only one write runs at a time. We also defer reading the
// snapshot until the queued work actually starts — at dequeue time we pull
// the LATEST `diffComments` from the store — which collapses a burst of N
// mutations into at most 2 in-flight writes per worktree (1 running + 1
// queued) and guarantees the last disk write reflects the newest state.
const persistQueueByWorktree: Map<string, Promise<void>> = new Map()

// Why: chain each new write onto the prior promise for this worktree so
// writes land in call order. We use `.then(..., ..)` with both handlers so a
// failing previous write doesn't break the chain — we still proceed with the
// next write. The queued work reads the latest list from the store via
// `get()` at dequeue time (not via a captured parameter) so it writes the
// most recent snapshot rather than a stale one from when it was enqueued.
// The returned promise resolves/rejects when THIS specific write commits so
// callers can preserve their optimistic-update + rollback flow.
function enqueuePersist(worktreeId: string, get: () => AppState): Promise<void> {
  const prior = persistQueueByWorktree.get(worktreeId) ?? Promise.resolve()
  const run = async (): Promise<void> => {
    const repoId = getRepoIdFromWorktreeId(worktreeId)
    const repoList = get().worktreesByRepo[repoId]
    const target = repoList?.find((w) => w.id === worktreeId)
    const latest = (target?.diffComments ?? []).map(normalizeDiffComment)
    await persist(settingsForWorktreeOwner(get(), worktreeId), worktreeId, latest)
  }
  const next = prior.then(run, run)
  persistQueueByWorktree.set(worktreeId, next)
  // Why: once this write settles, clear the queue entry only if no later
  // write has been chained on top. Otherwise the map should keep pointing at
  // the latest tail so subsequent enqueues chain onto the real in-flight
  // tail, not a stale resolved promise. Use `then(cleanup, cleanup)` (not
  // `finally`) so a rejection on `next` is fully consumed by this branch —
  // otherwise the `.finally()` chain propagates the rejection as an
  // unhandledRejection even though the caller `await`s `next` in its own
  // try/catch.
  const cleanup = (): void => {
    if (persistQueueByWorktree.get(worktreeId) === next) {
      persistQueueByWorktree.delete(worktreeId)
    }
  }
  next.then(cleanup, cleanup)
  return next
}

// Why: derive the next comment list from the latest store snapshot inside
// the `set` updater so two concurrent writes (rapid add+delete, or a
// delete-while-add-in-flight) can't clobber each other via a stale closure.
function mutateComments(
  set: Parameters<StateCreator<AppState, [], [], DiffCommentsSlice>>[0],
  worktreeId: string,
  mutate: (existing: DiffComment[]) => DiffComment[] | null
): { previous: DiffComment[] | undefined; next: DiffComment[] } | null {
  const repoId = getRepoIdFromWorktreeId(worktreeId)
  let previous: DiffComment[] | undefined
  let next: DiffComment[] | null = null
  set((s) => {
    const repoList = s.worktreesByRepo[repoId]
    if (!repoList) {
      return {}
    }
    const target = repoList.find((w) => w.id === worktreeId)
    if (!target) {
      return {}
    }
    previous = target.diffComments
    const computed = mutate(previous ?? [])
    if (computed === null) {
      return {}
    }
    next = computed
    const nextList: Worktree[] = repoList.map((w) =>
      w.id === worktreeId ? { ...w, diffComments: computed } : w
    )
    return { worktreesByRepo: { ...s.worktreesByRepo, [repoId]: nextList } }
  })
  if (next === null) {
    return null
  }
  return { previous, next }
}

// Why: if the IPC write fails, the optimistic renderer state drifts from
// disk. Roll back so what the user sees always matches what will survive a
// reload.
//
// Identity guard: we only revert when the current diffComments array is
// strictly identical (===) to the `next` array this mutation produced. If
// another mutation has already landed (e.g. Add B succeeded while Add A was
// still in flight), it will have replaced the array with a different
// identity. In that case we must leave the newer state alone — rolling back
// to our stale `previous` would erase B along with the failed A.
function rollback(
  set: Parameters<StateCreator<AppState, [], [], DiffCommentsSlice>>[0],
  worktreeId: string,
  previous: DiffComment[] | undefined,
  expectedCurrent: DiffComment[]
): void {
  const repoId = getRepoIdFromWorktreeId(worktreeId)
  set((s) => {
    const repoList = s.worktreesByRepo[repoId]
    if (!repoList) {
      return {}
    }
    const target = repoList.find((w) => w.id === worktreeId)
    // Why: if the worktree was removed between the optimistic mutation and
    // this rollback, there is nothing to restore. Bail out before remapping
    // `repoList` so we don't allocate a new outer-array identity and trigger
    // spurious subscriber notifications.
    if (!target) {
      return {}
    }
    // Why: only roll back if no other mutation landed since this one. If a
    // later write already replaced the comments array with a different
    // identity, our stale `previous` would erase that newer state.
    if (target.diffComments !== expectedCurrent) {
      return {}
    }
    const nextList: Worktree[] = repoList.map((w) =>
      w.id === worktreeId ? { ...w, diffComments: previous } : w
    )
    return { worktreesByRepo: { ...s.worktreesByRepo, [repoId]: nextList } }
  })
}

export const createDiffCommentsSlice: StateCreator<AppState, [], [], DiffCommentsSlice> = (
  set,
  get
) => ({
  ensureDiffCommentsHydrated: (worktreeId) =>
    worktreeId ? hydrateFromAnnotationService(worktreeId, get, set) : Promise.resolve(),

  getDiffComments: (worktreeId) => {
    // Why: accept null/undefined so callers with an optional active worktree
    // can pass it through without allocating a fresh `[]` fallback each
    // render, which would defeat the `EMPTY_COMMENTS` sentinel's referential
    // stability and trigger spurious re-renders in useAppStore selectors.
    if (!worktreeId) {
      return EMPTY_COMMENTS as DiffComment[]
    }
    const worktree = findWorktreeById(get().worktreesByRepo, worktreeId)
    if (!worktree?.diffComments) {
      // Why: cast the frozen sentinel to the mutable `DiffComment[]` return
      // type. The array is frozen at runtime so accidental mutation throws;
      // the cast only hides the `readonly` marker from consumers that never
      // mutate the list in practice.
      return EMPTY_COMMENTS as DiffComment[]
    }
    return worktree.diffComments
  },

  addDiffComment: async (input) => {
    const comment: DiffComment = normalizeDiffComment({
      ...input,
      id: generateId(),
      createdAt: Date.now()
    })
    const result = mutateComments(set, input.worktreeId, (existing) => [...existing, comment])
    if (!result) {
      return null
    }
    try {
      // Why: enqueue through the per-worktree queue so concurrent mutations
      // cannot land on disk out of call order. The queued write reads the
      // latest store snapshot at dequeue time, so it will reflect any newer
      // mutation that landed after this one was enqueued.
      await enqueuePersist(input.worktreeId, get)
      get().recordFeatureInteraction?.('review-notes')
      return comment
    } catch (err) {
      console.error('Failed to persist diff comments:', err)
      // Why: rollback's identity guard will no-op if a later mutation has
      // already replaced the in-memory list, so losing a successful newer
      // write is not possible here even though we queued in order.
      rollback(set, input.worktreeId, result.previous, result.next)
      return null
    }
  },

  updateDiffComment: async (worktreeId, commentId, body) => {
    // Why: trim trailing whitespace but reject an entirely-empty edit so we
    // don't end up with a saved note that renders as a blank card. Callers
    // should treat `false` as "edit not committed" and keep the editor open
    // so the user can either type more or cancel explicitly.
    const trimmed = body.trim()
    if (!trimmed) {
      return false
    }

    // Why: look up the current state OUTSIDE mutateComments so we can
    // distinguish "comment missing" (return false — likely an edit-while-
    // deleted race; the card should keep its draft and not silently close)
    // from "body unchanged" (return true — benign no-op; the card can close
    // the editor without surfacing an error).
    const repoId = getRepoIdFromWorktreeId(worktreeId)
    const repoList = get().worktreesByRepo[repoId]
    const target = repoList?.find((w) => w.id === worktreeId)
    const existing = target?.diffComments ?? []
    const existingIdx = existing.findIndex((c) => c.id === commentId)
    if (existingIdx === -1) {
      return false
    }
    if (existing[existingIdx].body === trimmed) {
      return true
    }

    const result = mutateComments(set, worktreeId, (current) => {
      const idx = current.findIndex((c) => c.id === commentId)
      if (idx === -1) {
        return null
      }
      if (current[idx].body === trimmed) {
        return null
      }
      const next = current.slice()
      // Why: editing a previously-sent note makes the agent's copy stale, so
      // the note should become eligible for the next Send notes action.
      next[idx] = { ...current[idx], body: trimmed, sentAt: undefined }
      return next
    })
    if (!result) {
      // Why: between the pre-check and the set updater, the comment vanished
      // or another mutation already wrote the same body. Treat as success so
      // the caller closes its editor.
      return true
    }
    try {
      await enqueuePersist(worktreeId, get)
      return true
    } catch (err) {
      console.error('Failed to persist diff comments:', err)
      rollback(set, worktreeId, result.previous, result.next)
      return false
    }
  },

  clearDeliveredDiffComments: async (worktreeId, comments) => {
    if (comments.length === 0) {
      return true
    }
    const snapshotsById = new Map(comments.map((comment) => [comment.id, comment]))
    const result = mutateComments(set, worktreeId, (existing) => {
      const next = existing.filter((comment) => {
        const snapshot = snapshotsById.get(comment.id)
        // Why: delivery is async. If the user edits a note before the prompt
        // is accepted by the agent, the old snapshot was sent but the current
        // note is a fresh pending note and must stay visible.
        return !snapshot || !deliverySnapshotMatches(comment, snapshot)
      })
      return next.length === existing.length ? null : next
    })
    if (!result) {
      return true
    }
    try {
      await enqueuePersist(worktreeId, get)
      get().recordFeatureInteraction?.('review-notes')
      return true
    } catch (err) {
      console.error('Failed to persist diff comments:', err)
      rollback(set, worktreeId, result.previous, result.next)
      return false
    }
  },

  markDiffCommentsSent: async (worktreeId, commentIds, sentAt = Date.now()) => {
    if (commentIds.length === 0) {
      return true
    }
    const ids = new Set(commentIds)
    const result = mutateComments(set, worktreeId, (existing) => {
      let changed = false
      const next = existing.map((comment) => {
        if (!ids.has(comment.id) || comment.sentAt === sentAt) {
          return comment
        }
        changed = true
        return { ...comment, sentAt }
      })
      return changed ? next : null
    })
    if (!result) {
      return true
    }
    try {
      await enqueuePersist(worktreeId, get)
      get().recordFeatureInteraction?.('review-notes')
      return true
    } catch (err) {
      console.error('Failed to persist diff comments:', err)
      rollback(set, worktreeId, result.previous, result.next)
      return false
    }
  },

  deleteDiffComment: async (worktreeId, commentId) => {
    const result = mutateComments(set, worktreeId, (existing) => {
      const next = existing.filter((c) => c.id !== commentId)
      return next.length === existing.length ? null : next
    })
    if (!result) {
      return
    }
    try {
      // Why: enqueue through the per-worktree queue so concurrent mutations
      // cannot land on disk out of call order. See enqueuePersist for the
      // ordering invariant.
      await enqueuePersist(worktreeId, get)
    } catch (err) {
      console.error('Failed to persist diff comments:', err)
      rollback(set, worktreeId, result.previous, result.next)
    }
  },

  clearDiffComments: async (worktreeId) => {
    const result = mutateComments(set, worktreeId, (existing) =>
      existing.length === 0 ? null : []
    )
    if (!result) {
      return true
    }
    try {
      await enqueuePersist(worktreeId, get)
      return true
    } catch (err) {
      console.error('Failed to persist diff comments:', err)
      rollback(set, worktreeId, result.previous, result.next)
      return false
    }
  },

  clearDiffCommentsForFile: async (worktreeId, filePath) => {
    const result = mutateComments(set, worktreeId, (existing) => {
      const next = existing.filter((c) => c.filePath !== filePath)
      return next.length === existing.length ? null : next
    })
    if (!result) {
      return true
    }
    try {
      await enqueuePersist(worktreeId, get)
      return true
    } catch (err) {
      console.error('Failed to persist diff comments:', err)
      rollback(set, worktreeId, result.previous, result.next)
      return false
    }
  }
})
