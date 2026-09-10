import { describe, expect, it } from 'vitest'
import { toDevServerExecutionHostId } from '../../../../shared/execution-host'
import { getFileExplorerOperationOwnerFromState } from './file-explorer-operation-owner'

type OwnerState = Parameters<typeof getFileExplorerOperationOwnerFromState>[0]

function stateForWorktree(hostId: string): OwnerState {
  return {
    settings: { activeRuntimeEnvironmentId: null },
    repos: [],
    worktreesByRepo: {
      'repo-1': [{ id: 'wt-1', hostId }]
    },
    detectedWorktreesByRepo: {},
    folderWorkspaces: [],
    projectGroups: [],
    restoredRuntimeHostIdByWorkspaceSessionKey: {}
  } as unknown as OwnerState
}

// Why: Phase 10's per-repo dev-server binding (repo.devServerId) surfaces
// as a worktree hostId of the form 'devServer:<id>' — a real, live shape
// once a repo is actually assigned to a dev server (AssignRepoToProject,
// ProjectDevServerSection). FileExplorerOperationOwner has no 'devServer'
// variant; operationOwnerFromHostId's switch previously had no case for it
// and returned `undefined` for the whole owner, crashing every caller of
// getFileExplorerOperationRoute (Quick Open, File Explorer, "+ New tab")
// the first time a worktree resolved to a devServer: host — found live on
// "aiops-v3" after it was assigned to a dev server this session.
describe('getFileExplorerOperationOwnerFromState', () => {
  it('resolves a devServer-hosted worktree to "unresolved" instead of returning undefined', () => {
    const state = stateForWorktree(toDevServerExecutionHostId('ds-1'))
    const owner = getFileExplorerOperationOwnerFromState(state, 'wt-1')
    expect(owner).toEqual({ kind: 'unresolved' })
  })

  it('still resolves a local-hosted worktree correctly', () => {
    const state = stateForWorktree('local')
    const owner = getFileExplorerOperationOwnerFromState(state, 'wt-1')
    expect(owner).toEqual({ kind: 'local' })
  })
})
