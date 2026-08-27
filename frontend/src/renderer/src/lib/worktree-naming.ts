/**
 * Sanitize a worktree name for use in branch names and directory paths.
 * Strips unsafe characters and collapses runs of special chars to a single hyphen.
 *
 * Relocated from the dead `frontend/src/main/ipc/worktree-logic.ts` tree
 * (never-runnable Electron-main-process code left over from before `desktop/`
 * was split into its own package) — this is a pure string sanitizer with no
 * Electron/IPC dependency, so it belongs in the renderer's own lib instead.
 * Near-identical copies of the real logic live in `backend/src/main/ipc/
 * worktree-logic.ts` and `desktop/src/main/ipc/worktree-logic.ts`.
 */
export function sanitizeWorktreeName(input: string): string {
  // Why: keep Unicode letters/numbers (CJK, accented Latin, etc.) so users can
  // name workspaces in their own language. Git ref-format permits non-ASCII
  // bytes, and modern filesystems handle UTF-8 paths. Only strip characters
  // git or the filesystem actually rejects.
  const sanitized = input
    .trim()
    .replace(/[^\p{L}\p{N}._-]+/gu, '-')
    .replace(/-+/g, '-')
    // Why: git check-ref-format rejects any ref containing `..`, so a prompt
    // like "../../foo" that survives slugification as `..-..-foo` would
    // produce a branch name git refuses to create. Collapse runs of dots
    // to a single dot before the leading/trailing trim so internal `..`
    // sequences can't reach git.
    .replace(/\.{2,}/g, '.')
    .replace(/^[.-]+|[.-]+$/g, '')

  if (!sanitized || sanitized === '.' || sanitized === '..') {
    throw new Error('Invalid worktree name')
  }

  return sanitized
}
