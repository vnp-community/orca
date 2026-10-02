export type McpToolText = { name: string; description: string }

export type McpToolDiff = {
  added: McpToolText[]
  removed: McpToolText[]
  changed: { name: string; before: string; after: string }[]
  unchanged: McpToolText[]
}

/** Compares the current probe against the last approved set by tool name. */
export function diffTools(
  current: readonly McpToolText[],
  approved: readonly McpToolText[]
): McpToolDiff {
  const old = new Map(approved.map((t) => [t.name, t]))
  const now = new Set(current.map((t) => t.name))
  const diff: McpToolDiff = { added: [], removed: [], changed: [], unchanged: [] }
  for (const t of current) {
    const prev = old.get(t.name)
    if (!prev) {
      diff.added.push(t)
    } else if (prev.description !== t.description) {
      diff.changed.push({ name: t.name, before: prev.description, after: t.description })
    } else {
      diff.unchanged.push(t)
    }
  }
  diff.removed = approved.filter((t) => !now.has(t.name))
  return diff
}

export const hasToolChanges = (d: McpToolDiff): boolean =>
  d.added.length + d.removed.length + d.changed.length > 0
