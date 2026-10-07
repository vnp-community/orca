import path from 'path'

export type RepoRelativePath = {
  file: string
  outside: boolean
}

export function toRepoRelative(
  printed: string,
  cwd: string,
  repoRoot: string,
  platform: NodeJS.Platform
): RepoRelativePath {
  const pathModule = platform === 'win32' ? path.win32 : path.posix

  const isAbsolute = pathModule.isAbsolute(printed)
  const absPath = isAbsolute ? printed : pathModule.resolve(cwd, printed)

  const relative = pathModule.relative(repoRoot, absPath)

  if (
    relative === '' ||
    relative === '..' ||
    relative.startsWith('..' + pathModule.sep) ||
    pathModule.isAbsolute(relative) // different drive on win32
  ) {
    if (relative === '') {
      return { file: '', outside: false } // Repository root itself
    }
    return { file: '', outside: true }
  }

  const normalized = relative.split(pathModule.sep).join('/')
  return { file: normalized, outside: false }
}
