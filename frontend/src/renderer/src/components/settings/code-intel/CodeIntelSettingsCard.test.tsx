// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createFakeCodeIntelBackend } from '@/test-support/code-intel-fake-backend'
import { CodeIntelSettingsCard, type CodeIntelSettingsApi } from './CodeIntelSettingsCard'

afterEach(cleanup)

function apiFor(backend: ReturnType<typeof createFakeCodeIntelBackend>): CodeIntelSettingsApi {
  return {
    get: () => backend.call('codeIntel.settings.get', {}) as never,
    set: (patch) => backend.call('codeIntel.settings.set', patch) as never
  }
}

describe('CodeIntelSettingsCard', () => {
  it('admin toggles send only the changed field and refresh', async () => {
    const backend = createFakeCodeIntelBackend({ role: 'admin' })
    render(<CodeIntelSettingsCard isAdmin api={apiFor(backend)} />)
    const quality = await screen.findByRole('checkbox', { name: 'Quality gate' })
    fireEvent.click(quality)
    await waitFor(() => expect(backend.callsTo('codeIntel.settings.set')).toHaveLength(1))
    expect(backend.callsTo('codeIntel.settings.set')[0].params).toEqual({ qualityGateEnabled: false })
    await waitFor(() => expect(quality.getAttribute('data-state')).toBe('unchecked'))
  })

  it('regular users get a read-only card', async () => {
    const backend = createFakeCodeIntelBackend()
    render(<CodeIntelSettingsCard isAdmin={false} api={apiFor(backend)} />)
    const box = await screen.findByRole('checkbox', { name: 'Code intelligence' })
    expect(box.hasAttribute('disabled')).toBe(true)
    expect(screen.getByText(/Ask an administrator/)).toBeTruthy()
  })

  it('disables quality with a reason when the master switch is off', async () => {
    const backend = createFakeCodeIntelBackend({ role: 'admin' })
    backend.setSettings({ codeIntelEnabled: false })
    render(<CodeIntelSettingsCard isAdmin api={apiFor(backend)} />)
    const quality = await screen.findByRole('checkbox', { name: 'Quality gate' })
    expect(quality.hasAttribute('disabled')).toBe(true)
    expect(screen.getByText('Turn on code intelligence first.')).toBeTruthy()
  })

  it('shows an inline error for a permission failure', async () => {
    const backend = createFakeCodeIntelBackend({ role: 'user' })
    render(<CodeIntelSettingsCard isAdmin api={apiFor(backend)} />)
    fireEvent.click(await screen.findByRole('checkbox', { name: 'Code intelligence' }))
    expect((await screen.findByRole('alert')).textContent).toMatch(/administrators/)
  })

  it('explains a tenant-on but server-off difference', async () => {
    const backend = createFakeCodeIntelBackend({ role: 'admin' })
    backend.setHandler('codeIntel.settings.get', () => {
      const s = backend.getSettings()
      return { ...s, effective: { ...s.effective, qualityGateEnabled: false } }
    })
    render(<CodeIntelSettingsCard isAdmin api={apiFor(backend)} />)
    expect(await screen.findByText(/server switch is currently off/)).toBeTruthy()
  })
})
