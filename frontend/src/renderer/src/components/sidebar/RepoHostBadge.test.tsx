// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAppStore } from '@/store'
import type { Repo } from '../../../../shared/types'
import { RepoHostBadge } from './RepoHostBadge'

const repo = (over: Partial<Repo> = {}): Repo => ({
  id: 'repo-1',
  path: '/srv/app',
  displayName: 'app',
  badgeColor: '',
  addedAt: 1,
  ...over
})

async function renderBadge(value: Repo): Promise<HTMLDivElement> {
  const container = document.createElement('div')
  document.body.appendChild(container)
  await act(async () => {
    createRoot(container).render(<RepoHostBadge repo={value} />)
  })
  return container
}

describe('RepoHostBadge', () => {
  const openModal = vi.fn()

  beforeEach(() => {
    ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    vi.clearAllMocks()
    useAppStore.setState({
      openModal,
      devServers: [{ id: 'ds-1', name: 'Mac Studio', status: 'connected' }]
    } as never)
    ;(window as unknown as { __ORCA_WEB_CLIENT__?: boolean }).__ORCA_WEB_CLIENT__ = true
  })

  afterEach(() => {
    delete (window as unknown as { __ORCA_WEB_CLIENT__?: boolean }).__ORCA_WEB_CLIENT__
    document.body.replaceChildren()
  })

  it('shows the dev server name for a bound repo', async () => {
    const container = await renderBadge(repo({ devServerId: 'ds-1' }))
    expect(container.querySelector('[data-testid="repo-host-label"]')?.textContent).toBe(
      'Mac Studio'
    )
  })

  it('warns about a repo with no dev server and opens the host picker from it', async () => {
    const container = await renderBadge(repo({ devServerId: '' }))

    const button = container.querySelector<HTMLButtonElement>('[data-testid="repo-host-unbound"]')
    expect(button?.textContent).toBe('No dev server')
    await act(async () => {
      button?.click()
    })
    expect(openModal).toHaveBeenCalledWith('repo-host-picker', { repoId: 'repo-1' })
  })

  it('renders nothing on desktop', async () => {
    delete (window as unknown as { __ORCA_WEB_CLIENT__?: boolean }).__ORCA_WEB_CLIENT__
    const container = await renderBadge(repo({ devServerId: '' }))
    expect(container.innerHTML).toBe('')
  })
})
