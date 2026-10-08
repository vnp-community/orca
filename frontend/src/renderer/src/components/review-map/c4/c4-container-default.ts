/**
 * c4-container-default.ts — FE-CV-TASK-055-02
 */

import type { ContainerRef } from '../../../../../shared/code-intel-architecture-types'
import { normalizeRepoPath } from './c4-overlay-model'

/**
 * Container holding the most changed files (by path prefix); ties break by name, then id.
 * Why not first-listed: server order is arbitrary and the reviewer cares about what changed.
 */
export function pickDefaultContainer(
  containers: readonly ContainerRef[],
  changedFiles: readonly string[]
): ContainerRef | null {
  if (containers.length === 0) {
    return null
  }
  const files = changedFiles.map(normalizeRepoPath)
  let best: { container: ContainerRef; hits: number } | null = null
  for (const container of containers) {
    const prefix = normalizeRepoPath(container.path)
    const hits = files.filter((f) => prefix === '' || prefix === '.' || f === prefix || f.startsWith(`${prefix}/`)).length
    if (
      best === null ||
      hits > best.hits ||
      (hits === best.hits &&
        (container.name.localeCompare(best.container.name) < 0 ||
          (container.name === best.container.name && container.id.localeCompare(best.container.id) < 0)))
    ) {
      best = { container, hits }
    }
  }
  if (best && best.hits > 0) {
    return best.container
  }
  return containers[0]
}
