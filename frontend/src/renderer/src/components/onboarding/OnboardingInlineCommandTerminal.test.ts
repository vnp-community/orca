import { describe, expect, it } from 'vitest'
import {
  getNextTerminalReadyRetryAttempt,
  isFloatingTerminalReady,
  READY_MAX_ATTEMPTS
} from './OnboardingInlineCommandTerminal'

// Live-bug regression: the web build's getFloatingTerminalCwd always
// resolves '' (no local filesystem to derive a path from), and the render
// gate used to be a truthy check (`cwd && tabId`) — '' is falsy in JS, so
// the pane was stuck on "Starting terminal..." forever for every web
// session, across every caller (CLI install, agent-skill setup,
// integrations step onboarding terminals).
describe('isFloatingTerminalReady', () => {
  it('is not ready while cwd has not loaded yet (null)', () => {
    expect(isFloatingTerminalReady(null, 'tab-1')).toBe(false)
  })

  it('is not ready while the tab has not been created yet', () => {
    expect(isFloatingTerminalReady('/home/user', null)).toBe(false)
    expect(isFloatingTerminalReady('', null)).toBe(false)
  })

  it('is ready once cwd resolves to an empty string (the web build default) and a tab exists', () => {
    expect(isFloatingTerminalReady('', 'tab-1')).toBe(true)
  })

  it('is ready once cwd resolves to a real path and a tab exists', () => {
    expect(isFloatingTerminalReady('/home/user', 'tab-1')).toBe(true)
  })
})

describe('getNextTerminalReadyRetryAttempt', () => {
  it('stops scheduling readiness checks after the capped number of attempts', () => {
    let attempt = 0
    let scheduledRetries = 0

    while (true) {
      const nextAttempt = getNextTerminalReadyRetryAttempt(attempt)
      if (nextAttempt === null) {
        break
      }
      scheduledRetries += 1
      attempt = nextAttempt
    }

    expect(scheduledRetries).toBe(READY_MAX_ATTEMPTS)
    expect(attempt).toBe(READY_MAX_ATTEMPTS)
    expect(getNextTerminalReadyRetryAttempt(READY_MAX_ATTEMPTS)).toBeNull()
  })
})
