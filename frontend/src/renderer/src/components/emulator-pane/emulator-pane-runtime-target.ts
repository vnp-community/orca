// emulator-pane-runtime-target.ts — CR-DS-009 / TASK-EMU-013: resolves the
// RuntimeClientTarget + projectId that emulator.* RPC calls should use.
//
// Why this exists: emulator.* calls used to hardcode `{ kind: 'local' }`,
// which only ever worked for Electron desktop (the local Mobile Emulator
// device.* handlers live in-process there). TASK-EMU-009 made backend-go's
// `emulator.*` wscompat channels resolve the target Mobile Emulator Agent
// from `projectId -> project.mobileEmulatorAgentId -> ResolveConnection`
// instead of the (git-specific, wrong-shape) connectionId they used before
// — so a `{ kind: 'environment' }` caller now needs to send `projectId`
// too. `getActiveRuntimeTarget` already resolves to `{ kind: 'local' }`
// whenever no environment is active (the desktop default), so swapping the
// hardcoded target for it is additive: desktop behavior is unchanged, and
// only an active `{ kind: 'environment' }` target starts sending projectId.
//
// projectId source: worktree.projectId (Worktree's "durable project
// identity" field — see frontend/src/shared/types.ts) via the store's
// getKnownWorktreeById, the same lookup workspace-port-localhost-label-
// selector.ts uses for the equivalent worktreeId -> projectId resolution.
// Absent for legacy repo-only workspaces that predate the Project entity —
// callers must tolerate `projectId` being undefined (backend-go's
// resolveEmulatorConnectionID already treats an empty projectId as the
// existing "not supported" honest-stub path, not an error).
import { useAppStore } from '@/store'
import { getActiveRuntimeTarget, type RuntimeClientTarget } from '@/runtime/runtime-rpc-client'

export type EmulatorPaneRuntimeTarget = {
  target: RuntimeClientTarget
  projectId: string | undefined
}

export function resolveEmulatorPaneRuntimeTarget(worktreeId: string): EmulatorPaneRuntimeTarget {
  const state = useAppStore.getState()
  const target = getActiveRuntimeTarget(state.settings)
  const worktree = state.getKnownWorktreeById?.(worktreeId)
  return { target, projectId: worktree?.projectId }
}
