/**
 * WorkspaceContextBridge — Compile-time selector (C2 conflict resolution)
 *
 * __ORCA_WORKSPACE_V6__ = true  → dùng WorkspaceContextV6
 * (default)                     → dùng WorkspaceContext (v5, giữ nguyên)
 *
 * App.tsx import từ đây thay vì import trực tiếp.
 *
 * NOTE: Conditional `export * from ternary` là Babel/Vite syntax — TypeScript
 * compiler không hỗ trợ. Dùng selector function thay thế.
 */
import { WorkspaceProvider, useWorkspace } from './WorkspaceContext'
import { WorkspaceProviderV6, useWorkspaceV6 } from './WorkspaceContextV6'

export { WorkspaceProvider } from './WorkspaceContext'
export { useWorkspace } from './WorkspaceContext'
export { WorkspaceProviderV6 } from './WorkspaceContextV6'
export { useWorkspaceV6 } from './WorkspaceContextV6'

declare const __ORCA_WORKSPACE_V6__: boolean

/**
 * Returns the active WorkspaceProvider component based on build flag.
 */
export function getWorkspaceProvider() {
  return __ORCA_WORKSPACE_V6__ ? WorkspaceProviderV6 : WorkspaceProvider
}

/**
 * Returns the active useWorkspace hook based on build flag.
 */
export function getUseWorkspace() {
  return __ORCA_WORKSPACE_V6__ ? useWorkspaceV6 : useWorkspace
}
