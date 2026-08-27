import { describe, expect, it } from 'vitest'
import {
  GRACE_MOUNT_DEFER_MS,
  shouldDeferTerminalPaneMount,
  type TerminalMountGateTab
} from './terminal-pending-host-mirror-mount-gate'

function makeTab(overrides: Partial<TerminalMountGateTab> = {}): TerminalMountGateTab {
  return {
    pendingActivationSpawn: true,
    ptyId: null,
    createdAt: 1000,
    ...overrides
  }
}

describe('shouldDeferTerminalPaneMount', () => {
  it('never defers on a non-host-mirrored (local/SSH-direct) worktree', () => {
    expect(shouldDeferTerminalPaneMount(makeTab(), false, 1000)).toBe(false)
  })

  it('defers a still-uncorrelated tab on a host-mirrored worktree within the grace window', () => {
    expect(shouldDeferTerminalPaneMount(makeTab({ createdAt: 1000 }), true, 1500)).toBe(true)
  })

  it('stops deferring once the grace window has elapsed', () => {
    const nowMs = 1000 + GRACE_MOUNT_DEFER_MS
    expect(shouldDeferTerminalPaneMount(makeTab({ createdAt: 1000 }), true, nowMs)).toBe(false)
  })

  it('never defers a tab that already has a ptyId (already promoted/local-spawned)', () => {
    expect(
      shouldDeferTerminalPaneMount(makeTab({ ptyId: 'pty-1' }), true, 1500)
    ).toBe(false)
  })

  it('never defers a tab that is not pendingActivationSpawn', () => {
    expect(
      shouldDeferTerminalPaneMount(makeTab({ pendingActivationSpawn: false }), true, 1500)
    ).toBe(false)
  })

  it('never defers a tab with pendingActivationSpawn=0 (falsy sentinel)', () => {
    expect(
      shouldDeferTerminalPaneMount(makeTab({ pendingActivationSpawn: 0 }), true, 1500)
    ).toBe(false)
  })
})
