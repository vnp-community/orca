// @vitest-environment happy-dom

import { describe, expect, it, vi } from 'vitest'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { FeatureSetupInlineTerminal } from './FeatureSetupInlineTerminal'

// Why: regression guard for the live bug where this terminal (reached via
// Settings/onboarding "Install CLI & Skills") always tried to spawn a
// host-local PTY on the web build — INFRA_TERMINAL_HOST_LOCAL_UNIMPLEMENTED,
// found live 2026-08-30 — because none of its callers threaded a devServerId
// down to OnboardingInlineCommandTerminal. Mocks the terminal itself: this
// file only needs to prove the prop reaches it, not re-test the terminal.
const receivedProps: { devServerId?: string | null }[] = []
vi.mock('./OnboardingInlineCommandTerminal', () => ({
  OnboardingInlineCommandTerminal: (props: { devServerId?: string | null }) => {
    receivedProps.push(props)
    return null
  }
}))

describe('FeatureSetupInlineTerminal', () => {
  it('forwards devServerId through to OnboardingInlineCommandTerminal', () => {
    receivedProps.length = 0
    const container = document.createElement('div')
    const root = createRoot(container)
    act(() => {
      root.render(
        <FeatureSetupInlineTerminal
          command="npx orca-cli install"
          selection={{
            orchestration: true,
            browserUse: false,
            computerUse: false,
            linearTickets: false
          }}
          devServerId="dev-01"
        />
      )
    })
    expect(receivedProps.at(-1)?.devServerId).toBe('dev-01')
    act(() => root.unmount())
  })

  it('forwards a null/undefined devServerId as-is (desktop callers keep local spawning)', () => {
    receivedProps.length = 0
    const container = document.createElement('div')
    const root = createRoot(container)
    act(() => {
      root.render(
        <FeatureSetupInlineTerminal
          command="npx orca-cli install"
          selection={{
            orchestration: true,
            browserUse: false,
            computerUse: false,
            linearTickets: false
          }}
        />
      )
    })
    expect(receivedProps.at(-1)?.devServerId).toBeUndefined()
    act(() => root.unmount())
  })
})
