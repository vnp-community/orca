/**
 * Plain data shapes for workspace git/file-tree state, consumed by
 * `WorkspaceContextV6.tsx` as the response type of the `workspace.refreshGitStatus`
 * RPC call.
 *
 * Relocated from the dead `frontend/src/main/workspace/WorkspaceService.ts`
 * tree (never-runnable Electron-main-process code left over from before
 * `desktop/` was split into its own package) — these are pure data types with
 * no coupling to the real `WorkspaceService` class (which stays back-end side;
 * it depends on server-only services like `ProjectServerRouter`).
 */

export type GitStatusFile = {
  path: string
  x: string // staged status char
  y: string // unstaged status char
}

export type GitStatus = {
  branch: string
  upstream?: string
  ahead: number
  behind: number
  staged: number
  unstaged: number
  untracked: number
  files: GitStatusFile[]
}

export type GitWorktree = {
  path: string
  branch: string
  head: string
  isMain: boolean
  isLocked: boolean
}

export type FileTreeNode = {
  name: string
  path: string
  isDir: boolean
  children?: FileTreeNode[]
}
