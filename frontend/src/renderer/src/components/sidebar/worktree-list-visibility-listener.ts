// Extracted from WorktreeList.tsx (TASK-BIGFILE-013). Logic is copied
// verbatim; this file exists only to shrink the source file's line count,
// not to change behavior.

export function installWorktreeVisibleRefreshVisibilityListener(onChange: () => void): () => void {
  document.addEventListener('visibilitychange', onChange)
  return () => document.removeEventListener('visibilitychange', onChange)
}
