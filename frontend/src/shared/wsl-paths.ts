export type WslUncPathInfo = {
  distro: string
  linuxPath: string
}

export function parseWslUncPath(path: string): WslUncPathInfo | null {
  // Why the guard: a ProjectHostSetup derived locally from a bare
  // project-service Repo/RemoteRepoView (no `path` field on the wire —
  // only `url`) can reach this via worktree-list-groups.ts's
  // getProjectSetupSurfaceKey with `path` genuinely undefined at runtime,
  // even though the TS type says `string`. Same bug class as
  // repo-display-labels.ts's normalizePathSegments guard (found live:
  // once the sidebar's "local-first" bootstrap fetch (web-preload-api.ts)
  // actually reached this projection for a real project-scoped repo, an
  // undefined path crashed the whole sidebar list, contained by an error
  // boundary but still a real regression).
  if (!path) {
    return null
  }
  const normalized = path.replace(/\\/g, '/')
  const match = normalized.match(/^\/\/(wsl\.localhost|wsl\$)\/([^/]+)(\/.*)?$/i)
  if (!match) {
    return null
  }

  return {
    distro: match[2],
    linuxPath: match[3] || '/'
  }
}

export function isWslUncPath(path: string): boolean {
  return parseWslUncPath(path) !== null
}

// Why: Windows folds the share (\\wsl$ aliases \\wsl.localhost), the distro, and
// drvfs /mnt/<drive> tails case-insensitively; the rest of the Linux path is not.
export function foldWslUncPathCaseInsensitiveParts(path: string): string | null {
  const parsed = parseWslUncPath(path)
  if (!parsed) {
    return null
  }
  // Why: the drvfs automount is literally lowercase /mnt — a case-variant like
  // /MNT is an ordinary case-sensitive Linux dir and must not be folded.
  const linuxPath = /^\/mnt\/[a-zA-Z](?:\/|$)/.test(parsed.linuxPath)
    ? parsed.linuxPath.toLowerCase()
    : parsed.linuxPath
  return `//wsl.localhost/${parsed.distro.toLowerCase()}${linuxPath === '/' ? '' : linuxPath}`
}
