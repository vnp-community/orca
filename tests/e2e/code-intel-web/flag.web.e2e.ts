import { expect, test, type Page } from '@playwright/test'
import { bootApp } from './support/code-intel-app-navigation'
import { devStackUrl } from './support/code-intel-dev-backend'
import { createFakeCodeIntelBackend, mockCodeIntelApp, mockUser } from './support/mock-code-intel-ws'

test.skip(!!devStackUrl, 'mocked variants only; the real stack runs dev-stack.web.e2e.ts')

const errorsOn = (page: Page): string[] => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  return errors
}

test('flag off: no code-intel stream, no code-intel page error', async ({ page }) => {
  const backend = createFakeCodeIntelBackend()
  backend.setSettings({ codeIntelEnabled: false })
  await mockCodeIntelApp(page, { backend, user: mockUser() })
  const errors = errorsOn(page)
  await bootApp(page)
  expect(backend.streamCount()).toBe(0)
  // Why: with the master flag off only settings.get may reach the backend.
  expect(
    backend.calls.map((c) => c.method).filter((m) => m !== 'codeIntel.settings.get')
  ).toEqual([])
  expect(errors.filter((m) => /code-?intel/i.test(m))).toEqual([])
})

test('quality off keeps code-intel channels allowed and refuses quality ones', async ({ page }) => {
  const backend = createFakeCodeIntelBackend()
  backend.setSettings({ qualityGateEnabled: false })
  await mockCodeIntelApp(page, { backend, user: mockUser() })
  await bootApp(page)
  const sel = backend.selector
  await expect(backend.call('codeIntel.status', sel)).resolves.toBeTruthy()
  await expect(backend.call('codeIntel.quality.gate', sel)).rejects.toThrow(
    /CODEINTEL_QUALITY_GATE_DISABLED/
  )
})

test('turned off mid-session: next call answers CODEINTEL_DISABLED without a toast', async ({
  page
}) => {
  const backend = createFakeCodeIntelBackend({ role: 'admin' })
  await mockCodeIntelApp(page, { backend, user: mockUser('admin') })
  await bootApp(page)
  await backend.call('codeIntel.settings.set', { codeIntelEnabled: false })
  await expect(backend.call('codeIntel.status', backend.selector)).rejects.toThrow(
    /CODEINTEL_DISABLED/
  )
  await expect(page.getByRole('status').filter({ hasText: /code ?intel/i })).toHaveCount(0)
})
