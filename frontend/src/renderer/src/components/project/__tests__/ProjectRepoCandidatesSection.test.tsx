// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { ProjectRepoCandidatesSection } from '../ProjectRepoCandidatesSection'
import { callRuntimeRpc } from '../../../runtime/runtime-rpc-client'

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  callRuntimeRpc: vi.fn(),
  getActiveRuntimeTarget: vi.fn().mockReturnValue({ type: 'local' }),
  RuntimeRpcCallError: class RuntimeRpcCallError extends Error {}
}))

vi.mock('../../../store', () => ({
  useAppStore: { getState: vi.fn().mockReturnValue({ settings: {} }) }
}))

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const mockRpc = vi.mocked(callRuntimeRpc)

const allRepos = [
  {
    id: 'vnp-asm',
    projectId: 'other-project',
    url: '/srv/vnp-asm',
    displayName: 'vnp-asm',
    position: 0,
    devServerId: 'ds-1'
  },
  {
    id: 'aiops-v3',
    projectId: 'other-project',
    url: '/opt/aiops-v3',
    displayName: 'aiops-v3',
    position: 1,
    devServerId: 'ds-2'
  },
  {
    id: 'already-here',
    projectId: 'p1',
    url: '/srv/already',
    displayName: 'already-here',
    position: 0,
    devServerId: 'ds-1'
  }
]

describe('ProjectRepoCandidatesSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'repo.list') {
        return Promise.resolve({ repos: allRepos })
      }
      return Promise.resolve(undefined)
    })
  })

  afterEach(cleanup)

  it('lists tenant-wide repos via repo.list with no projectId, excluding repos already in this project', async () => {
    render(
      <ProjectRepoCandidatesSection
        projectId="p1"
        existingRepoIds={new Set(['already-here'])}
        selectedDevServerIds={new Set()}
        onAdded={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'repo.list', {})
    })
    expect(screen.getByTestId('repo-candidate-vnp-asm')).toBeInTheDocument()
    expect(screen.getByTestId('repo-candidate-aiops-v3')).toBeInTheDocument()
    expect(screen.queryByTestId('repo-candidate-already-here')).not.toBeInTheDocument()
  })

  it('narrows the candidate list by the selected dev server(s)', async () => {
    render(
      <ProjectRepoCandidatesSection
        projectId="p1"
        existingRepoIds={new Set(['already-here'])}
        selectedDevServerIds={new Set(['ds-2'])}
        onAdded={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByTestId('repo-candidate-aiops-v3')).toBeInTheDocument()
    })
    expect(screen.queryByTestId('repo-candidate-vnp-asm')).not.toBeInTheDocument()
  })

  it('calls repo.assignToProject and refreshes on Add', async () => {
    const onAdded = vi.fn()
    render(
      <ProjectRepoCandidatesSection
        projectId="p1"
        existingRepoIds={new Set(['already-here'])}
        selectedDevServerIds={new Set()}
        onAdded={onAdded}
      />
    )
    await waitFor(() =>
      expect(screen.getByTestId('repo-candidate-add-vnp-asm')).toBeInTheDocument()
    )

    fireEvent.click(screen.getByTestId('repo-candidate-add-vnp-asm'))

    await waitFor(() => {
      expect(mockRpc).toHaveBeenCalledWith({ type: 'local' }, 'repo.assignToProject', {
        repoId: 'vnp-asm',
        targetProjectId: 'p1'
      })
      expect(onAdded).toHaveBeenCalled()
    })
  })

  it('shows an empty message when there are no candidates', async () => {
    mockRpc.mockImplementation((_target, method) => {
      if (method === 'repo.list') {
        return Promise.resolve({ repos: [] })
      }
      return Promise.resolve(undefined)
    })
    render(
      <ProjectRepoCandidatesSection
        projectId="p1"
        existingRepoIds={new Set()}
        selectedDevServerIds={new Set()}
        onAdded={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('No other repos available to add.')).toBeInTheDocument()
    })
  })
})
