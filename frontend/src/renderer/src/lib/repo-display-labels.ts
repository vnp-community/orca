type RepoDisplayLabelItem = {
  path: string
  displayName: string
}

// Why the guard: a repo created through project-service's Repo model
// ({id, projectId, url, displayName, position} — no `path` field at all)
// can reach this helper via the legacy sidebar's tenant-wide repo.list
// fetch (store/slices/repos.ts's fetchRepoCatalogForTarget has no project
// filter) — the TS type here says `path: string`, but that's a lie at
// runtime for this shape of repo. Live-reproduced: `path.replace` on
// undefined threw, crashing the whole sidebar list (contained by an error
// boundary, but still a real regression) the moment 2+ repos with the same
// (or both-empty) displayName triggered the collision-labeling path below.
function normalizePathSegments(path: string): string[] {
  if (!path) {
    return []
  }
  return path.replace(/\\/g, '/').replace(/\/+$/g, '').split('/').filter(Boolean)
}

function labelForDepth(item: RepoDisplayLabelItem, depth: number): string {
  const segments = normalizePathSegments(item.path)
  const suffix = segments.slice(Math.max(0, segments.length - depth))
  if (suffix.length === 0) {
    return item.displayName
  }
  suffix[suffix.length - 1] = item.displayName
  return suffix.join('/')
}

function hasDuplicateLabels(labels: readonly string[]): boolean {
  return new Set(labels).size !== labels.length
}

export function getRepoDisplayLabelsByPath(
  items: readonly RepoDisplayLabelItem[]
): Map<string, string> {
  const labels = new Map<string, string>()
  const itemsByName = new Map<string, RepoDisplayLabelItem[]>()

  for (const item of items) {
    const displayName = item.displayName || item.path
    labels.set(item.path, displayName)
    const colliding = itemsByName.get(displayName) ?? []
    colliding.push({ ...item, displayName })
    itemsByName.set(displayName, colliding)
  }

  for (const collidingItems of itemsByName.values()) {
    if (collidingItems.length < 2) {
      continue
    }
    const maxDepth = Math.max(
      ...collidingItems.map((item) => normalizePathSegments(item.path).length)
    )
    let depth = 1
    let nextLabels = collidingItems.map((item) => labelForDepth(item, depth))
    while (depth < maxDepth && hasDuplicateLabels(nextLabels)) {
      depth += 1
      nextLabels = collidingItems.map((item) => labelForDepth(item, depth))
    }
    collidingItems.forEach((item, index) => {
      labels.set(item.path, nextLabels[index] ?? item.displayName)
    })
  }

  return labels
}
