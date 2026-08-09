// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { afterEach, describe, it, expect, vi } from 'vitest'
import { render, screen, act, cleanup } from '@testing-library/react'
import React from 'react'
import {
  ConnectionStatusProvider,
  useConnectionStatus
} from '../ConnectionStatusProvider'
import type { IRpcClient } from '../../../../platform/rpc-client-interface'

// Why: happy-dom env doesn't auto-cleanup between tests
afterEach(() => cleanup())

function createMockClient(connected = true): IRpcClient {
  return {
    isConnected: vi.fn().mockReturnValue(connected),
    on: vi.fn().mockReturnValue(() => {}),
    off: vi.fn(),
    disconnect: vi.fn(),
    connect: vi.fn().mockResolvedValue(undefined),
    invoke: vi.fn(),
    send: vi.fn(),
    once: vi.fn()
  }
}

function TestConsumer(): React.JSX.Element {
  const status = useConnectionStatus()
  return <div data-testid="status">{status}</div>
}

describe('ConnectionStatusProvider', () => {
  it('provides "connected" status when client is connected', () => {
    const client = createMockClient(true)
    render(
      <ConnectionStatusProvider client={client} pollIntervalMs={100000}>
        <TestConsumer />
      </ConnectionStatusProvider>
    )
    expect(screen.getByTestId('status')).toHaveTextContent('connected')
  })

  it('provides "disconnected" status when client is disconnected', () => {
    const client = createMockClient(false)
    render(
      <ConnectionStatusProvider client={client} pollIntervalMs={100000}>
        <TestConsumer />
      </ConnectionStatusProvider>
    )
    expect(screen.getByTestId('status')).toHaveTextContent('disconnected')
  })

  it('updates status when connection drops', async () => {
    vi.useFakeTimers()
    const client = createMockClient(true)
    render(
      <ConnectionStatusProvider client={client} pollIntervalMs={100}>
        <TestConsumer />
      </ConnectionStatusProvider>
    )
    expect(screen.getByTestId('status')).toHaveTextContent('connected')

    vi.mocked(client.isConnected).mockReturnValue(false)
    await act(async () => {
      vi.advanceTimersByTime(150)
    })
    expect(screen.getByTestId('status')).toHaveTextContent('disconnected')
    vi.useRealTimers()
  })

  it('updates status when connection restores', async () => {
    vi.useFakeTimers()
    const client = createMockClient(false)
    render(
      <ConnectionStatusProvider client={client} pollIntervalMs={100}>
        <TestConsumer />
      </ConnectionStatusProvider>
    )
    expect(screen.getByTestId('status')).toHaveTextContent('disconnected')

    vi.mocked(client.isConnected).mockReturnValue(true)
    await act(async () => {
      vi.advanceTimersByTime(150)
    })
    expect(screen.getByTestId('status')).toHaveTextContent('connected')
    vi.useRealTimers()
  })

  it('renders children regardless of connection status', () => {
    const client = createMockClient(false)
    render(
      <ConnectionStatusProvider client={client} pollIntervalMs={100000}>
        <div data-testid="child">Content</div>
      </ConnectionStatusProvider>
    )
    expect(screen.getByTestId('child')).toBeInTheDocument()
  })
})
