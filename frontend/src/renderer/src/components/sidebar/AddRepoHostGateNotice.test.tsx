// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAppStore } from '@/store'
import { AddRepoHostGateNotice } from './AddRepoHostGateNotice'
import type { AddRepoHostGate } from './add-repo-web-host-gate'

async function renderGate(gate: AddRepoHostGate): Promise<HTMLDivElement> {
  const container = document.createElement('div')
  document.body.appendChild(container)
  await act(async () => {
    createRoot(container).render(<AddRepoHostGateNotice gate={gate} />)
  })
  return container
}

describe('AddRepoHostGateNotice', () => {
  const closeModal = vi.fn()
  const openSettingsTarget = vi.fn()
  const openSettingsPage = vi.fn()

  beforeEach(() => {
    ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    vi.clearAllMocks()
    useAppStore.setState({ closeModal, openSettingsTarget, openSettingsPage } as never)
  })

  afterEach(() => {
    document.body.replaceChildren()
  })

  it('renders nothing when the gate is ready', async () => {
    const container = await renderGate({ status: 'ready' })
    expect(container.innerHTML).toBe('')
  })

  it('explains that no dev server is connected and offers Open Dev Servers', async () => {
    const container = await renderGate({ status: 'no-host' })

    const notice = container.querySelector('[data-testid="add-repo-host-gate"]')
    expect(notice?.getAttribute('data-gate-status')).toBe('no-host')
    expect(notice?.textContent).toContain('No dev server is connected')

    const button = container.querySelector('button')
    expect(button?.textContent).toBe('Open Dev Servers')
    await act(async () => {
      button?.click()
    })
    expect(closeModal).toHaveBeenCalled()
    expect(openSettingsTarget).toHaveBeenCalledWith({ pane: 'dev-servers', repoId: null })
    expect(openSettingsPage).toHaveBeenCalled()
  })

  it('asks to choose a host when several are connected', async () => {
    const container = await renderGate({ status: 'choose-host' })
    expect(container.textContent).toContain('Choose the dev server this project lives on')
    expect(container.querySelector('button')).toBeNull()
  })

  it.each([
    ['disconnected', 'Mac Studio is offline'],
    ['error', 'Mac Studio is offline'],
    ['connecting', 'Mac Studio is reconnecting'],
    ['removed', 'Mac Studio is no longer available']
  ] as const)('keeps a dropped host (%s) visible with a reason', async (reason, text) => {
    const container = await renderGate({
      status: 'host-unavailable',
      hostId: 'devServer:ds-1',
      label: 'Mac Studio',
      reason
    })
    expect(container.textContent).toContain(text)
    expect(container.querySelector('[role="alert"]')).not.toBeNull()
  })
})
