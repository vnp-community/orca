// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAppStore } from '../../store'
import type { Repo } from '../../../../shared/types'
import { RepositoryDevServerHostSection } from './RepositoryDevServerHostSection'

const repo = (over: Partial<Repo> = {}): Repo => ({
  id: 'repo-1',
  path: '/srv/app',
  displayName: 'app',
  badgeColor: '',
  addedAt: 1,
  ...over
})

async function renderSection(value: Repo): Promise<HTMLDivElement> {
  const container = document.createElement('div')
  document.body.appendChild(container)
  await act(async () => {
    createRoot(container).render(<RepositoryDevServerHostSection repo={value} forceVisible />)
  })
  return container
}

describe('RepositoryDevServerHostSection', () => {
  const openModal = vi.fn()

  beforeEach(() => {
    ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    vi.clearAllMocks()
    useAppStore.setState({
      openModal,
      settingsSearchQuery: '',
      devServers: [{ id: 'ds-1', name: 'Mac Studio', status: 'connected' }]
    } as never)
    ;(window as unknown as { __ORCA_WEB_CLIENT__?: boolean }).__ORCA_WEB_CLIENT__ = true
  })

  afterEach(() => {
    delete (window as unknown as { __ORCA_WEB_CLIENT__?: boolean }).__ORCA_WEB_CLIENT__
    document.body.replaceChildren()
  })

  it('shows the dev server and its connection state, and offers to change it', async () => {
    const container = await renderSection(repo({ devServerId: 'ds-1' }))

    expect(container.querySelector('[data-testid="repo-host-summary"]')?.textContent).toBe(
      'Mac Studio · Connected'
    )
    expect(container.querySelector('[data-testid="repo-host-unbound-warning"]')).toBeNull()
    await act(async () => {
      container.querySelector<HTMLButtonElement>('[data-testid="repo-host-change"]')?.click()
    })
    expect(openModal).toHaveBeenCalledWith('repo-host-picker', { repoId: 'repo-1' })
  })

  it('says a host inherited from the project is inherited', async () => {
    const container = await renderSection(repo({ devServerId: 'ds-1', devServerInherited: true }))
    expect(container.querySelector('[data-testid="repo-host-summary"]')?.textContent).toBe(
      'Mac Studio · Connected · from the project'
    )
  })

  it('warns inline about a repo with no dev server instead of showing a host', async () => {
    const container = await renderSection(repo({ devServerId: '' }))

    expect(
      container.querySelector('[data-testid="repo-host-unbound-warning"]')?.textContent
    ).toContain('No dev server is assigned')
    expect(container.querySelector('[data-testid="repo-host-summary"]')).toBeNull()
  })

  it('renders nothing on desktop', async () => {
    delete (window as unknown as { __ORCA_WEB_CLIENT__?: boolean }).__ORCA_WEB_CLIENT__
    const container = await renderSection(repo({ devServerId: 'ds-1' }))
    expect(container.innerHTML).toBe('')
  })
})
