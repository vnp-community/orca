/**
 * reading-progress-merge.ts — FE-CV-TASK-052-02
 *
 * Merges reading progress entries from multiple sources and
 * enforces a 64 KiB size guard for serialization.
 *
 * @module components/review-map/reading-progress-merge
 */

import type { ReadingProgressEntry, SerializedReadingProgress } from './reading-order-model'

// 64 KiB limit for serialized progress
const MAX_PROGRESS_BYTES = 64 * 1024
const PROGRESS_VERSION = 1

/**
 * Merge two sets of reading progress entries.
 * More recent `updatedAt` wins on conflict.
 * Result is sorted by path for stable serialization.
 */
export function mergeReadingProgress(
  base: ReadingProgressEntry[],
  incoming: ReadingProgressEntry[]
): ReadingProgressEntry[] {
  const merged = new Map<string, ReadingProgressEntry>()

  for (const entry of base) {
    merged.set(entry.path, entry)
  }

  for (const entry of incoming) {
    const existing = merged.get(entry.path)
    if (!existing || entry.updatedAt > existing.updatedAt) {
      merged.set(entry.path, entry)
    }
  }

  return Array.from(merged.values()).sort((a, b) => a.path.localeCompare(b.path))
}

/**
 * Serialize reading progress to JSON string.
 * Returns null if the serialized size exceeds 64 KiB.
 */
export function serializeReadingProgress(
  entries: ReadingProgressEntry[]
): string | null {
  const payload: SerializedReadingProgress = { entries, version: PROGRESS_VERSION }
  const json = JSON.stringify(payload)
  const bytes = new TextEncoder().encode(json).length

  if (bytes > MAX_PROGRESS_BYTES) {
    // Over size limit — trim oldest entries until it fits
    const sorted = [...entries].sort((a, b) => a.updatedAt.localeCompare(b.updatedAt))
    while (sorted.length > 0) {
      sorted.shift() // Remove oldest
      const trimmed = JSON.stringify({ entries: sorted, version: PROGRESS_VERSION })
      if (new TextEncoder().encode(trimmed).length <= MAX_PROGRESS_BYTES) {
        return trimmed
      }
    }
    return null
  }

  return json
}

/**
 * Deserialize reading progress from JSON string.
 * Returns empty entries on parse failure (never throws).
 */
export function deserializeReadingProgress(json: string): ReadingProgressEntry[] {
  try {
    const parsed = JSON.parse(json) as { entries?: unknown[]; version?: unknown }
    if (!Array.isArray(parsed.entries)) return []

    return parsed.entries
      .filter((e): e is ReadingProgressEntry =>
        typeof e === 'object' &&
        e !== null &&
        typeof (e as ReadingProgressEntry).path === 'string' &&
        typeof (e as ReadingProgressEntry).status === 'string' &&
        typeof (e as ReadingProgressEntry).updatedAt === 'string'
      )
  } catch {
    return []
  }
}
