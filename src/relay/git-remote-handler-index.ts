/**
 * git-remote-handler-index.ts — Compile-time selector (Conflict C1 resolution)
 *
 * __ORCA_GIT_V6__ = true  → dùng gitRemoteHandlersV6 (v6 high-level methods)
 * __ORCA_GIT_V6__ = false → dùng gitRemoteHandlers  (v5: chỉ git.exec, git.execStream)
 *
 * Bundler (Vite) tree-shakes unused branch at compile time.
 *
 * Env: ORCA_FEATURE_GIT_V6=true pnpm dev   → v6
 *      pnpm dev                             → v5 (default)
 *
 * NOTE: Conditional `export * from ternary` is Babel/Vite only — TypeScript
 * compiler doesn't support it. We use a selector function instead.
 */
import { gitRemoteHandlers } from './git-remote-handler'
import { gitRemoteHandlersV6 } from './git-remote-handler-v6'

export type { GitExecResult } from './git-remote-handler'
export { validateGitArgs, ALLOWED_GIT_SUBCOMMANDS } from './git-remote-handler'
export { gitRemoteHandlersV6 } from './git-remote-handler-v6'
export { gitRemoteHandlers } from './git-remote-handler'

declare const __ORCA_GIT_V6__: boolean

/**
 * Returns the active git remote handlers based on build flag.
 * Use this instead of importing gitRemoteHandlers/gitRemoteHandlersV6 directly
 * to benefit from Vite tree-shaking.
 */
export function getGitRemoteHandlers() {
  return __ORCA_GIT_V6__ ? gitRemoteHandlersV6 : gitRemoteHandlers
}
