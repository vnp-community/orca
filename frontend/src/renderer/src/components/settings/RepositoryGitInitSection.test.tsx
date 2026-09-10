// @vitest-environment happy-dom

import React, { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Repo } from '../../../../shared/types'
import { useAppStore } from '../../store'
import { RepositoryGitInitSection } from './RepositoryGitInitSection'

const checkRepoIsNotAGitRepo = vi.fn()
vi.mock('@/lib/repo-git-status-check', () => ({
  checkRepoIsNotAGitRepo: (...args: unknown[]) => checkRepoIsNotAGitRepo(...args)
}))

let container: HTMLDivElement
let root: Root

function makeRepo(overrides: Partial<Repo> & Pick<Repo, 'id' | 'displayName' | 'path'>): Repo {
  return {
    badgeColor: '#737373',
    addedAt: 100,
    kind: 'git',
    projectId: 'default-project',
    ...overrides
  }
}

beforeEach(() => {
  checkRepoIsNotAGitRepo.mockReset()
  useAppStore.setState(useAppStore.getInitialState(), true)
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
})

afterEach(() => {
  act(() => {
    root.unmount()
  })
  container.remove()
  useAppStore.setState(useAppStore.getInitialState(), true)
})

async function renderSection(repo: Repo): Promise<void> {
  await act(async () => {
    root.render(
      React.createElement(RepositoryGitInitSection, {
        repo,
        forceVisible: true,
        searchQuery: '',
        searchEntries: []
      })
    )
    await Promise.resolve()
  })
}

describe('RepositoryGitInitSection', () => {
  it('renders nothing while the check is pending or resolves to a real git repo', async () => {
    checkRepoIsNotAGitRepo.mockResolvedValue(false)
    const repo = makeRepo({ id: 'aiops-v3', displayName: 'aiops-v3', path: '/opt/aiops-v3' })

    await renderSection(repo)

    expect(checkRepoIsNotAGitRepo).toHaveBeenCalledWith(
      { id: 'aiops-v3', projectId: 'default-project' },
      { activeRuntimeEnvironmentId: null }
    )
    expect(container.textContent).toBe('')
  })

  it('offers to initialize the repo when the check confirms it is not a git repo', async () => {
    checkRepoIsNotAGitRepo.mockResolvedValue(true)
    const openModal = vi.fn()
    useAppStore.setState({ openModal })
    const repo = makeRepo({ id: 'aiops-v3', displayName: 'aiops-v3', path: '/opt/aiops-v3' })

    await renderSection(repo)

    expect(container.textContent).toContain('Initialize Git repo')
    const button = Array.from(container.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Initialize as Git repo')
    )
    expect(button).toBeTruthy()

    act(() => {
      button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(openModal).toHaveBeenCalledTimes(1)
    const [modal, data] = openModal.mock.calls[0] as [string, Record<string, unknown>]
    expect(modal).toBe('init-repo-as-git')
    expect(data.repoId).toBe('aiops-v3')
    expect(data.folderPath).toBe('/opt/aiops-v3')
    expect(typeof data.onSuccess).toBe('function')
  })
})
