/**
 * AnnotationStore — persistence for inline code-review comments (BL-CR-02).
 *
 * Backs annotation.list / annotation.create (orca_annotations, migration 0018).
 * Follows ProjectService's pattern: pool.withConnection((db) => db.query(...)),
 * `?` placeholders, client-generated UUID ids.
 *
 * Exposed as a bootstrap-initialized singleton (init/get pair) rather than
 * threaded through RpcContext — same shape as credentials/index.ts's
 * WebCredentialStore, since the flat RpcMethod[] namespace files (see
 * runtime/rpc/methods/index.ts) aren't constructor-injected.
 *
 * @module main/code-review/annotation-store
 */

import { randomUUID } from 'node:crypto'
import type { IConnectionPool } from '../db/pool'

export type Annotation = {
  id: string
  lineNumber: number
  filePath: string
  content: string
  author: string
  authorInitials: string
  createdAt: number
}

export type ListAnnotationsParams = {
  projectId: string
  reviewId?: string
  filePath: string
  lineNumber: number
}

export type CreateAnnotationParams = {
  projectId: string
  reviewId?: string
  filePath: string
  lineNumber: number
  content: string
  authorId: string
}

type AnnotationRow = {
  id: string
  lineNumber: number
  filePath: string
  content: string
  authorId: string
  createdAt: number
}

// Why: no user-profile display-name lookup is wired into this small
// feature-owned store yet — derive a stable, readable label from the
// authenticated userId (email-local-part or raw id) instead of guessing.
function authorDisplayName(authorId: string): string {
  const localPart = authorId.includes('@') ? authorId.split('@')[0] : authorId
  return localPart || authorId
}

function initialsFromName(name: string): string {
  const parts = name.split(/[\s._-]+/).filter(Boolean)
  if (parts.length === 0) {return '?'}
  if (parts.length === 1) {return parts[0].slice(0, 2).toUpperCase()}
  return (parts[0][0] + parts[1][0]).toUpperCase()
}

function rowToAnnotation(row: AnnotationRow): Annotation {
  const author = authorDisplayName(row.authorId)
  return {
    id: row.id,
    lineNumber: row.lineNumber,
    filePath: row.filePath,
    content: row.content,
    author,
    authorInitials: initialsFromName(author),
    createdAt: row.createdAt
  }
}

export class AnnotationStore {
  constructor(private readonly pool: IConnectionPool) {}

  async list(params: ListAnnotationsParams): Promise<Annotation[]> {
    const conditions = ['project_id = ?', 'file_path = ?', 'line_number = ?']
    const values: (string | number)[] = [params.projectId, params.filePath, params.lineNumber]
    if (params.reviewId) {
      conditions.push('review_id = ?')
      values.push(params.reviewId)
    }
    const rows = await this.pool.withConnection((db) =>
      db.query<AnnotationRow>(
        `SELECT
           id,
           line_number as "lineNumber",
           file_path as "filePath",
           body as content,
           author_id as "authorId",
           created_at as "createdAt"
         FROM orca_annotations
         WHERE ${conditions.join(' AND ')}
         ORDER BY created_at ASC`,
        values
      )
    )
    return rows.map(rowToAnnotation)
  }

  async create(params: CreateAnnotationParams): Promise<Annotation> {
    const id = randomUUID()
    const now = Date.now()
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT INTO orca_annotations
           (id, project_id, review_id, file_path, line_number, body, author_id, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [
          id,
          params.projectId,
          params.reviewId ?? null,
          params.filePath,
          params.lineNumber,
          params.content,
          params.authorId,
          now
        ]
      )
    )
    return rowToAnnotation({
      id,
      lineNumber: params.lineNumber,
      filePath: params.filePath,
      content: params.content,
      authorId: params.authorId,
      createdAt: now
    })
  }
}

let _store: AnnotationStore | null = null

/** Initialize the singleton AnnotationStore. Called once during server bootstrap. */
export function initAnnotationStore(pool: IConnectionPool): void {
  _store = new AnnotationStore(pool)
}

/**
 * Return the initialized singleton AnnotationStore.
 * @throws Error if initAnnotationStore() has not been called yet.
 */
export function getAnnotationStore(): AnnotationStore {
  if (!_store) {
    throw new Error('[AnnotationStore] Not initialized. Call initAnnotationStore() first.')
  }
  return _store
}

/** FOR TESTING ONLY — reset the singleton between test files. */
export function resetAnnotationStore(): void {
  _store = null
}
