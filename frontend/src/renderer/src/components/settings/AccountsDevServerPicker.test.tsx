// @vitest-environment happy-dom

import '@testing-library/jest-dom/vitest'

import React from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AccountsDevServerPicker } from './AccountsDevServerPicker'
import {
  createCompatibleRuntimeStatusResponseIfNeeded,
  type RuntimeEnvironmentCallRequest
} from '../../runtime/runtime-compatibility-test-fixture'
import { clearRuntimeCompatibilityCacheForTests } from '../../runtime/runtime-rpc-client'

const REMOTE = { activeRuntimeEnvironmentId: 'env-1' }
const DEV_SERVER_PREFERENCE_KEY = 'orca.accountsDevServer.env-1'

function createMemoryStorage(): Storage {
  const map = new Map<string, string>()
  return {
    get length() {
      return map.size
    },
    clear: () => map.clear(),
    getItem: (key) => map.get(key) ?? null,
    key: (index) => [...map.keys()][index] ?? null,
    removeItem: (key) => map.delete(key),
    setItem: (key, value) => map.set(key, value)
  }
}

const runtimeEnvironmentCall = vi.fn()

beforeEach(() => {
  clearRuntimeCompatibilityCacheForTests()
  runtimeEnvironmentCall.mockReset()
  vi.stubGlobal('localStorage', createMemoryStorage())
  // Why not vi.stubGlobal('window', ...): that replaces happy-dom's real
  // window (and its `document`) with a plain object, breaking RTL's DOM
  // queries — assign onto the existing window instead.
  Object.assign(window, {
    api: {
      runtimeEnvironments: {
        call: (args: RuntimeEnvironmentCallRequest) =>
          createCompatibleRuntimeStatusResponseIfNeeded(args) ?? runtimeEnvironmentCall(args)
      }
    }
  })
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  Reflect.deleteProperty(window, 'api')
})

describe('AccountsDevServerPicker', () => {
  it('renders nothing when no runtime environment is active', () => {
    const { container } = render(
      <AccountsDevServerPicker settings={{ activeRuntimeEnvironmentId: null }} onReadyChange={vi.fn()} />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('shows an empty state and reports not-ready when no dev servers exist', async () => {
    runtimeEnvironmentCall.mockReturnValue({ id: 'call', ok: true, result: [] })
    const onReadyChange = vi.fn()

    render(<AccountsDevServerPicker settings={REMOTE} onReadyChange={onReadyChange} />)

    await waitFor(() => expect(screen.getByText(/no dev servers available/i)).toBeInTheDocument())
    expect(onReadyChange).toHaveBeenCalledWith(false, expect.stringContaining('Pick a dev server'))
  })

  it('restores the persisted dev server pick and reports ready when connected', async () => {
    localStorage.setItem(DEV_SERVER_PREFERENCE_KEY, 'ds-1')
    runtimeEnvironmentCall.mockImplementation((args: { method: string }) => {
      if (args.method === 'devServer.list') {
        return {
          id: 'call',
          ok: true,
          result: [
            { id: 'ds-1', name: 'Dev Box 1', status: 'connected' },
            { id: 'ds-2', name: 'Dev Box 2', status: 'connected' }
          ]
        }
      }
      return { id: 'call', ok: true, result: { connected: true, connectionId: 'conn-1' } }
    })
    const onReadyChange = vi.fn()

    render(<AccountsDevServerPicker settings={REMOTE} onReadyChange={onReadyChange} />)

    await waitFor(() => expect(screen.getByText('Dev Box 1')).toBeInTheDocument())
    await waitFor(() => expect(onReadyChange).toHaveBeenCalledWith(true, null))
    expect(screen.queryByText(/not currently connected/i)).not.toBeInTheDocument()
  })

  it('shows a not-connected warning and reports not-ready when the picked dev server is disconnected', async () => {
    localStorage.setItem(DEV_SERVER_PREFERENCE_KEY, 'ds-1')
    runtimeEnvironmentCall.mockImplementation((args: { method: string }) => {
      if (args.method === 'devServer.list') {
        return { id: 'call', ok: true, result: [{ id: 'ds-1', name: 'Dev Box 1', status: 'connected' }] }
      }
      return { id: 'call', ok: true, result: { connected: false, connectionId: '' } }
    })
    const onReadyChange = vi.fn()

    render(<AccountsDevServerPicker settings={REMOTE} onReadyChange={onReadyChange} />)

    await waitFor(() =>
      expect(screen.getByText(/not currently connected/i)).toBeInTheDocument()
    )
    expect(onReadyChange).toHaveBeenCalledWith(false, expect.stringContaining('not currently connected'))
  })

  it('persists a newly picked dev server and re-resolves its connection', async () => {
    runtimeEnvironmentCall.mockImplementation((args: { method: string }) => {
      if (args.method === 'devServer.list') {
        return {
          id: 'call',
          ok: true,
          result: [{ id: 'ds-1', name: 'Dev Box 1', status: 'connected' }]
        }
      }
      return { id: 'call', ok: true, result: { connected: true, connectionId: 'conn-1' } }
    })
    const onReadyChange = vi.fn()
    const user = userEvent.setup()

    render(<AccountsDevServerPicker settings={REMOTE} onReadyChange={onReadyChange} />)
    await waitFor(() => expect(screen.getByRole('combobox')).toBeInTheDocument())

    await user.click(screen.getByRole('combobox'))
    await user.click(await screen.findByRole('option', { name: 'Dev Box 1' }))

    await waitFor(() => expect(localStorage.getItem(DEV_SERVER_PREFERENCE_KEY)).toBe('ds-1'))
    await waitFor(() => expect(onReadyChange).toHaveBeenCalledWith(true, null))
  })
})
