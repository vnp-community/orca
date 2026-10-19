import { execFileSync } from 'node:child_process'

/** True when git itself accepts the folder as a work tree; a leftover `.git` directory that git rejects cannot be fingerprinted. */
export function isUsableGitRepo(path: string): boolean {
  try {
    return (
      execFileSync('git', ['-C', path, 'rev-parse', '--is-inside-work-tree'], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'ignore']
      }).trim() === 'true'
    )
  } catch {
    return false
  }
}
