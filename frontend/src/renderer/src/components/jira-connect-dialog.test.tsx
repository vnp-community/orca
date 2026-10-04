// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { JiraConnectDialog } from './jira-connect-dialog'

const mocks = vi.hoisted(() => ({
  connectJira: vi.fn(async () => ({ ok: true as const, viewer: { id: 'u1', displayName: 'A' } }))
}))

vi.mock('@/store', () => ({
  useAppStore: (selector: (state: unknown) => unknown) =>
    selector({
      connectJira: mocks.connectJira,
      settings: { activeRuntimeEnvironmentId: null }
    })
}))

vi.mock('@/hooks/useMountedRef', () => ({
  useMountedRef: () => ({ current: true })
}))

vi.mock('../runtime/runtime-shell-client', () => ({
  shellOpenUrl: vi.fn()
}))

afterEach(() => {
  cleanup()
  mocks.connectJira.mockClear()
})

function siteUrlInput(): HTMLElement {
  return screen.getByPlaceholderText(/atlassian\.net/i)
}

function tokenInput(): HTMLElement {
  return screen.getByPlaceholderText(/atlassian api token/i)
}

function connectButton(): HTMLElement {
  return screen.getByRole('button', { name: /^connect$/i })
}

describe('JiraConnectDialog', () => {
  // Regression test for BUG-013/CR-JIRA-001: a self-hosted Jira's Personal
  // Access Token needs Bearer auth (blank Email on the backend), but the
  // form used to require Email non-empty, making that case unsubmittable.
  it('enables Connect with Email left blank, given a site URL and a token', () => {
    render(<JiraConnectDialog open onOpenChange={() => {}} />)

    expect(connectButton()).toBeDisabled()

    fireEvent.change(siteUrlInput(), { target: { value: 'https://jr.servicehub.vn' } })
    fireEvent.change(tokenInput(), { target: { value: 'my-pat-token' } })

    expect(connectButton()).not.toBeDisabled()
  })

  it('calls connectJira with an empty email when the field is left blank', async () => {
    render(<JiraConnectDialog open onOpenChange={() => {}} />)

    fireEvent.change(siteUrlInput(), { target: { value: 'https://jr.servicehub.vn' } })
    fireEvent.change(tokenInput(), { target: { value: 'my-pat-token' } })
    fireEvent.click(connectButton())

    expect(mocks.connectJira).toHaveBeenCalledWith({
      siteUrl: 'https://jr.servicehub.vn',
      email: '',
      apiToken: 'my-pat-token'
    })
  })

  it('still requires a site URL and a token', () => {
    render(<JiraConnectDialog open onOpenChange={() => {}} />)
    expect(connectButton()).toBeDisabled()

    fireEvent.change(siteUrlInput(), { target: { value: 'https://jr.servicehub.vn' } })
    expect(connectButton()).toBeDisabled() // no token yet
  })
})
